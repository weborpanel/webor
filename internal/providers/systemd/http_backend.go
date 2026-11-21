package systemd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/sites"
)

// HTTPBackendUnitPrefix is applied to static-site HTTP backend units.
const HTTPBackendUnitPrefix = "webor-backend-"

// HTTPBackendUnitName returns webor-backend-<domain>.service.
func HTTPBackendUnitName(domain string) string {
	return HTTPBackendUnitPrefix + UnitBase(domain) + ".service"
}

// RenderHTTPBackendUnit builds an always-on systemd unit for a static site's HTTP backend.
// The process receives PORT, WBR_PORT, and HOST=127.0.0.1.
func RenderHTTPBackendUnit(site config.Site) (unitName string, content string, err error) {
	if site.Type != config.SiteTypeStatic {
		return "", "", fmt.Errorf("site %s: type %q (want static)", site.Name, site.Type)
	}
	if site.Backend == nil || !site.Backend.Enabled {
		return "", "", fmt.Errorf("site %s: backend not enabled", site.Name)
	}
	config.NormalizeBackend(site.Backend)
	if site.Backend.Port < config.BackendPortMin || site.Backend.Port > config.BackendPortMax {
		return "", "", fmt.Errorf("site %s: invalid backend port %d", site.Name, site.Backend.Port)
	}
	execPath := strings.TrimSpace(site.Backend.Exec)
	if execPath == "" {
		return "", "", fmt.Errorf("site %s: backend.exec is required", site.Name)
	}
	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(sites.Root(site.Name), execPath)
	}

	user := sites.Username(site.Name)
	root := sites.Root(site.Name)
	name := HTTPBackendUnitName(site.Name)

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webor HTTP backend %s\n", site.Name)
	fmt.Fprintf(&svc, "After=network.target\n")
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=simple\n")
	fmt.Fprintf(&svc, "User=%s\n", user)
	fmt.Fprintf(&svc, "Group=%s\n", user)
	fmt.Fprintf(&svc, "WorkingDirectory=%s\n", root)
	fmt.Fprintf(&svc, "Environment=HOST=127.0.0.1\n")
	fmt.Fprintf(&svc, "Environment=PORT=%d\n", site.Backend.Port)
	fmt.Fprintf(&svc, "Environment=WBR_PORT=%d\n", site.Backend.Port)
	if site.EnvFile != "" {
		env := site.EnvFile
		if !filepath.IsAbs(env) {
			env = filepath.Join(root, env)
		}
		fmt.Fprintf(&svc, "EnvironmentFile=-%s\n", env)
	}
	fmt.Fprintf(&svc, "ExecStart=%s\n", execPath)
	fmt.Fprintf(&svc, "Restart=on-failure\n")
	fmt.Fprintf(&svc, "RestartSec=2s\n")
	appendHardening(&svc, site, root)
	fmt.Fprintf(&svc, "\n[Install]\n")
	fmt.Fprintf(&svc, "WantedBy=multi-user.target\n")

	return name, svc.String(), nil
}
