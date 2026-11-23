// Package config defines the declarative Webor config schema and validation.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

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
	Version int     `yaml:"version"`
	Panel   Panel   `yaml:"panel"`
	Sites   []Site  `yaml:"sites"`
	Backup  *Backup `yaml:"backup,omitempty"`
	Mail    *Mail   `yaml:"mail,omitempty"` // reserved for later; ignored in v1
}

// Panel holds panel listen and domain settings.
type Panel struct {
	Domain string `yaml:"domain"`
	Port   int    `yaml:"port"` // 0 = random high port until a domain is set
}

// Site describes one hosted site or app.
type Site struct {
	Name             string            `yaml:"name"`
	Parent           string            `yaml:"parent,omitempty"` // subdomain of this parent site
	Aliases          []string          `yaml:"aliases,omitempty"`
	Type             string            `yaml:"type"` // static | app | container
	Root             string            `yaml:"root,omitempty"`
	Git              string            `yaml:"git,omitempty"`     // clone URL
	GitRef           string            `yaml:"git_ref,omitempty"` // branch/tag
	SourceDir        string            `yaml:"source_dir,omitempty"`
	BuildCommand     string            `yaml:"build_command,omitempty"`
	Exec             string            `yaml:"exec,omitempty"`
	Runtime          *SiteRuntime      `yaml:"runtime,omitempty"` // optional Node (managed or BYO)
	SocketActivation bool              `yaml:"socket_activation,omitempty"`
	SocketProxy      bool              `yaml:"socket_proxy,omitempty"` // proxyd + private backend (non-LISTEN_FDS apps)
	IdleTimeout      string            `yaml:"idle_timeout,omitempty"`
	EnvFile          string            `yaml:"env_file,omitempty"`
	// Database is legacy singular engine; migrated into Databases on load.
	Database  string         `yaml:"database,omitempty"`
	Databases []SiteDatabase `yaml:"databases,omitempty"`
	Cache     string         `yaml:"cache,omitempty"`
	// HTTPS enables automatic TLS for this site when the panel is not force-disabling HTTPS.
	HTTPS bool `yaml:"https,omitempty"`
	// Backend is legacy singular HTTP API; migrated into Backends on load.
	Backend  *SiteBackend  `yaml:"backend,omitempty"`
	Backends []SiteBackend `yaml:"backends,omitempty"` // HTTP APIs on a static site
	Limits   Limits        `yaml:"limits,omitempty"`
	Cron    []CronJob    `yaml:"cron,omitempty"`
	Jobs    []Job        `yaml:"jobs,omitempty"` // long-running Type=simple services
	Extra   map[string]string `yaml:"extra,omitempty"`
}

// Limits are per-site resource caps.
type Limits struct {
	DiskQuota string `yaml:"disk_quota,omitempty"`
	LogMax    string `yaml:"log_max,omitempty"`
	MemoryMax string `yaml:"memory_max,omitempty"`
	CPUQuota  string `yaml:"cpu_quota,omitempty"` // systemd CPUQuota, e.g. 100% = 1 core
}

// CronJob is a oneshot job for a site. Schedule is optional (empty = Run now only).
type CronJob struct {
	Name     string       `yaml:"name"`
	Schedule string       `yaml:"schedule,omitempty"` // 5-field cron; empty = no timer
	Exec     string       `yaml:"exec"`
	Runtime  *SiteRuntime `yaml:"runtime,omitempty"` // optional Node (managed or BYO)
}

