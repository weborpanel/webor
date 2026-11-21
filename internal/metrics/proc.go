// Package metrics samples host and per-site resource usage into SQLite and
// serves downsampled time series for the panel graphs.
package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type cpuTimes struct {
	total uint64
	idle  uint64
}

// parseProcStat reads the aggregate "cpu" line of /proc/stat.
func parseProcStat(data []byte) (cpuTimes, error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var t cpuTimes
		// user nice system idle iowait irq softirq steal; guest time is
		// already counted in user/nice.
		for i, f := range fields[1:] {
			if i >= 8 {
				break
			}
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return cpuTimes{}, fmt.Errorf("proc stat: %w", err)
			}
			t.total += v
			if i == 3 || i == 4 {
				t.idle += v
			}
		}
		return t, nil
	}
	return cpuTimes{}, fmt.Errorf("proc stat: no cpu line")
}

func cpuPercent(prev, cur cpuTimes) float64 {
	if cur.total <= prev.total {
		return 0
	}
	dt := float64(cur.total - prev.total)
	di := float64(cur.idle) - float64(prev.idle)
	if di < 0 {
		di = 0
	}
	return clampPct((1 - di/dt) * 100)
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

type memInfo struct {
	total     int64
	available int64
}

func parseMeminfo(data []byte) (memInfo, error) {
	vals := map[string]int64{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if len(fields) > 1 && strings.EqualFold(fields[1], "kB") {
			n *= 1024
		}
		vals[key] = n
	}
	total, ok := vals["MemTotal"]
	if !ok {
		return memInfo{}, fmt.Errorf("meminfo: MemTotal missing")
	}
	avail, ok := vals["MemAvailable"]
	if !ok {
		avail = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	return memInfo{total: total, available: avail}, nil
}

func parseLoadavg(data []byte) (l1, l5, l15 float64, err error) {
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("loadavg: unexpected %q", strings.TrimSpace(string(data)))
	}
	vals := make([]float64, 3)
	for i := range vals {
		if vals[i], err = strconv.ParseFloat(fields[i], 64); err != nil {
			return 0, 0, 0, fmt.Errorf("loadavg: %w", err)
		}
	}
	return vals[0], vals[1], vals[2], nil
}

// parseNetDev sums received/transmitted bytes over all non-loopback interfaces.
func parseNetDev(data []byte) (rx, tx uint64, err error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		iface, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(iface) == "lo" {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		r, err1 := strconv.ParseUint(fields[0], 10, 64)
		t, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		rx += r
		tx += t
	}
	return rx, tx, nil
}

func parseUptime(data []byte) (float64, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("uptime: empty")
	}
	return strconv.ParseFloat(fields[0], 64)
}

// ParseSize parses systemd-style byte sizes ("512M", "1G", "1048576").
// Returns 0 for empty, "infinity" or unparsable values.
func ParseSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" || s == "INFINITY" {
		return 0
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, "B"), "I")
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "K"):
		mult = 1 << 10
	case strings.HasSuffix(s, "M"):
		mult = 1 << 20
	case strings.HasSuffix(s, "G"):
		mult = 1 << 30
	case strings.HasSuffix(s, "T"):
		mult = 1 << 40
	}
	if mult > 1 {
		s = s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < 0 {
		return 0
	}
	return int64(v * float64(mult))
}
