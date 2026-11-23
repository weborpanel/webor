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

func TestPutSiteLimitsPresets(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	acct := &auth.Account{DB: db}
	tok, _, err := acct.EnsureSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(tok, "admin@example.com", "correct-horse"); err != nil {
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
			{Name: "limits.test", Type: config.SiteTypeStatic, Root: "public"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	srv := &api.Server{Account: acct, ConfigPath: cfgPath}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	req := httptest.NewRequest("PUT", "/api/sites/limits.test/limits", bytes.NewBufferString(
		`{"memory_max":"512M","cpu_quota":"100%","disk_quota":"2G"}`,
	))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	site, _, ok := cfg.FindSite("limits.test")
	if !ok {
		t.Fatal("site missing")
	}
	if site.Limits.MemoryMax != "512M" || site.Limits.CPUQuota != "100%" || site.Limits.DiskQuota != "2G" {
		t.Fatalf("limits=%+v", site.Limits)
	}

	req = httptest.NewRequest("PUT", "/api/sites/limits.test/limits", bytes.NewBufferString(
		`{"memory_max":"12X","cpu_quota":"","disk_quota":""}`,
	))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("want 400 for 12X, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("PUT", "/api/sites/limits.test/limits", bytes.NewBufferString(
		`{"memory_max":"","cpu_quota":"cpu","disk_quota":""}`,
	))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("want 400 for cpu, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("PUT", "/api/sites/limits.test/limits", bytes.NewBufferString(
		`{"memory_max":"","cpu_quota":"","disk_quota":"20G"}`,
	))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("want 400 for 20G, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/sites/limits.test/limits", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		OK      bool `json:"ok"`
		Options struct {
			Disk []struct {
				Value string `json:"value"`
			} `json:"disk"`
		} `json:"options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || !got.OK {
		t.Fatalf("body: %s err=%v", rec.Body.String(), err)
	}
	for _, o := range got.Options.Disk {
		if o.Value == "20G" {
			t.Fatal("panel must not offer 20G")
		}
	}
}
