package helper

import (
	"fmt"
	"strings"

	"github.com/webooza/webooza/internal/providers/systemd"
)

// ResolveSiteLogUnit maps a UI selector to a systemd unit for the site.
// Selectors: "app", "backend", "cron:<name>", "job:<name>".
func ResolveSiteLogUnit(domain, selector string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	sel := strings.TrimSpace(selector)
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	if sel == "" || sel == "app" {
		return systemd.ServiceUnitName(domain), nil
	}
	if sel == "backend" {
		return systemd.BackendServiceUnitName(domain), nil
	}
	if strings.HasPrefix(sel, "cron:") {
		job := strings.TrimSpace(strings.TrimPrefix(sel, "cron:"))
		if job == "" {
			return "", fmt.Errorf("cron job name is required")
		}
		return systemd.CronServiceUnitName(domain, job), nil
	}
	if strings.HasPrefix(sel, "job:") {
		job := strings.TrimSpace(strings.TrimPrefix(sel, "job:"))
		if job == "" {
			return "", fmt.Errorf("job name is required")
		}
		return systemd.JobServiceUnitName(domain, job), nil
	}
	return "", fmt.Errorf("unknown log unit selector %q (want app|backend|cron:<name>|job:<name>)", selector)
}

// SiteJournalTail tails journal lines for a site unit selector.
func (o *Ops) SiteJournalTail(domain, selector string, lines int) (unit string, text string, err error) {
	unit, err = ResolveSiteLogUnit(domain, selector)
	if err != nil {
		return "", "", err
	}
	base := systemd.UnitBase(domain)
	if !strings.Contains(unit, base) {
		return "", "", fmt.Errorf("unit %q not allowed for site %s", unit, domain)
	}
	if !strings.HasPrefix(unit, "webooza-app-") &&
		!strings.HasPrefix(unit, "webooza-cron-") &&
		!strings.HasPrefix(unit, "webooza-job-") {
		return "", "", fmt.Errorf("unit %q not a webooza site unit", unit)
	}
	text, err = o.JournalTail(unit, lines)
	return unit, text, err
}
