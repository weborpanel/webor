package helper

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/deploy"
	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
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
	HasSchedule bool   `json:"has_schedule"`
}

// InstallCronJob writes the oneshot service. When schedule is set, also writes
// and enables the matching timer. Empty schedule = oneshot-only (Run now).
// Service + timer are written then a single daemon-reload runs before enable.
func (o *Ops) InstallCronJob(site config.Site, job config.CronJob) (*systemd.CronUnits, error) {
	units, err := o.installCronJobUnits(site, job)
	if err != nil {
		return nil, err
	}
	if err := o.SystemctlDaemonReload(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(job.Schedule) != "" && units.Timer != "" {
		if err := o.SystemctlEnableNow(units.TimerName); err != nil {
			return nil, err
		}
	}
	return units, nil
}

// installCronJobUnits renders and writes cron service/timer files without daemon-reload.
func (o *Ops) installCronJobUnits(site config.Site, job config.CronJob) (*systemd.CronUnits, error) {
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	if err := sites.EnsureCronDir(site.Name, job.Name); err != nil {
		return nil, fmt.Errorf("crons dir: %w", err)
	}
	if err := o.ensureSiteRuntimeDir(site.Name); err != nil {
		return nil, err
	}
	if job.HasNodeRuntime() && job.Runtime != nil && job.Runtime.Command == "" {
		if _, err := o.EnsureNodeRuntime(context.Background(), job.Runtime.Version); err != nil {
			return nil, err
		}
	}
	units, err := systemd.RenderCronUnits(site, job, o.ScheduleTimezone())
	if err != nil {
		return nil, err
	}
	_ = logging.EnsureSiteStreamFile(site.Name, logging.CronStream(job.Name))
	files := map[string]string{units.ServiceName: units.Service}
	if strings.TrimSpace(job.Schedule) != "" && units.Timer != "" {
		files[units.TimerName] = units.Timer
	}
	for name, body := range files {
		if err := o.WriteUnitFileNoReload(site.Name, name, body); err != nil {
			return nil, err
		}
	}
	return &units, nil
}

// InstallSiteCron installs every cron job declared on the site and removes orphans.
// Unit files are written first, then a single daemon-reload, then timers are enabled.
func (o *Ops) InstallSiteCron(site config.Site) error {
	desired := make(map[string]struct{}, len(site.Cron))
	var enableTimers []string
	for _, job := range site.Cron {
		desired[strings.ToLower(strings.TrimSpace(job.Name))] = struct{}{}
		units, err := o.installCronJobUnits(site, job)
		if err != nil {
			return fmt.Errorf("cron %q: %w", job.Name, err)
		}
		if strings.TrimSpace(job.Schedule) != "" && units != nil && units.Timer != "" {
			enableTimers = append(enableTimers, units.TimerName)
		}
	}
	if err := o.removeOrphanCronUnits(site.Name, desired); err != nil {
		return err
	}
	if len(site.Cron) > 0 || len(enableTimers) > 0 {
		if err := o.SystemctlDaemonReload(); err != nil {
			return err
		}
	}
	for _, timer := range enableTimers {
		if err := o.SystemctlEnableNow(timer); err != nil {
			return err
		}
	}
	return nil
}

// RemoveCronJob stops/disables and deletes cron service + timer units.
func (o *Ops) RemoveCronJob(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	jobName = strings.TrimSpace(jobName)
	if domain == "" || jobName == "" {
		return fmt.Errorf("domain and job are required")
	}
	svc := systemd.CronServiceUnitName(domain, jobName)
	timer := systemd.CronTimerUnitName(domain, jobName)
	_ = o.SystemctlStop(timer)
	_ = o.SystemctlDisable(timer)
	_ = o.SystemctlStop(svc)
	_ = o.SystemctlDisable(svc)
	_ = os.Remove(filepath.Join("/etc/systemd/system", timer))
	_ = os.Remove(filepath.Join("/etc/systemd/system", svc))
	_ = o.SystemctlDaemonReload()
	_ = o.SystemctlResetFailed(timer)
	_ = o.SystemctlResetFailed(svc)
	return nil
}

// EnableCronTimer writes/enables the timer for a cron job that has a schedule.
func (o *Ops) EnableCronTimer(site config.Site, job config.CronJob) error {
	if strings.TrimSpace(job.Schedule) == "" {
		return fmt.Errorf("schedule is required to enable timer")
	}
	units, err := o.InstallCronJob(site, job)
	if err != nil {
		return err
	}
	if units.Timer == "" {
		return fmt.Errorf("no timer rendered")
	}
	return nil
}

// DisableCronTimer stops and disables the cron timer (service remains for Run now).
func (o *Ops) DisableCronTimer(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	jobName = strings.TrimSpace(jobName)
	timer := systemd.CronTimerUnitName(domain, jobName)
	_ = o.SystemctlStop(timer)
	_ = o.SystemctlDisable(timer)
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

	loc := scheduleLocation(o.ScheduleTimezone())
	props, err := systemctlShow(timer, "ActiveState", "LastTriggerUSec", "NextElapseUSecRealtime", "Result", "UnitFileState")
	if err == nil {
		if v := props["LastTriggerUSec"]; v != "" && v != "0" {
			res.LastTrigger = formatSystemdUsec(v, loc)
		}
		if v := props["NextElapseUSecRealtime"]; v != "" && v != "0" {
			res.NextElapse = formatSystemdUsec(v, loc)
		}
		if props["UnitFileState"] == "enabled" || props["ActiveState"] == "active" {
			res.HasSchedule = true
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

// DeployCronBinary installs a binary as crons/<job>/server (no auto-run).
func (o *Ops) DeployCronBinary(ctx context.Context, domain, job, binaryPath string) (string, error) {
	_ = ctx
	dest, err := o.deployTaskBinary(domain, job, binaryPath, true)
	return dest, err
}

// DeployCronZip extracts a zip into crons/<job>/.
func (o *Ops) DeployCronZip(ctx context.Context, domain, job, zipPath string) (string, error) {
	_ = ctx
	return o.deployTaskZip(domain, job, zipPath, true)
}

// ListCronDir lists a directory under crons/<job>/.
func (o *Ops) ListCronDir(domain, job, rel string) ([]PublicFSEntry, error) {
	if err := sites.EnsureCronDir(domain, job); err != nil {
		return nil, err
	}
	dir, err := sites.ResolveCronRel(domain, job, rel)
	if err != nil {
		return nil, err
	}
	return listDirEntries(dir)
}

func (o *Ops) removeOrphanCronUnits(domain string, desired map[string]struct{}) error {
	base := "webor-cron-" + systemd.UnitBase(domain) + "-"
	entries, err := os.ReadDir("/etc/systemd/system")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		rest := strings.TrimPrefix(name, base)
		var slug string
		switch {
		case strings.HasSuffix(rest, ".timer"):
			slug = strings.TrimSuffix(rest, ".timer")
		case strings.HasSuffix(rest, ".service"):
			slug = strings.TrimSuffix(rest, ".service")
		default:
			continue
		}
		keep := false
		for want := range desired {
			if systemd.CronJobSlug(want) == slug {
				keep = true
				break
			}
		}
		if keep {
			continue
		}
		// Best-effort: remove by reconstructing from slug as job name.
		_ = o.RemoveCronJob(domain, slug)
	}
	return nil
}

func (o *Ops) deployTaskBinary(domain, job, binaryPath string, cron bool) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	job = strings.TrimSpace(job)
	binaryPath = strings.TrimSpace(binaryPath)
	if domain == "" || job == "" || binaryPath == "" {
		return "", fmt.Errorf("domain, job, and binary path are required")
	}
	if err := o.CreateSiteUser(domain); err != nil {
		return "", err
	}
	var root string
	if cron {
		if err := sites.EnsureCronDir(domain, job); err != nil {
			return "", err
		}
		root = sites.CronDir(domain, job)
	} else {
		if err := sites.EnsureProcessDir(domain, job); err != nil {
			return "", err
		}
		root = sites.ProcessDir(domain, job)
	}
	dest := filepath.Join(root, sites.DefaultAppBinary)
	tmp := dest + ".tmp"
	src, err := os.Open(binaryPath)
	if err != nil {
		return "", err
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		_ = src.Close()
		return "", err
	}
	_, copyErr := io.Copy(out, src)
	_ = src.Close()
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Chmod(dest, 0o755)
	u := sites.Username(domain)
	if out, err := exec.Command("chown", "-R", u+":"+u, root).CombinedOutput(); err != nil {
		return "", fmt.Errorf("chown: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return dest, nil
}

func (o *Ops) deployTaskZip(domain, job, zipPath string, cron bool) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	job = strings.TrimSpace(job)
	zipPath = strings.TrimSpace(zipPath)
	if domain == "" || job == "" || zipPath == "" {
		return "", fmt.Errorf("domain, job, and zip path are required")
	}
	if err := o.CreateSiteUser(domain); err != nil {
		return "", err
	}
	var root string
	if cron {
		if err := sites.EnsureCronDir(domain, job); err != nil {
			return "", err
		}
		root = sites.CronDir(domain, job)
	} else {
		if err := sites.EnsureProcessDir(domain, job); err != nil {
			return "", err
		}
		root = sites.ProcessDir(domain, job)
	}
	if err := deploy.ExtractZip(zipPath, root); err != nil {
		return "", err
	}
	u := sites.Username(domain)
	if out, err := exec.Command("chown", "-R", u+":"+u, root).CombinedOutput(); err != nil {
		return "", fmt.Errorf("chown: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return root, nil
}

func listDirEntries(dir string) ([]PublicFSEntry, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]PublicFSEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		typ := "file"
		if e.IsDir() {
			typ = "dir"
		}
		out = append(out, PublicFSEntry{
			Name:  e.Name(),
			Type:  typ,
			Size:  info.Size(),
			Mtime: info.ModTime().Unix(),
			Mode:  info.Mode().Perm().String(),
		})
	}
	return out, nil
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
	cmd := exec.Command("journalctl", "--system", "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso", "-q")
	// Force short-iso wall times into the panel schedule zone (independent of host TZ).
	cmd.Env = append(os.Environ(), "TZ="+o.ScheduleTimezone())
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" || strings.Contains(msg, "No entries") || strings.Contains(msg, "-- No entries --") {
			return "", nil
		}
		return "", fmt.Errorf("journalctl %s: %w (%s)", unit, err, msg)
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

func scheduleLocation(tz string) *time.Location {
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.Local
}

func formatSystemdUsec(v string, loc *time.Location) string {
	usec, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || usec <= 0 {
		return v
	}
	if loc == nil {
		loc = time.Local
	}
	sec := usec / 1_000_000
	return time.Unix(sec, 0).In(loc).Format(time.RFC3339)
}
