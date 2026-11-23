package sites_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/sites"
)

func TestDefaultIndexHTML(t *testing.T) {
	html := sites.DefaultIndexHTML("Example.COM")
	for _, want := range []string{
		`data-webor-seed="static-v2"`,
		"example.com",
		"You're live",
		"Files",
		"Fraunces",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in index html", want)
		}
	}
	if strings.Contains(html, "<script") {
		t.Fatal("seed page must stay script-free")
	}
	escaped := sites.DefaultIndexHTML(`<img src=x onerror=alert(1)>.test`)
	if strings.Contains(escaped, "<img") {
		t.Fatal("domain must be HTML-escaped")
	}
	if !strings.Contains(escaped, "&lt;img") {
		t.Fatal("expected escaped domain markup")
	}
}
