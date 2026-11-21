package helper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

// SiteResources is a read-only snapshot of site disk/memory/cpu usage.
type SiteResources struct {
	Domain             string `json:"domain"`
	DiskUsedBytes      int64  `json:"disk_used_bytes"`
	DiskAvailBytes     int64  `json:"disk_avail_bytes"`
	DiskQuota          string `json:"disk_quota,omitempty"`
	MemoryCurrentBytes int64  `json:"memory_current_bytes"`
	MemoryMax          string `json:"memory_max,omitempty"`
	CPUUsageUSec       int64  `json:"cpu_usage_usec,omitempty"`
	AppActive          bool   `json:"app_active"`
	Unit               string `json:"unit"`
}

// SiteResources returns disk + cgroup/memory snapshot for a site.
func (o *Ops) SiteResources(domain string, site *config.Site) (*SiteResources, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	root := sites.Root(domain)
	res := &SiteResources{
		Domain: domain,
		Unit:   systemd.ServiceUnitName(domain),
	}
	if site != nil {
		res.DiskQuota = site.Limits.DiskQuota
		res.MemoryMax = site.Limits.MemoryMax
		if site.SocketProxy {
			res.Unit = systemd.BackendServiceUnitName(domain)
		}
	}

	used, err := dirSize(root)
	if err == nil {
		res.DiskUsedBytes = used
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err == nil {
		res.DiskAvailBytes = int64(st.Bavail) * int64(st.Bsize)
	}

	unit := res.Unit
	res.AppActive = o.SystemctlIsActive(unit)
	props, err := systemctlShow(unit, "MemoryCurrent", "CPUUsageNSec")
	if err == nil {
		if v := props["MemoryCurrent"]; v != "" && v != "[not set]" && v != "18446744073709551615" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
				res.MemoryCurrentBytes = n
			}
		}
		if v := props["CPUUsageNSec"]; v != "" && v != "[not set]" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
				res.CPUUsageUSec = n / 1000
			}
		}
	}
	// Optional cgroup cpu.stat fallback when systemctl lacks CPUUsageNSec.
	if res.CPUUsageUSec == 0 {
		if n, err := readCgroupCPUUsageUSec(unit); err == nil {
			res.CPUUsageUSec = n
		}
	}
	return res, nil
}

func dirSize(root string) (int64, error) {
	cmd := exec.Command("du", "-sb", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback walk if du fails (missing tool).
		var total int64
		errWalk := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				total += info.Size()
			}
			return nil
		})
		return total, errWalk
	}
	fields := strings.Fields(string(out))
	if len(fields) < 1 {
		return 0, fmt.Errorf("du: unexpected output %q", strings.TrimSpace(string(out)))
	}
	return strconv.ParseInt(fields[0], 10, 64)
}

func readCgroupCPUUsageUSec(unit string) (int64, error) {
	// systemctl show -p ControlGroup
	props, err := systemctlShow(unit, "ControlGroup")
	if err != nil {
		return 0, err
	}
	cg := strings.TrimSpace(props["ControlGroup"])
	if cg == "" || cg == "/" {
		return 0, fmt.Errorf("no control group")
	}
	path := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(cg, "/"), "cpu.stat")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "usage_usec" {
			return strconv.ParseInt(fields[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("usage_usec not found")
}
