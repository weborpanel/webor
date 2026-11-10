package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/sites"
)

// StaticZipOptions controls a static zip deploy.
type StaticZipOptions struct {
	// Now overrides the release id clock (tests).
	Now time.Time
	// SkipChown leaves ownership unchanged (unit tests without root).
	SkipChown bool
}

// StaticZip extracts a zip into a new release and atomically activates it.
// zipPath must already be on disk; callers confine it (helper validates).
func StaticZip(domain, zipPath string, opt StaticZipOptions) (releaseID string, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	st, err := os.Stat(zipPath)
	if err != nil {
		return "", fmt.Errorf("zip: %w", err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("zip path is a directory")
	}

	if err := os.MkdirAll(sites.ReleasesDir(domain), 0o755); err != nil {
		return "", err
	}
	_ = os.MkdirAll(sites.Logs(domain), 0o750)
	_ = os.MkdirAll(sites.Tmp(domain), 0o750)

	now := opt.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id := sites.NewReleaseID(now)
	rel := sites.Release(domain, id)
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(rel); os.IsNotExist(err) {
			break
		}
		now = now.Add(time.Second)
		id = sites.NewReleaseID(now)
		rel = sites.Release(domain, id)
	}
	if err := os.MkdirAll(rel, 0o755); err != nil {
		return "", err
	}
	if err := ExtractZip(zipPath, rel); err != nil {
		_ = os.RemoveAll(rel)
		return "", err
	}
	if err := sites.ActivateRelease(domain, id); err != nil {
		_ = os.RemoveAll(rel)
		return "", err
	}
	if !opt.SkipChown {
		if err := chownSite(domain); err != nil {
			return id, fmt.Errorf("activated %s but chown failed: %w", id, err)
		}
	}
	return id, nil
}

func chownSite(domain string) error {
	username := sites.Username(domain)
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	root := sites.Root(domain)
	cmd := exec.Command("chown", "-R", u.Uid+":"+u.Gid, root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
