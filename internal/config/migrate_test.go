package config_test

import (
	"testing"

	"github.com/webooza/webooza/internal/config"
)

func TestOpenDBAndMigrate(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow(`SELECT version FROM schema_migrations WHERE version = 1`).Scan(&version); err != nil {
		t.Fatalf("migration 1 missing: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_events'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("audit_events table missing")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='approvals'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("approvals table missing")
	}

	// Re-open should be idempotent.
	db2, err := config.OpenDB(dir)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	db2.Close()
}
