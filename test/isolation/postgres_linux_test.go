//go:build linux

package isolation_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/postgres"
	"github.com/weborpanel/webor/internal/sites"
)

// TestPostgresCrossSiteConnectDenial ensures site B's peer role cannot use site A's DB.
func TestPostgresCrossSiteConnectDenial(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Skip("psql not installed")
	}
	ops := helper.DefaultOps()
	st, err := ops.PostgresStatus()
	if err != nil || st == nil || !st.Installed || !st.Active {
		t.Skip("postgresql not active")
	}

	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	state := t.TempDir()
	a, b := "pg-iso-a.test", "pg-iso-b.test"
	if err := ops.CreateSiteUser(a); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateSiteUser(b); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ops.DropSiteDatabase(a, "postgres", "", state)
		_ = ops.DropSiteDatabase(b, "postgres", "", state)
		_ = ops.DeleteSiteUser(a, true)
		_ = ops.DeleteSiteUser(b, true)
	})

	if _, err := ops.EnsureSiteDatabase(a, "postgres", "", state); err != nil {
		t.Fatal(err)
	}
	if _, err := ops.EnsureSiteDatabase(b, "postgres", "", state); err != nil {
		t.Fatal(err)
	}

	dbA := postgres.DBName(a, "")
	roleB := postgres.PeerRole(b)
	cmd := exec.Command("runuser", "-u", roleB, "--", "psql", "-d", dbA, "-c", "SELECT 1")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected CONNECT denial, got success: %s", out)
	}
}
