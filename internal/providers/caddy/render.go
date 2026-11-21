package caddy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/sites"
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

// Render builds a Caddy JSON config for static and app sites in cfg.
// Container sites are skipped until a later milestone.
func Render(cfg *config.Config, opt RenderOptions) ([]byte, error) {
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
		hosts := append([]string{site.Name}, site.Aliases...)
		switch site.Type {
		case config.SiteTypeStatic:
			rootName := site.Root
			if rootName == "" {
				rootName = "public"
			}
			fsRoot := filepath.Join(sites.Root(site.Name), rootName)

			backendWholeHost := false
			if site.Backend != nil && site.Backend.Enabled && site.Backend.Port > 0 {
				config.NormalizeBackend(site.Backend)
				prefixes := site.Backend.BackendPathPrefixes()
				for _, p := range prefixes {
					if p == "/" {
						backendWholeHost = true
						break
					}
				}
				proxy := reverseProxyHandler(fmt.Sprintf("127.0.0.1:%d", site.Backend.Port), site.Backend.WebSocket)
				match := map[string]any{"host": hosts}
				if !backendWholeHost {
					match["path"] = config.CaddyPathMatchers(prefixes)
				}
				routes = append(routes, map[string]any{
					"match":  []map[string]any{match},
					"handle": []map[string]any{proxy},
				})
			}

			// Whole-host API backends (prefix "/") own the hostname; skip file_server.
			if !backendWholeHost {
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
		case config.SiteTypeApp:
			dial := "unix/" + sites.AppSocket(site.Name)
			routes = append(routes, map[string]any{
				"match": []map[string]any{{"host": hosts}},
				"handle": []map[string]any{
					{
						"handler": "reverse_proxy",
						"upstreams": []map[string]any{
							{"dial": dial},
						},
					},
				},
			})
		default:
			// container / unknown: skip
		}
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("no static or app sites to render")
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

func reverseProxyHandler(dial string, websocket bool) map[string]any {
	h := map[string]any{
		"handler": "reverse_proxy",
		"upstreams": []map[string]any{
			{"dial": dial},
		},
	}
	if websocket {
		// Disable response buffering so WebSocket / SSE streams flush promptly.
		h["flush_interval"] = -1
	}
	return h
}

// RenderStatic builds a Caddy JSON config for static sites only.
// Prefer Render when mixing static and app sites.
func RenderStatic(cfg *config.Config, opt RenderOptions) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	staticOnly := &config.Config{
		Version: cfg.Version,
		Panel:   cfg.Panel,
	}
	for _, s := range cfg.Sites {
		if s.Type == config.SiteTypeStatic {
			staticOnly.Sites = append(staticOnly.Sites, s)
		}
	}
	return Render(staticOnly, opt)
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

// AppSiteNames returns app site domain names from cfg.
func AppSiteNames(cfg *config.Config) []string {
	var names []string
	for _, s := range cfg.Sites {
		if s.Type == config.SiteTypeApp {
			names = append(names, strings.ToLower(s.Name))
		}
	}
	return names
}
