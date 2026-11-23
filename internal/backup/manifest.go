// Package backup defines snapshot manifest and path helpers for restic layouts.
package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
)

const ManifestSchemaVersion = 1

// Manifest is written at the root of each backup staging tree.
type Manifest struct {
	SchemaVersion int             `json:"schema_version"`
	Hostname      string          `json:"hostname"`
	CreatedAt     time.Time       `json:"created_at"`
	WeborVersion  string          `json:"webor_version,omitempty"`
	Sites         []ManifestSite  `json:"sites"`
}

// ManifestSite is enough for the restore UI checkbox list.
type ManifestSite struct {
	Name     string              `json:"name"`
	Parent   string              `json:"parent,omitempty"`
	Type     string              `json:"type"`
	Cache    string              `json:"cache,omitempty"`
	Backend  bool                `json:"backend,omitempty"`
	Databases []ManifestDatabase `json:"databases,omitempty"`
}

// ManifestDatabase records a parent DB included in dumps.
type ManifestDatabase struct {
	ID     string `json:"id"`
	Engine string `json:"engine"`
}

// BuildManifest creates a manifest from desired-state config.
func BuildManifest(cfg *config.Config, hostname, weborVersion string) Manifest {
	m := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Hostname:      hostname,
		CreatedAt:     time.Now().UTC(),
		WeborVersion:  weborVersion,
	}
	if cfg == nil {
		return m
	}
	for _, s := range cfg.Sites {
		site := s
		config.NormalizeSiteBackends(&site)
		ms := ManifestSite{
			Name:   site.Name,
			Parent: site.Parent,
			Type:   site.Type,
			Cache:  site.Cache,
		}
		if len(site.EnabledBackends()) > 0 {
			ms.Backend = true
		}
		for _, db := range site.Databases {
			ms.Databases = append(ms.Databases, ManifestDatabase{
				ID:     config.NormalizeDBID(site.Name, db.ID),
				Engine: strings.ToLower(strings.TrimSpace(db.Engine)),
			})
		}
		m.Sites = append(m.Sites, ms)
	}
	return m
}

// WriteManifest writes manifest.json under staging root.
func WriteManifest(stagingRoot string, m Manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stagingRoot, "manifest.json"), append(raw, '\n'), 0o644)
}

// ReadManifest loads manifest.json from a directory (staging or restored tree).
func ReadManifest(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	return &m, nil
}

// FindManifest walks a restore target for manifest.json (restic may nest paths).
func FindManifest(root string) (*Manifest, string, error) {
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Name() == "manifest.json" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return nil, "", fmt.Errorf("manifest.json not found under %s", root)
	}
	m, err := ReadManifest(filepath.Dir(found))
	return m, filepath.Dir(found), err
}

// SiteTreeRel returns the staging-relative path for a site tree.
func SiteTreeRel(domain string) string {
	return filepath.Join("srv", "sites", strings.ToLower(strings.TrimSpace(domain)))
}

// RestoreIncludes builds restic --include paths for selective restore.
func RestoreIncludes(sites []string, includePanel bool) []string {
	var out []string
	out = append(out, "manifest.json")
	if includePanel {
		out = append(out, "etc/webor", "var/lib/webor")
	}
	for _, s := range sites {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		out = append(out, SiteTreeRel(s))
		out = append(out, filepath.Join("dumps", "postgres"))
		out = append(out, filepath.Join("dumps", "mariadb"))
		out = append(out, filepath.Join("dumps", "sqlite", s))
	}
	if len(sites) > 0 {
		out = append(out, "dumps/valkey")
		out = append(out, "etc/webor/config.yaml")
	}
	return out
}
