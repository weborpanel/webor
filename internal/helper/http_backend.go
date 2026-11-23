package helper

import (
	"context"
	"fmt"
	"io"
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
	ID   string `json:"id,omitempty"`
}

// ensureSiteRuntimeDir creates /run/webor/sites/<domain> owned by the site user.
// Required before starting units with ProtectSystem=strict + ReadWritePaths on that path
// (NAMESPACE fails with "No such file or directory" if the path is missing).
func (o *Ops) ensureSiteRuntimeDir(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	dir := sites.SiteRuntime(domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("runtime dir: %w", err)
	}
	u := sites.Username(domain)
	if out, err := exec.Command("chown", u+":"+u, dir).CombinedOutput(); err != nil {
		return fmt.Errorf("chown runtime: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureHTTPBackend writes the systemd unit and enables it for one static-site backend.
func (o *Ops) EnsureHTTPBackend(site config.Site, backend config.SiteBackend) (*EnsureHTTPBackendResult, error) {
	if !backend.Enabled {
		return nil, fmt.Errorf("backend not enabled")
	}
	config.NormalizeBackend(&backend)
	if err := o.CreateSiteUser(site.Name); err != nil {
		return nil, err
	}
	if err := o.ensureSiteRuntimeDir(site.Name); err != nil {
		return nil, err
	}
	if err := sites.EnsureAPIsDir(site.Name, backend.ID); err != nil {
		return nil, fmt.Errorf("apis dir: %w", err)
	}
	unit, body, err := systemd.RenderHTTPBackendUnit(site, backend)
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
	_ = o.SystemctlStart(unit)
	return &EnsureHTTPBackendResult{Unit: unit, Port: backend.Port, ID: backend.ID}, nil
}

// RemoveHTTPBackend stops and removes the backend unit for a domain + backend id.
func (o *Ops) RemoveHTTPBackend(domain, id string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.ToLower(strings.TrimSpace(id))
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	if id == "" {
		id = "api"
	}
	unit := systemd.HTTPBackendUnitName(domain, id)
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
func (o *Ops) HTTPBackendActive(domain, id string) bool {
	return o.SystemctlIsActive(systemd.HTTPBackendUnitName(domain, id))
}

// RestartHTTPBackend resets failed state and try-restarts the backend unit.
func (o *Ops) RestartHTTPBackend(domain, id string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.ToLower(strings.TrimSpace(id))
	if domain == "" || id == "" {
		return fmt.Errorf("domain and id are required")
	}
	if err := o.ensureSiteRuntimeDir(domain); err != nil {
		return err
	}
	unit := systemd.HTTPBackendUnitName(domain, id)
	_ = o.SystemctlResetFailed(unit)
	if err := o.SystemctlTryRestart(unit); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

// DeployHTTPBackendResult is returned after installing a binary into apis/<id>/server.
type DeployHTTPBackendResult struct {
	Binary string `json:"binary"`
	ID     string `json:"id,omitempty"`
}

// DeployHTTPBackend installs a binary as apis/<id>/server and restarts the unit.
func (o *Ops) DeployHTTPBackend(ctx context.Context, domain, id, binaryPath string) (*DeployHTTPBackendResult, error) {
	_ = ctx
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.ToLower(strings.TrimSpace(id))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if id == "" {
		return nil, fmt.Errorf("backend id is required")
	}
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return nil, fmt.Errorf("binary path is required")
	}
	if err := o.CreateSiteUser(domain); err != nil {
		return nil, err
	}
	if err := o.ensureSiteRuntimeDir(domain); err != nil {
		return nil, err
	}
	if err := sites.EnsureAPIsDir(domain, id); err != nil {
		return nil, err
	}
	dest := filepath.Join(sites.APIs(domain, id), sites.DefaultAppBinary)
	tmp := dest + ".tmp"
	src, err := os.Open(binaryPath)
	if err != nil {
		return nil, err
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		_ = src.Close()
		return nil, err
	}
	_, copyErr := io.Copy(out, src)
	_ = src.Close()
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return nil, closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	_ = os.Chmod(dest, 0o755)

	u := sites.Username(domain)
	cmd := exec.Command("chown", "-R", u+":"+u, sites.APIs(domain, id))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("chown: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	unit := systemd.HTTPBackendUnitName(domain, id)
	_ = o.SystemctlResetFailed(unit)
	if err := o.SystemctlTryRestart(unit); err != nil {
		return nil, fmt.Errorf("restart backend: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	return &DeployHTTPBackendResult{Binary: dest, ID: id}, nil
}

// DeployHTTPBackendZipResult is returned after extracting a zip into apis/<id>/.
type DeployHTTPBackendZipResult struct {
	Root string `json:"root"`
	ID   string `json:"id,omitempty"`
}

// DeployHTTPBackendZip extracts a zip into apis/<id>/ and restarts the backend unit.
func (o *Ops) DeployHTTPBackendZip(ctx context.Context, domain, id, zipPath string) (*DeployHTTPBackendZipResult, error) {
	_ = ctx
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.ToLower(strings.TrimSpace(id))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if id == "" {
		return nil, fmt.Errorf("backend id is required")
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
	root := sites.APIs(domain, id)
	if err := sites.EnsureAPIsDir(domain, id); err != nil {
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
	unit := systemd.HTTPBackendUnitName(domain, id)
	_ = o.SystemctlResetFailed(unit)
	if err := o.SystemctlTryRestart(unit); err != nil {
		return nil, fmt.Errorf("restart backend: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	return &DeployHTTPBackendZipResult{Root: root, ID: id}, nil
}

// ListAPIDir lists a directory under apis/<id>/.
func (o *Ops) ListAPIDir(domain, id, rel string) ([]PublicFSEntry, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = strings.ToLower(strings.TrimSpace(id))
	if err := sites.EnsureAPIsDir(domain, id); err != nil {
		return nil, err
	}
	dir, err := sites.ResolveAPIsRel(domain, id, rel)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]PublicFSEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		typ := "file"
		if e.IsDir() {
			typ = "dir"
		}
		out = append(out, PublicFSEntry{
			Name:  e.Name(),
			Type:  typ,
			Size:  info.Size(),
			Mtime: info.ModTime().Unix(),
			Mode:  info.Mode().Perm().String(),
		})
	}
	return out, nil
}
