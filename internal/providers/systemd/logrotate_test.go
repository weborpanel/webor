package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/providers/systemd"
)

func TestRenderLogrotateUnits(t *testing.T) {
	unit, timer, bodyU, bodyT := systemd.RenderLogrotateUnits()
	if unit != "webor-logrotate.service" || timer != "webor-logrotate.timer" {
		t.Fatalf("%s %s", unit, timer)
	}
	if !strings.Contains(bodyU, "wbr logs rotate") {
		t.Fatalf("%s", bodyU)
	}
	if !strings.Contains(bodyT, "OnCalendar=hourly") {
		t.Fatalf("%s", bodyT)
	}
}