// Job is a long-running (Type=simple) per-site service with no schedule.
type Job struct {
	Name    string       `yaml:"name"`
	Exec    string       `yaml:"exec"`
	Runtime *SiteRuntime `yaml:"runtime,omitempty"` // optional Node (managed or BYO)
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
	for i := range c.Sites {
		NormalizeSiteDatabases(&c.Sites[i])
		NormalizeSiteBackends(&c.Sites[i])
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config as YAML to path.
func Save(path string, c *Config) error {
	for i := range c.Sites {
		NormalizeSiteDatabases(&c.Sites[i])
		NormalizeSiteBackends(&c.Sites[i])
	}
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
	if c.Backup != nil {
		NormalizeBackup(c.Backup)
		if err := ValidateBackup(c.Backup); err != nil {
			return err
		}
	}

	seen := make(map[string]struct{}, len(c.Sites))
	byName := make(map[string]Site, len(c.Sites))
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
		byName[key] = s

		switch s.Type {
		case SiteTypeStatic, SiteTypeApp, SiteTypeContainer:
		default:
			return fmt.Errorf("%s: invalid type %q (want static, app, or container)", prefix, s.Type)
		}
		parent := strings.ToLower(strings.TrimSpace(s.Parent))
		if parent != "" {
			if err := validateDomain(s.Parent); err != nil {
				return fmt.Errorf("%s.parent: %w", prefix, err)
			}
			if parent == key {
				return fmt.Errorf("%s.parent: cannot reference self", prefix)
			}
			if len(s.Databases) > 0 || strings.TrimSpace(s.Database) != "" {
				return fmt.Errorf("%s: child sites cannot declare databases (use parent)", prefix)
			}
			if strings.TrimSpace(s.Cache) != "" {
				return fmt.Errorf("%s: child sites cannot declare cache (use parent)", prefix)
			}
		}
		if cache := strings.ToLower(strings.TrimSpace(s.Cache)); cache != "" && cache != "valkey" && cache != "redis" {
			return fmt.Errorf("%s.cache: unsupported %q (want valkey)", prefix, s.Cache)
		}
		seenDB := map[string]struct{}{}
		for j, db := range s.Databases {
			id := NormalizeDBID(s.Name, db.ID)
			if err := validateDBID(id); err != nil {
				return fmt.Errorf("%s.databases[%d].id: %w", prefix, j, err)
			}
			if err := validateEngine(db.Engine); err != nil {
				return fmt.Errorf("%s.databases[%d].engine: %w", prefix, j, err)
			}
			if _, ok := seenDB[id]; ok {
				return fmt.Errorf("%s.databases[%d]: duplicate id %q", prefix, j, id)
			}
			seenDB[id] = struct{}{}
		}
		for j, alias := range s.Aliases {
			if err := validateDomain(alias); err != nil {
				return fmt.Errorf("%s.aliases[%d]: %w", prefix, j, err)
			}
		}
		if err := validateIdleTimeout(s); err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
		if err := ValidateLimits(s.Limits); err != nil {
			return fmt.Errorf("%s.limits: %w", prefix, err)
		}
		if err := ValidateSiteBackends(s.Type, s.Backends); err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
		if s.Runtime != nil {
			NormalizeRuntime(s.Runtime)
			if err := ValidateSiteRuntime(s.Type, s.Runtime); err != nil {
				return fmt.Errorf("%s: %w", prefix, err)
			}
		}
		seenCron := map[string]struct{}{}
		for j, job := range s.Cron {
			if strings.TrimSpace(job.Name) == "" {
				return fmt.Errorf("%s.cron[%d]: name is required", prefix, j)
			}
			if err := ValidateTaskName(job.Name); err != nil {
				return fmt.Errorf("%s.cron[%d].name: %w", prefix, j, err)
			}
			// Schedule is optional (oneshot / seed via Run now). When set, must be 5-field cron.
			if strings.TrimSpace(job.Schedule) != "" {
				if err := validateCronSchedule(job.Schedule); err != nil {
					return fmt.Errorf("%s.cron[%d].schedule: %w", prefix, j, err)
				}
			}
			if strings.TrimSpace(job.Exec) == "" {
				return fmt.Errorf("%s.cron[%d]: exec is required", prefix, j)
			}
			if err := ValidateNodeRuntimeFields(job.Runtime); err != nil {
				return fmt.Errorf("%s.cron[%d].runtime: %w", prefix, j, err)
			}
			key := strings.ToLower(strings.TrimSpace(job.Name))
			if _, ok := seenCron[key]; ok {
				return fmt.Errorf("%s.cron[%d]: duplicate job name %q", prefix, j, job.Name)
			}
			seenCron[key] = struct{}{}
		}
		seenJobs := map[string]struct{}{}
		for j, job := range s.Jobs {
			if strings.TrimSpace(job.Name) == "" {
				return fmt.Errorf("%s.jobs[%d]: name is required", prefix, j)
			}
			if err := ValidateTaskName(job.Name); err != nil {
				return fmt.Errorf("%s.jobs[%d].name: %w", prefix, j, err)
			}
			if strings.TrimSpace(job.Exec) == "" {
				return fmt.Errorf("%s.jobs[%d]: exec is required", prefix, j)
			}
			if err := ValidateNodeRuntimeFields(job.Runtime); err != nil {
				return fmt.Errorf("%s.jobs[%d].runtime: %w", prefix, j, err)
			}
			key := strings.ToLower(strings.TrimSpace(job.Name))
			if _, ok := seenJobs[key]; ok {
				return fmt.Errorf("%s.jobs[%d]: duplicate job name %q", prefix, j, job.Name)
			}
			seenJobs[key] = struct{}{}
		}
	}
	for i, s := range c.Sites {
		parent := strings.ToLower(strings.TrimSpace(s.Parent))
		if parent == "" {
			continue
		}
		if _, ok := byName[parent]; !ok {
			return fmt.Errorf("sites[%d].parent: unknown site %q", i, s.Parent)
		}
		if strings.TrimSpace(byName[parent].Parent) != "" {
			return fmt.Errorf("sites[%d].parent: %q is itself a child site", i, s.Parent)
		}
	}
	seenPorts := map[int]string{}
	for i, s := range c.Sites {
		for j, b := range s.Backends {
			if !b.Enabled || b.Port == 0 {
				continue
			}
			p := b.Port
			label := fmt.Sprintf("%s/%s", s.Name, b.ID)
			if other, ok := seenPorts[p]; ok {
				return fmt.Errorf("sites[%d].backends[%d].port: %d already used by %q", i, j, p, other)
			}
			seenPorts[p] = label
		}
	}
	if d := strings.ToLower(strings.TrimSpace(c.Panel.Domain)); d != "" {
		if name, ok := c.HostnameTaken(d); ok {
			return fmt.Errorf("panel.domain %q conflicts with site %q", c.Panel.Domain, name)
		}
	}
	return nil
}

// HostnameTaken reports whether host is already used as a site name or alias.
func (c *Config) HostnameTaken(host string) (siteName string, taken bool) {
	if c == nil {
		return "", false
	}
	want := strings.ToLower(strings.TrimSpace(host))
	if want == "" {
		return "", false
	}
	for _, s := range c.Sites {
		if strings.ToLower(s.Name) == want {
			return s.Name, true
		}
		for _, a := range s.Aliases {
			if strings.ToLower(strings.TrimSpace(a)) == want {
				return s.Name, true
			}
		}
	}
	return "", false
}

// PanelParentSite returns the apex site that owns panel.domain, if any.
// Ownership means panel.domain equals the apex or is a subdomain of it.
func (c *Config) PanelParentSite() string {
	if c == nil {
		return ""
	}
	domain := strings.ToLower(strings.TrimSpace(c.Panel.Domain))
	if domain == "" {
		return ""
	}
	for _, s := range c.Sites {
		if strings.TrimSpace(s.Parent) != "" {
			continue
		}
		apex := strings.ToLower(strings.TrimSpace(s.Name))
		if apex == "" {
			continue
		}
		if domain == apex || strings.HasSuffix(domain, "."+apex) {
			return s.Name
		}
	}
	return ""
}

// ClearPanelDomainIfOwned clears panel.domain when it belongs to the given apex site.
func (c *Config) ClearPanelDomainIfOwned(apex string) {
	if c == nil {
		return
	}
	apex = strings.ToLower(strings.TrimSpace(apex))
	if apex == "" {
		return
	}
	parent := strings.ToLower(c.PanelParentSite())
	domain := strings.ToLower(strings.TrimSpace(c.Panel.Domain))
	if parent == apex || domain == apex {
		c.Panel.Domain = ""
	}
}

// BuildPanelDomain joins a DNS label with an apex domain.
func BuildPanelDomain(label, apex string) (string, error) {
	label = strings.ToLower(strings.TrimSpace(label))
	apex = strings.ToLower(strings.TrimSpace(apex))
	if err := validateDomain(apex); err != nil {
		return "", fmt.Errorf("parent: %w", err)
	}
	if err := validateDomainLabel(label); err != nil {
		return "", fmt.Errorf("label: %w", err)
	}
	domain := label + "." + apex
	if err := validateDomain(domain); err != nil {
		return "", err
	}
	return domain, nil
}

func validateDomainLabel(label string) error {
	n := strings.TrimSpace(label)
	if n == "" {
		return fmt.Errorf("empty label")
	}
	if strings.Contains(n, ".") {
		return fmt.Errorf("label must be a single DNS label")
	}
	if len(n) > 63 {
		return fmt.Errorf("label too long")
	}
	if !domainRE.MatchString(n) {
		return fmt.Errorf("invalid label %q", label)
	}
	return nil
}

// validateIdleTimeout checks idle_timeout when set.
// Cooperative socket-activated apps exit after this idle period (WBR_IDLE_TIMEOUT).
// With socket_proxy, idle_timeout maps to systemd-socket-proxyd --exit-idle-time=.
func validateIdleTimeout(s Site) error {
	if s.SocketProxy {
		if s.Type != SiteTypeApp {
			return fmt.Errorf("socket_proxy is only valid for type app")
		}
		if !s.SocketActivation {
			return fmt.Errorf("socket_proxy requires socket_activation: true")
		}
	}
	raw := strings.TrimSpace(s.IdleTimeout)
	if raw == "" {
		return nil
	}
	if s.Type != SiteTypeApp {
		return fmt.Errorf("idle_timeout is only valid for type app")
	}
	if !s.SocketActivation {
		return fmt.Errorf("idle_timeout requires socket_activation: true")
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid idle_timeout %q: %w", raw, err)
	}
	if d < time.Second {
		return fmt.Errorf("idle_timeout must be at least 1s")
	}
	if d > 24*time.Hour {
		return fmt.Errorf("idle_timeout too large (max 24h)")
	}
	return nil
}

func validateCronSchedule(schedule string) error {
	fields := strings.Fields(strings.TrimSpace(schedule))
	if len(fields) != 5 {
		return fmt.Errorf("must have 5 cron fields (got %d)", len(fields))
	}
	return nil
}

var taskNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

// ValidateTaskName checks cron/process job names (unit-safe, no path chars).
func ValidateTaskName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("empty name")
	}
	if len(n) > 64 {
		return fmt.Errorf("name too long")
	}
	if !taskNameRE.MatchString(n) {
		return fmt.Errorf("invalid name %q (use letters, digits, _ or -)", name)
	}
	return nil
}

// FindCron returns a cron job by name on the site.
func (s Site) FindCron(name string) (CronJob, int, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for i, j := range s.Cron {
		if strings.ToLower(strings.TrimSpace(j.Name)) == want {
			return j, i, true
		}
	}
	return CronJob{}, -1, false
}

// FindJob returns a long-running job by name on the site.
func (s Site) FindJob(name string) (Job, int, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for i, j := range s.Jobs {
		if strings.ToLower(strings.TrimSpace(j.Name)) == want {
			return j, i, true
		}
	}
	return Job{}, -1, false
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
