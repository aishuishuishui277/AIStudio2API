//go:build android && authpoc

package app

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/authpoc"
)

type pendingAuthUpdate struct {
	gate                    authpoc.Gate
	cancel                  context.CancelFunc
	stage, target, identity string
	previous                [32]byte
	prepared                bool
	timer                   *time.Timer
}

var authUpdate struct {
	sync.Mutex
	pending *pendingAuthUpdate
}

func updateFailure(code string) map[string]any {
	r := map[string]any{"success": false, "error": code, "committed": false, "old_state_preserved": true, "scope": "candidate_update"}
	classifyProbeResult(r)
	return r
}

// PrepareAuthUpdate has no publishing authority. Only the second user confirmation
// calls CommitAuthUpdate. Cancellation, process death, and abandoned staging cannot publish.
func PrepareAuthUpdate(files string, key, raw []byte) (result map[string]any) {
	defer clear(key)
	defer clear(raw)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	p := &pendingAuthUpdate{cancel: cancel}
	authUpdate.Lock()
	if authUpdate.pending != nil {
		authUpdate.Unlock()
		cancel()
		return updateFailure("UPDATE_BUSY")
	}
	authUpdate.pending = p
	authUpdate.Unlock()
	accepted := false
	defer func() {
		if !accepted {
			cancel()
			authUpdate.Lock()
			if authUpdate.pending == p {
				authUpdate.pending = nil
			}
			authUpdate.Unlock()
			if p.stage != "" {
				os.RemoveAll(p.stage)
			} // Only this call's freshly created private stage.
		}
	}()
	if !filepath.IsAbs(files) || len(key) != 32 || len(raw) > 256*1024 {
		return updateFailure("INVALID_INPUT")
	}
	if err := aistudio.SetAndroidStorageKey(key); err != nil {
		return updateFailure("KEYSTORE_STATE_INVALID")
	}
	current, err := aistudio.NewAccountStore(filepath.Join(files, "core", "auth")).Load()
	if err != nil || len(current) > 1 {
		return updateFailure("SAVED_STATE_INVALID")
	}
	first := len(current) == 0
	if !first {
		p.target, p.identity = current[0].StoragePath, current[0].ID
		if p.identity != current[0].Config.Label {
			return updateFailure("SAVED_IDENTITY_INVALID")
		}
		previous, e := os.ReadFile(p.target)
		if e != nil {
			return updateFailure("PRIVATE_STORAGE_FAILURE")
		}
		p.previous = sha256.Sum256(previous)
		clear(previous)
	} else {
		p.target = filepath.Join(files, "core", "auth")
	}
	root := filepath.Join(files, "auth-candidates")
	if os.MkdirAll(root, 0700) != nil {
		return updateFailure("PRIVATE_STORAGE_FAILURE")
	}
	p.stage, err = os.MkdirTemp(root, "candidate-")
	if err != nil {
		return updateFailure("PRIVATE_STORAGE_FAILURE")
	}
	result = runPGProbe(files, append([]byte(nil), key...), raw, false, false, pgProbeOptions{context: ctx, runDirectory: p.stage, expectedAccount: p.identity})
	result["scope"] = "candidate_update"
	result["committed"] = false
	result["old_state_preserved"] = true
	if result["success"] != true {
		return result
	}
	candidate, err := aistudio.NewAccountStore(filepath.Join(p.stage, "auth")).Load()
	if err != nil || len(candidate) != 1 || (!first && candidate[0].ID != p.identity) {
		return updateFailure("CANDIDATE_RELOAD_FAILED")
	}
	if first {
		p.identity = candidate[0].ID
	}
	authUpdate.Lock()
	defer authUpdate.Unlock()
	if ctx.Err() != nil || !p.gate.PrepareForAccount(p.identity, candidate[0].ID) {
		return updateFailure("CANCELLED")
	}
	p.prepared = true
	accepted = true
	p.timer = time.AfterFunc(120*time.Second, func() { cancelAuthUpdate(p) })
	result["stage"] = "awaiting_commit_confirmation"
	result["candidate_encrypted_reload"] = true
	result["first_account"] = first
	return result
}

