package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
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
	if units.ServiceName != "webor-app-app-example-com.service" {
		t.Fatalf("service name=%q", units.ServiceName)
	}
	if units.SocketName != "webor-app-app-example-com.socket" {
		t.Fatalf("socket name=%q", units.SocketName)
	}
	for _, want := range []string{
		"User=" + sites.Username("app.example.com"),
		"ExecStart=" + sites.Root("app.example.com") + "/bin/server",
		"MemoryMax=256M",
		"Environment=WBR_IDLE_TIMEOUT=15m",
		"EnvironmentFile=-" + sites.Root("app.example.com") + "/.env",
		"Requires=webor-app-app-example-com.socket",
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

func TestRenderAppUnitsSocketProxy(t *testing.T) {
	if _, err := systemd.FindSocketProxyd(); err != nil {
		t.Skip("systemd-socket-proxyd not installed")
	}
	units, err := systemd.RenderAppUnits(config.Site{
		Name:             "app.example.com",
		Type:             config.SiteTypeApp,
		Exec:             "./bin/server",
		SocketActivation: true,
		SocketProxy:      true,
		IdleTimeout:      "3s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if units.BackendServiceName != "webor-app-app-example-com-backend.service" {
		t.Fatalf("backend=%q", units.BackendServiceName)
	}
	if !strings.Contains(units.Service, "systemd-socket-proxyd") {
		t.Fatalf("proxy service missing proxyd:\n%s", units.Service)
	}
	if !strings.Contains(units.Service, "--exit-idle-time=3s") {
		t.Fatalf("proxy missing idle flag:\n%s", units.Service)
	}
	backendSock := sites.AppBackendSocket("app.example.com")
	if !strings.Contains(units.BackendService, "Environment=WBR_SOCK="+backendSock) {
		t.Fatalf("backend missing WBR_SOCK:\n%s", units.BackendService)
	}
	if !strings.Contains(units.BackendService, "Restart=on-failure") {
		t.Fatalf("backend should stay up:\n%s", units.BackendService)
	}
}
