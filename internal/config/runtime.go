package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Runtime kinds for app sites.
const (
	RuntimeKindNode = "node"
)

var nodeVersionRE = regexp.MustCompile(`^v?\d+(\.\d+){0,2}$`)

// SiteRuntime selects an interpreter for type:app (optional).
// Empty Kind means raw ExecStart (binary path), unchanged from pre-runtime behavior.
type SiteRuntime struct {
	Kind    string `yaml:"kind,omitempty"`    // node
	Version string `yaml:"version,omitempty"` // managed: "22" or "22.14.0"
	Command string `yaml:"command,omitempty"` // BYO absolute path to node
}

// NormalizeRuntime trims fields.
func NormalizeRuntime(r *SiteRuntime) {
	if r == nil {
		return
	}
	r.Kind = strings.ToLower(strings.TrimSpace(r.Kind))
	r.Version = strings.TrimSpace(r.Version)
	r.Version = strings.TrimPrefix(r.Version, "v")
	r.Command = strings.TrimSpace(r.Command)
}

// ValidateNodeRuntimeFields checks kind/version/command for a Node runtime (any host).
func ValidateNodeRuntimeFields(r *SiteRuntime) error {
	if r == nil {
		return nil
	}
	NormalizeRuntime(r)
	if r.Kind == "" && r.Version == "" && r.Command == "" {
		return nil
	}
	if r.Kind == "" {
		return fmt.Errorf("runtime.kind is required")
	}
	if r.Kind != RuntimeKindNode {
		return fmt.Errorf("runtime.kind %q unsupported (want node)", r.Kind)
	}
	if r.Command != "" {
		if !filepath.IsAbs(r.Command) {
			return fmt.Errorf("runtime.command must be an absolute path")
		}
		if strings.Contains(r.Command, "..") {
			return fmt.Errorf("runtime.command must not contain ..")
		}
		return nil
	}
	if r.Version == "" {
		return fmt.Errorf("runtime.version or runtime.command is required")
	}
	if !nodeVersionRE.MatchString(r.Version) {
		return fmt.Errorf("runtime.version %q invalid", r.Version)
	}
	return nil
}

// ValidateSiteRuntime checks site-level runtime (type:app only).
func ValidateSiteRuntime(siteType string, r *SiteRuntime) error {
	if r == nil {
		return nil
	}
	NormalizeRuntime(r)
	if r.Kind == "" && r.Version == "" && r.Command == "" {
		return nil
	}
	if siteType != SiteTypeApp {
		return fmt.Errorf("runtime is only valid for type app")
	}
	return ValidateNodeRuntimeFields(r)
}

// ValidateBackendRuntime checks backend.runtime on a static HTTP backend.
func ValidateBackendRuntime(r *SiteRuntime) error {
	if r == nil {
		return nil
	}
	NormalizeRuntime(r)
	if r.Kind == "" && r.Version == "" && r.Command == "" {
		return nil
	}
	if err := ValidateNodeRuntimeFields(r); err != nil {
		return fmt.Errorf("backend.runtime: %w", err)
	}
	return nil
}

// HasNodeRuntime reports whether the site uses a Node interpreter.
func (s Site) HasNodeRuntime() bool {
	if s.Runtime == nil {
		return false
	}
	NormalizeRuntime(s.Runtime)
	return s.Runtime.Kind == RuntimeKindNode
}

// HasNodeRuntime reports whether the cron job uses a Node interpreter.
func (j CronJob) HasNodeRuntime() bool {
	if j.Runtime == nil {
		return false
	}
	NormalizeRuntime(j.Runtime)
	return j.Runtime.Kind == RuntimeKindNode
}

// HasNodeRuntime reports whether the long-running job uses a Node interpreter.
func (j Job) HasNodeRuntime() bool {
	if j.Runtime == nil {
		return false
	}
	NormalizeRuntime(j.Runtime)
	return j.Runtime.Kind == RuntimeKindNode
}
