package systemd_test

import (
	"strings"
	"testing"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/providers/systemd"
	"github.com/webooza/webooza/internal/sites"
)

func TestRenderCronUnits(t *testing.T) {
	units, err := systemd.RenderCronUnits(config.Site{
		Name:   "app.example.com",
		Type:   config.SiteTypeApp,
		Limits: config.Limits{MemoryMax: "64M"},
	}, config.CronJob{
		Name:     "nightly-cleanup",
		Schedule: "0 3 * * *",
		Exec:     "./bin/cleanup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if units.ServiceName != "webooza-cron-app-example-com-nightly-cleanup.service" {
		t.Fatalf("service=%q", units.ServiceName)
	}
	if units.TimerName != "webooza-cron-app-example-com-nightly-cleanup.timer" {
		t.Fatalf("timer=%q", units.TimerName)
	}
	for _, want := range []string{
		"Type=oneshot",
		"User=" + sites.Username("app.example.com"),
		"ExecStart=" + sites.Root("app.example.com") + "/bin/cleanup",
		"MemoryMax=64M",
	} {
		if !strings.Contains(units.Service, want) {
			t.Fatalf("service missing %q:\n%s", want, units.Service)
		}
	}
	for _, want := range []string{
		"OnCalendar=*-*-* 03:00:00",
		"Persistent=true",
		"Unit=" + units.ServiceName,
		"WantedBy=timers.target",
	} {
		if !strings.Contains(units.Timer, want) {
			t.Fatalf("timer missing %q:\n%s", want, units.Timer)
		}
	}
}
