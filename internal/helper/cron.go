package helper

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/providers/systemd"
)

// CronStatusResult is a thin status snapshot for a site cron job.
type CronStatusResult struct {
	Domain      string `json:"domain"`
	Job         string `json:"job"`
	Service     string `json:"service"`
	Timer       string `json:"timer"`
	TimerActive bool   `json:"timer_active"`
	LastResult  string `json:"last_result,omitempty"` // success|exit-code|… from Result=
	LastTrigger string `json:"last_trigger,omitempty"`
	NextElapse  string `json:"next_elapse,omitempty"`
	Failed      bool   `json:"failed"`
}

// InstallCronJob writes the oneshot service + timer and enables the timer.
func (o *Ops) InstallCronJob(site config.Site, job config.CronJob) (*systemd.CronUnits, error) {
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	units, err := systemd.RenderCronUnits(site, job)
	if err != nil {
		return nil, err
	}
	if err := o.WriteUnitFile(site.Name, units.ServiceName, units.Service); err != nil {
		return nil, err
	}
	if err := o.WriteUnitFile(site.Name, units.TimerName, units.Timer); err != nil {
		return nil, err
	}
	if err := o.SystemctlEnableNow(units.TimerName); err != nil {
		return nil, err
	}
	return &units, nil
}

// InstallSiteCron installs every cron job declared on the site.
func (o *Ops) InstallSiteCron(site config.Site) error {
	for _, job := range site.Cron {
		if _, err := o.InstallCronJob(site, job); err != nil {
			return fmt.Errorf("cron %q: %w", job.Name, err)
		}
	}
	return nil
}

// RunCronNow starts the oneshot cron service immediately (does not wait for the timer).
func (o *Ops) RunCronNow(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	svc := systemd.CronServiceUnitName(domain, jobName)
	_ = o.SystemctlResetFailed(svc)
	return o.SystemctlStart(svc)
}

// CronStatus returns timer/service status for a cron job.
func (o *Ops) CronStatus(domain, jobName string) (*CronStatusResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	jobName = strings.TrimSpace(jobName)
	svc := systemd.CronServiceUnitName(domain, jobName)
	timer := systemd.CronTimerUnitName(domain, jobName)

	res := &CronStatusResult{
		Domain:      domain,
		Job:         jobName,
		Service:     svc,
		Timer:       timer,
		TimerActive: o.SystemctlIsActive(timer),
	}

	props, err := systemctlShow(timer, "ActiveState", "LastTriggerUSec", "NextElapseUSecRealtime", "Result")
	if err == nil {
		if v := props["LastTriggerUSec"]; v != "" && v != "0" {
			res.LastTrigger = formatSystemdUsec(v)
		}
		if v := props["NextElapseUSecRealtime"]; v != "" && v != "0" {
			res.NextElapse = formatSystemdUsec(v)
		}
	}
	svcProps, err := systemctlShow(svc, "Result", "ActiveState")
	if err == nil {
		res.LastResult = svcProps["Result"]
		res.Failed = svcProps["Result"] == "exit-code" || svcProps["Result"] == "signal" ||
			svcProps["Result"] == "core-dump" || svcProps["Result"] == "timeout" ||
			svcProps["ActiveState"] == "failed"
	}
	return res, nil
}

// JournalTail returns recent journal lines for a unit (newest last).
func (o *Ops) JournalTail(unit string, lines int) (string, error) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return "", fmt.Errorf("unit is required")
	}
	if lines <= 0 {
		lines = 50
	}
	if lines > 500 {
		lines = 500
	}
	cmd := exec.Command("journalctl", "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("journalctl %s: %w (%s)", unit, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func systemctlShow(unit string, props ...string) (map[string]string, error) {
	args := []string{"show", unit}
	for _, p := range props {
		args = append(args, "-p", p)
	}
	cmd := exec.Command("systemctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("systemctl show %s: %w (%s)", unit, err, strings.TrimSpace(string(out)))
	}
	m := make(map[string]string, len(props))
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[k] = v
	}
	return m, nil
}

func formatSystemdUsec(v string) string {
	usec, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || usec <= 0 {
		return v
	}
	// systemd usec since epoch
	sec := usec / 1_000_000
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
