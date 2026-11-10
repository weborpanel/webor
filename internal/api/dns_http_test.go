package api_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/webooza/webooza/internal/api"
	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/dnscheck"
)

func TestSiteDNSChecklist(t *testing.T) {
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
	cfg := &config.Config{
		Version: 1,
		Sites:   []config.Site{{Name: "example.com", Type: config.SiteTypeStatic}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	fake := &dnscheck.Fake{
		A: map[string][]net.IP{
			"example.com": {net.ParseIP("203.0.113.10")},
		},
		NS: map[string][]string{
			"example.com": {"ns1.example.net"},
		},
		CAA: map[string][]dnscheck.CAA{
			"example.com": {{Tag: "issue", Value: "letsencrypt.org"}},
		},
	}
	srv := &api.Server{
		Account:    acct,
		ConfigPath: cfgPath,
		DNS:        fake,
		ExpectedA:  []string{"203.0.113.10"},
	}
	mux := http.NewServeMux()
	srv.MountSites(mux)

	req := httptest.NewRequest("GET", "/api/sites/example.com/dns", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK                  bool `json:"ok"`
		CanIssueCertificate bool `json:"can_issue_certificate"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK {
		t.Fatal(rec.Body.String())
	}
}