func cancelAuthUpdate(expected *pendingAuthUpdate) map[string]any {
	authUpdate.Lock()
	defer authUpdate.Unlock()
	p := authUpdate.pending
	if p == nil || (expected != nil && p != expected) {
		return updateFailure("NO_PENDING_UPDATE")
	}
	if !p.gate.Cancel() {
		return map[string]any{"success": false, "error": "COMMIT_IN_PROGRESS", "category": "UNKNOWN_ERROR", "commit_status": "unknown"}
	}
	p.cancel()
	if p.timer != nil {
		p.timer.Stop()
	}
	result := updateFailure("CANCELLED")
	if p.prepared {
		current, err := os.ReadFile(p.target)
		unchanged := err == nil && sha256.Sum256(current) == p.previous
		if filepath.Base(p.target) == "auth" {
			entries, e := os.ReadDir(p.target)
			unchanged = os.IsNotExist(e) || (e == nil && len(entries) == 0)
		}
		clear(current)
		result["old_state_preserved"] = unchanged
		result["old_ciphertext_verified_unchanged"] = unchanged
		if !unchanged {
			result["error"] = "STATE_CHANGED_EXTERNALLY"
		}
		os.RemoveAll(p.stage)
		authUpdate.pending = nil
		result["candidate_discarded"] = true
	}
	return result
}
func CancelAuthUpdate() map[string]any { return cancelAuthUpdate(nil) }

func CommitAuthUpdate() (result map[string]any) {
	authUpdate.Lock()
	defer authUpdate.Unlock()
	p := authUpdate.pending
	if p == nil || !p.prepared {
		return updateFailure("NO_PENDING_UPDATE")
	}
	defer func() {
		if p.timer != nil {
			p.timer.Stop()
		}
		p.cancel()
		os.RemoveAll(p.stage)
		authUpdate.pending = nil
	}()
	candidate, err := aistudio.NewAccountStore(filepath.Join(p.stage, "auth")).Load()
	if err != nil || len(candidate) != 1 || candidate[0].ID != p.identity {
		return updateFailure("ACCOUNT_MISMATCH")
	}
	encrypted, err := os.ReadFile(candidate[0].StoragePath)
	if err != nil {
		return updateFailure("PRIVATE_STORAGE_FAILURE")
	}
	defer clear(encrypted)
	if filepath.Base(p.target) == "auth" {
		committed, e := p.gate.Commit(func() (bool, error) { return authpoc.PublishFirst(filepath.Join(p.stage, "auth"), p.target) })
		r := map[string]any{"success": e == nil && committed, "committed": committed, "first_account": true, "directory_synced": e == nil, "stage": "update_complete"}
		if e != nil {
			r["error"] = "FIRST_ACCOUNT_PUBLISH_FAILED"
		} else {
			loaded, loadErr := aistudio.NewAccountStore(p.target).Load()
			r["encrypted_state_reload"] = loadErr == nil && len(loaded) == 1
			if loadErr != nil || len(loaded) != 1 {
				r["success"] = false
				r["error"] = "COMMITTED_RELOAD_FAILED"
			}
		}
		classifyProbeResult(r)
		return r
	}
	committed, err := p.gate.Commit(func() (bool, error) { return authpoc.Publish(context.Background(), p.target, p.previous, encrypted) })
	result = map[string]any{"success": err == nil && committed, "committed": committed, "old_state_preserved": !committed, "scope": "candidate_update", "same_account": true, "stage": "update_complete"}
	if err != nil {
		if committed {
			result["error"] = "COMMITTED_DURABILITY_UNKNOWN"
		} else {
			result["error"] = "PUBLISH_FAILED"
		}
		classifyProbeResult(result)
		return result
	}
	_, loadErr := aistudio.LoadStorageState(p.target)
	_, backupErr := aistudio.LoadStorageState(p.target + ".previous")
	result["encrypted_state_reload"] = loadErr == nil
	result["previous_encrypted_state_reload"] = backupErr == nil
	result["directory_synced"] = true
	if loadErr != nil || backupErr != nil {
		result["success"] = false
		result["error"] = "COMMITTED_RELOAD_FAILED"
	}
	classifyProbeResult(result)
	return result
}
