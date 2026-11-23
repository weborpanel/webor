package distro

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunCommands runs argv lists sequentially; returns the first error.
func RunCommands(cmds [][]string) error {
	for _, args := range cmds {
		if len(args) == 0 {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		if args[0] == "apt-get" {
			cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 400 {
				msg = msg[:400] + "…"
			}
			return fmt.Errorf("%s: %w (%s)", strings.Join(args, " "), err, msg)
		}
	}
	return nil
}

// LookManager returns "apt-get", "dnf", or "yum", or empty.
func LookManager() string {
	for _, name := range []string{"apt-get", "dnf", "yum"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// InstallPackages installs packages with the host package manager.
func InstallPackages(pkgs ...string) error {
	mgr := LookManager()
	switch mgr {
	case "apt-get":
		cmds := [][]string{
			{"apt-get", "update", "-qq"},
			append([]string{"apt-get", "install", "-y"}, pkgs...),
		}
		return RunCommands(cmds)
	case "dnf", "yum":
		return RunCommands([][]string{append([]string{mgr, "install", "-y"}, pkgs...)})
	default:
		return fmt.Errorf("no apt-get/dnf/yum found (need Debian/Ubuntu or AlmaLinux/RHEL)")
	}
}

// TryInstallPackages tries each package set until one succeeds.
func TryInstallPackages(sets ...[]string) error {
	var errs []string
	for _, pkgs := range sets {
		if err := InstallPackages(pkgs...); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
		}
	}
	return fmt.Errorf("package install failed: %s", strings.Join(errs, "; "))
}
