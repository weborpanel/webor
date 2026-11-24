package logging

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/weborpanel/webor/internal/sites"
)

// EnsureHostDir creates /var/log/webor with sensible permissions.
func EnsureHostDir() error {
	if err := os.MkdirAll(HostDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", HostDir, err)
	}
	_ = os.Chmod(HostDir, 0o755)
	backup := HostCurrentPath(StreamBackup)
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		f, err := os.OpenFile(backup, os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		_ = f.Close()
	}
	return nil
}

// EnsureSiteLogsDir creates the site logs directory writable by the site user and caddy.
// Mode 02775 (setgid) so new files inherit the caddy group when available.
func EnsureSiteLogsDir(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	dir := sites.Logs(domain)
	if err := os.MkdirAll(dir, 0o2755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	siteUser := sites.Username(domain)
	u, err := user.Lookup(siteUser)
	uid, gid := "", ""
	if err == nil {
		uid, gid = u.Uid, u.Gid
	}
	group := siteUser
	if _, err := user.LookupGroup("caddy"); err == nil {
		group = "caddy"
		if g, err := user.LookupGroup("caddy"); err == nil {
			gid = g.Gid
		}
	}

	if uid != "" && gid != "" {
		uidN, _ := strconv.Atoi(uid)
		gidN, _ := strconv.Atoi(gid)
		_ = os.Chown(dir, uidN, gidN)
	} else if uid != "" {
		cmd := exec.Command("chown", uid+":"+group, dir)
		_ = cmd.Run()
	}

	_ = os.Chmod(dir, 0o2775)

	// Ensure common current files exist with group-writable mode.
	for _, stream := range []string{StreamAccess, StreamApp, StreamBackend} {
		p := filepath.Join(dir, CurrentBasename(stream))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o660)
			if err != nil {
				continue
			}
			_ = f.Close()
		}
		_ = os.Chmod(p, 0o660)
		if uid != "" && gid != "" {
			uidN, _ := strconv.Atoi(uid)
			gidN, _ := strconv.Atoi(gid)
			_ = os.Chown(p, uidN, gidN)
		}
	}
	return nil
}

// EnsureSiteStreamFile creates one stream's current log file under the site logs dir.
func EnsureSiteStreamFile(domain, stream string) error {
	if err := EnsureSiteLogsDir(domain); err != nil {
		return err
	}
	if !ValidStreamName(stream) || stream == StreamBackup {
		return fmt.Errorf("invalid stream %q", stream)
	}
	p := SiteCurrentPath(domain, stream)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o660)
		if err != nil {
			return err
		}
		_ = f.Close()
	}
	_ = os.Chmod(p, 0o660)
	return nil
}
