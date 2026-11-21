//go:build linux

package isolation_test

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/sites"
)

func TestSiteUserCannotReadOtherSitePrivateFiles(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to create site users")
	}
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	ops := helper.DefaultOps()
	a, b := "iso-a.test", "iso-b.test"
	if err := ops.CreateSiteUser(a); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateSiteUser(b); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ops.DeleteSiteUser(a, true)
		_ = ops.DeleteSiteUser(b, true)
	})

	secret := filepath.Join(sites.Logs(b), "secret.log")
	if err := os.WriteFile(secret, []byte("top-secret\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	bu, err := user.Lookup(sites.Username(b))
	if err != nil {
		t.Fatal(err)
	}
	_ = exec.Command("chown", bu.Uid+":"+bu.Gid, secret).Run()
	_ = os.Chmod(sites.Logs(b), 0o750)

	// Site A user must not read B's private log.
	cmd := exec.Command("sudo", "-u", sites.Username(a), "cat", secret)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("site A read site B private file; out=%q", out)
	}

	// Public dirs are world-readable for Caddy; that is intentional.
	pubB := filepath.Join(sites.Public(b), "index.html")
	if err := os.WriteFile(pubB, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(sites.Public(b), 0o755)
	if _, err := os.ReadFile(pubB); err != nil {
		t.Fatalf("public should be readable: %v", err)
	}
}
