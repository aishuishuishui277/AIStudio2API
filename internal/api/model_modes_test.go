package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
)

const modeTestText = "夜色温柔。\n\n她说：你好，旅人。\n下一行🙂"

type modeTestService struct {
	generate func(context.Context, aistudio.GenerateRequest) (<-chan aistudio.Event, error)
}

func (s modeTestService) Models(context.Context) ([]aistudio.Model, error) { return nil, nil }
func (s modeTestService) CountTokens(context.Context, aistudio.TokenCountRequest) (aistudio.TokenCount, error) {
	return aistudio.TokenCount{}, nil
}
func (s modeTestService) Generate(ctx context.Context, request aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
	return s.generate(ctx, request)
}

func modeTestHandler(captured chan<- aistudio.GenerateRequest, global bool) http.Handler {
	service := modeTestService{generate: func(_ context.Context, request aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
		captured <- request
		events := make(chan aistudio.Event, 3)
		if request.Unary {
			events <- aistudio.Event{Kind: aistudio.EventText, Part: aistudio.Part{Text: modeTestText}}
		} else {
			events <- aistudio.Event{Kind: aistudio.EventText, Part: aistudio.Part{Text: "夜色温柔。\n"}}
			events <- aistudio.Event{Kind: aistudio.EventText, Part: aistudio.Part{Text: "\n她说：你好，旅人。\n下一行🙂"}}
		}
		events <- aistudio.Event{Kind: aistudio.EventFinish, FinishReason: "STOP"}
		close(events)
		return events, nil
	}}
	return NewHandler(service, Config{ResponseModes: func() ResponseModes {
		return ResponseModes{FakeStream: global, FakeStreamModels: []string{"gemini-3.8-flash-fake", "gemini-2.5-pro-fake", "gemini-3-pro-preview-fake", "gemini-3.1-flash-lite-fake"}}
	}})
}

func modeTestRequest(model string, stream bool) *http.Request {
	body, _ := json.Marshal(map[string]any{"model": model, "stream": stream, "messages": []map[string]string{
		{"role": "system", "content": "你是旅店老板，保持角色。"},
		{"role": "user", "content": "请打招呼。"},
		{"role": "assistant", "content": "欢迎来到旅店。"},
		{"role": "user", "content": "继续故事。"},
	}})
	return httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
}

func checkModeSSE(body, model string) (int, error) {
	if !utf8.ValidString(body) {
		return 0, errors.New("invalid UTF-8")
	}
	var text strings.Builder
	roles, done, finishes, pieces := 0, 0, 0, 0
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			done++
			continue
		}
		var chunk struct {
			Model   string `json:"model"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"delta"`
				Finish *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return 0, err
		}
		if chunk.Model != model {
			return 0, fmt.Errorf("response model changed to %q", chunk.Model)
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				return 0, errors.New("unexpected extra role message")
			}
			if choice.Delta.Role != "" {
				roles++
			}
			if choice.Delta.Content != "" {
				text.WriteString(choice.Delta.Content)
				pieces++
			}
			if choice.Finish != nil {
				finishes++
			}
		}
	}
	if text.String() != modeTestText || roles != 1 || done != 1 || finishes != 1 {
		return 0, fmt.Errorf("content/role/end mismatch: text=%q roles=%d done=%d finishes=%d", text.String(), roles, done, finishes)
	}
	return pieces, nil
}

func TestPerModelStreamingAndNonstream(t *testing.T) {
	for _, target := range []string{"gemini-3.8-flash", "gemini-2.5-pro", "gemini-3-pro-preview", "gemini-3.1-flash-lite"} {
		for _, fake := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				model := target
				if fake {
					model += "-fake"
				}
				t.Run(fmt.Sprintf("%s/stream=%v", model, stream), func(t *testing.T) {
					captured := make(chan aistudio.GenerateRequest, 1)
					w := httptest.NewRecorder()
					modeTestHandler(captured, false).ServeHTTP(w, modeTestRequest(model, stream))
					if w.Code != 200 {
						t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
					}
					request := <-captured
					if request.Model != model || request.Unary != (!stream || fake) {
						t.Fatalf("model/mode leaked: %s unary=%v", request.Model, request.Unary)
					}
					if len(request.Contents) != 3 || len(request.System) == 0 {
						t.Fatal("role card or multi-turn context lost")
					}
					if stream {
						pieces, err := checkModeSSE(w.Body.String(), model)
						if err != nil {
							t.Fatal(err)
						}
						want := 2
						if fake {
							want = 1
						}
						if pieces != want {
							t.Fatalf("got %d content pieces, want %d", pieces, want)
						}
					} else {
						if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
							t.Fatal("nonstream response is not JSON")
						}
						var result struct {
							Choices []struct {
								Message struct {
									Content string `json:"content"`
								} `json:"message"`
							} `json:"choices"`
						}
						if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if len(result.Choices) != 1 || result.Choices[0].Message.Content != modeTestText {
							t.Fatal("nonstream content mismatch")
						}
					}
				})
			}
		}
	}
}

