package ui_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/weborpanel/webor/internal/ui"
)

func TestHandlerServesIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ui.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Body.String(); got == "" {
		t.Fatal("empty body")
	}
}

func TestHandlerSPAFallback(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sites/example.com", nil)
	ui.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
