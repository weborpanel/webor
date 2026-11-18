package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultAppBinary is the release artifact name linked as bin/server.
const DefaultAppBinary = "server"

// AppBinaryPath returns /srv/sites/<domain>/bin/server (live exec symlink/file).
func AppBinaryPath(domain string) string {
	return filepath.Join(BinDir(domain), DefaultAppBinary)
}

// CurrentReleaseID returns the basename of the current symlink target when it
// points into releases/, or empty if unset/legacy.
func CurrentReleaseID(domain string) string {
	link := Current(domain)
	target, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	target = filepath.Clean(target)
	prefix := ReleasesDir(domain) + string(filepath.Separator)
	if !strings.HasPrefix(target, prefix) {
		return ""
	}
	return filepath.Base(target)
}

// ActivateAppRelease points current at releases/<id> and bin/server at
// current/<binaryName>. Does not touch public/ (static sites own that).
func ActivateAppRelease(domain, id, binaryName string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.TrimSpace(id)
	if binaryName == "" {
		binaryName = DefaultAppBinary
	}
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	if id == "" {
		return fmt.Errorf("release id is required")
	}
	if strings.Contains(id, "/") || strings.Contains(id, `\`) || strings.Contains(id, "..") {
		return fmt.Errorf("invalid release id %q", id)
	}
	if strings.Contains(binaryName, "/") || strings.Contains(binaryName, `\`) || binaryName == ".." {
		return fmt.Errorf("invalid binary name %q", binaryName)
	}

	rel := Release(domain, id)
	binInRelease := filepath.Join(rel, binaryName)
	st, err := os.Stat(binInRelease)
	if err != nil {
		return fmt.Errorf("release binary: %w", err)
	}
	if st.IsDir() {
		return fmt.Errorf("release binary %s is a directory", binaryName)
	}

	if err := os.MkdirAll(Root(domain), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(BinDir(domain), 0o755); err != nil {
		return err
	}
	if err := atomicSymlink(rel, Current(domain)); err != nil {
		return fmt.Errorf("current symlink: %w", err)
	}
	// bin/server -> ../current/server (stable ExecStart path for units).
	live := AppBinaryPath(domain)
	target := filepath.Join(Current(domain), binaryName)
	if err := atomicSymlink(target, live); err != nil {
		return fmt.Errorf("bin symlink: %w", err)
	}
	return nil
}
