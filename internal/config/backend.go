package config

import (
	"fmt"
	"regexp"
	"strings"
)

// BackendPortMin/Max define the auto-assigned loopback port pool for HTTP backends.
const (
	BackendPortMin = 20000
	BackendPortMax = 29999
)

var backendPathRE = regexp.MustCompile(`^/[A-Za-z0-9._/-]*$`)

// SiteBackend is an optional HTTP process attached to a static site (path-based proxy).
type SiteBackend struct {
	Enabled   bool     `yaml:"enabled,omitempty"`
	Prefix    string   `yaml:"prefix,omitempty"`    // primary path, default /api
	Paths     []string `yaml:"paths,omitempty"`     // extra prefixes (e.g. /ws)
	WebSocket bool     `yaml:"websocket,omitempty"` // upgrade-friendly reverse_proxy
	Exec      string   `yaml:"exec,omitempty"`      // relative to site root; default ./bin/server
	Port      int      `yaml:"port,omitempty"`      // assigned by Webor; loopback only
}

// NormalizeBackend fills defaults when the backend is enabled.
func NormalizeBackend(b *SiteBackend) {
	if b == nil || !b.Enabled {
		return
	}
	if strings.TrimSpace(b.Prefix) == "" {
		b.Prefix = "/api"
	}
	if strings.TrimSpace(b.Exec) == "" {
		b.Exec = "./bin/server"
	}
}

// BackendPathPrefixes returns unique path prefixes to match in Caddy (prefix + paths).
func (b SiteBackend) BackendPathPrefixes() []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = normalizeBackendPath(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(b.Prefix)
	for _, p := range b.Paths {
		add(p)
	}
	if len(out) == 0 {
		out = []string{"/api"}
	}
	return out
}

// CaddyPathMatchers expands each prefix into exact + wildcard matchers for Caddy.
func CaddyPathMatchers(prefixes []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, p := range prefixes {
		p = normalizeBackendPath(p)
		if p == "" {
			continue
		}
		for _, m := range []string{p, strings.TrimSuffix(p, "/") + "/*"} {
			if m == "/*" {
				m = "/*"
			}
			if p == "/" {
				m = "/*"
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}

func normalizeBackendPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

// ValidateBackendPath checks a single proxy path prefix.
func ValidateBackendPath(p string) error {
	p = normalizeBackendPath(p)
	if p == "" {
		return fmt.Errorf("empty path")
	}
	// "/" is allowed: whole-host proxy (typical for api.example.com).
	if strings.Contains(p, "..") {
		return fmt.Errorf("path must not contain ..")
	}
	if !backendPathRE.MatchString(p) {
		return fmt.Errorf("invalid path %q", p)
	}
	return nil
}

// ValidateSiteBackend validates backend config when present/enabled.
func ValidateSiteBackend(siteType string, b *SiteBackend) error {
	if b == nil || !b.Enabled {
		return nil
	}
	if siteType != SiteTypeStatic {
		return fmt.Errorf("backend is only valid for type static")
	}
	NormalizeBackend(b)
	if err := ValidateBackendPath(b.Prefix); err != nil {
		return fmt.Errorf("backend.prefix: %w", err)
	}
	for i, p := range b.Paths {
		if err := ValidateBackendPath(p); err != nil {
			return fmt.Errorf("backend.paths[%d]: %w", i, err)
		}
	}
	if strings.TrimSpace(b.Exec) == "" {
		return fmt.Errorf("backend.exec is required")
	}
	if strings.Contains(b.Exec, "..") {
		return fmt.Errorf("backend.exec must not contain ..")
	}
	if b.Port != 0 && (b.Port < BackendPortMin || b.Port > BackendPortMax) {
		return fmt.Errorf("backend.port %d out of range %d-%d", b.Port, BackendPortMin, BackendPortMax)
	}
	return nil
}
