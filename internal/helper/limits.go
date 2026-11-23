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

	switch site.Type {
	case config.SiteTypeApp:
		if err := o.applyAppUnitLimits(site); err != nil {
			return err
		}
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
			wasActive := o.SystemctlIsActive(unit)
			if err := o.WriteUnitFile(site.Name, unit, body); err != nil {
				return fmt.Errorf("backend %s: %w", b.ID, err)
			}
			if wasActive {
				_ = o.SystemctlResetFailed(unit)
				if err := o.SystemctlRestart(unit); err != nil {
					return fmt.Errorf("restart backend %s: %w", b.ID, err)
				}
			}
		}
	}

	for _, job := range site.Cron {
		units, err := systemd.RenderCronUnits(site, job)
		if err != nil {
			return fmt.Errorf("cron %q: %w", job.Name, err)
		}
		if err := o.WriteUnitFile(site.Name, units.ServiceName, units.Service); err != nil {
			return fmt.Errorf("cron %q: %w", job.Name, err)
		}
		if units.Timer != "" {
			if err := o.WriteUnitFile(site.Name, units.TimerName, units.Timer); err != nil {
				return fmt.Errorf("cron %q timer: %w", job.Name, err)
			}
		}
	}
	for _, job := range site.Jobs {
		units, err := systemd.RenderJobUnits(site, job)
		if err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
		wasActive := o.SystemctlIsActive(units.ServiceName)
		if err := o.WriteUnitFile(site.Name, units.ServiceName, units.Service); err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
		if wasActive {
			if err := o.SystemctlRestart(units.ServiceName); err != nil {
				return fmt.Errorf("restart job %q: %w", job.Name, err)
			}
		}
	}
	return nil
}

func (o *Ops) applyAppUnitLimits(site config.Site) error {
	units, err := systemd.RenderAppUnits(site)
	if err != nil {
		// No exec yet — limits stay in config until first deploy/ensure.
		return nil
	}
	if units.SocketName != "" {
		if err := o.WriteUnitFile(site.Name, units.SocketName, units.Socket); err != nil {
			return err
		}
	}
	if err := o.WriteUnitFile(site.Name, units.ServiceName, units.Service); err != nil {
		return err
	}
	if units.BackendServiceName != "" {
		wasActive := o.SystemctlIsActive(units.BackendServiceName)
		if err := o.WriteUnitFile(site.Name, units.BackendServiceName, units.BackendService); err != nil {
			return err
		}
		if wasActive {
			if err := o.SystemctlRestart(units.BackendServiceName); err != nil {
				return fmt.Errorf("restart backend: %w", err)
			}
		}
	}
	if o.SystemctlIsActive(units.ServiceName) {
		if err := o.SystemctlRestart(units.ServiceName); err != nil {
			return fmt.Errorf("restart app: %w", err)
		}
	}
	return nil
}
