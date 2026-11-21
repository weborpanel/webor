package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/api"
	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/metrics"
)

func TestMetricsEndpoints(t *testing.T) {
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

	srv := &api.Server{Account: acct, ConfigPath: filepath.Join(dir, "config.yaml")}
	mux := http.NewServeMux()
	srv.MountSites(mux)
	srv.MountMetrics(mux)

	do := func(method, path, body string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if authed {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sess})
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	for _, p := range []string{"/api/overview", "/api/metrics/host", "/api/sites/example.com/metrics", "/api/audit"} {
		if rec := do("GET", p, "", false); rec.Code != 401 {
			t.Fatalf("%s unauthenticated: %d", p, rec.Code)
		}
	}

	if rec := do("POST", "/api/sites", `{"name":"example.com","type":"static"}`, true); rec.Code != 200 {
		t.Fatalf("add site: %d %s", rec.Code, rec.Body.String())
	}

	st := &metrics.Store{DB: db}
	ts := time.Now().Unix() / metrics.ResRaw * metrics.ResRaw
	if err := st.InsertHost(metrics.ResRaw, metrics.HostRow{TS: ts, CPUPct: 12, MemUsed: 1, MemTotal: 2}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertSites(metrics.ResRaw, []metrics.SiteRow{{TS: ts, Site: "example.com", CPUPct: 3, MemBytes: 10}}); err != nil {
		t.Fatal(err)
	}

	rec := do("GET", "/api/metrics/host?range=24h", "", true)
	if rec.Code != 200 {
		t.Fatalf("host metrics: %d %s", rec.Code, rec.Body.String())
	}
	var hostResp struct {
		OK      bool `json:"ok"`
		Metrics struct {
			Step   int64                `json:"step"`
			T      []int64              `json:"t"`
			Series map[string][]float64 `json:"series"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &hostResp); err != nil {
		t.Fatal(err)
	}
	if !hostResp.OK || hostResp.Metrics.Step != 300 || len(hostResp.Metrics.T) != 1 || hostResp.Metrics.Series["cpu_pct"][0] != 12 {
		t.Fatalf("host metrics body: %s", rec.Body.String())
	}

	if rec := do("GET", "/api/metrics/host?range=30d", "", true); rec.Code != 400 {
		t.Fatalf("bad range: %d", rec.Code)
	}
	if rec := do("GET", "/api/sites/missing.example/metrics", "", true); rec.Code != 404 {
		t.Fatalf("missing site metrics: %d", rec.Code)
	}
	rec = do("GET", "/api/sites/example.com/metrics?range=1h", "", true)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"mem_bytes":[10]`)) {
		t.Fatalf("site metrics: %d %s", rec.Code, rec.Body.String())
	}

	rec = do("GET", "/api/overview", "", true)
	if rec.Code != 200 {
		t.Fatalf("overview: %d %s", rec.Code, rec.Body.String())
	}
	var ov struct {
		OK   bool `json:"ok"`
		Host struct {
			MemTotal int64 `json:"mem_total"`
			CPUCount int   `json:"cpu_count"`
		} `json:"host"`
		Sites []struct {
			Name  string `json:"name"`
			Spark *struct {
				T []int64 `json:"t"`
			} `json:"spark"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ov); err != nil {
		t.Fatal(err)
	}
	if !ov.OK || ov.Host.CPUCount < 1 || len(ov.Sites) != 1 || ov.Sites[0].Name != "example.com" {
		t.Fatalf("overview body: %s", rec.Body.String())
	}
	if ov.Sites[0].Spark == nil || len(ov.Sites[0].Spark.T) != 1 {
		t.Fatalf("overview spark: %s", rec.Body.String())
	}

	lg := &audit.Logger{DB: db}
	if err := lg.Append("panel", "AddSite", map[string]string{"domain": "example.com"}, "ok"); err != nil {
		t.Fatal(err)
	}
	rec = do("GET", "/api/audit?limit=10", "", true)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"operation":"AddSite"`)) {
		t.Fatalf("audit: %d %s", rec.Code, rec.Body.String())
	}
}
