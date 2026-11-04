package sqlitedb

import (
	"path/filepath"
	"testing"
)

func TestAndroidSQLitePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "android.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE probe (value TEXT); INSERT INTO probe VALUES ('Android ARM64 持久化')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		db.Close()
		t.Fatalf("WAL mode: %q, error: %v", mode, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`SELECT value FROM probe`).Scan(&value); err != nil || value != "Android ARM64 持久化" {
		t.Fatalf("persisted value: %q, error: %v", value, err)
	}
}
