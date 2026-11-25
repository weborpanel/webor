package helper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/providers/caddy"
	"github.com/weborpanel/webor/internal/unitfiles"
	"github.com/weborpanel/webor/internal/version"
)

const migrateStampName = "post-update-migrated"

// EnsureSystemUnits refreshes host systemd units from the embedded copies
// (ReadWritePaths, hardening). Restarts Caddy when webor-caddy.service changes
// so new sandbox paths take effect.
func (o *Ops) EnsureSystemUnits() error {
	_, err := unitfiles.Install(unitfiles.InstallOptions{RestartCaddy: true})
	return err
}

// PostUpdateMigrateIfNeeded runs PostUpdateMigrate once per binary version
// (stamp under state dir). Host units are still refreshed every call via
// EnsureSystemUnits.
func (o *Ops) PostUpdateMigrateIfNeeded(cfgPath, stateDir string) error {
	if o == nil {
		o = DefaultOps()
	}
	_ = o.EnsureSystemUnits()
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	ver := strings.TrimSpace(version.Version)
	if ver == "" {
		ver = "unknown"
	}
	stamp := filepath.Join(stateDir, migrateStampName)
	if prev, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(prev)) == ver {
		return nil
	}
	if err := o.PostUpdateMigrate(cfgPath); err != nil {
		return err
	}
	_ = os.MkdirAll(stateDir, 0o755)
	return os.WriteFile(stamp, []byte(ver+"\n"), 0o644)
}

// PostUpdateMigrate refreshes site units and Caddy after a binary upgrade.
// It rewrites systemd units (file log append), ensures log dirs, and re-renders
// caddy.json so access.log sinks exist for existing sites.
func (o *Ops) PostUpdateMigrate(cfgPath string) error {
	if o == nil {
		o = DefaultOps()
	}
	_ = o.EnsureSystemUnits()
	_ = logging.EnsureHostDir()
	_ = o.EnsureLogrotateSchedule()

	cfgPath = strings.TrimSpace(cfgPath)
	if cfgPath == "" {
		cfgPath = "/etc/webor/config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load config: %w", err)
	}

	var first error
	for _, site := range cfg.Sites {
		_ = logging.EnsureSiteLogsDir(site.Name)
		_ = o.EnsureSiteLogStreams(site)
		if err := o.ApplySiteLimits(site); err != nil {
			if first == nil {
				first = fmt.Errorf("site %s units: %w", site.Name, err)
			}
		}
	}

	disableHTTPS, httpListen, panelUpstream := postUpdateCaddyOpts(cfg)
	raw, err := caddy.Render(cfg, caddy.RenderOptions{
		HTTPListen:    httpListen,
		DisableHTTPS:  disableHTTPS,
		PanelUpstream: panelUpstream,
	})
	if err != nil {
		if first == nil {
			first = fmt.Errorf("caddy render: %w", err)
		}
		return first
	}
	caddyPath := "/etc/webor/caddy.json"
	if err := os.MkdirAll(filepath.Dir(caddyPath), 0o755); err != nil {
		if first == nil {
			first = err
		}
		return first
	}
	tmp := caddyPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		if first == nil {
			first = err
		}
		return first
	}
	if err := os.Rename(tmp, caddyPath); err != nil {
		_ = os.Remove(tmp)
		if first == nil {
			first = err
		}
		return first
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := o.ReloadCaddy(ctx, caddyPath); err != nil {
		if first == nil {
			first = fmt.Errorf("caddy reload: %w", err)
		}
	}
	return first
}

func postUpdateCaddyOpts(cfg *config.Config) (disableHTTPS bool, httpListen, panelUpstream string) {
	disableHTTPS = true
	httpListen = ":8080"
	anyHTTPS := false
	if cfg != nil {
		for _, s := range cfg.Sites {
			if s.HTTPS {
				anyHTTPS = true
				break
			}
		}
	}
	labOff, listenAddr, panelPort := readPanelEnvCaddyHints("/etc/webor/panel.env")
	if labOff {
		anyHTTPS = false
	}
	if anyHTTPS {
		disableHTTPS = false
		httpListen = ":443"
	}
	if listenAddr != "" {
		if up, err := caddy.PanelDial(listenAddr, 0); err == nil {
			panelUpstream = up
		}
	}
	if panelUpstream == "" && cfg != nil && strings.TrimSpace(cfg.Panel.Domain) != "" {
		port := cfg.Panel.Port
		if panelPort > 0 {
			port = panelPort
		}
		if up, err := caddy.PanelDial("", port); err == nil {
			panelUpstream = up
		}
	}
	return disableHTTPS, httpListen, panelUpstream
}

func readPanelEnvCaddyHints(path string) (disableHTTPS bool, listenAddr string, panelPort int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, "", 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		key, val, ok := strings.Cut(trim, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "WBR_PANEL_ARGS":
			fields := strings.Fields(val)
			for i, f := range fields {
				switch {
				case f == "-disable-https" || f == "-disable-https=true" || f == "-disable-https=1":
					disableHTTPS = true
				case strings.HasPrefix(f, "-disable-https="):
					v := strings.TrimPrefix(f, "-disable-https=")
					if v == "true" || v == "1" || v == "yes" {
						disableHTTPS = true
					}
				case f == "-listen" && i+1 < len(fields):
					listenAddr = fields[i+1]
				case strings.HasPrefix(f, "-listen="):
					listenAddr = strings.TrimPrefix(f, "-listen=")
				}
			}
		case "WBR_PANEL_PORT":
			var p int
			if _, err := fmt.Sscanf(val, "%d", &p); err == nil && p > 0 {
				panelPort = p
			}
		}
	}
	return disableHTTPS, listenAddr, panelPort
}
