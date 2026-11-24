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

func TestROSecretDSNAs(t *testing.T) {
	sec := postgres.ROSecret{
		User:     "webor_ro_wbr_app",
		Password: "secret",
		Database: "wbr_app",
		Host:     "127.0.0.1",
		Port:     5432,
	}
	ro := sec.DSN()
	if !strings.Contains(ro, "user=webor_ro_wbr_app") {
		t.Fatalf("RO DSN: %q", ro)
	}
	app := sec.DSNAs("wbr_app")
	if !strings.Contains(app, "user=wbr_app") {
		t.Fatalf("App DSN: %q", app)
	}
	if !strings.Contains(app, "password=secret") || !strings.Contains(app, "dbname=wbr_app") {
		t.Fatalf("App DSN missing fields: %q", app)
	}
}
