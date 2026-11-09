package authpoc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFirstAccountDoesNotReplaceSavedTree(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "candidate")
	target := filepath.Join(root, "auth")
	os.Mkdir(source, 0700)
	os.Mkdir(target, 0700)
	os.WriteFile(filepath.Join(target, "existing"), []byte("old"), 0600)
	committed, err := PublishFirst(source, target)
	if committed || !errors.Is(err, ErrConflict) {
		t.Fatal("existing tree was not protected")
	}
	data, _ := os.ReadFile(filepath.Join(target, "existing"))
	if string(data) != "old" {
		t.Fatal("existing changed")
	}
}
func TestFirstAccountPublishesCompleteTree(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "candidate")
	target := filepath.Join(root, "core", "auth")
	os.MkdirAll(filepath.Join(source, "account"), 0700)
	os.WriteFile(filepath.Join(source, "account", "fixture"), []byte("offline filesystem fixture"), 0600)
	committed, err := PublishFirst(source, target)
	if err != nil || !committed {
		t.Fatal(committed, err)
	}
	if _, err = os.Stat(filepath.Join(target, "account", "fixture")); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("candidate still visible")
	}
}
