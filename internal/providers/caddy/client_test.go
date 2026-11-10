package caddy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webooza/webooza/internal/providers/caddy"
)

func TestValidateJSON(t *testing.T) {
	if err := caddy.ValidateJSON([]byte(`{"admin":{}}`)); err != nil {
		t.Fatal(err)
	}
	if err := caddy.ValidateJSON([]byte(`[]`)); err == nil {
		t.Fatal("expected rejection of array")
	}
	if err := caddy.ValidateJSON([]byte(`{`)); err == nil {
		t.Fatal("expected rejection of broken JSON")
	}
}

func TestLoadSuccessAndFailure(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/load" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if strings.Contains(gotBody, `"bad"`) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"loading config: boom"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := caddy.NewClient(srv.URL, "")
	ctx := context.Background()
	if err := client.Load(ctx, []byte(`{"apps":{}}`)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"apps"`) {
		t.Fatalf("unexpected body %q", gotBody)
	}

	err := client.Load(ctx, []byte(`{"bad":true}`))
	if err == nil {
		t.Fatal("expected load error")
	}
	le, ok := err.(*caddy.LoadError)
	if !ok {
		t.Fatalf("want LoadError, got %T: %v", err, err)
	}
	if le.Status != 400 {
		t.Fatalf("status = %d", le.Status)
	}
}

func TestApplyProbeRollback(t *testing.T) {
	dir := t.TempDir()
	lastGood := filepath.Join(dir, "last-good.json")
	good := []byte(`{"apps":{"http":{"servers":{"srv":{}}}}}`)
	if err := os.WriteFile(lastGood, good, 0o600); err != nil {
		t.Fatal(err)
	}

	loads := 0
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" {
			loads++
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer admin.Close()

	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer probe.Close()

	client := caddy.NewClient(admin.URL, lastGood)
	err := client.Apply(context.Background(), []byte(`{"apps":{"http":{}}}`), caddy.ApplyOptions{
		Probe: &caddy.ProbeOptions{URL: probe.URL, ExpectStatus: 200},
	})
	if err == nil {
		t.Fatal("expected probe failure")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("expected rollback message, got %v", err)
	}
	if loads != 2 {
		t.Fatalf("loads = %d, want 2 (new + rollback)", loads)
	}
}

func TestApplyPersistsLastGood(t *testing.T) {
	dir := t.TempDir()
	lastGood := filepath.Join(dir, "last-good.json")

	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer admin.Close()

	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer probe.Close()

	cfg := []byte(`{"apps":{"http":{"servers":{"s":{}}}}}`)
	client := caddy.NewClient(admin.URL, lastGood)
	if err := client.Apply(context.Background(), cfg, caddy.ApplyOptions{
		Probe: &caddy.ProbeOptions{URL: probe.URL},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(lastGood)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(cfg) {
		t.Fatalf("last-good mismatch")
	}
}
