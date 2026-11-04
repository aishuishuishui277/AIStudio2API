//go:build android

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/app"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
)

type session struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	err    string
	url    string
}

var core struct {
	sync.Mutex
	current *session
}

//export CoreStart
func CoreStart(directory, encodedKey *C.char) *C.char {
	core.Lock()
	if core.current != nil {
		core.Unlock()
		return C.CString("Core is already starting or running")
	}
	key, err := base64.StdEncoding.DecodeString(C.GoString(encodedKey))
	if err != nil || len(key) != 32 {
		core.Unlock()
		return C.CString("Invalid Android data key")
	}
	ctx, cancel := context.WithCancel(context.Background())
	current := &session{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	core.current = current
	core.Unlock()
	ready := make(chan string, 1)
	dir := C.GoString(directory)
	go func() {
		err := app.RunAndroid(ctx, dir, key, func(url string) {
			core.Lock()
			current.url = url
			core.Unlock()
			ready <- url
		})
		clear(key)
		core.Lock()
		if err != nil {
			current.err = err.Error()
		}
		close(current.done)
		core.Unlock()
	}()
	select {
	case <-ready:
		return C.CString("")
	case <-current.done:
		core.Lock()
		message := current.err
		core.current = nil
		core.Unlock()
		if message == "" {
			message = "Core stopped before becoming ready"
		}
		return C.CString(message)
	case <-time.After(30 * time.Second):
		cancel()
		return C.CString("Core startup timed out; stop the service before retrying")
	}
}

//export CoreStop
func CoreStop() *C.char {
	core.Lock()
	current := core.current
	core.Unlock()
	if current == nil {
		return C.CString("")
	}
	current.cancel()
	select {
	case <-current.done:
		core.Lock()
		if core.current == current {
			core.current = nil
		}
		core.Unlock()
		return C.CString("")
	case <-time.After(20 * time.Second):
		return C.CString("Core shutdown timed out; see logs")
	}
}

//export CoreStatus
func CoreStatus() *C.char {
	core.Lock()
	defer core.Unlock()
	current := core.current
	if current == nil {
		return C.CString("STOPPED")
	}
	select {
	case <-current.done:
		return C.CString("FAILED: " + current.err)
	default:
		if current.url == "" {
			return C.CString("STARTING")
		}
		return C.CString("RUNNING")
	}
}

//export CoreProbe
func CoreProbe() *C.char {
	result := map[string]any{"authenticated": false}
	probe := func() error {
		core.Lock()
		current := core.current
		core.Unlock()
		if current == nil {
			return context.Canceled
		}
		cfg, err := config.Load(".env")
		if err != nil {
			return err
		}
		proxy := ""
		if values := config.ProxyList(cfg.Proxy); len(values) > 0 {
			proxy = values[0]
		}
		client, err := aistudio.NewProxyHTTPClient(proxy)
		if err != nil {
			return err
		}
		defer client.CloseIdleConnections()
		client.Timeout = 20 * time.Second
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		ctx, cancel := context.WithTimeout(current.ctx, 20*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://aistudio.google.com/", nil)
		if err != nil {
			return err
		}
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0")
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		_, _ = io.CopyN(io.Discard, response.Body, 1024)
		result["http_status"], result["protocol"] = response.StatusCode, response.Proto
		return nil
	}
	if err := probe(); err != nil {
		result["error"] = err.Error()
	}
	data, _ := json.Marshal(result)
	return C.CString(string(data))
}

//export CoreFree
func CoreFree(value *C.char) { C.free(unsafe.Pointer(value)) }

func main() {}
