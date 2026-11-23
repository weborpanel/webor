package helper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

// JobStatusResult is status for a long-running site process.
type JobStatusResult struct {
	Domain  string `json:"domain"`
	Job     string `json:"job"`
	Service string `json:"service"`
	Active  bool   `json:"active"`
	Failed  bool   `json:"failed"`
}

// InstallJob writes and enables a long-running job service (starts it).
func (o *Ops) InstallJob(site config.Site, job config.Job) (*systemd.JobUnits, error) {
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	if err := sites.EnsureProcessDir(site.Name, job.Name); err != nil {
		return nil, fmt.Errorf("processes dir: %w", err)
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

// InstallJobUnit writes the job unit and enables it without forcing a start
// when the binary may not exist yet (wizard create before deploy).
func (o *Ops) InstallJobUnit(site config.Site, job config.Job, start bool) (*systemd.JobUnits, error) {
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	if err := sites.EnsureProcessDir(site.Name, job.Name); err != nil {
		return nil, fmt.Errorf("processes dir: %w", err)
	}
	units, err := systemd.RenderJobUnits(site, job)
	if err != nil {
		return nil, err
	}
	if err := o.WriteUnitFile(site.Name, units.ServiceName, units.Service); err != nil {
		return nil, err
	}
	if err := o.SystemctlEnable(units.ServiceName); err != nil {
		return nil, err
	}
	if start {
		_ = o.SystemctlResetFailed(units.ServiceName)
		if err := o.SystemctlStart(units.ServiceName); err != nil {
			return &units, err
		}
	}
	return &units, nil
}

// InstallSiteJobs installs every long-running job declared on the site and removes orphans.
func (o *Ops) InstallSiteJobs(site config.Site) error {
	desired := make(map[string]struct{}, len(site.Jobs))
	for _, job := range site.Jobs {
		desired[strings.ToLower(strings.TrimSpace(job.Name))] = struct{}{}
		if _, err := o.InstallJob(site, job); err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
	}
	return o.removeOrphanJobUnits(site.Name, desired)
}

// RemoveJob stops/disables and deletes a long-running job unit.
func (o *Ops) RemoveJob(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	jobName = strings.TrimSpace(jobName)
	if domain == "" || jobName == "" {
		return fmt.Errorf("domain and job are required")
	}
	svc := systemd.JobServiceUnitName(domain, jobName)
	_ = o.SystemctlStop(svc)
	_ = o.SystemctlDisable(svc)
	_ = os.Remove(filepath.Join("/etc/systemd/system", svc))
	_ = o.SystemctlDaemonReload()
	_ = o.SystemctlResetFailed(svc)
	return nil
}

// JobIsActive reports whether the job service is active.
func (o *Ops) JobIsActive(domain, jobName string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return o.SystemctlIsActive(systemd.JobServiceUnitName(domain, jobName))
}

// JobStatus returns active/failed for a process job.
func (o *Ops) JobStatus(domain, jobName string) (*JobStatusResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	jobName = strings.TrimSpace(jobName)
	svc := systemd.JobServiceUnitName(domain, jobName)
	res := &JobStatusResult{
		Domain:  domain,
		Job:     jobName,
		Service: svc,
		Active:  o.SystemctlIsActive(svc),
	}
	props, err := systemctlShow(svc, "ActiveState", "Result")
	if err == nil {
		res.Failed = props["ActiveState"] == "failed" ||
			props["Result"] == "exit-code" || props["Result"] == "signal" ||
			props["Result"] == "core-dump" || props["Result"] == "timeout"
	}
	return res, nil
}

// StartJob starts a long-running job unit.
func (o *Ops) StartJob(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	svc := systemd.JobServiceUnitName(domain, jobName)
	_ = o.SystemctlResetFailed(svc)
	return o.SystemctlStart(svc)
}

// StopJob stops a long-running job unit.
func (o *Ops) StopJob(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return o.SystemctlStop(systemd.JobServiceUnitName(domain, jobName))
}

// RestartJob restarts (or starts) a long-running job unit.
func (o *Ops) RestartJob(domain, jobName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	svc := systemd.JobServiceUnitName(domain, jobName)
	_ = o.SystemctlResetFailed(svc)
	if err := o.SystemctlTryRestart(svc); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

// DeployProcessBinary installs a binary as processes/<job>/server and restarts if active.
func (o *Ops) DeployProcessBinary(ctx context.Context, domain, job, binaryPath string) (string, error) {
	dest, err := o.deployTaskBinary(domain, job, binaryPath, false)
	if err != nil {
		return "", err
	}
	svc := systemd.JobServiceUnitName(domain, job)
	if o.SystemctlIsActive(svc) {
		_ = o.SystemctlResetFailed(svc)
		_ = o.SystemctlTryRestart(svc)
		time.Sleep(200 * time.Millisecond)
	}
	_ = ctx
	return dest, nil
}

// DeployProcessZip extracts a zip into processes/<job>/ and restarts if active.
func (o *Ops) DeployProcessZip(ctx context.Context, domain, job, zipPath string) (string, error) {
	root, err := o.deployTaskZip(domain, job, zipPath, false)
	if err != nil {
		return "", err
	}
	svc := systemd.JobServiceUnitName(domain, job)
	if o.SystemctlIsActive(svc) {
		_ = o.SystemctlResetFailed(svc)
		_ = o.SystemctlTryRestart(svc)
		time.Sleep(200 * time.Millisecond)
	}
	_ = ctx
	return root, nil
}

// ListProcessDir lists a directory under processes/<job>/.
func (o *Ops) ListProcessDir(domain, job, rel string) ([]PublicFSEntry, error) {
	if err := sites.EnsureProcessDir(domain, job); err != nil {
		return nil, err
	}
	dir, err := sites.ResolveProcessRel(domain, job, rel)
	if err != nil {
		return nil, err
	}
	return listDirEntries(dir)
}

func (o *Ops) removeOrphanJobUnits(domain string, desired map[string]struct{}) error {
	base := "webor-job-" + systemd.UnitBase(domain) + "-"
	entries, err := os.ReadDir("/etc/systemd/system")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) || !strings.HasSuffix(name, ".service") {
			continue
		}
		slug := strings.TrimSuffix(strings.TrimPrefix(name, base), ".service")
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
		_ = o.RemoveJob(domain, slug)
	}
	return nil
}
