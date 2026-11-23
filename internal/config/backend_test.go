package config_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestValidateBackendPath(t *testing.T) {
	if err := config.ValidateBackendPath("/api"); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateBackendPath("/"); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateBackendPath("/../etc"); err == nil {
		t.Fatal("expected error for ..")
	}
}

func TestParseBackendLegacyMigrates(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    type: static
    root: public
    backend:
      enabled: true
      prefix: /api
      paths: [/ws]
      websocket: true
      exec: ./bin/server
      port: 20017
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sites[0].Backend != nil {
		t.Fatal("legacy backend should be cleared after migrate")
	}
	if len(cfg.Sites[0].Backends) != 1 {
		t.Fatalf("backends=%d", len(cfg.Sites[0].Backends))
	}
	b := cfg.Sites[0].Backends[0]
	if !b.Enabled || b.Port != 20017 || !b.WebSocket || b.ID != "api" {
		t.Fatalf("%+v", b)
	}
	prefs := b.BackendPathPrefixes()
	if len(prefs) != 2 || prefs[0] != "/api" || prefs[1] != "/ws" {
		t.Fatalf("%v", prefs)
	}
}

func TestParseMultipleBackends(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    type: static
    root: public
    backends:
      - id: api
        enabled: true
        prefix: /api
        exec: server.js
        port: 20001
        runtime:
          kind: node
          version: "22"
      - id: ssr
        enabled: true
        prefix: /ssr
        exec: ./server
        port: 20002
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sites[0].Backends) != 2 {
		t.Fatalf("%d", len(cfg.Sites[0].Backends))
	}
}

func TestBackendOnlyOnStatic(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: app.example.com
    type: app
    exec: ./bin/server
    backend:
      enabled: true
      port: 20001
`))
	if err == nil || !strings.Contains(err.Error(), "static") {
		t.Fatalf("want static-only error, got %v", err)
	}
}

func TestDuplicateBackendPorts(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: a.example.com
    type: static
    backends:
      - enabled: true
        prefix: /api
        exec: ./server
        port: 20010
  - name: b.example.com
    type: static
    backends:
      - enabled: true
        prefix: /api
        exec: ./server
        port: 20010
`))
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("want duplicate port error, got %v", err)
	}
}

func TestOverlappingBackendPaths(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    type: static
    backends:
      - id: a
        enabled: true
        prefix: /api
        exec: ./server
        port: 20001
      - id: b
        enabled: true
        prefix: /api/v2
        exec: ./server
        port: 20002
`))
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("want overlap error, got %v", err)
	}
}

func TestBackendNodeRuntime(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    type: static
    root: public
    backend:
      enabled: true
      prefix: /api
      exec: server.js
      port: 20019
      runtime:
        kind: node
        version: "22"
`))
	if err != nil {
		t.Fatal(err)
	}
	b := cfg.Sites[0].Backends[0]
	if !b.HasNodeRuntime() || b.Runtime.Version != "22" {
		t.Fatalf("%+v", b)
	}
}

func TestBackendNodeRuntimeRejectsRelativeCommand(t *testing.T) {
	err := config.ValidateSiteBackend(config.SiteTypeStatic, &config.SiteBackend{
		Enabled: true,
		Prefix:  "/api",
		Exec:    "server.js",
		Port:    20020,
		Runtime: &config.SiteRuntime{Kind: "node", Command: "relative/node"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBackendIDFromPrefix(t *testing.T) {
	if got := config.BackendIDFromPrefix("/back-api"); got != "back-api" {
		t.Fatalf("got %q", got)
	}
	if got := config.BackendIDFromPrefix("/"); got != "root" {
		t.Fatalf("got %q", got)
	}
	if got := config.BackendIDFromPrefix("/api"); got != "api" {
		t.Fatalf("got %q", got)
	}
}

func TestUniqueBackendID(t *testing.T) {
	taken := map[string]bool{"api": true, "api-2": true}
	got := config.UniqueBackendID("api", func(id string) bool { return taken[id] })
	if got != "api-3" {
		t.Fatalf("got %q", got)
	}
	got = config.UniqueBackendID("ssr", func(id string) bool { return taken[id] })
	if got != "ssr" {
		t.Fatalf("got %q", got)
	}
}

func TestCaddyPathMatchers(t *testing.T) {
	m := config.CaddyPathMatchers([]string{"/api", "/ws"})
	joined := strings.Join(m, ",")
	for _, want := range []string{"/api", "/api/*", "/ws", "/ws/*"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, m)
		}
	}
}
