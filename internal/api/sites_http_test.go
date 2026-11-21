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

func TestSitesCRUD(t *testing.T) {
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
	srv := &api.Server{Account: acct, ConfigPath: cfgPath}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	req := httptest.NewRequest("POST", "/api/sites", bytes.NewBufferString(`{"name":"example.com","type":"static"}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/sites", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		OK    bool `json:"ok"`
		Sites []struct {
			Name string `json:"name"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || !listed.OK || len(listed.Sites) != 1 {
		t.Fatalf("list body: %s err=%v", rec.Body.String(), err)
	}

	req = httptest.NewRequest("DELETE", "/api/sites/example.com?delete_data=1", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteParentRemovesChildren(t *testing.T) {
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
			{Name: "parent-mdb.test", Type: config.SiteTypeStatic, Root: "public"},
			{Name: "app.parent-mdb.test", Parent: "parent-mdb.test", Type: config.SiteTypeStatic, Root: "public"},
			{Name: "other.example.com", Type: config.SiteTypeStatic, Root: "public"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	srv := &api.Server{Account: acct, ConfigPath: cfgPath}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	req := httptest.NewRequest("DELETE", "/api/sites/parent-mdb.test?delete_data=1", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete parent: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		OK              bool     `json:"ok"`
		RemovedChildren []string `json:"removed_children"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || !out.OK {
		t.Fatalf("body: %s err=%v", rec.Body.String(), err)
	}
	if len(out.RemovedChildren) != 1 || out.RemovedChildren[0] != "app.parent-mdb.test" {
		t.Fatalf("removed_children: %#v", out.RemovedChildren)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sites) != 1 || cfg.Sites[0].Name != "other.example.com" {
		t.Fatalf("remaining sites: %+v", cfg.Sites)
	}
}

func TestSitesRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := &api.Server{Account: &auth.Account{DB: db}, ConfigPath: filepath.Join(dir, "c.yaml")}
	mux := http.NewServeMux()
	srv.MountSites(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/sites", nil))
	if rec.Code != 401 {
		t.Fatalf("got %d", rec.Code)
	}
}
