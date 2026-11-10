package sites_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/webooza/webooza/internal/sites"
)

func TestActivateReleaseAtomic(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "example.test"
	id1 := "20260101T000000Z"
	id2 := "20260101T000100Z"
	for _, id := range []string{id1, id2} {
		if err := os.MkdirAll(sites.Release(domain, id), 0o755); err != nil {
			t.Fatal(err)
		}
		body := id
		if err := os.WriteFile(filepath.Join(sites.Release(domain, id), "index.html"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := sites.ActivateRelease(domain, id1); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(sites.Public(domain))
	if err != nil || filepath.Base(got) != id1 {
		t.Fatalf("public=%q err=%v", got, err)
	}
	if err := sites.ActivateRelease(domain, id2); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(sites.Public(domain), "index.html"))
	if err != nil || string(body) != id2 {
		t.Fatalf("body=%q err=%v", body, err)
	}
	cur, err := os.Readlink(sites.Current(domain))
	if err != nil || filepath.Base(cur) != id2 {
		t.Fatalf("current=%q err=%v", cur, err)
	}
}

func TestEnsureReleaseLayout(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	id, err := sites.EnsureReleaseLayout("a.example", time.Date(2026, 11, 10, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if id != "20261110T010203Z" {
		t.Fatalf("id=%q", id)
	}
	target, err := os.Readlink(sites.Public("a.example"))
	if err != nil || filepath.Base(target) != id {
		t.Fatalf("public=%q err=%v", target, err)
	}
	// Second call is idempotent.
	id2, err := sites.EnsureReleaseLayout("a.example", time.Now().UTC())
	if err != nil || id2 != id {
		t.Fatalf("id2=%q err=%v", id2, err)
	}
}

func TestEnsureReleaseLayoutKeepsLegacyPublicDir(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	pub := sites.Public("legacy.example")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	id, err := sites.EnsureReleaseLayout("legacy.example", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		t.Fatalf("expected empty id for legacy dir, got %q", id)
	}
	fi, err := os.Lstat(pub)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Fatalf("legacy public should remain a directory")
	}
}

func TestNewReleaseID(t *testing.T) {
	id := sites.NewReleaseID(time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))
	if id != "20261110T120000Z" {
		t.Fatalf("got %q", id)
	}
}

func TestPublicPath(t *testing.T) {
	if sites.Public("Example.COM") != "/srv/sites/example.com/public" {
		t.Fatal(sites.Public("Example.COM"))
	}
}
