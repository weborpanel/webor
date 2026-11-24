package caddy

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/logging"
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
	// PanelUpstream is the dial address for panel.domain (e.g. 127.0.0.1:21215).
	// When empty, PanelDial(cfg.Panel.Port) is used if panel.domain is set.
	PanelUpstream string
}

// PanelDial builds a loopback dial target for the panel reverse proxy.
func PanelDial(panelListen string, port int) (string, error) {
	listen := strings.TrimSpace(panelListen)
	if listen != "" {
		if _, p, err := net.SplitHostPort(listen); err == nil && p != "" {
			return net.JoinHostPort("127.0.0.1", p), nil
		}
		if strings.HasPrefix(listen, ":") {
			return "127.0.0.1" + listen, nil
		}
	}
	if port > 0 && port <= 65535 {
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
	}
	return "", fmt.Errorf("panel upstream port unknown")
}

// Render builds a Caddy JSON config for static and app sites in cfg.
// Container sites are skipped until a later milestone.
// When panel.domain is set, a reverse_proxy route for the panel is prepended.
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
	loggerNames := map[string]string{}
	accessLoggers := map[string]any{}

	if domain := strings.TrimSpace(cfg.Panel.Domain); domain != "" {
		upstream := strings.TrimSpace(opt.PanelUpstream)
		if upstream == "" {
			var err error
			upstream, err = PanelDial("", cfg.Panel.Port)
			if err != nil {
				return nil, fmt.Errorf("panel.domain set but %w", err)
			}
		}
		routes = append(routes, map[string]any{
			"match":  []map[string]any{{"host": []string{domain}}},
			"handle": []map[string]any{reverseProxyHandler(upstream, true)},
		})
	}
	for _, site := range cfg.Sites {
		hosts := append([]string{site.Name}, site.Aliases...)
		if site.Type == config.SiteTypeStatic || site.Type == config.SiteTypeApp {
			loggerID := accessLoggerID(site.Name)
			accessPath := logging.SiteCurrentPath(site.Name, logging.StreamAccess)
			accessLoggers[loggerID] = map[string]any{
				"writer": map[string]any{
					"output":   "file",
					"filename": accessPath,
				},
				"encoder": map[string]any{"format": "json"},
				"include": []string{"http.log.access." + loggerID},
			}
			for _, h := range hosts {
				loggerNames[h] = loggerID
			}
		}
		switch site.Type {
		case config.SiteTypeStatic:
			rootName := site.Root
			if rootName == "" {
				rootName = "public"
			}
			fsRoot := filepath.Join(sites.Root(site.Name), rootName)

			backendWholeHost := false
			for i := range site.Backends {
				b := &site.Backends[i]
				if !b.Enabled || b.Port <= 0 {
					continue
				}
				config.NormalizeBackend(b)
				prefixes := b.BackendPathPrefixes()
				whole := false
				for _, p := range prefixes {
					if p == "/" {
						whole = true
						backendWholeHost = true
						break
					}
				}
				proxy := reverseProxyHandler(fmt.Sprintf("127.0.0.1:%d", b.Port), b.WebSocket)
				match := map[string]any{"host": hosts}
				if !whole {
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
		return nil, fmt.Errorf("no panel domain or static/app sites to render")
	}

	server := map[string]any{
		"listen": []string{listen},
		"routes": routes,
	}
	if opt.DisableHTTPS {
		server["automatic_https"] = map[string]any{"disable": true}
	}
	if len(loggerNames) > 0 {
		server["logs"] = map[string]any{
			"default_logger_name": "discard_access",
			"logger_names":        loggerNames,
		}
		accessLoggers["discard_access"] = map[string]any{
			"writer":  map[string]any{"output": "discard"},
			"include": []string{"http.log.access.discard_access"},
		}
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
	if len(accessLoggers) > 0 {
		doc["logging"] = map[string]any{
			"logs": accessLoggers,
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func accessLoggerID(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, ".", "_")
	d = strings.ReplaceAll(d, "-", "_")
	var b strings.Builder
	for _, r := range d {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "site"
	}
	return "site_" + out
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
