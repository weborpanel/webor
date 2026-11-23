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
	MaxBackends    = 8
)

var (
	backendPathRE = regexp.MustCompile(`^/[A-Za-z0-9._/-]*$`)
	backendIDRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
)

// SiteBackend is an optional HTTP process attached to a static site (path-based proxy).
type SiteBackend struct {
	ID        string       `yaml:"id,omitempty"`
	Enabled   bool         `yaml:"enabled,omitempty"`
	Prefix    string       `yaml:"prefix,omitempty"`    // primary path, default /api
	Paths     []string     `yaml:"paths,omitempty"`     // extra prefixes (e.g. /ws)
	WebSocket bool         `yaml:"websocket,omitempty"` // upgrade-friendly reverse_proxy
	Exec      string       `yaml:"exec,omitempty"`      // binary path or Node entry; relative to apis/<id>/
	Port      int          `yaml:"port,omitempty"`      // assigned by Webor; loopback only
	Runtime   *SiteRuntime `yaml:"runtime,omitempty"`   // optional Node (managed or BYO)
}

// NormalizeBackend fills defaults when the backend is enabled.
func NormalizeBackend(b *SiteBackend) {
	if b == nil || !b.Enabled {
		return
	}
	if strings.TrimSpace(b.Prefix) == "" {
		b.Prefix = "/api"
	}
	b.Prefix = normalizeBackendPath(b.Prefix)
	if strings.TrimSpace(b.ID) == "" {
		b.ID = BackendIDFromPrefix(b.Prefix)
	}
	b.ID = strings.ToLower(strings.TrimSpace(b.ID))
	NormalizeRuntime(b.Runtime)
	if strings.TrimSpace(b.Exec) == "" {
		if b.HasNodeRuntime() {
			b.Exec = "server.js"
		} else {
			b.Exec = "./server"
		}
	}
}

// UniqueBackendID returns base, or base-2, base-3, … when taken.
// taken reports whether an id is already in use.
func UniqueBackendID(base string, taken func(id string) bool) string {
	base = strings.ToLower(strings.TrimSpace(base))
	if base == "" {
		base = "api"
	}
	if err := ValidateBackendID(base); err != nil {
		base = "api"
	}
	if taken == nil || !taken(base) {
		return base
	}
	for i := 2; i < 100; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if len(cand) > 32 {
			trim := 32 - len(fmt.Sprintf("-%d", i))
			if trim < 1 {
				trim = 1
			}
			cand = fmt.Sprintf("%s-%d", base[:trim], i)
			cand = strings.Trim(cand, "-")
		}
		if !taken(cand) {
			return cand
		}
	}
	return base
}

// BackendIDFromPrefix derives a filesystem-safe id from a path prefix.
func BackendIDFromPrefix(prefix string) string {
	p := normalizeBackendPath(prefix)
	if p == "" || p == "/" {
		return "root"
	}
	p = strings.TrimPrefix(p, "/")
	p = strings.ToLower(p)
	var b strings.Builder
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '/' || r == '_' || r == '-' || r == '.':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return "api"
	}
	if len(out) > 32 {
		out = out[:32]
		out = strings.TrimRight(out, "-")
	}
	if !backendIDRE.MatchString(out) {
		return "api"
	}
	return out
}

// HasNodeRuntime reports whether the backend uses a Node interpreter.
func (b *SiteBackend) HasNodeRuntime() bool {
	if b == nil || b.Runtime == nil {
		return false
	}
	NormalizeRuntime(b.Runtime)
	return b.Runtime.Kind == RuntimeKindNode
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

// ValidateBackendID checks a backend id.
func ValidateBackendID(id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return fmt.Errorf("empty id")
	}
	if !backendIDRE.MatchString(id) {
		return fmt.Errorf("invalid id %q", id)
	}
	return nil
}

// ValidateSiteBackend validates a single backend config when present/enabled.
func ValidateSiteBackend(siteType string, b *SiteBackend) error {
	if b == nil || !b.Enabled {
		return nil
	}
	if siteType != SiteTypeStatic {
		return fmt.Errorf("backend is only valid for type static")
	}
	NormalizeBackend(b)
	if err := ValidateBackendID(b.ID); err != nil {
		return fmt.Errorf("backend.id: %w", err)
	}
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
	if err := ValidateBackendRuntime(b.Runtime); err != nil {
		return err
	}
	if b.Port != 0 && (b.Port < BackendPortMin || b.Port > BackendPortMax) {
		return fmt.Errorf("backend.port %d out of range %d-%d", b.Port, BackendPortMin, BackendPortMax)
	}
	return nil
}

// pathOverlaps reports whether two normalized prefixes conflict (equal or one is a parent of the other).
func pathOverlaps(a, b string) bool {
	a = normalizeBackendPath(a)
	b = normalizeBackendPath(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if a == "/" || b == "/" {
		return true
	}
	if strings.HasPrefix(a+"/", b+"/") || strings.HasPrefix(b+"/", a+"/") {
		return true
	}
	return false
}

// ValidateSiteBackends validates the backends slice for a site (ids, paths, cap).
func ValidateSiteBackends(siteType string, backends []SiteBackend) error {
	if len(backends) == 0 {
		return nil
	}
	if siteType != SiteTypeStatic {
		return fmt.Errorf("backends are only valid for type static")
	}
	if len(backends) > MaxBackends {
		return fmt.Errorf("at most %d backends allowed", MaxBackends)
	}
	seenID := map[string]struct{}{}
	var claimed []string
	for i := range backends {
		b := &backends[i]
		if !b.Enabled {
			continue
		}
		if err := ValidateSiteBackend(siteType, b); err != nil {
			return fmt.Errorf("backends[%d]: %w", i, err)
		}
		id := strings.ToLower(strings.TrimSpace(b.ID))
		if _, ok := seenID[id]; ok {
			return fmt.Errorf("backends[%d]: duplicate id %q", i, id)
		}
		seenID[id] = struct{}{}
		prefs := b.BackendPathPrefixes()
		for _, p := range prefs {
			for _, c := range claimed {
				if pathOverlaps(p, c) {
					return fmt.Errorf("backends[%d]: path %q overlaps %q", i, p, c)
				}
			}
			claimed = append(claimed, p)
		}
	}
	return nil
}

// NormalizeSiteBackends migrates legacy Site.Backend into Backends and clears Backend.
func NormalizeSiteBackends(s *Site) {
	if s == nil {
		return
	}
	if s.Backend != nil && len(s.Backends) == 0 {
		b := *s.Backend
		if b.Enabled || b.Port != 0 || strings.TrimSpace(b.Exec) != "" || strings.TrimSpace(b.Prefix) != "" {
			b.Enabled = true
			NormalizeBackend(&b)
			s.Backends = []SiteBackend{b}
		}
	}
	s.Backend = nil
	for i := range s.Backends {
		s.Backends[i].Enabled = true
		NormalizeBackend(&s.Backends[i])
	}
}

// FindBackend returns a backend by id.
func (s Site) FindBackend(id string) (SiteBackend, int, bool) {
	want := strings.ToLower(strings.TrimSpace(id))
	for i, b := range s.Backends {
		if strings.ToLower(strings.TrimSpace(b.ID)) == want {
			return b, i, true
		}
	}
	return SiteBackend{}, -1, false
}

// EnabledBackends returns backends marked enabled (after normalize, all entries).
func (s Site) EnabledBackends() []SiteBackend {
	var list []SiteBackend
	for _, b := range s.Backends {
		if b.Enabled {
			list = append(list, b)
		}
	}
	return list
}
