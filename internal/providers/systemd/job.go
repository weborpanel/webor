package systemd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/sites"
)

// JobUnits holds a rendered long-running job service.
type JobUnits struct {
	ServiceName string
	Service     string
}

// JobServiceUnitName returns webooza-job-<domain>-<job>.service.
func JobServiceUnitName(domain, job string) string {
	return "webooza-job-" + UnitBase(domain) + "-" + CronJobSlug(job) + ".service"
}

// RenderJobUnits builds a Type=simple Restart=on-failure unit for a site job.
func RenderJobUnits(site config.Site, job config.Job) (JobUnits, error) {
	if strings.TrimSpace(job.Name) == "" {
		return JobUnits{}, fmt.Errorf("job name is required")
	}
	execPath := strings.TrimSpace(job.Exec)
	if execPath == "" {
		return JobUnits{}, fmt.Errorf("job %s: exec is required", job.Name)
	}
	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(sites.Root(site.Name), execPath)
	}
	user := sites.Username(site.Name)
	root := sites.Root(site.Name)
	svcName := JobServiceUnitName(site.Name, job.Name)

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webooza job %s (%s)\n", job.Name, site.Name)
	fmt.Fprintf(&svc, "After=network.target\n")
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=simple\n")
	fmt.Fprintf(&svc, "User=%s\n", user)
	fmt.Fprintf(&svc, "Group=%s\n", user)
	fmt.Fprintf(&svc, "WorkingDirectory=%s\n", root)
	fmt.Fprintf(&svc, "ExecStart=%s\n", execPath)
	fmt.Fprintf(&svc, "Restart=on-failure\n")
	appendHardening(&svc, site, root)
	fmt.Fprintf(&svc, "\n[Install]\n")
	fmt.Fprintf(&svc, "WantedBy=multi-user.target\n")

	return JobUnits{ServiceName: svcName, Service: svc.String()}, nil
}
