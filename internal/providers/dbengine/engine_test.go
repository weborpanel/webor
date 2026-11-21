package dbengine_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/providers/dbengine"
)

func TestNormalizeAndSlug(t *testing.T) {
	e, err := dbengine.Normalize(" Postgres ")
	if err != nil || e != dbengine.Postgres {
		t.Fatalf("got %q %v", e, err)
	}
	if dbengine.NeedsApproval(dbengine.SQLite) {
		t.Fatal("sqlite should not need approval")
	}
	if !dbengine.NeedsApproval(dbengine.MariaDB) {
		t.Fatal("mariadb needs approval")
	}
	if !strings.Contains(dbengine.Slug("app.example.com"), "example") {
		t.Fatal(dbengine.Slug("app.example.com"))
	}
}
