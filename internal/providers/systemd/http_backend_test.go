package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
)

func TestRenderHTTPBackendUnit(t *testing.T) {
	site := config.Site{
		Name: "example.com",
		Type: config.SiteTypeStatic,
		Backend: &config.SiteBackend{
			Enabled:   true,
			Prefix:    "/api",
			WebSocket: true,
			Exec:      "./bin/server",
			Port:      20017,
		},
	}
	name, body, err := systemd.RenderHTTPBackendUnit(site)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "webor-backend-") || !strings.HasSuffix(name, ".service") {
		t.Fatalf("unit name %q", name)
	}
	for _, want := range []string{
		"Environment=PORT=20017",
		"Environment=WBR_PORT=20017",
		"Environment=HOST=127.0.0.1",
		"User=wbr_example_com",
		"ExecStart=/srv/sites/example.com/bin/server",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in\n%s", want, body)
		}
	}
}
