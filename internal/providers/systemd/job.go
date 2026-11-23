package systemd

import (
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/sites"
)

// JobUnits holds a rendered long-running job service.
type JobUnits struct {
	ServiceName string
	Service     string
}

// JobServiceUnitName returns webor-job-<domain>-<job>.service.
func JobServiceUnitName(domain, job string) string {
	return "webor-job-" + UnitBase(domain) + "-" + CronJobSlug(job) + ".service"
}

// RenderJobUnits builds a Type=simple Restart=on-failure unit for a site job.
func RenderJobUnits(site config.Site, job config.Job) (JobUnits, error) {
	return RenderJobUnitsWithNode(site, job, "")
}

// RenderJobUnitsWithNode is like RenderJobUnits but allows a test override for the node binary.
func RenderJobUnitsWithNode(site config.Site, job config.Job, nodeBinOverride string) (JobUnits, error) {
	if strings.TrimSpace(job.Name) == "" {
		return JobUnits{}, fmt.Errorf("job name is required")
	}
	appExec, err := ResolveTaskExec(site.Name, job.Exec, job.Runtime, nodeBinOverride)
	if err != nil {
		return JobUnits{}, fmt.Errorf("job %s: %w", job.Name, err)
	}
	user := sites.Username(site.Name)
	root := sites.Root(site.Name)
	svcName := JobServiceUnitName(site.Name, job.Name)

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webor job %s (%s)\n", job.Name, site.Name)
	fmt.Fprintf(&svc, "After=network.target\n")
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=simple\n")
	fmt.Fprintf(&svc, "User=%s\n", user)
	fmt.Fprintf(&svc, "Group=%s\n", user)
	fmt.Fprintf(&svc, "WorkingDirectory=%s\n", root)
	if appExec.PathEnv != "" {
		fmt.Fprintf(&svc, "Environment=PATH=%s:/usr/local/bin:/usr/bin:/bin\n", appExec.PathEnv)
	}
	fmt.Fprintf(&svc, "ExecStart=%s\n", appExec.ExecStart)
	fmt.Fprintf(&svc, "Restart=on-failure\n")
	appendHardening(&svc, site, root)
	fmt.Fprintf(&svc, "\n[Install]\n")
	fmt.Fprintf(&svc, "WantedBy=multi-user.target\n")

	return JobUnits{ServiceName: svcName, Service: svc.String()}, nil
}
