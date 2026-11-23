package helper

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/deploy"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

// DeployAppBinaryResult is returned after a successful app binary deploy.
type DeployAppBinaryResult struct {
	ReleaseID string `json:"release_id"`
	Binary    string `json:"binary"`
}

// DeployAppBinary installs a binary into a new release, activates it, restarts
// the app unit, probes the site Unix socket, and rolls back on probe failure.
func (o *Ops) DeployAppBinary(ctx context.Context, domain, binaryPath string) (*DeployAppBinaryResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if err := validateAppBinaryPath(domain, binaryPath); err != nil {
		return nil, err
	}
	if err := o.CreateSiteUser(domain); err != nil {
		return nil, err
	}

	prev := sites.CurrentReleaseID(domain)
	id, dest, err := deploy.InstallAppBinary(domain, binaryPath, deploy.AppBinaryOptions{})
	if err != nil {
		return nil, err
	}
	if err := sites.ActivateAppRelease(domain, id, sites.DefaultAppBinary); err != nil {
		_ = os.RemoveAll(sites.Release(domain, id))
		return nil, err
	}

	u := sites.Username(domain)
	cmd := exec.Command("chown", "-R", u+":"+u, sites.Root(domain))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("chown: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	svc := systemd.ServiceUnitName(domain)
	backend := systemd.BackendServiceUnitName(domain)
	// Stop activated service (app or proxyd). Restart private backend when present
	// so socket_proxy sites pick up the new binary without LISTEN_FDS.
	_ = o.SystemctlStop(svc)
	_ = o.SystemctlResetFailed(backend)
	_ = o.SystemctlStop(backend)
	_ = o.SystemctlStart(backend) // ignore error when backend unit is absent (native mode)
	if err := o.ensureAppSocket(domain); err != nil {
		if rbErr := rollbackAppRelease(o, domain, prev, svc); rbErr != nil {
			return nil, fmt.Errorf("socket not ready (%v) and rollback failed (%w)", err, rbErr)
		}
		return nil, fmt.Errorf("socket not ready, rolled back: %w", err)
	}

	if err := probeAppSocket(ctx, o, domain); err != nil {
		if rbErr := rollbackAppRelease(o, domain, prev, svc); rbErr != nil {
			return nil, fmt.Errorf("probe failed (%v) and rollback failed (%w)", err, rbErr)
		}
		return nil, fmt.Errorf("probe failed, rolled back: %w", err)
	}

	return &DeployAppBinaryResult{ReleaseID: id, Binary: dest}, nil
}

// DeployAppZipResult is returned after extracting a zip into the app site root.
type DeployAppZipResult struct {
	Root string `json:"root"`
}

// DeployAppZip extracts a zip into the site root (Node WorkingDirectory) and
// restarts the app unit so the new entry is picked up.
func (o *Ops) DeployAppZip(ctx context.Context, domain, zipPath string) (*DeployAppZipResult, error) {
	_ = ctx
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		return nil, fmt.Errorf("zip path is required")
	}
	if err := o.CreateSiteUser(domain); err != nil {
		return nil, err
	}
	if err := o.ensureSiteRuntimeDir(domain); err != nil {
		return nil, err
	}
	root := sites.Root(domain)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if err := deploy.ExtractZip(zipPath, root); err != nil {
		return nil, err
	}
	u := sites.Username(domain)
	cmd := exec.Command("chown", "-R", u+":"+u, root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("chown: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	svc := systemd.ServiceUnitName(domain)
	backend := systemd.BackendServiceUnitName(domain)
	_ = o.SystemctlStop(svc)
	_ = o.SystemctlResetFailed(backend)
	_ = o.SystemctlStop(backend)
	_ = o.SystemctlStart(backend)
	if err := o.ensureAppSocket(domain); err != nil {
		return nil, fmt.Errorf("socket not ready: %w", err)
	}
	return &DeployAppZipResult{Root: root}, nil
}

// DeployAppGit clones/fetches a repo, runs build_command with limits, then
// deploys the resulting artifact via DeployAppBinary.
func (o *Ops) DeployAppGit(ctx context.Context, domain, gitURL, gitRef, buildCommand, artifactRel, memoryMax string, timeoutSec int) (*DeployAppBinaryResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if strings.TrimSpace(gitURL) == "" {
		return nil, fmt.Errorf("git url is required")
	}
	if strings.TrimSpace(artifactRel) == "" {
		artifactRel = filepath.Join("bin", sites.DefaultAppBinary)
	}
	if strings.Contains(artifactRel, "..") {
		return nil, fmt.Errorf("invalid artifact path")
	}
	if err := o.CreateSiteUser(domain); err != nil {
		return nil, err
	}

	src := filepath.Join(sites.Root(domain), "src")
	if err := gitSync(ctx, src, gitURL, gitRef); err != nil {
		return nil, err
	}

	cmd := strings.TrimSpace(buildCommand)
	if cmd == "" {
		cmd = "true"
	}
	if _, err := o.RunBuild(ctx, domain, "cd src && "+cmd, memoryMax, timeoutSec); err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}

	artifact := filepath.Join(src, artifactRel)
	// Allow DeployAppBinary to read from the site tree.
	return o.DeployAppBinary(ctx, domain, artifact)
}

func rollbackAppRelease(o *Ops, domain, prevID, svc string) error {
	if prevID == "" {
		return fmt.Errorf("no previous release to restore")
	}
	if err := sites.ActivateAppRelease(domain, prevID, sites.DefaultAppBinary); err != nil {
		return err
	}
	u := sites.Username(domain)
	_ = exec.Command("chown", "-R", u+":"+u, sites.Root(domain)).Run()
	_ = o.SystemctlStop(svc)
	backend := systemd.BackendServiceUnitName(domain)
	_ = o.SystemctlResetFailed(backend)
	_ = o.SystemctlStop(backend)
	_ = o.SystemctlStart(backend)
	if err := o.ensureAppSocket(domain); err != nil {
		return err
	}
	// Wake previous binary via the listening socket.
	return probeAppSocket(context.Background(), o, domain)
}

// ProbeApp ensures the site socket is listening and a request succeeds.
func (o *Ops) ProbeApp(ctx context.Context, domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	backend := systemd.BackendServiceUnitName(domain)
	_ = o.SystemctlResetFailed(backend)
	if !o.SystemctlIsActive(backend) {
		_ = o.SystemctlStart(backend) // ignore when backend unit is absent
	}
	if err := o.ensureAppSocket(domain); err != nil {
		return err
	}
	return probeAppSocket(ctx, o, domain)
}

// ensureAppSocket clears start-limit failures and makes sure the .socket unit
// is listening again after a bad activation storm.
func (o *Ops) ensureAppSocket(domain string) error {
	sock := systemd.SocketUnitName(domain)
	svc := systemd.ServiceUnitName(domain)
	backend := systemd.BackendServiceUnitName(domain)
	_ = o.SystemctlResetFailed(svc)
	_ = o.SystemctlResetFailed(sock)
	_ = o.SystemctlResetFailed(backend)
	if o.SystemctlIsActive(sock) {
		return nil
	}
	return o.SystemctlStart(sock)
}

func probeAppSocket(ctx context.Context, o *Ops, domain string) error {
	sockPath := sites.AppSocket(domain)
	svc := systemd.ServiceUnitName(domain)
	// Few attempts only: each refused/failed activation counts toward
	// systemd start-limit and can take the socket unit down.
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt > 0 {
			_ = o.SystemctlStop(svc)
			if err := o.ensureAppSocket(domain); err != nil {
				last = err
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}
		}
		last = probeUnixOnce(ctx, sockPath)
		if last == nil {
			return nil
		}
	}
	return last
}

func probeUnixOnce(ctx context.Context, sockPath string) error {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", sockPath)
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func validateAppBinaryPath(domain, binaryPath string) error {
	clean := filepath.Clean(binaryPath)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("binary_path must be absolute")
	}
	allowed := []string{
		sites.Tmp(domain) + string(filepath.Separator),
		filepath.Join("/var/lib/webor/uploads", domain) + string(filepath.Separator),
		sites.Root(domain) + string(filepath.Separator),
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(clean, prefix) {
			return nil
		}
	}
	return fmt.Errorf("binary_path %q not under allowed directories", clean)
}

func gitSync(ctx context.Context, dir, url, ref string) error {
	if ref == "" {
		ref = "HEAD"
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		cmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin", ref)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		cmd = exec.CommandContext(ctx, "git", "-C", dir, "checkout", "-f", "FETCH_HEAD")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	_ = os.RemoveAll(dir)
	args := []string{"clone", "--depth", "1"}
	if ref != "" && ref != "HEAD" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dir)
	cmd := exec.CommandContext(ctx, "git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
