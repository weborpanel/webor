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

// SystemctlStart starts a unit.
func (o *Ops) SystemctlStart(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return fmt.Errorf("empty unit name")
	}
	cmd := exec.Command("systemctl", "start", unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start %s: %w (%s)", unit, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SystemctlRestart restarts a unit.
func (o *Ops) SystemctlRestart(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return fmt.Errorf("empty unit name")
	}
	cmd := exec.Command("systemctl", "restart", unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restart %s: %w (%s)", unit, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SystemctlResetFailed clears the failed state for a unit (start-limit recovery).
func (o *Ops) SystemctlResetFailed(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return fmt.Errorf("empty unit name")
	}
	cmd := exec.Command("systemctl", "reset-failed", unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return nil
		}
		return fmt.Errorf("reset-failed %s: %w (%s)", unit, err, msg)
	}
	return nil
}

// SystemctlTryRestart restarts a unit if it is active; otherwise starts it.
// For socket-activated apps, restarting the .service picks up a new binary
// while leaving the .socket listening.
func (o *Ops) SystemctlTryRestart(unit string) error {
	if o.SystemctlIsActive(unit) {
		return o.SystemctlRestart(unit)
	}
	cmd := exec.Command("systemctl", "start", unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Socket-activated services may refuse a direct start when the socket
		// is already handling activation; treat that as OK and rely on probe.
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "refuses to start") || strings.Contains(msg, "unit may be requested") {
			return nil
		}
		return fmt.Errorf("start %s: %w (%s)", unit, err, msg)
	}
	return nil
}
