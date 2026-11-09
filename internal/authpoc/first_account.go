package authpoc

import (
	"os"
	"path/filepath"
)

// PublishFirst installs a validated private account tree under an exclusive process lease.
// It never replaces an existing nonempty account tree. Rename is the commit point.
func PublishFirst(candidate, target string) (bool, error) {
	if entries, err := os.ReadDir(target); err == nil {
		if len(entries) != 0 {
			return false, ErrConflict
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := filepath.Walk(candidate, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return ErrInvalidEnvelope
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return f.Sync()
	}); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return false, err
	}
	// Remove only the empty destination, never any prior saved account.
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.Rename(candidate, target); err != nil {
		return false, err
	}
	return true, syncDirectory(filepath.Dir(target))
}
