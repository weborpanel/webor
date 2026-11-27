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

func TestAdminerPathsAndSQLiteDSN(t *testing.T) {
	domain := "app.example.com"
	def := dbengine.DefaultID(domain)
	if got := dbengine.AdminerSlug(domain, ""); got != def {
		t.Fatalf("default slug: got %q want %q", got, def)
	}
	if got := dbengine.AdminerBrowsePath(domain, ""); got != "/db/"+def+"/" {
		t.Fatalf("browse default: %q", got)
	}
	if got := dbengine.AdminerEditPath(domain, ""); got != "/db/"+def+"/edit/" {
		t.Fatalf("edit default: %q", got)
	}
	if got := dbengine.AdminerSlug(domain, "analytics"); got != "app_example_com_analytics" {
		t.Fatalf("secondary slug: %q", got)
	}
	if got := dbengine.AdminerBrowsePath(domain, "analytics"); got != "/db/app_example_com_analytics/" {
		t.Fatalf("browse secondary: %q", got)
	}
	if got := dbengine.AdminerEditPath(domain, "analytics"); got != "/db/app_example_com_analytics/edit/" {
		t.Fatalf("edit secondary: %q", got)
	}
	// Instance ids that already include the hostname must not be prefixed again.
	inst := "app_example_com_postgres_1"
	if got := dbengine.AdminerSlug(domain, inst); got != inst {
		t.Fatalf("instance slug: got %q want %q", got, inst)
	}
	if got := dbengine.AdminerBrowsePath(domain, inst); got != "/db/"+inst+"/" {
		t.Fatalf("instance browse: %q", got)
	}
	if !dbengine.MatchAdminerSegment(domain, inst, inst) {
		t.Fatal("instance id segment should match")
	}
	if !dbengine.MatchAdminerSegment(domain, inst, "app_example_com_"+inst) {
		t.Fatal("legacy doubled slug should still match")
	}
	if dbengine.MatchAdminerSegment(domain, inst, "other") {
		t.Fatal("unrelated segment matched")
	}
	if !dbengine.MatchAdminerSegment(domain, "analytics", "analytics") {
		t.Fatal("short id segment should match")
	}
	ro := dbengine.SQLiteDSN(domain, def)
	rw := dbengine.SQLiteRWDSN(domain, def)
	if !strings.Contains(ro, "mode=ro") {
		t.Fatalf("RO DSN missing mode=ro: %q", ro)
	}
	if strings.Contains(rw, "mode=ro") {
		t.Fatalf("RW DSN should not be mode=ro: %q", rw)
	}
	if !strings.HasPrefix(rw, "file:") {
		t.Fatalf("RW DSN: %q", rw)
	}
}
