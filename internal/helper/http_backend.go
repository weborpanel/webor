package helper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/deploy"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

// EnsureHTTPBackendResult is returned after installing/starting the backend unit.
type EnsureHTTPBackendResult struct {
	Unit string `json:"unit"`
	Port int    `json:"port"`
}

// EnsureHTTPBackend writes the systemd unit and enables it for a static site backend.
func (o *Ops) EnsureHTTPBackend(site config.Site) (*EnsureHTTPBackendResult, error) {
	if site.Backend == nil || !site.Backend.Enabled {
		return nil, fmt.Errorf("backend not enabled")
	}
	config.NormalizeBackend(site.Backend)
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	// ReadWritePaths includes SiteRuntime; systemd NAMESPACE fails if it is missing.
	if err := os.MkdirAll(sites.SiteRuntime(site.Name), 0o755); err != nil {
		return nil, fmt.Errorf("runtime dir: %w", err)
	}
	unit, body, err := systemd.RenderHTTPBackendUnit(site)
	if err != nil {
		return nil, err
	}
	if err := o.WriteUnitFile(site.Name, unit, body); err != nil {
		return nil, err
	}
	_ = o.SystemctlResetFailed(unit)
	cmd := exec.Command("systemctl", "enable", unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("enable %s: %w (%s)", unit, err, strings.TrimSpace(string(out)))
	}
	// Start may fail until a binary is deployed; unit is still installed.
	_ = o.SystemctlStart(unit)
	return &EnsureHTTPBackendResult{Unit: unit, Port: site.Backend.Port}, nil
}

// RemoveHTTPBackend stops and removes the backend unit for a domain.
func (o *Ops) RemoveHTTPBackend(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	unit := systemd.HTTPBackendUnitName(domain)
	_ = o.SystemctlStop(unit)
	cmd := exec.Command("systemctl", "disable", unit)
	_, _ = cmd.CombinedOutput()
	path := filepath.Join("/etc/systemd/system", unit)
	_ = os.Remove(path)
	_ = o.SystemctlDaemonReload()
	_ = o.SystemctlResetFailed(unit)
	return nil
}

// HTTPBackendActive reports whether the backend unit is active.
func (o *Ops) HTTPBackendActive(domain string) bool {
	return o.SystemctlIsActive(systemd.HTTPBackendUnitName(domain))
}

// DeployHTTPBackend installs a binary release and restarts the HTTP backend unit.
func (o *Ops) DeployHTTPBackend(ctx context.Context, domain, binaryPath string) (*DeployAppBinaryResult, error) {
	_ = ctx
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

	unit := systemd.HTTPBackendUnitName(domain)
	_ = o.SystemctlResetFailed(unit)
	if err := o.SystemctlTryRestart(unit); err != nil {
		return nil, fmt.Errorf("restart backend: %w", err)
	}
	// Brief settle; binary may exit immediately if misconfigured — caller can check active.
	time.Sleep(200 * time.Millisecond)
	return &DeployAppBinaryResult{ReleaseID: id, Binary: dest}, nil
}
