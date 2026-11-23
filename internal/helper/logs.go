package helper

import (
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
)

// ResolveSiteLogUnit maps a UI selector to a systemd unit for the site.
// Selectors: "app", "backend", "api:<id>", "cron:<name>", "job:<name>".
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
	if strings.HasPrefix(sel, "api:") {
		id := strings.TrimSpace(strings.TrimPrefix(sel, "api:"))
		if id == "" {
			return "", fmt.Errorf("api id is required")
		}
		if err := config.ValidateBackendID(id); err != nil {
			return "", fmt.Errorf("api id: %w", err)
		}
		return systemd.HTTPBackendUnitName(domain, id), nil
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
	return "", fmt.Errorf("unknown log unit selector %q (want app|backend|api:<id>|cron:<name>|job:<name>)", selector)
}

func allowedSiteUnit(unit string) bool {
	return strings.HasPrefix(unit, "webor-app-") ||
		strings.HasPrefix(unit, "webor-cron-") ||
		strings.HasPrefix(unit, "webor-job-") ||
		strings.HasPrefix(unit, "webor-backend-")
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
	if !allowedSiteUnit(unit) {
		return "", "", fmt.Errorf("unit %q not a webor site unit", unit)
	}
	text, err = o.JournalTail(unit, lines)
	return unit, text, err
}

// SiteJournalTailAPIs concatenates journal tails for multiple HTTP API backends.
func (o *Ops) SiteJournalTailAPIs(domain string, ids []string, lines int) (units []string, text string, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, "", fmt.Errorf("domain is required")
	}
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
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		unit, chunk, err := o.SiteJournalTail(domain, "api:"+id, per)
		if err != nil {
			fmt.Fprintf(&b, "===== %s (error) =====\n%s\n\n", id, err.Error())
			continue
		}
		units = append(units, unit)
		fmt.Fprintf(&b, "===== %s (%s) =====\n", id, unit)
		b.WriteString(strings.TrimRight(chunk, "\n"))
		b.WriteString("\n\n")
	}
	return units, strings.TrimRight(b.String(), "\n") + "\n", nil
}
