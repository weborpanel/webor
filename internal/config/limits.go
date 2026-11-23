package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Panel presets for the Settings UI. Higher values stay yaml-only.
var (
	MemoryPresets = []string{"128M", "256M", "512M", "1G", "2G", "4G", "8G"}
	DiskPresets   = []string{"10M", "50M", "100M", "256M", "512M", "1G", "2G", "5G", "10G"} // panel max 10G
	// DefaultDiskQuota is applied to new sites from the panel.
	DefaultDiskQuota = "10M"
	// CPUCorePresets are whole-core choices shown as "1c", "2c", …
	CPUCorePresets = []int{1, 2, 4, 8}
)

var (
	sizeRE     = regexp.MustCompile(`(?i)^[0-9]+[KMGT]?$`)
	cpuQuotaRE = regexp.MustCompile(`(?i)^[0-9]+%$`)
)

const panelDiskMaxBytes = 10 << 30 // 10 GiB

// ParseByteSize parses systemd-style sizes ("512M", "1G"). Empty → 0.
func ParseByteSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
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

// ValidateSizeString accepts empty or a systemd size like 256M / 2G.
func ValidateSizeString(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if !sizeRE.MatchString(v) {
		return fmt.Errorf("invalid size %q (want e.g. 512M or 1G)", v)
	}
	if ParseByteSize(v) <= 0 {
		return fmt.Errorf("invalid size %q", v)
	}
	return nil
}

// ValidateCPUQuota accepts empty or a positive percent like 100%.
func ValidateCPUQuota(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if !cpuQuotaRE.MatchString(v) {
		return fmt.Errorf("invalid cpu_quota %q (want e.g. 100%%)", v)
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(v), "%"))
	if err != nil || n <= 0 || n > 12800 {
		return fmt.Errorf("invalid cpu_quota %q", v)
	}
	return nil
}

// CPUQuotaFromCores maps whole cores to systemd CPUQuota (1 → 100%).
func CPUQuotaFromCores(cores int) string {
	if cores <= 0 {
		return ""
	}
	return fmt.Sprintf("%d%%", cores*100)
}

// CoresFromCPUQuota maps 100% → 1. Non-multiples of 100 return 0.
func CoresFromCPUQuota(quota string) int {
	quota = strings.TrimSpace(quota)
	if quota == "" || !strings.HasSuffix(quota, "%") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSuffix(quota, "%"))
	if err != nil || n <= 0 || n%100 != 0 {
		return 0
	}
	return n / 100
}

// LimitOption is one selectable preset for the panel.
type LimitOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// MemoryOptionsForHost returns memory presets that fit on the host (or all if hostMemBytes ≤ 0).
func MemoryOptionsForHost(hostMemBytes int64) []LimitOption {
	out := make([]LimitOption, 0, len(MemoryPresets)+1)
	out = append(out, LimitOption{Value: "", Label: "No limit"})
	for _, p := range MemoryPresets {
		b := ParseByteSize(p)
		if hostMemBytes > 0 && b > hostMemBytes {
			continue
		}
		out = append(out, LimitOption{Value: p, Label: formatSizeLabel(p)})
	}
	return out
}

// DiskOptionsForPanel returns disk presets up to 10G (panel ceiling).
func DiskOptionsForPanel() []LimitOption {
	out := make([]LimitOption, 0, len(DiskPresets)+1)
	out = append(out, LimitOption{Value: "", Label: "No quota"})
	for _, p := range DiskPresets {
		out = append(out, LimitOption{Value: p, Label: formatSizeLabel(p)})
	}
	return out
}

// CPUOptionsForHost returns whole-core presets that fit on the host (or all if cpuCount ≤ 0).
func CPUOptionsForHost(cpuCount int) []LimitOption {
	out := make([]LimitOption, 0, len(CPUCorePresets)+1)
	out = append(out, LimitOption{Value: "", Label: "No limit"})
	for _, cores := range CPUCorePresets {
		if cpuCount > 0 && cores > cpuCount {
			continue
		}
		out = append(out, LimitOption{
			Value: CPUQuotaFromCores(cores),
			Label: fmt.Sprintf("%dc", cores),
		})
	}
	return out
}

func formatSizeLabel(p string) string {
	p = strings.TrimSpace(p)
	u := strings.ToUpper(p)
	switch {
	case strings.HasSuffix(u, "M"):
		return strings.TrimSuffix(u, "M") + " MB"
	case strings.HasSuffix(u, "G"):
		return strings.TrimSuffix(u, "G") + " GB"
	default:
		return p
	}
}

// NormalizeLimits trims limit fields.
func NormalizeLimits(l *Limits) {
	if l == nil {
		return
	}
	l.MemoryMax = strings.TrimSpace(l.MemoryMax)
	l.CPUQuota = strings.TrimSpace(l.CPUQuota)
	l.DiskQuota = strings.TrimSpace(l.DiskQuota)
	l.LogMax = strings.TrimSpace(l.LogMax)
}

// ValidateLimits checks yaml/API size and CPU formats (does not enforce panel presets).
func ValidateLimits(l Limits) error {
	NormalizeLimits(&l)
	if err := ValidateSizeString(l.MemoryMax); err != nil {
		return fmt.Errorf("memory_max: %w", err)
	}
	if err := ValidateSizeString(l.DiskQuota); err != nil {
		return fmt.Errorf("disk_quota: %w", err)
	}
	if err := ValidateSizeString(l.LogMax); err != nil {
		return fmt.Errorf("log_max: %w", err)
	}
	if err := ValidateCPUQuota(l.CPUQuota); err != nil {
		return err
	}
	return nil
}

// ValidatePanelLimits restricts values to panel presets (disk ≤ 10G, listed memory/CPU).
func ValidatePanelLimits(l Limits, hostMemBytes int64, cpuCount int) error {
	_, err := CanonicalPanelLimits(l, hostMemBytes, cpuCount)
	return err
}

// CanonicalPanelLimits validates panel presets and returns canonical spelling (e.g. 512M).
// Empty fields clear the limit. hostMemBytes/cpuCount filter what is allowed.
func CanonicalPanelLimits(l Limits, hostMemBytes int64, cpuCount int) (Limits, error) {
	NormalizeLimits(&l)
	if err := ValidateLimits(l); err != nil {
		return Limits{}, err
	}
	if l.MemoryMax != "" {
		ok := false
		for _, o := range MemoryOptionsForHost(hostMemBytes) {
			if strings.EqualFold(o.Value, l.MemoryMax) {
				ok = true
				l.MemoryMax = o.Value
				break
			}
		}
		if !ok {
			return Limits{}, fmt.Errorf("memory_max %q is not a panel preset for this host", l.MemoryMax)
		}
	}
	if l.CPUQuota != "" {
		ok := false
		for _, o := range CPUOptionsForHost(cpuCount) {
			if o.Value == l.CPUQuota {
				ok = true
				break
			}
		}
		if !ok {
			return Limits{}, fmt.Errorf("cpu_quota %q is not a panel preset for this host", l.CPUQuota)
		}
	}
	if l.DiskQuota != "" {
		ok := false
		for _, o := range DiskOptionsForPanel() {
			if strings.EqualFold(o.Value, l.DiskQuota) {
				ok = true
				l.DiskQuota = o.Value
				break
			}
		}
		if !ok {
			if ParseByteSize(l.DiskQuota) > panelDiskMaxBytes {
				return Limits{}, fmt.Errorf("disk_quota above 10G must be set in config.yaml")
			}
			return Limits{}, fmt.Errorf("disk_quota %q is not a panel preset", l.DiskQuota)
		}
	}
	return l, nil
}
