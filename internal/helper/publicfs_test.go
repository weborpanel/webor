package helper_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/sites"
)

func seedPublic(t *testing.T, domain string) {
	t.Helper()
	if err := os.MkdirAll(sites.Public(domain), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sites.Tmp(domain), 0o750); err != nil {
		t.Fatal(err)
	}
}

func TestPublicFSBlocksDataEscape(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "fs.example"
	seedPublic(t, domain)
	ops := helper.DefaultOps()

	dataDir := filepath.Join(sites.Root(domain), "data")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "app.db")
	if err := os.WriteFile(dbPath, []byte("secret"), 0o640); err != nil {
		t.Fatal(err)
	}

	if _, err := ops.ListPublicDir(domain, "../data"); err == nil {
		t.Fatal("expected list ../data to fail")
	}
	if _, _, err := ops.ReadPublicFile(domain, "../data/app.db"); err == nil {
		t.Fatal("expected read ../data/app.db to fail")
	}
	if err := ops.DeletePublicPath(domain, "../data/app.db", false); err == nil {
		t.Fatal("expected delete ../data/app.db to fail")
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatal(err)
	}
}

func TestPublicFSListMkdirRenameDelete(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "mgr.example"
	seedPublic(t, domain)
	ops := helper.DefaultOps()
	if err := ops.MkdirPublic(domain, "assets"); err != nil {
		t.Fatal(err)
	}
	if err := ops.WritePublicFile(domain, "assets/a.txt", []byte("one")); err != nil {
		t.Fatal(err)
	}
	entries, err := ops.ListPublicDir(domain, "assets")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "a.txt" {
		t.Fatalf("%+v", entries)
	}
	if err := ops.RenamePublic(domain, "assets/a.txt", "assets/b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := ops.DeletePublicPath(domain, "assets/b.txt", false); err != nil {
		t.Fatal(err)
	}
	entries, err = ops.ListPublicDir(domain, "assets")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("%+v", entries)
	}
}

func TestExtractPublicArchiveIntoSubdir(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "zipsub.example"
	seedPublic(t, domain)
	ops := helper.DefaultOps()
	if err := ops.MkdirPublic(domain, "bundle"); err != nil {
		t.Fatal(err)
	}
	zp := filepath.Join(sites.Tmp(domain), "site.zip")
	f, err := os.Create(zp)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	fw, _ := zw.Create("dist/index.html")
	_, _ = fw.Write([]byte("from-zip"))
	_ = zw.Close()
	_ = f.Close()

	dest, err := ops.ExtractPublicArchive(domain, zp, "bundle", "zip")
	if err != nil {
		t.Fatal(err)
	}
	if dest != filepath.Join(sites.Public(domain), "bundle") {
		t.Fatalf("dest=%q", dest)
	}
	body, err := os.ReadFile(filepath.Join(sites.Public(domain), "bundle", "dist", "index.html"))
	if err != nil || string(body) != "from-zip" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}
