package app

import "github.com/Mag1cFall/AIStudio2API/internal/aistudio"

// Standalone app tests use a synthetic key and temp directories, never Keystore
// or the installed application's real account state.
func init() {
	if err := aistudio.SetAndroidStorageKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		panic(err)
	}
}
