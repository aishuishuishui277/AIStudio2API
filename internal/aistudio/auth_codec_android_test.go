package aistudio

import (
	"bytes"
	"testing"
)

func TestAndroidStateEncryption(t *testing.T) {
	key := bytes.Repeat([]byte{0x41}, 32)
	if err := SetAndroidStorageKey(key); err != nil {
		t.Fatal(err)
	}
	plain := []byte(`{"cookies":[{"name":"SID","value":"synthetic-secret"}]}`)
	sealed, err := encodeStoredState(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("synthetic-secret")) {
		t.Fatal("credentials were written as plaintext")
	}
	again, err := encodeStoredState(plain)
	if err != nil || bytes.Equal(sealed, again) {
		t.Fatal("encryption did not use a fresh nonce")
	}
	opened, err := decodeStoredState(sealed)
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatal("encrypted state round trip failed")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := decodeStoredState(sealed); err == nil {
		t.Fatal("modified ciphertext was accepted")
	}
	if _, err := decodeStoredState(plain); err == nil {
		t.Fatal("plaintext bypassed the import path")
	}
	if err := SetAndroidStorageKey(bytes.Repeat([]byte{0x42}, 32)); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeStoredState(again); err == nil {
		t.Fatal("wrong key was accepted")
	}
	if err := SetAndroidStorageKey([]byte("short")); err == nil {
		t.Fatal("invalid data key was accepted")
	}
}
