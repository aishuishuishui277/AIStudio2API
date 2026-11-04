//go:build android

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	_ "time/tzdata"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
)

// RunAndroid uses the existing service in a dedicated Android process; it never exits the JVM.
func RunAndroid(ctx context.Context, directory string, key []byte, ready func(string)) error {
	if !filepath.IsAbs(directory) {
		return errors.New("Android data directory must be absolute")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := os.Chdir(directory); err != nil {
		return err
	}
	for _, sub := range []string{"auth", "runtime/logs", "tmp"} {
		if err := os.MkdirAll(sub, 0o700); err != nil {
			return err
		}
	}
	for name, value := range map[string]string{
		"TMPDIR":               filepath.Join(directory, "tmp"),
		"XDG_CACHE_HOME":       filepath.Join(directory, "cache"),
		"XDG_CONFIG_HOME":      filepath.Join(directory, "config"),
		"WAA_BACKEND":          config.WAABackendSpiderMonkey,
		"AISTUDIO_AUTH_STATES": "auth",
		"LISTEN_ADDR":          "127.0.0.1:2048",
		"PREVIEW_LISTEN_ADDR":  "127.0.0.1:2049",
		"AUTO_START":           "false",
		"REQUEST_BODY_LOG":     "false",
	} {
		if err := os.Setenv(name, value); err != nil {
			return err
		}
	}
	if err := aistudio.SetAndroidStorageKey(key); err != nil {
		return err
	}
	clear(key)
	initial, err := os.OpenFile(".env", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_, writeErr := initial.WriteString("WARM_WORKER_LIMIT=1\nMAX_ACTIVE_WORKERS=2\nWARM_STARTUP_CONCURRENCY=1\nPER_ACCOUNT_CONCURRENCY=1\nLOG_MAX_TOTAL_MB=50\n")
		if err := errors.Join(writeErr, initial.Close()); err != nil {
			return fmt.Errorf("initialize Android configuration: %w", err)
		}
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	managedBy = "Android APK"
	cfg, err := config.Load(".env")
	if err != nil {
		return fmt.Errorf("load Android configuration: %w", err)
	}
	logs := &logOutput{dir: filepath.Join("runtime", "logs")}
	if err := logs.set(cfg.LogFile, cfg.LogMaxTotalMB); err != nil {
		return err
	}
	defer logs.set(false, 0)
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, nil)))
	serviceReady := make(chan serviceReady, 1)
	done := make(chan error, 1)
	var restarting atomic.Bool
	go func() {
		_, err := runService(ctx, func() {}, cfg, commandOptions{}, logs, "", &restarting, serviceReady)
		done <- err
	}()
	select {
	case result := <-serviceReady:
		if result.err == nil {
			ready(result.url)
		}
	case err := <-done:
		return err
	}
	return <-done
}
