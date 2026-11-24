package systemd

import (
	"fmt"
	"strings"
)

const (
	LogrotateServiceUnit = "webor-logrotate.service"
	LogrotateTimerUnit   = "webor-logrotate.timer"
)

// RenderLogrotateUnits returns the oneshot service + hourly timer that rotates Webor log files.
func RenderLogrotateUnits() (unit, timer, bodyUnit, bodyTimer string) {
	unit = LogrotateServiceUnit
	timer = LogrotateTimerUnit

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webor log file rotation\n")
	fmt.Fprintf(&svc, "After=webor-helper.service\n")
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=oneshot\n")
	fmt.Fprintf(&svc, "ExecStart=/usr/local/bin/wbr logs rotate\n")
	fmt.Fprintf(&svc, "Nice=10\n")

	var tim strings.Builder
	fmt.Fprintf(&tim, "[Unit]\n")
	fmt.Fprintf(&tim, "Description=Webor log rotation timer\n")
	fmt.Fprintf(&tim, "Requires=%s\n", LogrotateServiceUnit)
	fmt.Fprintf(&tim, "\n[Timer]\n")
	fmt.Fprintf(&tim, "OnCalendar=hourly\n")
	fmt.Fprintf(&tim, "Persistent=true\n")
	fmt.Fprintf(&tim, "Unit=%s\n", LogrotateServiceUnit)
	fmt.Fprintf(&tim, "\n[Install]\n")
	fmt.Fprintf(&tim, "WantedBy=timers.target\n")

	return unit, timer, svc.String(), tim.String()
}
