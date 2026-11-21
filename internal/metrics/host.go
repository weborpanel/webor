package metrics

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// HostSnapshot is the current state of the VPS as shown on the dashboard.
type HostSnapshot struct {
	Hostname  string  `json:"hostname"`
	Kernel    string  `json:"kernel"`
	CPUCount  int     `json:"cpu_count"`
	UptimeSec int64   `json:"uptime_sec"`
	CPUPct    float64 `json:"cpu_pct"`
	MemUsed   int64   `json:"mem_used"`
	MemTotal  int64   `json:"mem_total"`
	Load1     float64 `json:"load1"`
	Load5     float64 `json:"load5"`
	Load15    float64 `json:"load15"`
	DiskUsed  int64   `json:"disk_used"`
	DiskTotal int64   `json:"disk_total"`
	NetRx     float64 `json:"net_rx"` // bytes/sec
	NetTx     float64 `json:"net_tx"` // bytes/sec
}

// HostCollector reads host stats from procfs and statfs.
type HostCollector struct {
	ProcRoot string // default /proc
	DiskPath string // filesystem to report; default /
}

func (c *HostCollector) proc(name string) ([]byte, error) {
	root := c.ProcRoot
	if root == "" {
		root = "/proc"
	}
	return os.ReadFile(filepath.Join(root, name))
}

func (c *HostCollector) readCPU() (cpuTimes, error) {
	data, err := c.proc("stat")
	if err != nil {
		return cpuTimes{}, err
	}
	return parseProcStat(data)
}

func (c *HostCollector) readNet() (rx, tx uint64, err error) {
	data, err := c.proc("net/dev")
	if err != nil {
		return 0, 0, err
	}
	return parseNetDev(data)
}

// Static returns everything that does not need a delta between two reads
// (CPU % and network rates are filled in by the Sampler).
func (c *HostCollector) Static() HostSnapshot {
	snap := HostSnapshot{CPUCount: runtime.NumCPU()}
	snap.Hostname, _ = os.Hostname()
	if data, err := c.proc("sys/kernel/osrelease"); err == nil {
		snap.Kernel = strings.TrimSpace(string(data))
	}
	if data, err := c.proc("uptime"); err == nil {
		if up, err := parseUptime(data); err == nil {
			snap.UptimeSec = int64(up)
		}
	}
	if data, err := c.proc("meminfo"); err == nil {
		if mi, err := parseMeminfo(data); err == nil {
			snap.MemTotal = mi.total
			snap.MemUsed = mi.total - mi.available
		}
	}
	if data, err := c.proc("loadavg"); err == nil {
		snap.Load1, snap.Load5, snap.Load15, _ = parseLoadavg(data)
	}
	path := c.DiskPath
	if path == "" {
		path = "/"
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err == nil {
		bs := int64(st.Bsize)
		snap.DiskTotal = int64(st.Blocks) * bs
		snap.DiskUsed = (int64(st.Blocks) - int64(st.Bfree)) * bs
	}
	return snap
}
