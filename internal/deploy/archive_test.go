package deploy_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/deploy"
)

func writeTar(t *testing.T, path string, files map[string]string, gzipOn bool) {
	t.Helper()
	var buf bytes.Buffer
	var w *tar.Writer
	var gz *gzip.Writer
	if gzipOn {
		gz = gzip.NewWriter(&buf)
		w = tar.NewWriter(gz)
	} else {
		w = tar.NewWriter(&buf)
	}
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := w.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTarIntoSubdir(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, "site.tar")
	writeTar(t, tp, map[string]string{"a/index.html": "hi", "a/b.css": "x"}, false)
	dest := filepath.Join(dir, "out", "assets")
	if err := deploy.ExtractTar(tp, dest, deploy.ExtractOptions{FlattenSingleTopDir: false}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "a", "index.html"))
	if err != nil || string(body) != "hi" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, "site.tar.gz")
	writeTar(t, tp, map[string]string{"index.html": "gz"}, true)
	dest := filepath.Join(dir, "out")
	if err := deploy.ExtractTarGz(tp, dest, deploy.ExtractOptions{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "index.html"))
	if err != nil || string(body) != "gz" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestExtractTarRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, "evil.tar")
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: "../outside.html", Mode: 0o644, Size: 4}
	_ = w.WriteHeader(hdr)
	_, _ = w.Write([]byte("nope"))
	_ = w.Close()
	_ = os.WriteFile(tp, buf.Bytes(), 0o644)
	if err := deploy.ExtractTar(tp, filepath.Join(dir, "out"), deploy.ExtractOptions{}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestExtractArchiveDetect(t *testing.T) {
	if deploy.DetectArchiveFormat("x.TAR.GZ") != "tar.gz" {
		t.Fatal(deploy.DetectArchiveFormat("x.TAR.GZ"))
	}
	if deploy.DetectArchiveFormat("a.tgz") != "tar.gz" {
		t.Fatal()
	}
	if deploy.DetectArchiveFormat("a.zip") != "zip" {
		t.Fatal()
	}
}

func TestExtractZipOptsNoFlatten(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "site.zip")
	writeZip(t, zp, map[string]string{"dist/index.html": "keep"})
	dest := filepath.Join(dir, "out")
	if err := deploy.ExtractZipOpts(zp, dest, deploy.ExtractOptions{FlattenSingleTopDir: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "dist", "index.html")); err != nil {
		t.Fatal(err)
	}
}
