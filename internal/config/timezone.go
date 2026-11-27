package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ValidateTimezone accepts IANA names or "UTC".
func ValidateTimezone(tz string) error {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return fmt.Errorf("timezone is empty")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("unknown timezone %q", tz)
	}
	return nil
}

// HostTimezone returns the IANA name from /etc/localtime, or "UTC".
func HostTimezone() string {
	link, err := os.Readlink("/etc/localtime")
	if err != nil {
		return "UTC"
	}
	const marker = "zoneinfo/"
	if i := strings.Index(link, marker); i >= 0 {
		tz := filepath.ToSlash(link[i+len(marker):])
		if ValidateTimezone(tz) == nil {
			return tz
		}
	}
	return "UTC"
}

// EffectiveTimezone returns panel.timezone if set, otherwise HostTimezone().
func EffectiveTimezone(c *Config) string {
	if c != nil {
		if tz := strings.TrimSpace(c.Panel.Timezone); tz != "" {
			return tz
		}
	}
	return HostTimezone()
}
