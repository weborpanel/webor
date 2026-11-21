package sites_test

import (
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/sites"
)

func TestResolvePublicRelBlocksData(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "ex.com"
	if _, err := sites.ResolvePublicRel(domain, "../data/app.db"); err == nil {
		t.Fatal("expected escape rejection")
	}
	if _, err := sites.ResolvePublicPath(domain, "../data/app.db"); err == nil {
		t.Fatal("expected escape rejection")
	}
	root, err := sites.ResolvePublicRel(domain, "")
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Clean(sites.Public(domain)) {
		t.Fatalf("root=%q", root)
	}
}
