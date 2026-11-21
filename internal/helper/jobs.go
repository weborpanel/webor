package helper

import (
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
)

// InstallJob writes and enables a long-running job service.
func (o *Ops) InstallJob(site config.Site, job config.Job) (*systemd.JobUnits, error) {
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	units, err := systemd.RenderJobUnits(site, job)
	if err != nil {
		return nil, err
	}
	if err := o.WriteUnitFile(site.Name, units.ServiceName, units.Service); err != nil {
		return nil, err
	}
	if err := o.SystemctlEnableNow(units.ServiceName); err != nil {
		return nil, err
	}
	return &units, nil
}

// InstallSiteJobs installs every long-running job declared on the site.
func (o *Ops) InstallSiteJobs(site config.Site) error {
	for _, job := range site.Jobs {
		if _, err := o.InstallJob(site, job); err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
	}
	return nil
}

// JobIsActive reports whether the job service is active.
func (o *Ops) JobIsActive(domain, jobName string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return o.SystemctlIsActive(systemd.JobServiceUnitName(domain, jobName))
}
