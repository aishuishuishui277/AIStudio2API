//go:build android && authpoc

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

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

	markerPath := filepath.Join(files, "pg-auth-3a-result.json")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		return errSavedAuthRequired
	}
	var previous struct {
		Success bool   `json:"success"`
		Stage   string `json:"stage"`
	}
	if json.Unmarshal(marker, &previous) != nil || !previous.Success || previous.Stage != "complete" {
		return errSavedAuthRequired
	}
	markerInfo, err := os.Stat(markerPath)
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(files, "pg-auth-3a", "run-*", "auth", "*", "storage-state.json"))
	if err != nil || len(paths) == 0 {
		return errSavedAuthRequired
	}
	type candidate struct {
		path     string
		modified time.Time
	}
	candidates := []candidate{}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.ModTime().After(markerInfo.ModTime()) {
			return errSavedSourceAmbiguous
		}
		candidates = append(candidates, candidate{path, info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modified.After(candidates[j].modified) })
	if len(candidates) > 1 && candidates[0].modified.Equal(candidates[1].modified) {
		return errSavedSourceAmbiguous
	}
	source := candidates[0].path
	original, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(original, []byte("A2A-ANDROID-STATE-1\x00")) {
		return errSavedStateInvalid
	}
	before := sha256.Sum256(original)
	clear(original)
	sourceAccounts, err := aistudio.NewAccountStore(filepath.Dir(source)).Load()
	if err != nil || len(sourceAccounts) != 1 {
		return errSavedStateInvalid
	}
	account := sourceAccounts[0]
	if _, err := aistudio.NewSigner().Authorization(account.StorageState); err != nil {
		return errSavedAuthRequired
	}
	// Stage the complete directory, using the original encryption codec and same key.
	stage, err := os.MkdirTemp(root, ".migration-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // Only this function's newly created scratch directory.
	stagedAccount := filepath.Join(stage, "auth", account.ID)
	if err := os.MkdirAll(stagedAccount, 0700); err != nil {
		return err
	}
	config, err := os.ReadFile(account.ConfigPath)
	if err != nil {
		return err
	}
	if err := writeSavedConfig(filepath.Join(stagedAccount, "account.json"), config); err != nil {
		return err
	}
	if err := aistudio.WriteStorageState(filepath.Join(stagedAccount, "storage-state.json"), account.StorageState); err != nil {
		return err
	}
	check, err := aistudio.NewAccountStore(filepath.Join(stage, "auth")).Load()
	if err != nil || len(check) != 1 || check[0].ID != account.ID {
		return errSavedStateInvalid
	}
	if err := syncSavedDir(stagedAccount); err != nil {
		return err
	}
	if err := syncSavedDir(filepath.Join(stage, "auth")); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(stage, "auth"), target); err != nil {
		return err
	}
	if err := syncSavedDir(root); err != nil {
		return err
	}
	after, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	unchanged := before == sha256.Sum256(after)
	clear(after)
	if !unchanged {
		return errSavedSourceAmbiguous
	}
	result["source"] = "latest_successful_3a_encrypted_state"
	result["migration_performed"] = true
	result["original_state_unchanged"] = true
	result["identity_preserved"] = true
	return nil
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
