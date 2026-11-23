package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/api"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
)

func TestDeployTokenAndCIProbe(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	acct := &auth.Account{DB: db}
	setup, _, err := acct.EnsureSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(setup, "admin@example.com", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	sess, _, err := acct.CreateSession("127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{
		Version: 1,
		Sites: []config.Site{
			{Name: "app.example.com", Type: config.SiteTypeApp, Exec: "server.js", Runtime: &config.SiteRuntime{Kind: "node", Version: "22"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	srv := &api.Server{Account: acct, ConfigPath: cfgPath}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	// Token cannot create sites
	req := httptest.NewRequest("POST", "/api/sites", bytes.NewBufferString(`{"name":"evil.test","type":"static"}`))
	req.Header.Set("Authorization", "Bearer unused")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("create site with bearer: %d %s", rec.Code, rec.Body.String())
	}

	// Mint token via session
	req = httptest.NewRequest("POST", "/api/sites/app.example.com/tokens", bytes.NewBufferString(`{"name":"ci"}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create token: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || !created.OK || created.Token == "" {
		t.Fatalf("create body: %s", rec.Body.String())
	}

	// CI probe with token
	req = httptest.NewRequest("GET", "/api/sites/app.example.com/ci", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("ci: %d %s", rec.Code, rec.Body.String())
	}
	var ci struct {
		OK            bool   `json:"ok"`
		Type          string `json:"type"`
		SuggestedKind string `json:"suggested_kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ci); err != nil || !ci.OK || ci.Type != "app" || ci.SuggestedKind != "node" {
		t.Fatalf("ci body: %s", rec.Body.String())
	}

	// Missing site
	req = httptest.NewRequest("GET", "/api/sites/missing.example.com/ci", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		// token bound to app.example.com → 403 for other site name
		t.Fatalf("missing other site: want 403 got %d %s", rec.Code, rec.Body.String())
	}

	// Token for correct site but site deleted from config → 404
	if err := config.Save(cfgPath, &config.Config{Version: 1, Sites: nil}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest("GET", "/api/sites/app.example.com/ci", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("deleted site: want 404 got %d %s", rec.Code, rec.Body.String())
	}
}
