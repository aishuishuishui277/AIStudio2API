package authpoc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestClassificationDoesNotConfuse403WithExpiredLogin(t *testing.T) {
	cases := []struct {
		name string
		e    Evidence
		want string
	}{
		{"signed out", Evidence{SignedOut: true}, "AUTH_REQUIRED"},
		{"missing local login cookie", Evidence{MissingLoginCookie: true}, "AUTH_REQUIRED"},
		{"network timeout", Evidence{Network: true}, "NETWORK_FAILURE"},
		{"opaque 403", Evidence{HTTPStatus: 403}, "UPSTREAM_FORBIDDEN"},
		{"opaque 401", Evidence{HTTPStatus: 401}, "UNKNOWN_ERROR"},
		{"account restriction", Evidence{HTTPStatus: 403, AccountRestricted: true}, "ACCOUNT_RESTRICTED"},
		{"quota", Evidence{HTTPStatus: 429}, "QUOTA_EXCEEDED"},
		{"WAA", Evidence{WAA: true}, "WAA_FAILURE"},
		{"network while WAA", Evidence{WAA: true, Network: true}, "NETWORK_FAILURE"},
		{"unknown", Evidence{}, "UNKNOWN_ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.e)
			if got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
			if NeedsLogin(got) != (c.want == "AUTH_REQUIRED") {
				t.Fatal("login hint")
			}
		})
	}
}

// Non-sensitive opaque stand-ins test file/state mechanics only, never Google acceptance.
var oldEnvelope = []byte("A2A-ANDROID-STATE-1\x00non-sensitive-old-test-envelope")
var newEnvelope = []byte("A2A-ANDROID-STATE-1\x00non-sensitive-new-test-envelope")

func targetFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "storage-state.json")
	if err := os.WriteFile(p, oldEnvelope, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func assertBytes(t *testing.T, p string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("unexpected saved bytes")
	}
}

func TestPublishCancellationConflictAndFailuresKeepOld(t *testing.T) {
	for _, phase := range []string{"cancelled", "conflict", "backup_synced", "candidate_synced", "invalid-envelope"} {
		t.Run(phase, func(t *testing.T) {
			p := targetFile(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			digest := sha256.Sum256(oldEnvelope)
			candidate := newEnvelope
			if phase == "cancelled" {
				cancel()
			}
			if phase == "conflict" {
				digest = [32]byte{}
			}
			if phase == "invalid-envelope" {
				candidate = []byte("plaintext rejected")
			}
			committed, err := publish(ctx, p, digest, candidate, func(at string) error {
				if at == phase {
					return errors.New("offline injected filesystem failure")
				}
				return nil
			})
			if err == nil || committed {
				t.Fatal("unexpected commit")
			}
			assertBytes(t, p, oldEnvelope)
		})
	}
}
func TestPublishRetainsPreviousAndReportsPostCommitFailure(t *testing.T) {
	for _, syncFailure := range []bool{false, true} {
		p := targetFile(t)
		committed, err := publish(context.Background(), p, sha256.Sum256(oldEnvelope), newEnvelope, func(at string) error {
			if syncFailure && at == "renamed" {
				return errors.New("offline injected dir sync failure")
			}
			return nil
		})
		if !committed || ((err != nil) != syncFailure) {
			t.Fatal("incorrect commit result")
		}
		assertBytes(t, p, newEnvelope)
		assertBytes(t, p+".previous", oldEnvelope)
	}
}
func TestGateCancellationAndConcurrentCommit(t *testing.T) {
	var g Gate
	g.Cancel()
	if g.Prepare() {
		t.Fatal("cancelled update prepared")
	}
	if _, err := g.Commit(func() (bool, error) { t.Fatal("cancelled commit ran"); return false, nil }); err == nil {
		t.Fatal("missing cancellation")
	}
	var ready Gate
	ready.Prepare()
	ready.Cancel()
	if _, err := ready.Commit(func() (bool, error) { t.Fatal("cancelled prepared update committed"); return false, nil }); err == nil {
		t.Fatal("missing cancellation")
	}
	var active Gate
	active.Prepare()
	entered, release := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		active.Commit(func() (bool, error) { close(entered); <-release; return true, nil })
	}()
	<-entered
	if active.Cancel() {
		t.Fatal("cannot promise cancellation beyond commit boundary")
	}
	close(release)
	wg.Wait()
	if _, err := active.Commit(func() (bool, error) { t.Fatal("duplicate commit"); return true, nil }); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestCrashRecoveryUsesOnlyCommittedTarget(t *testing.T) {
	if path := os.Getenv("AUTHPOC_CRASH_TARGET"); path != "" {
		phase := os.Getenv("AUTHPOC_CRASH_PHASE")
		publish(context.Background(), path, sha256.Sum256(oldEnvelope), newEnvelope, func(at string) error {
			if at == phase {
				os.Exit(91)
			}
			return nil
		})
		os.Exit(92)
	}
	for _, phase := range []string{"backup_synced", "candidate_synced", "renamed"} {
		t.Run(phase, func(t *testing.T) {
			p := targetFile(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashRecoveryUsesOnlyCommittedTarget$")
			cmd.Env = append(os.Environ(), "AUTHPOC_CRASH_TARGET="+p, "AUTHPOC_CRASH_PHASE="+phase)
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 91 {
				t.Fatal("crash injection failed")
			}
			want := oldEnvelope
			if phase == "renamed" {
				want = newEnvelope
			}
			assertBytes(t, p, want)
			assertBytes(t, p+".previous", oldEnvelope)
		})
	}
}

func TestDifferentAccountCannotPublish(t *testing.T) {
	for _, pair := range [][2]string{{"original@example.test", "other@example.test"}, {"", ""}} {
		var g Gate
		if g.PrepareForAccount(pair[0], pair[1]) {
			t.Fatal("identity mismatch accepted")
		}
		if _, err := g.Commit(func() (bool, error) { t.Fatal("mismatch reached publisher"); return true, nil }); err == nil {
			t.Fatal("missing guard")
		}
	}
}

func TestExplicitRollbackUsesPreviousCompleteEnvelope(t *testing.T) {
	p := targetFile(t)
	if committed, err := Publish(context.Background(), p, sha256.Sum256(oldEnvelope), newEnvelope); !committed || err != nil {
		t.Fatal("publish failed")
	}
	previous, err := os.ReadFile(p + ".previous")
	if err != nil {
		t.Fatal(err)
	}
	if committed, err := Publish(context.Background(), p, sha256.Sum256(newEnvelope), previous); !committed || err != nil {
		t.Fatal("offline rollback failed")
	}
	assertBytes(t, p, oldEnvelope)
}
