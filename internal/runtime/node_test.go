package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	wbrruntime "github.com/weborpanel/webor/internal/runtime"
)

func TestDistURL(t *testing.T) {
	got := wbrruntime.DistURL("22.14.0", "linux", "x64")
	want := "https://nodejs.org/dist/v22.14.0/node-v22.14.0-linux-x64.tar.xz"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveNodeBinaryCommand(t *testing.T) {
	bin, err := wbrruntime.ResolveNodeBinary(&config.SiteRuntime{
		Kind:    config.RuntimeKindNode,
		Command: "/usr/bin/node",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if bin != "/usr/bin/node" {
		t.Fatalf("%q", bin)
	}
}

func TestAliasMajorRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	prev := wbrruntime.NodeRoot
	wbrruntime.NodeRoot = tmp
	t.Cleanup(func() { wbrruntime.NodeRoot = prev })

	fullDir := filepath.Join(tmp, "v22.14.0")
	if err := os.MkdirAll(filepath.Join(fullDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fullDir, "bin", "node"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := wbrruntime.AliasMajor("22", "22.14.0"); err != nil {
		t.Fatal(err)
	}
	got, err := wbrruntime.ReadMajorAlias("22")
	if err != nil {
		t.Fatal(err)
	}
	if got != "22.14.0" {
		t.Fatalf("%q", got)
	}
	bin, err := wbrruntime.ResolveNodeBinary(&config.SiteRuntime{
		Kind:    config.RuntimeKindNode,
		Version: "22",
	}, "22.14.0")
	if err != nil {
		t.Fatal(err)
	}
	if bin != filepath.Join(tmp, "v22.14.0", "bin", "node") {
		t.Fatalf("%q", bin)
	}
}
