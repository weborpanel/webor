package helper

import (
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
)

// ApplySiteLimits rewrites units that carry MemoryMax/CPUQuota and restarts
// only units that are already active (idle socket-activated apps keep the new
// unit file for the next wake). Disk quota is config-only.
func (o *Ops) ApplySiteLimits(site config.Site) error {
	site.Name = strings.ToLower(strings.TrimSpace(site.Name))
	if site.Name == "" {
		return fmt.Errorf("domain is required")
	}
	config.NormalizeLimits(&site.Limits)
	if err := config.ValidateLimits(site.Limits); err != nil {
		return err
	}

	files := make(map[string]string)
	var restart []string

	switch site.Type {
	case config.SiteTypeApp:
		extra, toRestart, err := o.appUnitLimitFiles(site)
		if err != nil {
			return err
		}
		for k, v := range extra {
			files[k] = v
		}
		restart = append(restart, toRestart...)
	case config.SiteTypeStatic:
		for _, b := range site.Backends {
			if !b.Enabled {
				continue
			}
			config.NormalizeBackend(&b)
			unit, body, err := systemd.RenderHTTPBackendUnit(site, b)
			if err != nil {
				return fmt.Errorf("backend %s: %w", b.ID, err)
			}
			files[unit] = body
			if o.SystemctlIsActive(unit) {
				restart = append(restart, unit)
			}
		}
	}

	tz := o.ScheduleTimezone()
	for _, job := range site.Cron {
		units, err := systemd.RenderCronUnits(site, job, tz)
		if err != nil {
			return fmt.Errorf("cron %q: %w", job.Name, err)
		}
		files[units.ServiceName] = units.Service
		if units.Timer != "" {
			files[units.TimerName] = units.Timer
		}
	}
	for _, job := range site.Jobs {
		units, err := systemd.RenderJobUnits(site, job)
		if err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
		files[units.ServiceName] = units.Service
		if o.SystemctlIsActive(units.ServiceName) {
			restart = append(restart, units.ServiceName)
		}
	}

	if len(files) == 0 {
		return nil
	}
	if err := o.writeUnitFiles(site.Name, files); err != nil {
		return err
	}
	for _, unit := range restart {
		_ = o.SystemctlResetFailed(unit)
		if err := o.SystemctlRestart(unit); err != nil {
			return fmt.Errorf("restart %s: %w", unit, err)
		}
	}
	return nil
}

func (o *Ops) appUnitLimitFiles(site config.Site) (files map[string]string, restart []string, err error) {
	units, err := systemd.RenderAppUnits(site)
	if err != nil {
		// No exec yet — limits stay in config until first deploy/ensure.
		return nil, nil, nil
	}
	files = map[string]string{units.ServiceName: units.Service}
	if units.SocketName != "" {
		files[units.SocketName] = units.Socket
	}
	if units.BackendServiceName != "" {
		files[units.BackendServiceName] = units.BackendService
		if o.SystemctlIsActive(units.BackendServiceName) {
			restart = append(restart, units.BackendServiceName)
		}
	}
	if o.SystemctlIsActive(units.ServiceName) {
		restart = append(restart, units.ServiceName)
	}
	return files, restart, nil
}
