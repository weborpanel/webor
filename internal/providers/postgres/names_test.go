package postgres_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/postgres"
)

func TestNames(t *testing.T) {
	domain := "app.example.com"
	id := dbengine.DefaultID(domain)
	db := postgres.DBName(domain, id)
	if !strings.HasPrefix(db, "wbr_") {
		t.Fatalf("db name %q", db)
	}
	if postgres.PeerRole(domain) != db {
		t.Fatalf("peer role mismatch")
	}
	if postgres.AppRole(domain, id) != db {
		t.Fatalf("app role mismatch for default id")
	}
	ro := postgres.ReadOnlyRole(domain, id)
	if !strings.HasPrefix(ro, "webor_ro_") {
		t.Fatalf("ro role %q", ro)
	}
	if !strings.Contains(postgres.PeerURL(domain, id), db) {
		t.Fatalf("peer url missing db")
	}
	if postgres.SecretFileName(domain, id) != "app.example.com.app_example_com.postgres.ro.json" {
		t.Fatalf("secret file name %q", postgres.SecretFileName(domain, id))
	}
	extra := postgres.AppRole(domain, "notes")
	if !strings.Contains(extra, "notes") {
		t.Fatalf("extra app role %q", extra)
	}
}
