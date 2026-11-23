package systemd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/sites"
)

// CronUnits holds rendered oneshot service + timer for one cron job.
type CronUnits struct {
	ServiceName string
	TimerName   string
	Service     string
	Timer       string
	OnCalendar  []string
}

var nonJob = regexp.MustCompile(`[^a-z0-9-]+`)

// CronJobSlug sanitizes a cron job name for use in unit basenames.
func CronJobSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = nonJob.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "job"
	}
	if !unicode.IsLetter(rune(s[0])) && !unicode.IsDigit(rune(s[0])) {
		s = "j-" + s
	}
	if len(s) > 48 {
		s = s[:48]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// CronServiceUnitName returns webor-cron-<domain>-<job>.service.
func CronServiceUnitName(domain, job string) string {
	return "webor-cron-" + UnitBase(domain) + "-" + CronJobSlug(job) + ".service"
}

// CronTimerUnitName returns webor-cron-<domain>-<job>.timer.
func CronTimerUnitName(domain, job string) string {
	return "webor-cron-" + UnitBase(domain) + "-" + CronJobSlug(job) + ".timer"
}

// RenderCronUnits builds a Type=oneshot service for a cron job.
// When schedule is non-empty, also builds a matching OnCalendar timer.
// Empty schedule = oneshot-only (Run now / seed); no timer unit.
func RenderCronUnits(site config.Site, job config.CronJob) (CronUnits, error) {
	if strings.TrimSpace(job.Name) == "" {
		return CronUnits{}, fmt.Errorf("cron job name is required")
	}
	execPath := strings.TrimSpace(job.Exec)
	if execPath == "" {
		return CronUnits{}, fmt.Errorf("cron job %q: exec is required", job.Name)
	}
	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(sites.Root(site.Name), execPath)
	}

	user := sites.Username(site.Name)
	root := sites.Root(site.Name)
	svcName := CronServiceUnitName(site.Name, job.Name)
	timerName := CronTimerUnitName(site.Name, job.Name)

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webor cron %s (%s)\n", job.Name, site.Name)
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=oneshot\n")
	fmt.Fprintf(&svc, "User=%s\n", user)
	fmt.Fprintf(&svc, "Group=%s\n", user)
	fmt.Fprintf(&svc, "WorkingDirectory=%s\n", root)
	fmt.Fprintf(&svc, "ExecStart=%s\n", execPath)
	fmt.Fprintf(&svc, "Nice=10\n")
	appendHardening(&svc, site, root)

	out := CronUnits{
		ServiceName: svcName,
		TimerName:   timerName,
		Service:     svc.String(),
	}

	schedule := strings.TrimSpace(job.Schedule)
	if schedule == "" {
		return out, nil
	}
	cals, err := CronToOnCalendar(schedule)
	if err != nil {
		return CronUnits{}, fmt.Errorf("cron job %q schedule: %w", job.Name, err)
	}

	var tim strings.Builder
	fmt.Fprintf(&tim, "[Unit]\n")
	fmt.Fprintf(&tim, "Description=Webor cron timer %s (%s)\n", job.Name, site.Name)
	fmt.Fprintf(&tim, "Requires=%s\n", svcName)
	fmt.Fprintf(&tim, "\n[Timer]\n")
	for _, cal := range cals {
		fmt.Fprintf(&tim, "OnCalendar=%s\n", cal)
	}
	fmt.Fprintf(&tim, "Persistent=true\n")
	fmt.Fprintf(&tim, "Unit=%s\n", svcName)
	fmt.Fprintf(&tim, "\n[Install]\n")
	fmt.Fprintf(&tim, "WantedBy=timers.target\n")

	out.Timer = tim.String()
	out.OnCalendar = cals
	return out, nil
}