func TestConcurrentModelModes(t *testing.T) {
	const count = 64
	captured := make(chan aistudio.GenerateRequest, count)
	handler := modeTestHandler(captured, false)
	var wg sync.WaitGroup
	for index := range count {
		wg.Go(func() {
			model := "gemini-3.8-flash"
			if index%2 != 0 {
				model += "-fake"
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, modeTestRequest(model, true))
			if w.Code != 200 {
				t.Errorf("HTTP %d", w.Code)
				return
			}
			if _, err := checkModeSSE(w.Body.String(), model); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	close(captured)
	real, fake := 0, 0
	for request := range captured {
		want := strings.HasSuffix(request.Model, "-fake")
		if request.Unary != want {
			t.Fatal("concurrent request selected the other model's mode")
		}
		if want {
			fake++
		} else {
			real++
		}
	}
	if real != count/2 || fake != count/2 {
		t.Fatalf("real=%d fake=%d", real, fake)
	}
}

func TestGlobalFakeStreamAndClientNameVariants(t *testing.T) {
	captured := make(chan aistudio.GenerateRequest, 1)
	w := httptest.NewRecorder()
	modeTestHandler(captured, true).ServeHTTP(w, modeTestRequest("gemini-3.8-flash", true))
	if !(<-captured).Unary {
		t.Fatal("global FAKE_STREAM=true stopped working")
	}
	s := &server{config: Config{ResponseModes: func() ResponseModes { return ResponseModes{FakeStreamModels: []string{"gemini-3.8-flash-fake"}} }}}
	for _, model := range []string{"gemini-3.8-flash-fake", "models/gemini-3.8-flash-fake", "gemini-3.8-flash-fake(low)", "gemini-3.8-flash-fake-search(low)"} {
		if !s.fakeStream(model) {
			t.Fatalf("client alias lost mode: %s", model)
		}
	}
	if s.fakeStream("gemini-3.8-flash") {
		t.Fatal("upstream model was also forced into fake streaming")
	}
}

func TestFakeStreamCancellationAndFailure(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		started, canceled := make(chan struct{}), make(chan struct{})
		service := modeTestService{generate: func(ctx context.Context, request aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			if !request.Unary {
				t.Error("fake request was not unary")
			}
			events := make(chan aistudio.Event)
			close(started)
			go func() { <-ctx.Done(); close(events); close(canceled) }()
			return events, nil
		}}
		handler := NewHandler(service, Config{ResponseModes: func() ResponseModes { return ResponseModes{FakeStreamModels: []string{"gemini-3.8-flash-fake"}} }})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			handler.ServeHTTP(httptest.NewRecorder(), modeTestRequest("gemini-3.8-flash-fake", true).WithContext(ctx))
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("request did not start")
		}
		cancel()
		select {
		case <-canceled:
		case <-time.After(5 * time.Second):
			t.Fatal("upstream context not canceled")
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("handler did not release resources")
		}
	})
	t.Run("failure", func(t *testing.T) {
		service := modeTestService{generate: func(context.Context, aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			return nil, errors.New("test upstream failure")
		}}
		handler := NewHandler(service, Config{ResponseModes: func() ResponseModes { return ResponseModes{FakeStream: true} }})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, modeTestRequest("gemini-3.8-flash-fake", true))
		if w.Code < 400 || strings.Contains(w.Body.String(), "[DONE]") || !strings.Contains(w.Body.String(), "error") {
			t.Fatalf("error disguised as success: %d %s", w.Code, w.Body.String())
		}
	})
}
