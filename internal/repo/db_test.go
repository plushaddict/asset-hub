package repo

import (
	"path/filepath"
	"testing"
)

func TestMigrate(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='system_settings'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("system_settings missing: %v n=%d", err, n)
	}
}