package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestParseExample(t *testing.T) {
	path := filepath.Join("testdata", "example.yaml")
	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load example: %v", err)
	}
	if c.Version != 1 {
		t.Fatalf("version = %d, want 1", c.Version)
	}
	if c.Panel.Domain != "panel.example.com" {
		t.Fatalf("panel.domain = %q", c.Panel.Domain)
	}
	if len(c.Sites) != 2 {
		t.Fatalf("sites = %d, want 2", len(c.Sites))
	}
	if c.Sites[0].Type != config.SiteTypeStatic {
		t.Fatalf("sites[0].type = %q", c.Sites[0].Type)
	}
	if c.Sites[1].Type != config.SiteTypeApp {
		t.Fatalf("sites[1].type = %q", c.Sites[1].Type)
	}
}

func TestRejectEmptySiteName(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: ""
    type: static
`))
	if err == nil {
		t.Fatal("expected error for empty site name")
	}
}

func TestRejectInvalidSiteType(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    type: php
`))
	if err == nil {
		t.Fatal("expected error for invalid site type")
	}
}

func TestRejectBadVersion(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 99
panel:
  port: 0
`))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestIdleTimeoutRequiresSocketActivation(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: app.example.com
    type: app
    exec: ./bin/server
    socket_activation: false
    idle_timeout: 15m
`))
	if err == nil {
		t.Fatal("expected error for idle_timeout without socket_activation")
	}
}

func TestIdleTimeoutOK(t *testing.T) {
	c, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: app.example.com
    type: app
    exec: ./bin/server
    socket_activation: true
    idle_timeout: 15m
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Sites[0].IdleTimeout != "15m" {
		t.Fatalf("idle_timeout=%q", c.Sites[0].IdleTimeout)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	src, err := os.ReadFile(filepath.Join("testdata", "example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Save(path, c); err != nil {
		t.Fatal(err)
	}
	again, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Sites[0].Name != "example.com" {
		t.Fatalf("round-trip name = %q", again.Sites[0].Name)
	}
}
