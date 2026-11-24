// Package logging defines site/host log file paths, rotation, and safe reads.
package logging

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/weborpanel/webor/internal/sites"
)

// HostDir is where host-level Webor logs live (backup runs, etc.).
// Tests may override it.
var HostDir = "/var/log/webor"

// DefaultHostLogMax is the size threshold for rotating host logs (e.g. backup.log).
const DefaultHostLogMax = "100M"

// DefaultRetentionDays is how many calendar days of rotated files to keep per stream.
const DefaultRetentionDays = 14

// DefaultSiteLogMax is used when a site has no limits.log_max set.
const DefaultSiteLogMax = "50M"

var nonSlug = regexp.MustCompile(`[^a-z0-9-]+`)

// StreamSlug sanitizes a job/api id for use in log basenames.
func StreamSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = nonSlug.ReplaceAllString(s, "-")
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

// Stream names for current (unrotated) files.
const (
	StreamAccess  = "access"
	StreamApp     = "app"
	StreamBackend = "backend"
	StreamBackup  = "backup"
)

// APIStream returns api-<id>.
func APIStream(id string) string {
	return "api-" + StreamSlug(id)
}

// CronStream returns cron-<job>.
func CronStream(job string) string {
	return "cron-" + StreamSlug(job)
}

// JobStream returns job-<job>.
func JobStream(job string) string {
	return "job-" + StreamSlug(job)
}

// SiteCurrentPath returns /srv/sites/<domain>/logs/<stream>.log
func SiteCurrentPath(domain, stream string) string {
	return filepath.Join(sites.Logs(domain), CurrentBasename(stream))
}

// HostCurrentPath returns /var/log/webor/<stream>.log
func HostCurrentPath(stream string) string {
	return filepath.Join(HostDir, CurrentBasename(stream))
}

// CurrentBasename returns <stream>.log
func CurrentBasename(stream string) string {
	stream = strings.TrimSpace(stream)
	return stream + ".log"
}

// SelectorToStream maps a UI/journal selector to a log stream name.
// Returns empty for "apis" (multi) or unknown.
func SelectorToStream(selector string) string {
	sel := strings.TrimSpace(selector)
	switch {
	case sel == "" || sel == "app":
		return StreamApp
	case sel == "backend":
		return StreamBackend
	case sel == "access":
		return StreamAccess
	case strings.HasPrefix(sel, "api:"):
		id := strings.TrimSpace(strings.TrimPrefix(sel, "api:"))
		if id == "" {
			return ""
		}
		return APIStream(id)
	case strings.HasPrefix(sel, "cron:"):
		job := strings.TrimSpace(strings.TrimPrefix(sel, "cron:"))
		if job == "" {
			return ""
		}
		return CronStream(job)
	case strings.HasPrefix(sel, "job:"):
		job := strings.TrimSpace(strings.TrimPrefix(sel, "job:"))
		if job == "" {
			return ""
		}
		return JobStream(job)
	default:
		return ""
	}
}

var (
	reCurrent  = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)\.log$`)
	reRotated  = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)\.log\.(\d{4}-\d{2}-\d{2})(?:\.(\d+))?$`)
	reSafeName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
)

// ParseLogBasename validates and parses a log file basename.
// kind is "current" or "rotated"; date/index apply to rotated files.
func ParseLogBasename(name string) (stream, kind, date string, index int, err error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || strings.Contains(name, "..") || !reSafeName.MatchString(name) {
		return "", "", "", 0, fmt.Errorf("invalid log file name")
	}
	if m := reCurrent.FindStringSubmatch(name); m != nil {
		if !ValidStreamName(m[1]) {
			return "", "", "", 0, fmt.Errorf("invalid log stream")
		}
		return m[1], "current", "", 0, nil
	}
	if m := reRotated.FindStringSubmatch(name); m != nil {
		if !ValidStreamName(m[1]) {
			return "", "", "", 0, fmt.Errorf("invalid log stream")
		}
		idx := 0
		if m[3] != "" {
			fmt.Sscanf(m[3], "%d", &idx)
		}
		return m[1], "rotated", m[2], idx, nil
	}
	return "", "", "", 0, fmt.Errorf("invalid log file name")
}

// ValidStreamName checks allowlisted stream prefixes.
func ValidStreamName(stream string) bool {
	stream = strings.ToLower(strings.TrimSpace(stream))
	switch stream {
	case StreamAccess, StreamApp, StreamBackend, StreamBackup:
		return true
	}
	return strings.HasPrefix(stream, "api-") ||
		strings.HasPrefix(stream, "cron-") ||
		strings.HasPrefix(stream, "job-")
}

// ValidHostStreamName allows only host streams (backup for v1).
func ValidHostStreamName(stream string) bool {
	return stream == StreamBackup
}

// RotatedBasename builds access.log.2026-11-23 or access.log.2026-11-23.1
func RotatedBasename(stream, date string, index int) string {
	base := CurrentBasename(stream) + "." + date
	if index > 0 {
		return fmt.Sprintf("%s.%d", base, index)
	}
	return base
}
