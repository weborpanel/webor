package helper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

// ListSiteLogFiles lists allowlisted log files for a site.
func (o *Ops) ListSiteLogFiles(domain string) ([]logging.FileInfo, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	return logging.ListDir(sites.Logs(domain), false)
}

// ReadSiteLogFile tails a site log file by basename.
func (o *Ops) ReadSiteLogFile(domain, name string, lines int) (logging.FileInfo, string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return logging.FileInfo{}, "", fmt.Errorf("domain is required")
	}
	path, info, err := logging.ResolveSafePath(sites.Logs(domain), name, false)
	if err != nil {
		return logging.FileInfo{}, "", err
	}
	text, err := logging.TailFile(path, lines)
	return info, text, err
}

// ListHostLogFiles lists host log files under /var/log/webor.
func (o *Ops) ListHostLogFiles() ([]logging.FileInfo, error) {
	return logging.ListDir(logging.HostDir, true)
}

// ReadHostLogFile tails a host log file by basename.
func (o *Ops) ReadHostLogFile(name string, lines int) (logging.FileInfo, string, error) {
	path, info, err := logging.ResolveSafePath(logging.HostDir, name, true)
	if err != nil {
		return logging.FileInfo{}, "", err
	}
	text, err := logging.TailFile(path, lines)
	return info, text, err
}

// SiteLogTail prefers the current file for a unit selector; falls back to journal.
func (o *Ops) SiteLogTail(domain, selector string, lines int, apiIDs []string) (unit string, text string, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	sel := strings.TrimSpace(selector)
	if sel == "apis" {
		return o.siteLogTailAPIs(domain, apiIDs, lines)
	}
	stream := logging.SelectorToStream(sel)
	if stream != "" {
		path := logging.SiteCurrentPath(domain, stream)
		if _, err := os.Stat(path); err == nil {
			text, err := logging.TailFile(path, lines)
			return path, text, err
		}
		// Access logs are file-only (Caddy); never fall back to journal.
		if stream == logging.StreamAccess {
			_ = logging.EnsureSiteStreamFile(domain, logging.StreamAccess)
			text, err := logging.TailFile(path, lines)
			return path, text, err
		}
	}
	return o.SiteJournalTail(domain, sel, lines)
}

func (o *Ops) siteLogTailAPIs(domain string, ids []string, lines int) (string, string, error) {
	if lines < 1 {
		lines = 100
	}
	per := lines
	if len(ids) > 1 {
		per = lines / len(ids)
		if per < 20 {
			per = 20
		}
	}
	var b strings.Builder
	var units []string
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		unit, chunk, err := o.SiteLogTail(domain, "api:"+id, per, nil)
		if err != nil {
			fmt.Fprintf(&b, "===== %s (error) =====\n%s\n\n", id, err.Error())
			continue
		}
		units = append(units, unit)
		fmt.Fprintf(&b, "===== %s (%s) =====\n", id, unit)
		b.WriteString(strings.TrimRight(chunk, "\n"))
		b.WriteString("\n\n")
	}
	return strings.Join(units, ","), strings.TrimRight(b.String(), "\n") + "\n", nil
}

// RotateAllLogs rotates site and host log files using config limits.
func (o *Ops) RotateAllLogs(cfg *config.Config) error {
	_ = logging.EnsureHostDir()
	if err := logging.RotateDir(logging.RotateDirOptions{
		Dir:             logging.HostDir,
		MaxBytes:        logging.MaxBytesFromLogMax(logging.DefaultHostLogMax),
		HostOnlyStreams: []string{logging.StreamBackup},
	}); err != nil {
		return fmt.Errorf("host logs: %w", err)
	}
	if cfg == nil {
		return nil
	}
	for _, site := range cfg.Sites {
		dir := sites.Logs(site.Name)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		if err := logging.RotateDir(logging.RotateDirOptions{
			Dir:      dir,
			MaxBytes: logging.MaxBytesFromLogMax(site.Limits.LogMax),
		}); err != nil {
			return fmt.Errorf("site %s logs: %w", site.Name, err)
		}
	}
	return nil
}

// EnsureLogrotateSchedule installs the hourly webor-logrotate timer.
func (o *Ops) EnsureLogrotateSchedule() error {
	_ = logging.EnsureHostDir()
	unit, timer, bodyUnit, bodyTimer := systemd.RenderLogrotateUnits()
	if err := o.WriteUnitFile("webor", unit, bodyUnit); err != nil {
		return err
	}
	if err := o.WriteUnitFile("webor", timer, bodyTimer); err != nil {
		return err
	}
	return o.SystemctlEnableNow(timer)
}

// EnsureSiteLogStreams creates current log files for a site's configured streams.
func (o *Ops) EnsureSiteLogStreams(site config.Site) error {
	if err := logging.EnsureSiteLogsDir(site.Name); err != nil {
		return err
	}
	_ = logging.EnsureSiteStreamFile(site.Name, logging.StreamAccess)
	if site.Type == config.SiteTypeApp {
		_ = logging.EnsureSiteStreamFile(site.Name, logging.StreamApp)
		if site.SocketProxy {
			_ = logging.EnsureSiteStreamFile(site.Name, logging.StreamBackend)
		}
	}
	for _, b := range site.EnabledBackends() {
		_ = logging.EnsureSiteStreamFile(site.Name, logging.APIStream(b.ID))
	}
	for _, job := range site.Cron {
		_ = logging.EnsureSiteStreamFile(site.Name, logging.CronStream(job.Name))
	}
	for _, job := range site.Jobs {
		_ = logging.EnsureSiteStreamFile(site.Name, logging.JobStream(job.Name))
	}
	return nil
}

// LogFilesJSON marshals a file list for IPC messages.
func LogFilesJSON(files []logging.FileInfo) string {
	raw, _ := json.Marshal(map[string]any{"files": files})
	return string(raw)
}

// LogFileReadJSON marshals a read result for IPC.
func LogFileReadJSON(info logging.FileInfo, text string) string {
	raw, _ := json.Marshal(map[string]any{
		"file": info,
		"text": text,
	})
	return string(raw)
}

// RelSiteLogs is the basename of the site logs directory (for backup exclude).
func RelSiteLogs() string {
	return filepath.Base(sites.Logs("x"))
}
