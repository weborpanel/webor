// Package systemd renders systemd unit files for Webooza-managed workloads.
package systemd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/sites"
)

// AppUnits holds rendered unit basenames and file bodies for one app site.
type AppUnits struct {
	ServiceName string
	SocketName  string // empty when socket activation is off
	Service     string
	Socket      string // empty when socket activation is off
}

// UnitPrefix is applied to all generated unit basenames.
const UnitPrefix = "webooza-app-"

// UnitBase returns a systemd-safe basename fragment from a domain
// (e.g. app.example.com → app-example-com).
func UnitBase(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, ".", "-")
	d = strings.ReplaceAll(d, "_", "-")
	var b strings.Builder
	for _, r := range d {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "site"
	}
	return out
}

// ServiceUnitName returns webooza-app-<domain>.service.
func ServiceUnitName(domain string) string {
	return UnitPrefix + UnitBase(domain) + ".service"
}

// SocketUnitName returns webooza-app-<domain>.socket.
func SocketUnitName(domain string) string {
	return UnitPrefix + UnitBase(domain) + ".socket"
}

// RenderAppUnits builds .service and optional .socket contents for a type:app site.
func RenderAppUnits(site config.Site) (AppUnits, error) {
	if site.Type != config.SiteTypeApp {
		return AppUnits{}, fmt.Errorf("site %s: type %q (want app)", site.Name, site.Type)
	}
	execPath := strings.TrimSpace(site.Exec)
	if execPath == "" {
		return AppUnits{}, fmt.Errorf("site %s: exec is required", site.Name)
	}
	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(sites.Root(site.Name), execPath)
	}

	user := sites.Username(site.Name)
	sock := sites.AppSocket(site.Name)
	svcName := ServiceUnitName(site.Name)
	root := sites.Root(site.Name)

	var svc strings.Builder
	fmt.Fprintf(&svc, "[Unit]\n")
	fmt.Fprintf(&svc, "Description=Webooza app %s\n", site.Name)
	if site.SocketActivation {
		sockName := SocketUnitName(site.Name)
		fmt.Fprintf(&svc, "Requires=%s\n", sockName)
		fmt.Fprintf(&svc, "After=%s\n", sockName)
	} else {
		fmt.Fprintf(&svc, "After=network.target\n")
	}
	fmt.Fprintf(&svc, "\n[Service]\n")
	fmt.Fprintf(&svc, "Type=simple\n")
	fmt.Fprintf(&svc, "User=%s\n", user)
	fmt.Fprintf(&svc, "Group=%s\n", user)
	fmt.Fprintf(&svc, "WorkingDirectory=%s\n", root)
	fmt.Fprintf(&svc, "ExecStart=%s\n", execPath)
	if site.EnvFile != "" {
		env := site.EnvFile
		if !filepath.IsAbs(env) {
			env = filepath.Join(root, env)
		}
		fmt.Fprintf(&svc, "EnvironmentFile=-%s\n", env)
	}
	if site.Limits.MemoryMax != "" {
		fmt.Fprintf(&svc, "MemoryMax=%s\n", site.Limits.MemoryMax)
	}
	if site.SocketActivation {
		idle := strings.TrimSpace(site.IdleTimeout)
		if idle != "" {
			// Cooperative apps exit after idle; socket unit stays up for wake-on-request.
			fmt.Fprintf(&svc, "Environment=WBZ_IDLE_TIMEOUT=%s\n", idle)
		}
	}
	// Restart=no so an intentional idle exit scales to zero instead of looping.
	fmt.Fprintf(&svc, "Restart=no\n")
	fmt.Fprintf(&svc, "NoNewPrivileges=true\n")
	fmt.Fprintf(&svc, "ProtectSystem=strict\n")
	fmt.Fprintf(&svc, "ProtectHome=true\n")
	fmt.Fprintf(&svc, "PrivateTmp=true\n")
	fmt.Fprintf(&svc, "ReadWritePaths=%s %s\n", root, sites.SiteRuntime(site.Name))
	fmt.Fprintf(&svc, "\n[Install]\n")
	fmt.Fprintf(&svc, "WantedBy=multi-user.target\n")

	out := AppUnits{
		ServiceName: svcName,
		Service:     svc.String(),
	}

	if !site.SocketActivation {
		return out, nil
	}

	sockName := SocketUnitName(site.Name)
	var sk strings.Builder
	fmt.Fprintf(&sk, "[Unit]\n")
	fmt.Fprintf(&sk, "Description=Webooza app socket %s\n", site.Name)
	// Do not PartOf= the service: stopping the app for idle/wake must leave
	// the listening socket up so the next request can activate it again.
	fmt.Fprintf(&sk, "\n[Socket]\n")
	fmt.Fprintf(&sk, "ListenStream=%s\n", sock)
	fmt.Fprintf(&sk, "SocketMode=0666\n")
	fmt.Fprintf(&sk, "DirectoryMode=0755\n")
	fmt.Fprintf(&sk, "\n[Install]\n")
	fmt.Fprintf(&sk, "WantedBy=sockets.target\n")

	out.SocketName = sockName
	out.Socket = sk.String()
	return out, nil
}
