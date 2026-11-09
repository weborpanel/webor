// Package config defines the declarative Webooza config schema and validation.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Supported site workload types.
const (
	SiteTypeStatic    = "static"
	SiteTypeApp       = "app"
	SiteTypeContainer = "container"
)

var domainRE = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// Config is the top-level desired-state document.
type Config struct {
	Version int    `yaml:"version"`
	Panel   Panel  `yaml:"panel"`
	Sites   []Site `yaml:"sites"`
	Mail    *Mail  `yaml:"mail,omitempty"` // reserved for later; ignored in v1
}

// Panel holds panel listen and domain settings.
type Panel struct {
	Domain string `yaml:"domain"`
	Port   int    `yaml:"port"` // 0 = random high port until a domain is set
}

// Site describes one hosted site or app.
type Site struct {
	Name             string            `yaml:"name"`
	Aliases          []string          `yaml:"aliases,omitempty"`
	Type             string            `yaml:"type"` // static | app | container
	Root             string            `yaml:"root,omitempty"`
	BuildCommand     string            `yaml:"build_command,omitempty"`
	Exec             string            `yaml:"exec,omitempty"`
	SocketActivation bool              `yaml:"socket_activation,omitempty"`
	IdleTimeout      string            `yaml:"idle_timeout,omitempty"`
	EnvFile          string            `yaml:"env_file,omitempty"`
	Database         string            `yaml:"database,omitempty"`
	Cache            string            `yaml:"cache,omitempty"`
	Limits           Limits            `yaml:"limits,omitempty"`
	Cron             []CronJob         `yaml:"cron,omitempty"`
	Extra            map[string]string `yaml:"extra,omitempty"`
}

// Limits are per-site resource caps.
type Limits struct {
	DiskQuota string `yaml:"disk_quota,omitempty"`
	LogMax    string `yaml:"log_max,omitempty"`
	MemoryMax string `yaml:"memory_max,omitempty"`
}

// CronJob is a scheduled job for a site.
type CronJob struct {
	Name     string `yaml:"name"`
	Schedule string `yaml:"schedule"`
	Exec     string `yaml:"exec"`
}

// Mail is reserved so adding mail later does not break existing configs.
type Mail struct {
	Enabled bool `yaml:"enabled,omitempty"`
}

// Load reads and validates a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse unmarshals YAML bytes and validates the result.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config as YAML to path.
func Save(path string, c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Validate checks required fields and basic naming rules.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d (want 1)", c.Version)
	}
	if c.Panel.Port < 0 || c.Panel.Port > 65535 {
		return fmt.Errorf("panel.port out of range: %d", c.Panel.Port)
	}
	if c.Panel.Domain != "" {
		if err := validateDomain(c.Panel.Domain); err != nil {
			return fmt.Errorf("panel.domain: %w", err)
		}
	}

	seen := make(map[string]struct{}, len(c.Sites))
	for i, s := range c.Sites {
		prefix := fmt.Sprintf("sites[%d]", i)
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("%s: name is required", prefix)
		}
		if err := validateDomain(s.Name); err != nil {
			return fmt.Errorf("%s.name: %w", prefix, err)
		}
		key := strings.ToLower(s.Name)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%s: duplicate site name %q", prefix, s.Name)
		}
		seen[key] = struct{}{}

		switch s.Type {
		case SiteTypeStatic, SiteTypeApp, SiteTypeContainer:
		default:
			return fmt.Errorf("%s: invalid type %q (want static, app, or container)", prefix, s.Type)
		}
		for j, alias := range s.Aliases {
			if err := validateDomain(alias); err != nil {
				return fmt.Errorf("%s.aliases[%d]: %w", prefix, j, err)
			}
		}
		for j, job := range s.Cron {
			if strings.TrimSpace(job.Name) == "" {
				return fmt.Errorf("%s.cron[%d]: name is required", prefix, j)
			}
			if strings.TrimSpace(job.Schedule) == "" {
				return fmt.Errorf("%s.cron[%d]: schedule is required", prefix, j)
			}
			if strings.TrimSpace(job.Exec) == "" {
				return fmt.Errorf("%s.cron[%d]: exec is required", prefix, j)
			}
		}
	}
	return nil
}

func validateDomain(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("empty domain")
	}
	if len(n) > 253 {
		return fmt.Errorf("domain too long")
	}
	if !domainRE.MatchString(n) {
		return fmt.Errorf("invalid domain %q", name)
	}
	return nil
}
