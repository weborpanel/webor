package systemd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/sites"
)

// HTTPBackendUnitPrefix is applied to static-site HTTP backend units.
const HTTPBackendUnitPrefix = "webor-backend-"

// HTTPBackendUnitName returns webor-backend-<domain>-<id>.service.
func HTTPBackendUnitName(domain, id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		id = "api"
	}
	return HTTPBackendUnitPrefix + UnitBase(domain) + "-" + UnitBase(id) + ".service"
}

// RenderHTTPBackendUnit builds an always-on systemd unit for one HTTP backend on a static site.
func RenderHTTPBackendUnit(site config.Site, backend config.SiteBackend) (unitName string, content string, err error) {
	return RenderHTTPBackendUnitWithNode(site, backend, "")
}

// RenderHTTPBackendUnitWithNode is like RenderHTTPBackendUnit but allows a test override for the node binary.
func RenderHTTPBackendUnitWithNode(site config.Site, backend config.SiteBackend, nodeBinOverride string) (unitName string, content string, err error) {
	if site.Type != config.SiteTypeStatic {
		return "", "", fmt.Errorf("site %s: type %q (want static)", site.Name, site.Type)
	}
	if !backend.Enabled {
		return "", "", fmt.Errorf("site %s: backend not enabled", site.Name)
	}
	config.NormalizeBackend(&backend)
	if backend.Port < config.BackendPortMin || backend.Port > config.BackendPortMax {
		return "", "", fmt.Errorf("site %s backend %s: invalid port %d", site.Name, backend.ID, backend.Port)
	}

	appExec, err := ResolveBackendExec(site, backend, nodeBinOverride)
	if err != nil {
		return "", "", err
	}

	user := sites.Username(site.Name)
	siteRoot := sites.Root(site.Name)
	apiRoot := sites.APIs(site.Name, backend.ID)
	name := HTTPBackendUnitName(site.Name, backend.ID)

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webor HTTP backend %s (%s)\n", site.Name, backend.ID)
	fmt.Fprintf(&svc, "After=network.target\n")
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=simple\n")
	fmt.Fprintf(&svc, "User=%s\n", user)
	fmt.Fprintf(&svc, "Group=%s\n", user)
	fmt.Fprintf(&svc, "WorkingDirectory=%s\n", apiRoot)
	fmt.Fprintf(&svc, "Environment=HOST=127.0.0.1\n")
	fmt.Fprintf(&svc, "Environment=PORT=%d\n", backend.Port)
	fmt.Fprintf(&svc, "Environment=WBR_PORT=%d\n", backend.Port)
	if appExec.PathEnv != "" {
		fmt.Fprintf(&svc, "Environment=PATH=%s:/usr/local/bin:/usr/bin:/bin\n", appExec.PathEnv)
	}
	if site.EnvFile != "" {
		env := site.EnvFile
		if !filepath.IsAbs(env) {
			env = filepath.Join(siteRoot, env)
		}
		fmt.Fprintf(&svc, "EnvironmentFile=-%s\n", env)
	}
	fmt.Fprintf(&svc, "ExecStart=%s\n", appExec.ExecStart)
	fmt.Fprintf(&svc, "Restart=on-failure\n")
	fmt.Fprintf(&svc, "RestartSec=2s\n")
	appendHardening(&svc, site, siteRoot, logging.SiteCurrentPath(site.Name, logging.APIStream(backend.ID)))
	fmt.Fprintf(&svc, "\n[Install]\n")
	fmt.Fprintf(&svc, "WantedBy=multi-user.target\n")

	return name, svc.String(), nil
}
