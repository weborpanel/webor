// Package distro detects Debian vs RHEL-family hosts and supplies package install hints.
package distro

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

// Family is a coarse OS family for package management.
type Family string

const (
	FamilyDebian  Family = "debian"
	FamilyRHEL    Family = "rhel"
	FamilyUnknown Family = "unknown"
)

// Info describes the host package manager.
type Info struct {
	Family  Family
	ID      string // from /etc/os-release ID=
	Pretty  string
	Manager string // apt-get or dnf (or yum)
}

// Detect reads /etc/os-release and looks for apt-get / dnf / yum.
func Detect() Info {
	info := Info{Family: FamilyUnknown}
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			val = strings.Trim(val, `"'`)
			switch key {
			case "ID":
				info.ID = strings.ToLower(val)
			case "ID_LIKE":
				if info.ID == "" {
					if fields := strings.Fields(val); len(fields) > 0 {
						info.ID = strings.ToLower(fields[0])
					}
				}
				like := strings.ToLower(val)
				if strings.Contains(like, "rhel") || strings.Contains(like, "fedora") || strings.Contains(like, "centos") {
					info.Family = FamilyRHEL
				}
				if strings.Contains(like, "debian") || strings.Contains(like, "ubuntu") {
					info.Family = FamilyDebian
				}
			case "PRETTY_NAME":
				info.Pretty = val
			}
		}
	}
	switch info.ID {
	case "debian", "ubuntu", "raspbian", "linuxmint", "pop":
		info.Family = FamilyDebian
	case "rhel", "centos", "almalinux", "rocky", "fedora", "ol", "amzn":
		info.Family = FamilyRHEL
	}
	if _, err := exec.LookPath("apt-get"); err == nil && info.Family != FamilyRHEL {
		info.Family = FamilyDebian
		info.Manager = "apt-get"
	}
	if info.Manager == "" {
		if _, err := exec.LookPath("dnf"); err == nil {
			info.Manager = "dnf"
			if info.Family == FamilyUnknown {
				info.Family = FamilyRHEL
			}
		} else if _, err := exec.LookPath("yum"); err == nil {
			info.Manager = "yum"
			if info.Family == FamilyUnknown {
				info.Family = FamilyRHEL
			}
		} else if _, err := exec.LookPath("apt-get"); err == nil {
			info.Manager = "apt-get"
			if info.Family == FamilyUnknown {
				info.Family = FamilyDebian
			}
		}
	}
	if info.Family == FamilyDebian && info.Manager == "" {
		info.Manager = "apt-get"
	}
	if info.Family == FamilyRHEL && info.Manager == "" {
		info.Manager = "dnf"
	}
	return info
}

// IsRHEL reports RHEL-family (Alma, Rocky, RHEL, …).
func (i Info) IsRHEL() bool { return i.Family == FamilyRHEL }

// IsDebian reports Debian/Ubuntu-family.
func (i Info) IsDebian() bool { return i.Family == FamilyDebian }

// InstallHintPostgres returns operator commands to install and start Postgres.
func (i Info) InstallHintPostgres() string {
	if i.IsRHEL() {
		return "sudo dnf install -y postgresql-server postgresql\nsudo postgresql-setup --initdb\nsudo systemctl enable --now postgresql"
	}
	return "sudo apt-get update && sudo apt-get install -y postgresql postgresql-client\nsudo systemctl enable --now postgresql"
}

// StartHintPostgres returns how to start Postgres.
func (i Info) StartHintPostgres() string {
	return "sudo systemctl enable --now postgresql"
}

// InstallHintMariaDB returns operator commands to install and start MariaDB.
func (i Info) InstallHintMariaDB() string {
	if i.IsRHEL() {
		return "sudo dnf install -y mariadb-server\nsudo systemctl enable --now mariadb"
	}
	return "sudo apt-get update && sudo apt-get install -y mariadb-server\nsudo systemctl enable --now mariadb"
}

// StartHintMariaDB returns how to start MariaDB.
func (i Info) StartHintMariaDB() string {
	return "sudo systemctl enable --now mariadb"
}

// InstallHintValkey returns operator commands for Valkey or Redis.
func (i Info) InstallHintValkey() string {
	if i.IsRHEL() {
		return "sudo dnf install -y redis\nsudo systemctl enable --now redis\n(Valkey may be available via EPEL/COPR; the panel accepts redis or valkey.)"
	}
	return "sudo apt-get update && sudo apt-get install -y valkey\nsudo systemctl enable --now valkey-server\n(If valkey is unavailable, install redis-server instead — the panel accepts either.)"
}

// StartHintValkey returns how to start cache.
func (i Info) StartHintValkey() string {
	if i.IsRHEL() {
		return "sudo systemctl enable --now redis"
	}
	return "sudo systemctl enable --now valkey-server"
}

// InstallHintRestic returns how to install restic.
func (i Info) InstallHintRestic() string {
	if i.IsRHEL() {
		return "sudo dnf install -y restic   # needs EPEL: sudo dnf install -y epel-release"
	}
	return "sudo apt-get install -y restic"
}
