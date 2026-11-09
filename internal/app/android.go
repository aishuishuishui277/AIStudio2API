//go:build android

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	_ "time/tzdata"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
	"github.com/Mag1cFall/AIStudio2API/internal/i18n"
)

// RunAndroid uses the existing service in a dedicated Android process; it never exits the JVM.
func RunAndroid(ctx context.Context, directory string, key []byte, ready func(string)) error {
	i18n.Stdout, i18n.Stderr = io.Discard, io.Discard
	log.SetOutput(io.Discard)
	if !filepath.IsAbs(directory) {
		return errors.New("Android data directory must be absolute")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := os.Chdir(directory); err != nil {
		return err
	}
	for _, sub := range []string{"auth", "runtime/logs", "tmp", "cache", "config"} {
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
		"LISTEN_ADDR":          "127.0.0.1:2148",
		"PREVIEW_LISTEN_ADDR":  "127.0.0.1:2149",
		"AUTO_START":           "true",
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
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			initial.Close()
			return err
		}
		credentials := "PROXY_API_KEY=" + hex.EncodeToString(secret) + "\nADMIN_AUTH_ENABLED=true\nADMIN_USERNAME=admin\n"
		if _, err := rand.Read(secret); err != nil {
			initial.Close()
			return err
		}
		credentials += "ADMIN_PASSWORD=" + hex.EncodeToString(secret) + "\n"
		clear(secret)
		_, writeErr := initial.WriteString(credentials + "WARM_WORKER_LIMIT=1\nMAX_ACTIVE_WORKERS=2\nWARM_STARTUP_CONCURRENCY=1\nPER_ACCOUNT_CONCURRENCY=1\nLOG_MAX_TOTAL_MB=50\n")
		if err := errors.Join(writeErr, initial.Close()); err != nil {
			return fmt.Errorf("initialize Android configuration: %w", err)
		}
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	managedBy = "Android APK"
	if err := config.EnsureAndroidModelModes(".env"); err != nil {
		return fmt.Errorf("initialize Android model modes: %w", err)
	}
	cfg, err := config.Load(".env")
	if err != nil {
		return fmt.Errorf("load Android configuration: %w", err)
	}
	logs := &logOutput{dir: filepath.Join("runtime", "logs")}
	if err := logs.set(cfg.LogFile, cfg.LogMaxTotalMB); err != nil {
		return err
	}
	defer logs.set(false, 0)
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.Discard, nil)))
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
