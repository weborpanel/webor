package caddy_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/caddy"
	"github.com/weborpanel/webor/internal/sites"
)

func TestRenderStaticSkipsApps(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    aliases: [www.example.com]
    type: static
    root: public
  - name: api.example.com
    type: app
    exec: ./bin/server
`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := caddy.RenderStatic(cfg, caddy.RenderOptions{DisableHTTPS: true, HTTPListen: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "/srv/sites/example.com/public") {
		t.Fatalf("missing file root: %s", s)
	}
	if !strings.Contains(s, "www.example.com") {
		t.Fatal("missing alias host")
	}
	if strings.Contains(s, "api.example.com") {
		t.Fatal("app site should be skipped by RenderStatic")
	}
	if !strings.Contains(s, `"disable": true`) && !strings.Contains(s, `"disable":true`) {
		t.Fatalf("expected automatic_https disabled: %s", s)
	}
}

func TestRenderIncludesAppUnixProxy(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: example.com
    type: static
    root: public
  - name: app.example.com
    type: app
    exec: ./bin/server
    socket_activation: true
`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := caddy.Render(cfg, caddy.RenderOptions{DisableHTTPS: true, HTTPListen: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "/srv/sites/example.com/public") {
		t.Fatalf("missing static root: %s", s)
	}
	wantDial := "unix/" + sites.AppSocket("app.example.com")
	if !strings.Contains(s, wantDial) {
		t.Fatalf("missing unix dial %q in %s", wantDial, s)
	}
	if !strings.Contains(s, "reverse_proxy") {
		t.Fatal("missing reverse_proxy handler")
	}
	if !strings.Contains(s, "app.example.com") {
		t.Fatal("missing app host")
	}
}
