package helper

import (
	"fmt"
	"os/exec"
	"strings"
)

// SystemctlDaemonReload runs systemctl daemon-reload.
func (o *Ops) SystemctlDaemonReload() error {
	cmd := exec.Command("systemctl", "daemon-reload")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SystemctlEnableNow enables and starts a unit (socket or service).
func (o *Ops) SystemctlEnableNow(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return fmt.Errorf("empty unit name")
	}
	cmd := exec.Command("systemctl", "enable", "--now", unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enable --now %s: %w (%s)", unit, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SystemctlStop stops a unit if it exists; missing units are ignored.
func (o *Ops) SystemctlStop(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return fmt.Errorf("empty unit name")
	}
	cmd := exec.Command("systemctl", "stop", unit)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "not loaded") || strings.Contains(msg, "not found") {
			return nil
		}
		return fmt.Errorf("stop %s: %w (%s)", unit, err, msg)
	}
	return nil
}

// SystemctlIsActive returns true when systemctl is-active reports active.
func (o *Ops) SystemctlIsActive(unit string) bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", unit)
	return cmd.Run() == nil
}
