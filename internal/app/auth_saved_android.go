//go:build android && authpoc

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
)

var (
	errSavedAuthRequired    = errors.New("saved authentication required")
	errSavedSourceAmbiguous = errors.New("saved source ambiguous")
	errSavedStateInvalid    = errors.New("saved state invalid")
)

// Only the first restore visits 3A. Never fall back if the persistent directory exists.
// Original 3A files are read-only; no account import or online verification here.
func prepareSavedAccount(files, root string, result map[string]any) error {
	target := filepath.Join(root, "auth")
	if _, err := os.Lstat(target); err == nil {
		accounts, err := aistudio.NewAccountStore(target).Load()
		if err != nil {
			return errSavedStateInvalid
		}
		if len(accounts) != 1 {
			return errSavedAuthRequired
		}
		if _, err := aistudio.NewSigner().Authorization(accounts[0].StorageState); err != nil {
			return errSavedAuthRequired
		}
		result["migration_performed"] = false
		result["source"] = "persistent_account"
		result["identity_preserved"] = accounts[0].ID == accounts[0].Config.Label
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// 3B-2 never scans or re-imports historical 3A runs.
	return errSavedAuthRequired
}

func writeSavedConfig(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func syncSavedDir(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// Audit only flags; never return a private filename, identity, state, or ciphertext.
func auditSavedAccount(root string, before [32]byte, result map[string]any) {
	accounts, err := aistudio.NewAccountStore(filepath.Join(root, "auth")).Load()
	if err != nil || len(accounts) != 1 {
		result["success"] = false
		result["error"] = "SAVED_STATE_RELOAD_FAILED"
		return
	}
	data, err := os.ReadFile(accounts[0].StoragePath)
	encrypted := err == nil && bytes.HasPrefix(data, []byte("A2A-ANDROID-STATE-1\x00"))
	clear(data)
	result["encrypted_state_saved"] = encrypted
	result["post_run_state_reload"] = encrypted
	result["cookie_state_changed"] = savedCookieDigest(accounts[0].StorageState) != before
	result["failed_write_preserves_previous"] = checkSavedWriteFailure(root, accounts[0].StorageState)
	if result["failed_write_preserves_previous"] != true {
		result["success"] = false
		result["error"] = "ATOMIC_WRITE_CHECK_FAILED"
	}
	result["identity_preserved"] = accounts[0].ID == accounts[0].Config.Label
	if !encrypted {
		result["success"] = false
		result["error"] = "SAVED_STATE_ENCRYPTION_FAILED"
	}
}

func savedCookieDigest(state aistudio.StorageState) [32]byte {
	data, _ := json.Marshal(state.Cookies)
	digest := sha256.Sum256(data)
	clear(data)
	return digest
}

// Exercise the existing codec on an isolated private clone, never on the live account.
// An actual directory permission failure must leave the previous encrypted file loadable.
func checkSavedWriteFailure(root string, state aistudio.StorageState) bool {
	scratch, err := os.MkdirTemp(root, ".atomic-check-")
	if err != nil {
		return false
	}
	defer func() { os.Chmod(scratch, 0700); os.RemoveAll(scratch) }()
	file := filepath.Join(scratch, "storage-state.json")
	if aistudio.WriteStorageState(file, state) != nil {
		return false
	}
	before, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	defer clear(before)
	if os.Chmod(scratch, 0500) != nil {
		return false
	}
	writeErr := aistudio.WriteStorageState(file, state)
	after, readErr := os.ReadFile(file)
	defer clear(after)
	loaded, loadErr := aistudio.LoadStorageState(file)
	return writeErr != nil && readErr == nil && loadErr == nil && bytes.Equal(before, after) && savedCookieDigest(loaded) == savedCookieDigest(state)
}
