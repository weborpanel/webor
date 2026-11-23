package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
)

func TestRenderJobUnitsLimits(t *testing.T) {
	units, err := systemd.RenderJobUnits(config.Site{
		Name:   "app.example.com",
		Type:   config.SiteTypeApp,
		Limits: config.Limits{MemoryMax: "128M", CPUQuota: "100%"},
	}, config.Job{Name: "worker", Exec: "./bin/worker"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MemoryMax=128M", "CPUQuota=100%", "Type=simple"} {
		if !strings.Contains(units.Service, want) {
			t.Fatalf("missing %q:\n%s", want, units.Service)
		}
	}
	if strings.Contains(units.Service, "CPUQuota=100%\nCPUQuota=") {
		t.Fatal("duplicate CPUQuota")
	}
}
