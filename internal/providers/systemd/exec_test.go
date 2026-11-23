package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func TestResolveAppExecRawBinary(t *testing.T) {
	got, err := systemd.ResolveAppExec(config.Site{
		Name: "api.example.com",
		Type: config.SiteTypeApp,
		Exec: "./bin/server",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := sites.Root("api.example.com") + "/bin/server"
	if got.ExecStart != want {
		t.Fatalf("got %q want %q", got.ExecStart, want)
	}
}

func TestResolveAppExecNode(t *testing.T) {
	got, err := systemd.ResolveAppExec(config.Site{
		Name: "app.example.com",
		Type: config.SiteTypeApp,
		Exec: "server.js",
		Runtime: &config.SiteRuntime{
			Kind:    config.RuntimeKindNode,
			Version: "22",
		},
	}, "/var/lib/webor/runtimes/node/v22.14.0/bin/node")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.ExecStart, "/var/lib/webor/runtimes/node/v22.14.0/bin/node ") {
		t.Fatalf("%q", got.ExecStart)
	}
	if !strings.Contains(got.ExecStart, sites.Root("app.example.com")+"/server.js") {
		t.Fatalf("%q", got.ExecStart)
	}
	if got.PathEnv == "" {
		t.Fatal("expected PATH env")
	}
}

func TestResolveBackendExecBinaryArgs(t *testing.T) {
	backend := config.SiteBackend{
		ID:      "api",
		Enabled: true,
		Exec:    "./server --verbose",
		Port:    20022,
	}
	got, err := systemd.ResolveBackendExec(config.Site{
		Name:     "static.example.com",
		Type:     config.SiteTypeStatic,
		Backends: []config.SiteBackend{backend},
	}, backend, "")
	if err != nil {
		t.Fatal(err)
	}
	want := sites.APIs("static.example.com", "api") + "/server --verbose"
	if got.ExecStart != want {
		t.Fatalf("got %q want %q", got.ExecStart, want)
	}
}

func TestResolveBackendExecNode(t *testing.T) {
	backend := config.SiteBackend{
		ID:      "api",
		Enabled: true,
		Exec:    "server.js",
		Port:    20021,
		Runtime: &config.SiteRuntime{
			Kind:    config.RuntimeKindNode,
			Version: "22",
		},
	}
	got, err := systemd.ResolveBackendExec(config.Site{
		Name:     "static.example.com",
		Type:     config.SiteTypeStatic,
		Backends: []config.SiteBackend{backend},
	}, backend, "/var/lib/webor/runtimes/node/v22.14.0/bin/node")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.ExecStart, "/var/lib/webor/runtimes/node/v22.14.0/bin/node ") {
		t.Fatalf("%q", got.ExecStart)
	}
	if !strings.Contains(got.ExecStart, sites.APIs("static.example.com", "api")+"/server.js") {
		t.Fatalf("%q", got.ExecStart)
	}
}

func TestRenderAppUnitsWithNodePATH(t *testing.T) {
	// Custom command avoids needing installed managed node on disk.
	units, err := systemd.RenderAppUnits(config.Site{
		Name: "app.example.com",
		Type: config.SiteTypeApp,
		Exec: "server.js",
		Runtime: &config.SiteRuntime{
			Kind:    config.RuntimeKindNode,
			Command: "/usr/bin/node",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(units.Service, "ExecStart=/usr/bin/node ") {
		t.Fatalf("%s", units.Service)
	}
	if !strings.Contains(units.Service, "Environment=PATH=/usr/bin:") {
		t.Fatalf("%s", units.Service)
	}
}
