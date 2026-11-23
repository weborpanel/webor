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

func TestCronCreatePersistsConfig(t *testing.T) {
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
		Sites:   []config.Site{{Name: "cron.test", Type: config.SiteTypeStatic, Root: "public"}},
	}); err != nil {
		t.Fatal(err)
	}

	srv := &api.Server{Account: acct, ConfigPath: cfgPath}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	body, _ := json.Marshal(map[string]any{"name": "seed", "exec": "./crons/seed/server"})
	req := httptest.NewRequest(http.MethodPost, "/api/sites/cron.test/cron", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// Helper install may fail without root/systemd; config must still persist.
	if rec.Code != 200 && rec.Code != 500 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	site, _, ok := loaded.FindSite("cron.test")
	if !ok {
		t.Fatal("site missing")
	}
	if len(site.Cron) != 1 || site.Cron[0].Name != "seed" {
		t.Fatalf("cron=%#v", site.Cron)
	}
	if site.Cron[0].Schedule != "" {
		t.Fatalf("expected empty schedule, got %q", site.Cron[0].Schedule)
	}
}

func TestProcessCreatePersistsConfig(t *testing.T) {
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
		Sites:   []config.Site{{Name: "proc.test", Type: config.SiteTypeStatic, Root: "public"}},
	}); err != nil {
		t.Fatal(err)
	}

	srv := &api.Server{Account: acct, ConfigPath: cfgPath}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	body, _ := json.Marshal(map[string]any{"name": "worker", "start": false})
	req := httptest.NewRequest(http.MethodPost, "/api/sites/proc.test/processes", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 && rec.Code != 500 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	site, _, ok := loaded.FindSite("proc.test")
	if !ok {
		t.Fatal("site missing")
	}
	if len(site.Jobs) != 1 || site.Jobs[0].Name != "worker" {
		t.Fatalf("jobs=%#v", site.Jobs)
	}
}
