// Package unitfiles embeds the host systemd units shipped with Webor.
// Canonical copies live in install/; keep these in sync (TestMatchInstall).
package unitfiles

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed webor.service webor-helper.service webor-caddy.service
var FS embed.FS

// Names are the unit files installed under /etc/systemd/system.
var Names = []string{
	"webor.service",
	"webor-helper.service",
	"webor-caddy.service",
}

// DefaultUnitDir is where systemd reads host units.
const DefaultUnitDir = "/etc/systemd/system"

// InstallOptions controls Install.
type InstallOptions struct {
	// DestDir defaults to DefaultUnitDir.
	DestDir string
	// SkipCaddy skips webor-caddy.service when true, or when caddy is not on PATH
	// and no existing unit file is present (auto-detect).
	SkipCaddy bool
	// ForceCaddy installs webor-caddy.service even if caddy is missing (tests).
	ForceCaddy bool
	// SkipDaemonReload skips systemctl daemon-reload (tests / non-root).
	SkipDaemonReload bool
	// RestartCaddy restarts webor-caddy when the unit changed, or when the
	// unit is already failed (try-restart is a no-op for failed units).
	RestartCaddy bool
}

// Install writes embedded units to DestDir when content differs.
// Returns whether any file changed.
func Install(opt InstallOptions) (changed bool, err error) {
	dest := opt.DestDir
	if dest == "" {
		dest = DefaultUnitDir
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return false, err
	}
	skipCaddy := shouldSkipCaddy(opt, dest)
	for _, name := range Names {
		if name == "webor-caddy.service" && skipCaddy {
			continue
		}
		body, err := FS.ReadFile(name)
		if err != nil {
			return changed, fmt.Errorf("embed %s: %w", name, err)
		}
		path := filepath.Join(dest, name)
		if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, body) {
			continue
		}
		tmp := path + ".new"
		if err := os.WriteFile(tmp, body, 0o644); err != nil {
			return changed, err
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return changed, err
		}
		changed = true
	}
	if changed && !opt.SkipDaemonReload {
		out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput()
		if err != nil {
			return true, fmt.Errorf("daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	if opt.RestartCaddy && !skipCaddy {
		RestartCaddy(changed)
	}
	return changed, nil
}

// InstallFromDir copies unit files from a release install/ directory (tarball).
// Missing files are skipped; same SkipCaddy / reload semantics as Install.
func InstallFromDir(srcDir string, opt InstallOptions) (changed bool, err error) {
	dest := opt.DestDir
	if dest == "" {
		dest = DefaultUnitDir
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return false, err
	}
	skipCaddy := shouldSkipCaddy(opt, dest)
	for _, name := range Names {
		if name == "webor-caddy.service" && skipCaddy {
			continue
		}
		src := filepath.Join(srcDir, name)
		body, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return changed, err
		}
		path := filepath.Join(dest, name)
		if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, body) {
			continue
		}
		tmp := path + ".new"
		if err := os.WriteFile(tmp, body, 0o644); err != nil {
			return changed, err
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return changed, err
		}
		changed = true
	}
	if changed && !opt.SkipDaemonReload {
		out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput()
		if err != nil {
			return true, fmt.Errorf("daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	if opt.RestartCaddy && !skipCaddy {
		RestartCaddy(changed)
	}
	return changed, nil
}

func shouldSkipCaddy(opt InstallOptions, destDir string) bool {
	if opt.SkipCaddy {
		return true
	}
	if opt.ForceCaddy {
		return false
	}
	if _, err := exec.LookPath("caddy"); err == nil {
		return false
	}
	// Keep refreshing an existing unit even if caddy is briefly off PATH.
	if _, err := os.Stat(filepath.Join(destDir, "webor-caddy.service")); err == nil {
		return false
	}
	return true
}

// CaddyFailed reports whether webor-caddy is in the systemd "failed" state.
func CaddyFailed() bool {
	out, err := exec.Command("systemctl", "is-failed", "webor-caddy").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "failed" {
		return true
	}
	// is-failed exits non-zero for "active" etc.; still parse stdout.
	return strings.TrimSpace(string(out)) == "failed"
}

// RestartCaddy restarts webor-caddy when forceOrChanged is true or the unit is
// failed. Otherwise try-restart (no-op if inactive). try-restart alone never
// recovers a failed unit after ReadWritePaths fixes.
func RestartCaddy(forceOrChanged bool) {
	if _, err := os.Stat(filepath.Join(DefaultUnitDir, "webor-caddy.service")); err != nil {
		return
	}
	if forceOrChanged || CaddyFailed() {
		_ = exec.Command("systemctl", "restart", "webor-caddy").Run()
		return
	}
	_ = exec.Command("systemctl", "try-restart", "webor-caddy").Run()
}
