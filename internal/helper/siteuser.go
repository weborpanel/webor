package helper

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

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

// CreateSiteUser creates (or reconciles) the per-site Linux user and a public/
// document root (Virtualmin-style). Users upload HTML into public/.
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
	for _, d := range []string{sites.Logs(domain), sites.Tmp(domain)} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		_ = os.Chmod(d, 0o750)
	}
	if err := sites.EnsurePublicDir(domain); err != nil {
		return fmt.Errorf("public dir: %w", err)
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
	_ = os.Chmod(sites.Public(domain), 0o755)
	return nil
}

// WritePublicFile writes a file under the site's public/ document root.
func (o *Ops) WritePublicFile(domain, relPath string, data []byte) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	full, err := sites.ResolvePublicPath(domain, relPath)
	if err != nil {
		return err
	}
	if err := sites.EnsurePublicDir(domain); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Chmod(full, 0o644)
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
	if clean == filepath.Clean(sites.RootDir) || clean == "/" {
		return fmt.Errorf("refusing to delete sites root")
	}
	if !strings.HasPrefix(clean, filepath.Clean(sites.RootDir)+string(filepath.Separator)) {
		return fmt.Errorf("refusing to delete unexpected path %q", root)
	}
	if err := os.RemoveAll(clean); err != nil {
		return fmt.Errorf("remove %s: %w", clean, err)
	}
	return nil
}
