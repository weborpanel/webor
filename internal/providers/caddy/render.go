package caddy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/sites"
)

// RenderOptions controls how config.yaml is turned into Caddy JSON.
type RenderOptions struct {
	// HTTPListen is the bind address for the shared HTTP server (lab default ":8080").
	HTTPListen string
	// AdminListen defaults to 127.0.0.1:2019.
	AdminListen string
	// DisableHTTPS turns off automatic HTTPS (lab / no DNS).
	DisableHTTPS bool
}

// RenderStatic builds a Caddy JSON config for static sites in cfg.
// App/container sites are skipped until later milestones.
func RenderStatic(cfg *config.Config, opt RenderOptions) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	listen := opt.HTTPListen
	if listen == "" {
		listen = ":8080"
	}
	admin := opt.AdminListen
	if admin == "" {
		admin = "127.0.0.1:2019"
	}

	var routes []map[string]any
	for _, site := range cfg.Sites {
		if site.Type != config.SiteTypeStatic {
			continue
		}
		rootName := site.Root
		if rootName == "" {
			rootName = "public"
		}
		fsRoot := filepath.Join(sites.Root(site.Name), rootName)
		hosts := append([]string{site.Name}, site.Aliases...)
		routes = append(routes, map[string]any{
			"match": []map[string]any{{"host": hosts}},
			"handle": []map[string]any{
				{
					"handler": "file_server",
					"root":    fsRoot,
				},
			},
		})
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("no static sites to render")
	}

	server := map[string]any{
		"listen": []string{listen},
		"routes": routes,
	}
	if opt.DisableHTTPS {
		server["automatic_https"] = map[string]any{"disable": true}
	}

	doc := map[string]any{
		"admin": map[string]any{"listen": admin},
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"srv0": server,
				},
			},
		},
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// StaticSiteNames returns static site domain names from cfg.
func StaticSiteNames(cfg *config.Config) []string {
	var names []string
	for _, s := range cfg.Sites {
		if s.Type == config.SiteTypeStatic {
			names = append(names, strings.ToLower(s.Name))
		}
	}
	return names
}
