package aistudio

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/camoufoxnative"
)

func TestAndroidTokenizer(t *testing.T) {
	text := strings.Repeat("Android ARM64 本地上下文 verification.\n", 2000)
	count := localTextTokens(text)
	if count < 2000 || count > int64(len(text)) {
		t.Fatalf("invalid local token count: %d", count)
	}
	if cached := localTextTokens(text); cached != count {
		t.Fatalf("cached token count changed: %d != %d", cached, count)
	}
}

func TestAndroidFingerprint(t *testing.T) {
	options := camoufoxnative.Options{
		StorageStatePath: filepath.Join(t.TempDir(), "storage-state.json"),
		Locale:           "en-US", Timezone: "Asia/Shanghai",
	}
	fingerprint, err := camoufoxnative.AccountFingerprint(options)
	if err != nil {
		t.Fatal(err)
	}
	profile, userAgent, _ := profileFromFingerprint(fingerprint)
	if !strings.Contains(userAgent, "Firefox/152.0") || fingerprint["timezone"] != "Asia/Shanghai" || len(profile.Navigator) == 0 {
		t.Fatal("Android could not produce the existing Firefox WAA profile")
	}
}

func TestAndroidPublicTLS(t *testing.T) {
	if os.Getenv("A2A_ANDROID_PUBLIC_TLS") != "1" {
		t.Skip("public Google TLS/HTTP2 diagnostic is opt-in and never uses account state")
	}
	client, err := newGoWAAHTTPClient(os.Getenv("A2A_ANDROID_TEST_PROXY"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	client.Timeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://aistudio.google.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, 1024)
	if response.ProtoMajor != 2 {
		t.Fatalf("expected HTTP/2, got %s", response.Proto)
	}
	t.Logf("Public bootstrap: HTTP %d, %s; account authentication and model generation were not tested", response.StatusCode, response.Proto)
}
