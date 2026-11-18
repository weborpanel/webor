package systemd_test

import (
	"strings"
	"testing"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/providers/systemd"
	"github.com/webooza/webooza/internal/sites"
)

func TestRenderAppUnitsSocketActivated(t *testing.T) {
	units, err := systemd.RenderAppUnits(config.Site{
		Name:             "app.example.com",
		Type:             config.SiteTypeApp,
		Exec:             "./bin/server",
		SocketActivation: true,
		IdleTimeout:      "15m",
		EnvFile:          ".env",
		Limits:           config.Limits{MemoryMax: "256M"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if units.ServiceName != "webooza-app-app-example-com.service" {
		t.Fatalf("service name=%q", units.ServiceName)
	}
	if units.SocketName != "webooza-app-app-example-com.socket" {
		t.Fatalf("socket name=%q", units.SocketName)
	}
	for _, want := range []string{
		"User=" + sites.Username("app.example.com"),
		"ExecStart=" + sites.Root("app.example.com") + "/bin/server",
		"MemoryMax=256M",
		"Environment=WBZ_IDLE_TIMEOUT=15m",
		"EnvironmentFile=-" + sites.Root("app.example.com") + "/.env",
		"Requires=webooza-app-app-example-com.socket",
		"NoNewPrivileges=true",
		"Restart=no",
	} {
		if !strings.Contains(units.Service, want) {
			t.Fatalf("service missing %q:\n%s", want, units.Service)
		}
	}
	sockPath := sites.AppSocket("app.example.com")
	if !strings.Contains(units.Socket, "ListenStream="+sockPath) {
		t.Fatalf("socket missing ListenStream:\n%s", units.Socket)
	}
	if strings.Contains(units.Socket, "PartOf=") {
		t.Fatalf("socket must not PartOf service (breaks wake-on-request):\n%s", units.Socket)
	}
}

func TestRenderAppUnitsAlwaysOn(t *testing.T) {
	units, err := systemd.RenderAppUnits(config.Site{
		Name:             "api.example.com",
		Type:             config.SiteTypeApp,
		Exec:             "/usr/local/bin/api",
		SocketActivation: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if units.SocketName != "" || units.Socket != "" {
		t.Fatalf("expected no socket unit, got %+v", units)
	}
	if strings.Contains(units.Service, "Requires=") {
		t.Fatalf("always-on service should not Require socket:\n%s", units.Service)
	}
	if !strings.Contains(units.Service, "ExecStart=/usr/local/bin/api") {
		t.Fatalf("missing abs exec:\n%s", units.Service)
	}
}

func TestRenderAppUnitsRequiresExec(t *testing.T) {
	_, err := systemd.RenderAppUnits(config.Site{
		Name: "x.example.com",
		Type: config.SiteTypeApp,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
