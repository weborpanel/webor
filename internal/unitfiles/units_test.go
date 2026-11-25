package unitfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMatchInstall(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	installDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "install")
	for _, name := range Names {
		want, err := os.ReadFile(filepath.Join(installDir, name))
		if err != nil {
			t.Fatalf("install/%s: %v", name, err)
		}
		got, err := FS.ReadFile(name)
		if err != nil {
			t.Fatalf("embed %s: %v", name, err)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("%s drifted from install/%s — copy install units into internal/unitfiles/", name, name)
		}
	}
}

func TestInstallFromDir(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	body := []byte("[Unit]\nDescription=test\n")
	for _, name := range Names {
		if err := os.WriteFile(filepath.Join(src, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	changed, err := InstallFromDir(src, InstallOptions{
		DestDir:          dest,
		ForceCaddy:       true,
		SkipDaemonReload: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	for _, name := range Names {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, body) {
			t.Fatalf("%s: got %q", name, got)
		}
	}
	changed, err = InstallFromDir(src, InstallOptions{
		DestDir:          dest,
		ForceCaddy:       true,
		SkipDaemonReload: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second install should be no-op")
	}
}

func TestInstallEmbedded(t *testing.T) {
	dest := t.TempDir()
	changed, err := Install(InstallOptions{
		DestDir:          dest,
		ForceCaddy:       true,
		SkipDaemonReload: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected first install to change")
	}
	raw, err := os.ReadFile(filepath.Join(dest, "webor-caddy.service"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("ReadWritePaths=")) || !bytes.Contains(raw, []byte("/srv/sites")) {
		t.Fatalf("caddy unit missing /srv/sites ReadWritePaths:\n%s", raw)
	}
}
