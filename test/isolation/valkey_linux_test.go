//go:build linux

package isolation_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/valkey"
)

// TestValkeyCrossSiteKeyDenial ensures site B ACL cannot touch site A's key prefix.
func TestValkeyCrossSiteKeyDenial(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	ops := helper.DefaultOps()
	st, err := ops.ValkeyStatus()
	if err != nil || st == nil || !st.Installed || !st.Active {
		t.Skip("valkey/redis not active")
	}
	cli := st.Binary
	if cli == "" {
		t.Skip("no cli")
	}

	state := t.TempDir()
	a, b := "vk-iso-a.test", "vk-iso-b.test"
	t.Cleanup(func() {
		_ = ops.DropSiteCache(a, state)
		_ = ops.DropSiteCache(b, state)
	})

	resA, err := ops.EnsureSiteCache(a, "valkey", state)
	if err != nil {
		t.Fatal(err)
	}
	resB, err := ops.EnsureSiteCache(b, "valkey", state)
	if err != nil {
		t.Fatal(err)
	}

	secA, err := valkey.ReadSecret(valkey.SecretPath(state, a))
	if err != nil {
		t.Fatal(err)
	}
	secB, err := valkey.ReadSecret(valkey.SecretPath(state, b))
	if err != nil {
		t.Fatal(err)
	}

	keyA := resA.Prefix + "secret"
	authArgs := func(user, pass string, cmd ...string) []string {
		return append([]string{"--user", user, "-a", pass, "--no-auth-warning"}, cmd...)
	}
	// A can set its own key.
	if out, err := exec.Command(cli, authArgs(secA.User, secA.Password, "SET", keyA, "1")...).CombinedOutput(); err != nil {
		t.Fatalf("A SET own key: %v (%s)", err, out)
	}
	// B must not GET A's key.
	out, err := exec.Command(cli, authArgs(secB.User, secB.Password, "GET", keyA)...).CombinedOutput()
	joined := string(out)
	if err == nil && !strings.Contains(joined, "NOPERM") && strings.TrimSpace(joined) == "1" {
		t.Fatalf("isolation FAIL: B read A's key: %s", joined)
	}
	_ = resB
}
