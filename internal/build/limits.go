package build

import "time"

// DefaultLimits protects a small VPS from runaway site builds.
var DefaultLimits = Limits{
	MemoryMax: "512M",
	CPUQuota:  "100%",
	Timeout:   10 * time.Minute,
	Nice:      10,
}

// Limits are applied to every transient build unit.
type Limits struct {
	MemoryMax string
	CPUQuota  string
	Timeout   time.Duration
	Nice      int
}

func (l Limits) withDefaults() Limits {
	out := DefaultLimits
	if l.MemoryMax != "" {
		out.MemoryMax = l.MemoryMax
	}
	if l.CPUQuota != "" {
		out.CPUQuota = l.CPUQuota
	}
	if l.Timeout > 0 {
		out.Timeout = l.Timeout
	}
	if l.Nice != 0 {
		out.Nice = l.Nice
	}
	return out
}
