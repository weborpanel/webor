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

// CreateSiteUser creates (or reconciles) the per-site Linux user and directories.
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

	dirs := []string{
		root,
		filepath.Join(root, "public"),
		filepath.Join(root, "logs"),
		filepath.Join(root, "tmp"),
	}
	for _, d := range dirs {
		mode := os.FileMode(0o750)
		if d == root || strings.HasSuffix(d, string(filepath.Separator)+"public") {
			mode = 0o755
		}
		if err := os.MkdirAll(d, mode); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		_ = os.Chmod(d, mode)
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
	if err := os.Chmod(filepath.Join(root, "public"), 0o755); err != nil {
		return err
	}
	return nil
}
