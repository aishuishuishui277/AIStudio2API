package authpoc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var ErrConflict = errors.New("saved state changed during validation")
var ErrNotPrepared = errors.New("update not prepared")
var ErrInvalidEnvelope = errors.New("not an encrypted state envelope")

// Gate linearizes Cancel/Commit. Once committing starts, cancellation cannot be
// reported as successful. A process crash leaves either old or new complete state.
type Gate struct {
	mu    sync.Mutex
	state string
}

func (g *Gate) Prepare() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.state != "" {
		return false
	}
	g.state = "prepared"
	return true
}

// PrepareForAccount rejects a different or missing identity before commit is possible.
func (g *Gate) PrepareForAccount(expected, observed string) bool {
	if expected == "" || observed != expected {
		return false
	}
	return g.Prepare()
}
func (g *Gate) Cancel() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.state == "committed" || g.state == "committing" {
		return false
	}
	g.state = "cancelled"
	return true
}
func (g *Gate) Commit(fn func() (bool, error)) (bool, error) {
	g.mu.Lock()
	if g.state != "prepared" {
		g.mu.Unlock()
		return false, ErrNotPrepared
	}
	g.state = "committing"
	g.mu.Unlock()
	committed, err := fn()
	g.mu.Lock()
	if committed {
		g.state = "committed"
	} else {
		g.state = "failed"
	}
	g.mu.Unlock()
	return committed, err
}

// Publish replaces only ciphertext. The caller has already decrypted and verified
// the candidate with the original PG codec and checked the same Google identity.
// committed remains true if the final directory sync fails after the rename.
func Publish(ctx context.Context, target string, expected [32]byte, encrypted []byte) (committed bool, err error) {
	return publish(ctx, target, expected, encrypted, nil)
}

func publish(ctx context.Context, target string, expected [32]byte, encrypted []byte, fault func(string) error) (bool, error) {
	if !bytes.HasPrefix(encrypted, []byte("A2A-ANDROID-STATE-1\x00")) {
		return false, ErrInvalidEnvelope
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, ErrInvalidEnvelope
	}
	previous, err := os.ReadFile(target)
	if err != nil {
		return false, err
	}
	defer clear(previous)
	if sha256.Sum256(previous) != expected {
		return false, ErrConflict
	}
	if !bytes.HasPrefix(previous, []byte("A2A-ANDROID-STATE-1\x00")) {
		return false, ErrInvalidEnvelope
	}
	directory := filepath.Dir(target)
	// Last complete ciphertext is retained in the same private directory for explicit recovery.
	if err := replaceSynced(target+".previous", previous); err != nil {
		return false, err
	}
	if fault != nil {
		if err := fault("backup_synced"); err != nil {
			return false, err
		}
	}
	temporary, err := writeTemporary(directory, encrypted)
	if err != nil {
		return false, err
	}
	defer os.Remove(temporary)
	if err := syncDirectory(directory); err != nil {
		return false, err
	}
	if fault != nil {
		if err := fault("candidate_synced"); err != nil {
			return false, err
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	current, err := os.ReadFile(target)
	if err != nil {
		return false, err
	}
	unchanged := sha256.Sum256(current) == expected
	clear(current)
	if !unchanged {
		return false, ErrConflict
	}
	if err := os.Rename(temporary, target); err != nil {
		return false, err
	}
	if fault != nil {
		if err := fault("renamed"); err != nil {
			return true, err
		}
	}
	return true, syncDirectory(directory)
}

func writeTemporary(directory string, encrypted []byte) (string, error) {
	f, err := os.CreateTemp(directory, ".auth-update-*.tmp")
	if err != nil {
		return "", err
	}
	name := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	if err := f.Chmod(0600); err != nil {
		return "", err
	}
	if _, err := f.Write(encrypted); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}
func replaceSynced(target string, encrypted []byte) error {
	name, err := writeTemporary(filepath.Dir(target), encrypted)
	if err != nil {
		return err
	}
	defer os.Remove(name)
	if err := os.Rename(name, target); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(target))
}
func syncDirectory(directory string) error {
	f, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
