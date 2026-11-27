package reconcile

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/deploy"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/caddy"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

// AppApplyOptions controls the Milestone 2 app-site apply path.
type AppApplyOptions struct {
	CaddyConfigPath string
	CaddyAdminURL   string
	CaddyLastGood   string
	HTTPListen      string
	DisableHTTPS    bool
	PanelUpstream   string
	// SampleBinary, when set, is installed as the site Exec target for lab demos.
	SampleBinary string
}

// ApplyAppSites creates site users, installs units (and optional sample binary),
// enables sockets/services, renders Caddy JSON for static+app sites, and reloads Caddy.
func ApplyAppSites(ctx context.Context, cfg *config.Config, ops *helper.Ops, opt AppApplyOptions) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if ops == nil {
		ops = helper.DefaultOps()
	}
	if opt.CaddyConfigPath == "" {
		opt.CaddyConfigPath = "/etc/webor/caddy.json"
	}
	if opt.HTTPListen == "" {
		opt.HTTPListen = ":8080"
	}
	if ops.CaddyAdminURL == "" && opt.CaddyAdminURL != "" {
		ops.CaddyAdminURL = opt.CaddyAdminURL
	}
	if ops.CaddyLastGood == "" && opt.CaddyLastGood != "" {
		ops.CaddyLastGood = opt.CaddyLastGood
	}

	var applied []string
	for _, site := range cfg.Sites {
		if site.Type != config.SiteTypeApp {
			continue
		}
		if err := applyOneApp(ctx, site, ops, opt.SampleBinary); err != nil {
			return fmt.Errorf("site %s: %w", site.Name, err)
		}
		applied = append(applied, site.Name)
	}
	if len(applied) == 0 {
		return fmt.Errorf("no app sites in config")
	}

	raw, err := caddy.Render(cfg, caddy.RenderOptions{
		HTTPListen:    opt.HTTPListen,
		DisableHTTPS:  opt.DisableHTTPS,
		PanelUpstream: opt.PanelUpstream,
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(opt.CaddyConfigPath), 0o755); err != nil {
		return err
	}
	tmp := opt.CaddyConfigPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, opt.CaddyConfigPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return ops.ReloadCaddy(ctx, opt.CaddyConfigPath)
}

func applyOneApp(ctx context.Context, site config.Site, ops *helper.Ops, sampleBinary string) error {
	if err := ops.CreateSiteUser(site.Name); err != nil {
		return err
	}
	if err := os.MkdirAll(sites.SiteRuntime(site.Name), 0o755); err != nil {
		return fmt.Errorf("runtime dir: %w", err)
	}
	if err := os.MkdirAll(sites.BinDir(site.Name), 0o755); err != nil {
		return fmt.Errorf("bin dir: %w", err)
	}
	// Site user must own the runtime dir so socket_proxy backends can bind WBR_SOCK.
	uEarly := sites.Username(site.Name)
	if out, err := exec.Command("chown", uEarly+":"+uEarly, sites.SiteRuntime(site.Name)).CombinedOutput(); err != nil {
		return fmt.Errorf("chown runtime: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	units, err := systemd.RenderAppUnits(site)
	if err != nil {
		return err
	}
	if units.SocketName != "" {
		_ = ops.SystemctlStop(units.ServiceName)
		_ = ops.SystemctlStop(units.SocketName)
		if units.BackendServiceName != "" {
			_ = ops.SystemctlStop(units.BackendServiceName)
		}
	}
	unitFiles := map[string]string{units.ServiceName: units.Service}
	if units.SocketName != "" {
		unitFiles[units.SocketName] = units.Socket
	}
	if units.BackendServiceName != "" {
		unitFiles[units.BackendServiceName] = units.BackendService
	}
	for name, body := range unitFiles {
		if err := ops.WriteUnitFileNoReload(site.Name, name, body); err != nil {
			return err
		}
	}
	if err := ops.SystemctlDaemonReload(); err != nil {
		return err
	}

	u := sites.Username(site.Name)
	cmd := exec.Command("chown", "-R", u+":"+u, sites.Root(site.Name))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Install sample binary before enabling units so proxyd backends have a
	// working ExecStart on first start (avoids start-limit storms).
	if sampleBinary != "" {
		staged := filepath.Join(sites.Tmp(site.Name), sites.DefaultAppBinary)
		if err := installSampleBinary(sampleBinary, staged); err != nil {
			return err
		}
		id, _, err := deploy.InstallAppBinary(site.Name, staged, deploy.AppBinaryOptions{})
		if err != nil {
			return err
		}
		if err := sites.ActivateAppRelease(site.Name, id, sites.DefaultAppBinary); err != nil {
			return err
		}
		_ = exec.Command("chown", "-R", u+":"+u, sites.Root(site.Name)).Run()
	}

	if units.BackendServiceName != "" {
		_ = ops.SystemctlResetFailed(units.BackendServiceName)
		if err := ops.SystemctlEnableNow(units.BackendServiceName); err != nil {
			return err
		}
	}
	if units.SocketName != "" {
		_ = ops.SystemctlResetFailed(units.SocketName)
		_ = ops.SystemctlResetFailed(units.ServiceName)
		if err := ops.SystemctlEnableNow(units.SocketName); err != nil {
			return err
		}
	} else if err := ops.SystemctlEnableNow(units.ServiceName); err != nil {
		return err
	}

	if err := ops.InstallSiteCron(site); err != nil {
		return err
	}
	if err := ops.InstallSiteJobs(site); err != nil {
		return err
	}

	if sampleBinary == "" {
		return nil
	}
	// Wake via public socket (native activation or proxyd) to verify the release.
	return ops.ProbeApp(ctx, site.Name)
}

func installSampleBinary(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(dest, 0o755)
}
