package config

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	dbIDRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	slugRE = regexp.MustCompile(`[^a-z0-9]+`)
)

// SiteDatabase is one parent-owned database attachment.
type SiteDatabase struct {
	ID     string `yaml:"id"`
	Engine string `yaml:"engine"`
}

// DefaultDBID returns the domain-derived default database id (slug of parent).
func DefaultDBID(parent string) string {
	d := strings.ToLower(strings.TrimSpace(parent))
	d = slugRE.ReplaceAllString(d, "_")
	d = strings.Trim(d, "_")
	if d == "" {
		d = "site"
	}
	return d
}

// NormalizeDBID returns id or DefaultDBID(parent) when blank.
func NormalizeDBID(parent, id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return DefaultDBID(parent)
	}
	return id
}

// IsDefaultDBID reports whether id is the parent domain default.
func IsDefaultDBID(parent, id string) bool {
	return NormalizeDBID(parent, id) == DefaultDBID(parent)
}

// NormalizeSiteDatabases migrates legacy Site.Database into Databases and clears Database.
func NormalizeSiteDatabases(s *Site) {
	if s == nil {
		return
	}
	legacy := strings.ToLower(strings.TrimSpace(s.Database))
	if legacy != "" && len(s.Databases) == 0 {
		s.Databases = []SiteDatabase{{
			ID:     DefaultDBID(s.Name),
			Engine: legacy,
		}}
	}
	s.Database = ""
	for i := range s.Databases {
		s.Databases[i].ID = NormalizeDBID(s.Name, s.Databases[i].ID)
		s.Databases[i].Engine = strings.ToLower(strings.TrimSpace(s.Databases[i].Engine))
	}
}

// FindDatabase returns a database entry by id.
func (s Site) FindDatabase(id string) (SiteDatabase, bool) {
	want := NormalizeDBID(s.Name, id)
	for _, d := range s.Databases {
		if NormalizeDBID(s.Name, d.ID) == want {
			return d, true
		}
	}
	return SiteDatabase{}, false
}

// ChildrenOf returns sites whose Parent matches parentName.
func (c *Config) ChildrenOf(parentName string) []Site {
	if c == nil {
		return nil
	}
	parent := strings.ToLower(strings.TrimSpace(parentName))
	var out []Site
	for _, s := range c.Sites {
		if strings.ToLower(strings.TrimSpace(s.Parent)) == parent {
			out = append(out, s)
		}
	}
	return out
}

// FindSite returns a site by name.
func (c *Config) FindSite(name string) (Site, int, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for i, s := range c.Sites {
		if strings.ToLower(s.Name) == want {
			return s, i, true
		}
	}
	return Site{}, -1, false
}

func validateDBID(id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if !dbIDRE.MatchString(id) {
		return fmt.Errorf("invalid database id %q", id)
	}
	return nil
}

func validateEngine(engine string) error {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "sqlite", "postgres", "mariadb":
		return nil
	case "":
		return fmt.Errorf("engine is required")
	default:
		return fmt.Errorf("unsupported engine %q", engine)
	}
}
