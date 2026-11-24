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

func TestRenderStaticWithHTTPBackend(t *testing.T) {
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
	raw, err := caddy.Render(cfg, caddy.RenderOptions{DisableHTTPS: true, HTTPListen: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "127.0.0.1:20017") {
		t.Fatalf("missing dial: %s", s)
	}
	if !strings.Contains(s, "/api") || !strings.Contains(s, "/ws") {
		t.Fatalf("missing path matchers: %s", s)
	}
	if !strings.Contains(s, `"flush_interval": -1`) && !strings.Contains(s, `"flush_interval":-1`) {
		t.Fatalf("expected websocket flush_interval: %s", s)
	}
	if !strings.Contains(s, "/srv/sites/example.com/public") {
		t.Fatalf("missing file_server root: %s", s)
	}
}

func TestRenderStaticBackendWholeHost(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
sites:
  - name: api.example.com
    parent: example.com
    type: static
    root: public
    backend:
      enabled: true
      prefix: /
      websocket: true
      exec: ./bin/server
      port: 20018
  - name: example.com
    type: static
    root: public
`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := caddy.Render(cfg, caddy.RenderOptions{DisableHTTPS: true, HTTPListen: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "127.0.0.1:20018") {
		t.Fatalf("missing dial: %s", s)
	}
	if strings.Contains(s, "/srv/sites/api.example.com/public") {
		t.Fatalf("whole-host backend should skip file_server: %s", s)
	}
	if !strings.Contains(s, "/srv/sites/example.com/public") {
		t.Fatalf("parent static file_server missing: %s", s)
	}
}

func TestRenderPanelDomainOnly(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  domain: panel.example.com
  port: 21215
sites: []
`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := caddy.Render(cfg, caddy.RenderOptions{DisableHTTPS: true, HTTPListen: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "panel.example.com") {
		t.Fatalf("missing panel host: %s", s)
	}
	if !strings.Contains(s, "127.0.0.1:21215") {
		t.Fatalf("missing panel dial: %s", s)
	}
}

func TestRenderPanelDomainWithSite(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  domain: cp.example.com
  port: 0
sites:
  - name: example.com
    type: static
    root: public
`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := caddy.Render(cfg, caddy.RenderOptions{
		DisableHTTPS:  true,
		HTTPListen:    ":8080",
		PanelUpstream: "127.0.0.1:30964",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "cp.example.com") || !strings.Contains(s, "127.0.0.1:30964") {
		t.Fatalf("missing panel route: %s", s)
	}
	if !strings.Contains(s, "/srv/sites/example.com/public") {
		t.Fatalf("missing site: %s", s)
	}
}

func TestRenderRejectsPanelDomainCollision(t *testing.T) {
	_, err := config.Parse([]byte(`
version: 1
panel:
  domain: example.com
  port: 21215
sites:
  - name: example.com
    type: static
    root: public
`))
	if err == nil {
		t.Fatal("expected panel.domain collision with site")
	}
}

func TestPanelDial(t *testing.T) {
	d, err := caddy.PanelDial("0.0.0.0:21215", 0)
	if err != nil || d != "127.0.0.1:21215" {
		t.Fatalf("got %q %v", d, err)
	}
	d, err = caddy.PanelDial(":8787", 0)
	if err != nil || d != "127.0.0.1:8787" {
		t.Fatalf("got %q %v", d, err)
	}
	d, err = caddy.PanelDial("", 30964)
	if err != nil || d != "127.0.0.1:30964" {
		t.Fatalf("got %q %v", d, err)
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
	if !strings.Contains(s, "/srv/sites/example.com/logs/access.log") {
		t.Fatalf("missing static access log path: %s", s)
	}
	if !strings.Contains(s, "/srv/sites/app.example.com/logs/access.log") {
		t.Fatalf("missing app access log path: %s", s)
	}
	if !strings.Contains(s, `"logging"`) {
		t.Fatal("missing top-level logging block")
	}
}
