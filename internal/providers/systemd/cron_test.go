package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func TestRenderCronUnits(t *testing.T) {
	units, err := systemd.RenderCronUnits(config.Site{
		Name:   "app.example.com",
		Type:   config.SiteTypeApp,
		Limits: config.Limits{MemoryMax: "64M", CPUQuota: "50%"},
	}, config.CronJob{
		Name:     "nightly-cleanup",
		Schedule: "0 3 * * *",
		Exec:     "./bin/cleanup",
	}, "Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	if units.ServiceName != "webor-cron-app-example-com-nightly-cleanup.service" {
		t.Fatalf("service=%q", units.ServiceName)
	}
	if units.TimerName != "webor-cron-app-example-com-nightly-cleanup.timer" {
		t.Fatalf("timer=%q", units.TimerName)
	}
	for _, want := range []string{
		"Type=oneshot",
		"User=" + sites.Username("app.example.com"),
		"ExecStart=" + sites.Root("app.example.com") + "/bin/cleanup",
		"MemoryMax=64M",
		"CPUQuota=50%",
		"RuntimeDirectory=webor/sites/app.example.com",
		"RuntimeDirectoryPreserve=yes",
	} {
		if !strings.Contains(units.Service, want) {
			t.Fatalf("service missing %q:\n%s", want, units.Service)
		}
	}
	for _, want := range []string{
		"OnCalendar=*-*-* 03:00:00 Asia/Kolkata",
		"Persistent=true",
		"Unit=" + units.ServiceName,
		"WantedBy=timers.target",
	} {
		if !strings.Contains(units.Timer, want) {
			t.Fatalf("timer missing %q:\n%s", want, units.Timer)
		}
	}
}

func TestRenderCronUnitsOneshotOnly(t *testing.T) {
	units, err := systemd.RenderCronUnits(config.Site{
		Name: "app.example.com",
		Type: config.SiteTypeApp,
	}, config.CronJob{
		Name: "seed",
		Exec: "./crons/seed/server",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if units.Timer != "" {
		t.Fatalf("expected empty timer for oneshot-only, got %q", units.Timer)
	}
	if !strings.Contains(units.Service, "Type=oneshot") {
		t.Fatalf("service:\n%s", units.Service)
	}
}
