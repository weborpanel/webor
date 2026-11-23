package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func TestRenderHTTPBackendUnit(t *testing.T) {
	backend := config.SiteBackend{
		ID:        "api",
		Enabled:   true,
		Prefix:    "/api",
		WebSocket: true,
		Exec:      "./server",
		Port:      20017,
	}
	site := config.Site{
		Name:     "example.com",
		Type:     config.SiteTypeStatic,
		Backends: []config.SiteBackend{backend},
	}
	name, body, err := systemd.RenderHTTPBackendUnit(site, backend)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "webor-backend-") || !strings.Contains(name, "-api.") || !strings.HasSuffix(name, ".service") {
		t.Fatalf("unit name %q", name)
	}
	apiRoot := sites.APIs("example.com", "api")
	for _, want := range []string{
		"Environment=PORT=20017",
		"Environment=WBR_PORT=20017",
		"Environment=HOST=127.0.0.1",
		"User=wbr_example_com",
		"WorkingDirectory=" + apiRoot,
		"RuntimeDirectory=webor/sites/example.com",
		"RuntimeDirectoryPreserve=yes",
		"ExecStart=" + apiRoot + "/server",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in\n%s", want, body)
		}
	}
}

func TestRenderHTTPBackendUnitNode(t *testing.T) {
	backend := config.SiteBackend{
		ID:      "api",
		Enabled: true,
		Prefix:  "/api",
		Exec:    "server.js",
		Port:    20018,
		Runtime: &config.SiteRuntime{
			Kind:    config.RuntimeKindNode,
			Command: "/usr/bin/node",
		},
	}
	site := config.Site{
		Name:     "example.com",
		Type:     config.SiteTypeStatic,
		Backends: []config.SiteBackend{backend},
	}
	_, body, err := systemd.RenderHTTPBackendUnit(site, backend)
	if err != nil {
		t.Fatal(err)
	}
	want := "ExecStart=/usr/bin/node " + sites.APIs("example.com", "api") + "/server.js"
	if !strings.Contains(body, want) {
		t.Fatalf("%s", body)
	}
	if !strings.Contains(body, "Environment=PATH=/usr/bin:") {
		t.Fatalf("%s", body)
	}
}
