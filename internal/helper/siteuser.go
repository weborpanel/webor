package helper

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/sites"
)

// SiteUsername derives a Linux username from a domain (max 32 chars).
func SiteUsername(domain string) string {
	return sites.Username(domain)
}

// SiteRoot returns /srv/sites/<domain>.
func SiteRoot(domain string) string {
	return sites.Root(domain)
}

// CreateSiteUser creates (or reconciles) the per-site Linux user and directories.
// New sites get a releases/ tree with public → active release. Existing M0
// public/ directories are left in place.
func (o *Ops) CreateSiteUser(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	username := sites.Username(domain)
	root := sites.Root(domain)

	if err := os.MkdirAll(sites.RootDir, 0o755); err != nil {
		return fmt.Errorf("mkdir sites root: %w", err)
	}

	if _, err := user.Lookup(username); err != nil {
		cmd := exec.Command("useradd",
			"--system",
			"--user-group",
			"--home-dir", root,
			"--no-create-home",
			"--shell", "/usr/sbin/nologin",
			username,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("useradd %s: %w (%s)", username, err, strings.TrimSpace(string(out)))
		}
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", root, err)
	}
	for _, d := range []string{sites.Logs(domain), sites.Tmp(domain), sites.ReleasesDir(domain)} {
		mode := os.FileMode(0o750)
		if err := os.MkdirAll(d, mode); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		_ = os.Chmod(d, mode)
	}

	if _, err := sites.EnsureReleaseLayout(domain, time.Now().UTC()); err != nil {
		return fmt.Errorf("release layout: %w", err)
	}

	// Ensure public is world-traversable/readable for Caddy (file_server).
	pub := sites.Public(domain)
	if fi, err := os.Lstat(pub); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 && fi.IsDir() {
			_ = os.Chmod(pub, 0o755)
		}
	}

	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", username, err)
	}
	cmd := exec.Command("chown", "-R", u.Uid+":"+u.Gid, root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown %s: %w (%s)", root, err, strings.TrimSpace(string(out)))
	}
	if err := os.Chmod(root, 0o755); err != nil {
		return err
	}
	return nil
}

// DeleteSiteUser removes the per-site Linux user. When deleteData is true the
// site directory tree under /srv/sites/<domain> is removed as well.
func (o *Ops) DeleteSiteUser(domain string, deleteData bool) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	username := sites.Username(domain)
	root := sites.Root(domain)

	if _, err := user.Lookup(username); err == nil {
		cmd := exec.Command("userdel", username)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("userdel %s: %w (%s)", username, err, strings.TrimSpace(string(out)))
		}
	}

	if !deleteData {
		return nil
	}

	clean := filepath.Clean(root)
	if clean != root && !strings.HasPrefix(clean, filepath.Clean(sites.RootDir)+string(filepath.Separator)) {
		return fmt.Errorf("refusing to delete unexpected path %q", root)
	}
	if clean == filepath.Clean(sites.RootDir) || clean == "/" {
		return fmt.Errorf("refusing to delete sites root")
	}
	if err := os.RemoveAll(clean); err != nil {
		return fmt.Errorf("remove %s: %w", clean, err)
	}
	return nil
}
