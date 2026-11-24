// Package panelenv reads and rewrites /etc/webor/panel.env listen settings.
package panelenv

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/weborpanel/webor/internal/update"
)

const (
	// DefaultEnvPath is the systemd EnvironmentFile for webor.service.
	DefaultEnvPath = update.DefaultPanelEnv

	envArgsKey = "WBR_PANEL_ARGS"
	envPortKey = "WBR_PANEL_PORT"

	// HostLocal binds the panel to loopback only (no public IP:port).
	HostLocal = "127.0.0.1"
	// HostPublic binds the panel on all interfaces (first-run / fallback).
	HostPublic = "0.0.0.0"
)

// ValidateListenHost accepts only the two hosts used for lock/unlock.
func ValidateListenHost(host string) error {
	switch strings.TrimSpace(host) {
	case HostLocal, HostPublic:
		return nil
	default:
		return fmt.Errorf("listen host must be %s or %s", HostLocal, HostPublic)
	}
}

// SetListenHost rewrites -listen HOST:PORT in WBR_PANEL_ARGS (keeps port)
// and upserts WBR_PANEL_PORT. Does not restart the unit.
func SetListenHost(envPath, host string) (listenAddr string, err error) {
	if envPath == "" {
		envPath = DefaultEnvPath
	}
	if err := ValidateListenHost(host); err != nil {
		return "", err
	}
	host = strings.TrimSpace(host)

	raw, err := os.ReadFile(envPath)
	if err != nil {
		return "", fmt.Errorf("read panel.env: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var (
		port      int
		foundArgs bool
		foundPort bool
	)
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			out = append(out, line)
			continue
		}
		key, val, ok := strings.Cut(trim, "=")
		if !ok {
			out = append(out, line)
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case envArgsKey:
			rewritten, p, err := rewriteListenArgs(val, host)
			if err != nil {
				return "", err
			}
			port = p
			foundArgs = true
			out = append(out, key+"="+rewritten)
		case envPortKey:
			foundPort = true
			// Keep as-is for now; rewrite after we know the port from -listen.
			out = append(out, line)
		default:
			out = append(out, line)
		}
	}
	if !foundArgs {
		return "", fmt.Errorf("%s missing in panel.env", envArgsKey)
	}
	if port <= 0 {
		return "", fmt.Errorf("could not determine panel listen port from %s", envArgsKey)
	}
	listenAddr = net.JoinHostPort(host, strconv.Itoa(port))

	// Upsert WBR_PANEL_PORT to match -listen.
	portLine := envPortKey + "=" + strconv.Itoa(port)
	if foundPort {
		for i, line := range out {
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "#") {
				continue
			}
			key, _, ok := strings.Cut(trim, "=")
			if ok && strings.TrimSpace(key) == envPortKey {
				out[i] = portLine
				break
			}
		}
	} else {
		out = append(out, portLine)
	}

	body := strings.Join(out, "\n") + "\n"
	tmp := envPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o640); err != nil {
		return "", fmt.Errorf("write panel.env: %w", err)
	}
	if err := os.Rename(tmp, envPath); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Chmod(envPath, 0o640)
	if uid, gid, err := lookupWeborUIDGID(); err == nil {
		_ = os.Chown(envPath, uid, gid)
	}
	return listenAddr, nil
}

// rewriteListenArgs replaces the host in -listen HOST:PORT (or -listen=…).
func rewriteListenArgs(argsLine, host string) (rewritten string, port int, err error) {
	fields := strings.Fields(argsLine)
	if len(fields) == 0 {
		return "", 0, fmt.Errorf("empty %s", envArgsKey)
	}
	changed := false
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "-listen" && i+1 < len(fields):
			newAddr, p, err := replaceListenAddr(fields[i+1], host)
			if err != nil {
				return "", 0, err
			}
			fields[i+1] = newAddr
			port = p
			changed = true
			i++
		case strings.HasPrefix(f, "-listen="):
			val := strings.TrimPrefix(f, "-listen=")
			newAddr, p, err := replaceListenAddr(val, host)
			if err != nil {
				return "", 0, err
			}
			fields[i] = "-listen=" + newAddr
			port = p
			changed = true
		}
	}
	if !changed {
		return "", 0, fmt.Errorf("-listen not found in %s", envArgsKey)
	}
	return strings.Join(fields, " "), port, nil
}

func replaceListenAddr(addr, host string) (string, int, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0, fmt.Errorf("empty -listen address")
	}
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		// Bare :PORT form.
		if strings.HasPrefix(addr, ":") {
			n, err := strconv.Atoi(strings.TrimPrefix(addr, ":"))
			if err != nil || n <= 0 || n > 65535 {
				return "", 0, fmt.Errorf("invalid -listen port in %q", addr)
			}
			return net.JoinHostPort(host, strconv.Itoa(n)), n, nil
		}
		return "", 0, fmt.Errorf("invalid -listen %q: %w", addr, err)
	}
	_ = h
	n, err := strconv.Atoi(p)
	if err != nil || n <= 0 || n > 65535 {
		return "", 0, fmt.Errorf("invalid -listen port in %q", addr)
	}
	return net.JoinHostPort(host, p), n, nil
}

func lookupWeborUIDGID() (uid, gid int, err error) {
	u, err := user.Lookup("webor")
	if err != nil {
		return 0, 0, err
	}
	uid, err1 := strconv.Atoi(u.Uid)
	gid, err2 := strconv.Atoi(u.Gid)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("parse webor uid/gid")
	}
	return uid, gid, nil
}
