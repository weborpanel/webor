package helper

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const sitesRoot = "/srv/sites"

var nonUser = regexp.MustCompile(`[^a-z0-9_]`)

// SiteUsername derives a Linux username from a domain (max 32 chars).
func SiteUsername(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, ".", "_")
	d = strings.ReplaceAll(d, "-", "_")
	d = nonUser.ReplaceAllString(d, "_")
	d = strings.Trim(d, "_")
	if d == "" {
		d = "site"
	}
	if !unicode.IsLetter(rune(d[0])) {
		d = "s_" + d
	}
	name := "wbz_" + d
	if len(name) > 32 {
		name = name[:32]
		name = strings.TrimRight(name, "_")
	}
	return name
}

// SiteRoot returns /srv/sites/<domain>.
func SiteRoot(domain string) string {
	return filepath.Join(sitesRoot, strings.ToLower(strings.TrimSpace(domain)))
}

// CreateSiteUser creates (or reconciles) the per-site Linux user and directories.
func (o *Ops) CreateSiteUser(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	username := SiteUsername(domain)
	root := SiteRoot(domain)

	if err := os.MkdirAll(sitesRoot, 0o755); err != nil {
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
	// Re-apply traverse/read bits after chown (site root + public stay world-traversable for Caddy).
	if err := os.Chmod(root, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(root, "public"), 0o755); err != nil {
		return err
	}
	return nil
}
