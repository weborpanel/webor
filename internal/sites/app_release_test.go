package sites_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/sites"
)

func TestActivateAppRelease(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "app.example.com"
	id1 := "20260101T000001Z"
	id2 := "20260101T000002Z"
	for _, id := range []string{id1, id2} {
		dir := sites.Release(domain, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "server"), []byte("bin-"+id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := sites.ActivateAppRelease(domain, id1, ""); err != nil {
		t.Fatal(err)
	}
	if got := sites.CurrentReleaseID(domain); got != id1 {
		t.Fatalf("current=%q", got)
	}
	body, err := os.ReadFile(sites.AppBinaryPath(domain))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "bin-"+id1 {
		t.Fatalf("body=%q", body)
	}
	if err := sites.ActivateAppRelease(domain, id2, "server"); err != nil {
		t.Fatal(err)
	}
	if got := sites.CurrentReleaseID(domain); got != id2 {
		t.Fatalf("current=%q", got)
	}
	body, err = os.ReadFile(sites.AppBinaryPath(domain))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "bin-"+id2 {
		t.Fatalf("body=%q", body)
	}
}
