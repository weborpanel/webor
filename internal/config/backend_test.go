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

func TestParseBackend(t *testing.T) {
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
	b := cfg.Sites[0].Backend
	if b == nil || !b.Enabled || b.Port != 20017 || !b.WebSocket {
		t.Fatalf("%+v", b)
	}
	prefs := b.BackendPathPrefixes()
	if len(prefs) != 2 || prefs[0] != "/api" || prefs[1] != "/ws" {
		t.Fatalf("%v", prefs)
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
    backend:
      enabled: true
      prefix: /api
      exec: ./bin/server
      port: 20010
  - name: b.example.com
    type: static
    backend:
      enabled: true
      prefix: /api
      exec: ./bin/server
      port: 20010
`))
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("want duplicate port error, got %v", err)
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
