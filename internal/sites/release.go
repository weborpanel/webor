package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewReleaseID returns a UTC timestamp id suitable for a release directory name.
func NewReleaseID(now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.UTC().Format("20060102T150405Z")
}

// ActivateRelease atomically points public (and current) at releases/<id>.
// The release directory must already exist under the site root.
func ActivateRelease(domain, id string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.TrimSpace(id)
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	if id == "" {
		return fmt.Errorf("release id is required")
	}
	if strings.Contains(id, "/") || strings.Contains(id, `\`) || strings.Contains(id, "..") {
		return fmt.Errorf("invalid release id %q", id)
	}

	rel := Release(domain, id)
	st, err := os.Stat(rel)
	if err != nil {
		return fmt.Errorf("release %s: %w", id, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("release %s is not a directory", id)
	}

	root := Root(domain)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	// public must resolve to the release dir for Caddy (single symlink hop).
	if err := atomicSymlink(rel, Public(domain)); err != nil {
		return fmt.Errorf("public symlink: %w", err)
	}
	// current mirrors public for operators / deploy tooling.
	if err := atomicSymlink(rel, Current(domain)); err != nil {
		return fmt.Errorf("current symlink: %w", err)
	}
	return nil
}

// EnsureReleaseLayout creates releases/ and an initial empty release if public is missing.
// If public already exists as a real directory (M0 layout), it is left alone.
func EnsureReleaseLayout(domain string, now time.Time) (releaseID string, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	root := Root(domain)
	pub := Public(domain)

	if err := os.MkdirAll(ReleasesDir(domain), 0o755); err != nil {
		return "", err
	}

	fi, err := os.Lstat(pub)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(pub)
			if filepath.IsAbs(target) && strings.HasPrefix(target, ReleasesDir(domain)+string(filepath.Separator)) {
				return filepath.Base(target), nil
			}
			return "", nil
		}
		if fi.IsDir() {
			// Legacy M0: real public/ directory — keep serving it.
			return "", nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	id := NewReleaseID(now)
	rel := Release(domain, id)
	if err := os.MkdirAll(rel, 0o755); err != nil {
		return "", err
	}
	if err := ActivateRelease(domain, id); err != nil {
		return "", err
	}
	_ = root
	return id, nil
}

func atomicSymlink(target, linkPath string) error {
	dir := filepath.Dir(linkPath)
	tmp, err := os.CreateTemp(dir, ".symlink-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpName)

	if err := os.Symlink(target, tmpName); err != nil {
		return err
	}
	if err := os.Rename(tmpName, linkPath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
