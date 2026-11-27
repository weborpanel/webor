package helper

import (
	"strings"

	"github.com/weborpanel/webor/internal/config"
)

const defaultConfigPath = "/etc/webor/config.yaml"

// ScheduleTimezone returns the IANA zone for cron/backup timers.
func (o *Ops) ScheduleTimezone() string {
	path := defaultConfigPath
	if o != nil && strings.TrimSpace(o.BackupEnvPath) != "" {
		// lab tests sometimes override env; config path stays default unless we add ConfigPath later
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.HostTimezone()
	}
	return config.EffectiveTimezone(cfg)
}
