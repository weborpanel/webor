package systemd

import (
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/logging"
)

const (
	BackupServiceUnit = "webor-backup.service"
	BackupTimerUnit   = "webor-backup.timer"
)

// RenderBackupUnits returns service+timer bodies for scheduled BackupRun.
// Empty bodies mean the timer should be removed (backup disabled or no schedule).
func RenderBackupUnits(cfg *config.Config) (unit, timer, bodyUnit, bodyTimer string, err error) {
	unit = BackupServiceUnit
	timer = BackupTimerUnit
	if cfg == nil || cfg.Backup == nil || !cfg.Backup.Enabled || strings.TrimSpace(cfg.Backup.Schedule) == "" {
		return unit, timer, "", "", nil
	}
	cals, err := CronToOnCalendar(cfg.Backup.Schedule)
	if err != nil {
		return "", "", "", "", fmt.Errorf("backup.schedule: %w", err)
	}
	cals = WithTimezone(cals, config.EffectiveTimezone(cfg))

	logFile := logging.HostCurrentPath(logging.StreamBackup)
	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webor full-system backup (restic)\n")
	fmt.Fprintf(&svc, "After=network-online.target webor-helper.service\n")
	fmt.Fprintf(&svc, "Wants=network-online.target\n")
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=oneshot\n")
	fmt.Fprintf(&svc, "ExecStart=/usr/local/bin/wbr backup run\n")
	fmt.Fprintf(&svc, "StandardOutput=append:%s\n", logFile)
	fmt.Fprintf(&svc, "StandardError=append:%s\n", logFile)
	fmt.Fprintf(&svc, "Nice=10\n")
	fmt.Fprintf(&svc, "IOSchedulingClass=best-effort\n")
	fmt.Fprintf(&svc, "IOSchedulingPriority=7\n")

	var tim strings.Builder
	fmt.Fprintf(&tim, "[Unit]\n")
	fmt.Fprintf(&tim, "Description=Webor backup timer\n")
	fmt.Fprintf(&tim, "Requires=%s\n", BackupServiceUnit)
	fmt.Fprintf(&tim, "\n[Timer]\n")
	for _, cal := range cals {
		fmt.Fprintf(&tim, "OnCalendar=%s\n", cal)
	}
	fmt.Fprintf(&tim, "Persistent=true\n")
	fmt.Fprintf(&tim, "Unit=%s\n", BackupServiceUnit)
	fmt.Fprintf(&tim, "\n[Install]\n")
	fmt.Fprintf(&tim, "WantedBy=timers.target\n")

	return unit, timer, svc.String(), tim.String(), nil
}
