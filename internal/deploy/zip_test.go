package deploy_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/webooza/webooza/internal/deploy"
	"github.com/webooza/webooza/internal/sites"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipFlat(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "site.zip")
	writeZip(t, zp, map[string]string{"index.html": "hello", "css/a.css": "body{}"})
	dest := filepath.Join(dir, "out")
	if err := deploy.ExtractZip(zp, dest); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "index.html"))
	if err != nil || string(body) != "hello" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestExtractZipSingleTopDir(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "site.zip")
	writeZip(t, zp, map[string]string{"dist/index.html": "from-dist"})
	dest := filepath.Join(dir, "out")
	if err := deploy.ExtractZip(zp, dest); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "index.html"))
	if err != nil || string(body) != "from-dist" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "evil.zip")
	f, err := os.Create(zp)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	fw, err := w.Create("../outside.html")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("nope"))
	_ = w.Close()
	_ = f.Close()
	if err := deploy.ExtractZip(zp, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestStaticZipActivate(t *testing.T) {
	prev := sites.RootDir
	sites.RootDir = t.TempDir()
	t.Cleanup(func() { sites.RootDir = prev })

	domain := "zip.example"
	dir := t.TempDir()
	zp := filepath.Join(dir, "a.zip")
	writeZip(t, zp, map[string]string{"index.html": "v1"})
	id, err := deploy.StaticZip(domain, zp, deploy.StaticZipOptions{
		Now:       time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC),
		SkipChown: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "20261110T160000Z" {
		t.Fatalf("id=%q", id)
	}
	body, err := os.ReadFile(filepath.Join(sites.Public(domain), "index.html"))
	if err != nil || string(body) != "v1" {
		t.Fatalf("body=%q err=%v", body, err)
	}

	zp2 := filepath.Join(dir, "b.zip")
	writeZip(t, zp2, map[string]string{"index.html": "v2"})
	id2, err := deploy.StaticZip(domain, zp2, deploy.StaticZipOptions{
		Now:       time.Date(2026, 11, 10, 16, 0, 1, 0, time.UTC),
		SkipChown: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id2 == id {
		t.Fatal("expected new release id")
	}
	body, err = os.ReadFile(filepath.Join(sites.Public(domain), "index.html"))
	if err != nil || string(body) != "v2" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}
