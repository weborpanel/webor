package audit_test

import (
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/config"
)

func TestAppendChains(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	log := &audit.Logger{DB: db, Now: func() time.Time { return now }}

	if err := log.Append("panel", "RequestApproval", map[string]any{"op": "DeleteSiteUser"}, "ok"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := log.Append("wbr", "Approve", map[string]any{"id": "abc"}, "ok"); err != nil {
		t.Fatal(err)
	}

	var n int
	var h1, h2, prev2 string
	if err := db.QueryRow(`SELECT COUNT(1) FROM audit_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d", n)
	}
	if err := db.QueryRow(`SELECT hash FROM audit_events ORDER BY id ASC LIMIT 1`).Scan(&h1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT prev_hash, hash FROM audit_events ORDER BY id DESC LIMIT 1`).Scan(&prev2, &h2); err != nil {
		t.Fatal(err)
	}
	if prev2 != h1 {
		t.Fatalf("chain broken: prev=%q want=%q", prev2, h1)
	}
	if h1 == "" || h2 == "" || h1 == h2 {
		t.Fatalf("unexpected hashes %q %q", h1, h2)
	}
}
