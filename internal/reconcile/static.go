package reconcile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/caddy"
	"github.com/weborpanel/webor/internal/sites"
)

// StaticApplyOptions controls the Milestone 0 static-site apply path.
type StaticApplyOptions struct {
	CaddyConfigPath string // e.g. /etc/webor/caddy.json
	CaddyAdminURL   string
	CaddyLastGood   string
	HTTPListen      string
	DisableHTTPS    bool
	// PanelUpstream dials the panel process for panel.domain (e.g. 127.0.0.1:21215).
	PanelUpstream string
	// SeedIndex writes a demo index.html when public/ is empty.
	SeedIndex bool
	IndexBody string
}

// ApplyStaticSites creates site users/dirs for static sites, optionally seeds
// index.html, renders Caddy JSON, and reloads Caddy via the helper ops surface.
func ApplyStaticSites(ctx context.Context, cfg *config.Config, ops helper.SiteFS, opt StaticApplyOptions) error {
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
	if local, ok := ops.(*helper.Ops); ok {
		if local.CaddyAdminURL == "" && opt.CaddyAdminURL != "" {
			local.CaddyAdminURL = opt.CaddyAdminURL
		}
		if local.CaddyLastGood == "" && opt.CaddyLastGood != "" {
			local.CaddyLastGood = opt.CaddyLastGood
		}
	}

	for _, site := range cfg.Sites {
		if site.Type != config.SiteTypeStatic {
			continue
		}
		if err := ops.CreateSiteUser(site.Name); err != nil {
			return fmt.Errorf("site %s: %w", site.Name, err)
		}
		if opt.SeedIndex {
			// CreateSiteUser already seeds index.html when missing. Keep a local
			// fallback for in-process Ops when the site root uses a custom path.
			rootName := site.Root
			if rootName == "" {
				rootName = "public"
			}
			pub := sites.Public(site.Name)
			if rootName != "public" {
				pub = filepath.Join(sites.Root(site.Name), rootName)
			}
			body := opt.IndexBody
			if body == "" {
				body = sites.DefaultIndexHTML(site.Name)
			}
			if err := seedIndexIfMissing(pub, body); err != nil {
				// Non-fatal when panel cannot write /srv/sites (helper already seeded).
				_ = err
			}
		}
	}

	// Full Render so app sites stay in Caddy when the panel reloads after HTTPS/DNS changes.
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

func seedIndexIfMissing(publicDir, body string) error {
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		return err
	}
	index := filepath.Join(publicDir, "index.html")
	if _, err := os.Stat(index); err == nil {
		return nil
	}
	if err := os.WriteFile(index, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Chmod(index, 0o644)
}
