package api_test

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/api"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/dnscheck"
)

func setupPanelTest(t *testing.T) (sess string, srv *api.Server, mux *http.ServeMux, cfgPath string) {
	t.Helper()
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	acct := &auth.Account{DB: db}
	tok, _, err := acct.EnsureSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(tok, "admin@example.com", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	sess, _, err = acct.CreateSession("127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{
		Version: 1,
		Sites:   []config.Site{{Name: "example.com", Type: config.SiteTypeStatic, Root: "public"}},
	}); err != nil {
		t.Fatal(err)
	}
	srv = &api.Server{
		Account:     acct,
		ConfigPath:  cfgPath,
		PanelListen: "0.0.0.0:21215",
		HTTPListen:  ":8080",
		DisableHTTPS: true,
		CaddyConfigPath: filepath.Join(dir, "caddy.json"),
	}
	mux = http.NewServeMux()
	srv.MountSites(mux)
	srv.MountPanel(mux)
	return sess, srv, mux, cfgPath
}

func TestPanelDomainPutDelete(t *testing.T) {
	sess, _, mux, cfgPath := setupPanelTest(t)

	req := httptest.NewRequest("PUT", "/api/panel/domain", bytes.NewBufferString(`{"parent":"example.com","label":"panel"}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	var putBody struct {
		OK     bool   `json:"ok"`
		Domain string `json:"domain"`
		Parent string `json:"parent_site"`
		Label  string `json:"label"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &putBody); err != nil {
		t.Fatal(err)
	}
	if !putBody.OK || putBody.Domain != "panel.example.com" || putBody.Parent != "example.com" || putBody.Label != "panel" {
		t.Fatalf("put body: %+v", putBody)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Panel.Domain != "panel.example.com" || cfg.Panel.Port != 21215 {
		t.Fatalf("saved panel: %+v", cfg.Panel)
	}

	req = httptest.NewRequest("GET", "/api/panel", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}

	// Collision with existing site name reserved for panel.
	req = httptest.NewRequest("POST", "/api/sites/example.com/subdomains", bytes.NewBufferString(`{"name":"panel.example.com"}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 409 {
		t.Fatalf("subdomain collide: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("DELETE", "/api/panel/domain", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	cfg, err = config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Panel.Domain != "" {
		t.Fatalf("domain still set: %q", cfg.Panel.Domain)
	}
}

func TestPanelDomainCollisionWithSite(t *testing.T) {
	sess, _, mux, cfgPath := setupPanelTest(t)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Sites = append(cfg.Sites, config.Site{
		Name: "app.example.com", Parent: "example.com", Type: config.SiteTypeStatic, Root: "public",
	})
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("PUT", "/api/panel/domain", bytes.NewBufferString(`{"parent":"example.com","label":"app"}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 409 {
		t.Fatalf("want 409, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPanelDNS(t *testing.T) {
	sess, srv, mux, cfgPath := setupPanelTest(t)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Panel.Domain = "panel.example.com"
	cfg.Panel.Port = 21215
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	srv.DNS = &dnscheck.Fake{
		A: map[string][]net.IP{
			"panel.example.com": {net.ParseIP("203.0.113.10")},
		},
		NS: map[string][]string{
			"example.com": {"ns1.example.net"},
		},
		CAA: map[string][]dnscheck.CAA{
			"example.com": {{Tag: "issue", Value: "letsencrypt.org"}},
		},
	}
	srv.ExpectedA = []string{"203.0.113.10"}

	req := httptest.NewRequest("GET", "/api/panel/dns", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("dns: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK                  bool   `json:"ok"`
		Domain              string `json:"domain"`
		CanIssueCertificate bool   `json:"can_issue_certificate"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.Domain != "panel.example.com" {
		t.Fatalf("dns body: %+v", body)
	}
}
