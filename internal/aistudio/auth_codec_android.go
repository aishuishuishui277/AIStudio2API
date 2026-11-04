package aistudio

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"sync"
)

var androidStateKey struct {
	sync.RWMutex
	aead cipher.AEAD
}

var stateMagic = []byte("A2A-ANDROID-STATE-1\x00")

// SetAndroidStorageKey installs the app's Keystore-wrapped data key before loading accounts.
func SetAndroidStorageKey(key []byte) error {
	if len(key) != 32 {
		return errors.New("Android storage key must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	androidStateKey.Lock()
	androidStateKey.aead = aead
	androidStateKey.Unlock()
	return nil
}

func encodeStoredState(data []byte) ([]byte, error) {
	androidStateKey.RLock()
	defer androidStateKey.RUnlock()
	aead := androidStateKey.aead
	if aead == nil {
		return nil, errors.New("Android credential storage is locked")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	result := append(append([]byte(nil), stateMagic...), nonce...)
	return aead.Seal(result, nonce, data, stateMagic), nil
}

func decodeStoredState(data []byte) ([]byte, error) {
	androidStateKey.RLock()
	defer androidStateKey.RUnlock()
	aead := androidStateKey.aead
	if aead == nil || !bytes.HasPrefix(data, stateMagic) || len(data) < len(stateMagic)+aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("Android credential storage is locked or invalid; use account import for desktop JSON")
	}
	data = data[len(stateMagic):]
	plaintext, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], stateMagic)
	if err != nil {
		return nil, errors.New("Android credential decryption failed")
	}
	return plaintext, nil
}
