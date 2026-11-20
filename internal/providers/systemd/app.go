// Package systemd renders systemd unit files for Webooza-managed workloads.
package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/sites"
)

// AppUnits holds rendered unit basenames and file bodies for one app site.
type AppUnits struct {
	ServiceName        string // activated service (app or proxyd)
	SocketName         string // empty when socket activation is off
	BackendServiceName string // nonempty when socket_proxy
	Service            string
	Socket             string
	BackendService     string
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

// BackendServiceUnitName returns webooza-app-<domain>-backend.service.
func BackendServiceUnitName(domain string) string {
	return UnitPrefix + UnitBase(domain) + "-backend.service"
}

// FindSocketProxyd returns the path to systemd-socket-proxyd.
func FindSocketProxyd() (string, error) {
	if p, err := exec.LookPath("systemd-socket-proxyd"); err == nil {
		return p, nil
	}
	for _, p := range []string{
		"/lib/systemd/systemd-socket-proxyd",
		"/usr/lib/systemd/systemd-socket-proxyd",
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("systemd-socket-proxyd not found")
}

// IdleTimeoutToProxydFlag converts a Go duration string to a systemd time span
// suitable for --exit-idle-time= (e.g. 3s → 3s, 15m → 15min).
func IdleTimeoutToProxydFlag(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return "", err
	}
	if d < time.Second {
		return "", fmt.Errorf("idle_timeout must be at least 1s")
	}
	sec := int(d.Seconds())
	if sec%3600 == 0 {
		return fmt.Sprintf("%dh", sec/3600), nil
	}
	if sec%60 == 0 {
		return fmt.Sprintf("%dmin", sec/60), nil
	}
	return fmt.Sprintf("%ds", sec), nil
}

func appendHardening(svc *strings.Builder, site config.Site, root string) {
	if site.Limits.MemoryMax != "" {
		fmt.Fprintf(svc, "MemoryMax=%s\n", site.Limits.MemoryMax)
	}
	if site.Limits.LogMax != "" {
		// Best-effort per-unit journal size hint (journald also has SystemMaxUse).
		fmt.Fprintf(svc, "LogRateLimitIntervalSec=30s\n")
		fmt.Fprintf(svc, "LogRateLimitBurst=1000\n")
	}
	fmt.Fprintf(svc, "NoNewPrivileges=true\n")
	fmt.Fprintf(svc, "ProtectSystem=strict\n")
	fmt.Fprintf(svc, "ProtectHome=true\n")
	fmt.Fprintf(svc, "PrivateTmp=true\n")
	fmt.Fprintf(svc, "ReadWritePaths=%s %s\n", root, sites.SiteRuntime(site.Name))
}

func renderSocketUnit(domain, sockPath string) string {
	var sk strings.Builder
	fmt.Fprintf(&sk, "[Unit]\n")
	fmt.Fprintf(&sk, "Description=Webooza app socket %s\n", domain)
	fmt.Fprintf(&sk, "\n[Socket]\n")
	fmt.Fprintf(&sk, "ListenStream=%s\n", sockPath)
	fmt.Fprintf(&sk, "SocketMode=0666\n")
	fmt.Fprintf(&sk, "DirectoryMode=0755\n")
	fmt.Fprintf(&sk, "\n[Install]\n")
	fmt.Fprintf(&sk, "WantedBy=sockets.target\n")
	return sk.String()
}

// RenderAppUnits builds .service and optional .socket contents for a type:app site.
// When SocketProxy is set, the activated service is systemd-socket-proxyd and a
// separate always-on backend runs the app on AppBackendSocket.
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
	pubSock := sites.AppSocket(site.Name)
	svcName := ServiceUnitName(site.Name)
	root := sites.Root(site.Name)

	if site.SocketProxy {
		if !site.SocketActivation {
			return AppUnits{}, fmt.Errorf("site %s: socket_proxy requires socket_activation", site.Name)
		}
		proxyd, err := FindSocketProxyd()
		if err != nil {
			return AppUnits{}, err
		}
		backendSock := sites.AppBackendSocket(site.Name)
		backendName := BackendServiceUnitName(site.Name)
		idleFlag, err := IdleTimeoutToProxydFlag(site.IdleTimeout)
		if err != nil {
			return AppUnits{}, fmt.Errorf("site %s: %w", site.Name, err)
		}

		var proxy strings.Builder
		fmt.Fprintf(&proxy, "[Unit]\n")
		fmt.Fprintf(&proxy, "Description=Webooza socket-proxyd %s\n", site.Name)
		fmt.Fprintf(&proxy, "Requires=%s %s\n", SocketUnitName(site.Name), backendName)
		fmt.Fprintf(&proxy, "After=%s %s\n", SocketUnitName(site.Name), backendName)
		fmt.Fprintf(&proxy, "\n[Service]\n")
		fmt.Fprintf(&proxy, "Type=simple\n")
		if idleFlag != "" {
			fmt.Fprintf(&proxy, "ExecStart=%s --exit-idle-time=%s %s\n", proxyd, idleFlag, backendSock)
		} else {
			fmt.Fprintf(&proxy, "ExecStart=%s %s\n", proxyd, backendSock)
		}
		fmt.Fprintf(&proxy, "Restart=no\n")
		fmt.Fprintf(&proxy, "\n[Install]\n")
		fmt.Fprintf(&proxy, "WantedBy=multi-user.target\n")

		var backend strings.Builder
		fmt.Fprintf(&backend, "[Unit]\n")
		fmt.Fprintf(&backend, "Description=Webooza app backend %s\n", site.Name)
		fmt.Fprintf(&backend, "After=network.target\n")
		fmt.Fprintf(&backend, "\n[Service]\n")
		fmt.Fprintf(&backend, "Type=simple\n")
		fmt.Fprintf(&backend, "User=%s\n", user)
		fmt.Fprintf(&backend, "Group=%s\n", user)
		fmt.Fprintf(&backend, "WorkingDirectory=%s\n", root)
		fmt.Fprintf(&backend, "Environment=WBZ_SOCK=%s\n", backendSock)
		if site.EnvFile != "" {
			env := site.EnvFile
			if !filepath.IsAbs(env) {
				env = filepath.Join(root, env)
			}
			fmt.Fprintf(&backend, "EnvironmentFile=-%s\n", env)
		}
		fmt.Fprintf(&backend, "ExecStart=%s\n", execPath)
		fmt.Fprintf(&backend, "Restart=on-failure\n")
		appendHardening(&backend, site, root)
		fmt.Fprintf(&backend, "\n[Install]\n")
		fmt.Fprintf(&backend, "WantedBy=multi-user.target\n")

		return AppUnits{
			ServiceName:        svcName,
			SocketName:         SocketUnitName(site.Name),
			BackendServiceName: backendName,
			Service:            proxy.String(),
			Socket:             renderSocketUnit(site.Name, pubSock),
			BackendService:     backend.String(),
		}, nil
	}

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
	if site.SocketActivation {
		idle := strings.TrimSpace(site.IdleTimeout)
		if idle != "" {
			fmt.Fprintf(&svc, "Environment=WBZ_IDLE_TIMEOUT=%s\n", idle)
		}
	}
	fmt.Fprintf(&svc, "Restart=no\n")
	appendHardening(&svc, site, root)
	fmt.Fprintf(&svc, "\n[Install]\n")
	fmt.Fprintf(&svc, "WantedBy=multi-user.target\n")

	out := AppUnits{
		ServiceName: svcName,
		Service:     svc.String(),
	}
	if !site.SocketActivation {
		return out, nil
	}
	out.SocketName = SocketUnitName(site.Name)
	out.Socket = renderSocketUnit(site.Name, pubSock)
	return out, nil
}
