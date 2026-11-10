package auth_test

import (
	"testing"
	"time"

	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/config"
)

func TestApprovalLifecycle(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	store := &auth.Store{DB: db, TTL: time.Minute, Now: func() time.Time { return now }}

	args := map[string]string{"domain": "example.com"}
	a, err := store.Create("DangerousDemo", args)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == "" {
		t.Fatal("empty id")
	}

	// Helper must reject before console approve.
	if err := store.Consume(a.ID, "DangerousDemo", args); err == nil {
		t.Fatal("expected not approved")
	}

	if err := store.Approve(a.ID, "root@console"); err != nil {
		t.Fatal(err)
	}

	// Wrong args must fail.
	if err := store.Consume(a.ID, "DangerousDemo", map[string]string{"domain": "other.com"}); err == nil {
		t.Fatal("expected args mismatch")
	}

	if err := store.Consume(a.ID, "DangerousDemo", args); err != nil {
		t.Fatal(err)
	}
	// Single-use.
	if err := store.Consume(a.ID, "DangerousDemo", args); err == nil {
		t.Fatal("expected already used")
	}
}

func TestApprovalExpiry(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	store := &auth.Store{DB: db, TTL: time.Minute, Now: func() time.Time { return now }}
	args := map[string]any{"x": 1}
	a, err := store.Create("DangerousDemo", args)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Approve(a.ID, "root"); err != nil {
		t.Fatal(err)
	}
	store.Now = func() time.Time { return now.Add(2 * time.Minute) }
	if err := store.Consume(a.ID, "DangerousDemo", args); err == nil {
		t.Fatal("expected expired")
	}
}
