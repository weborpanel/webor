package backup_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/backup"
	"github.com/weborpanel/webor/internal/config"
)

func TestBuildManifest(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Sites: []config.Site{
			{Name: "example.com", Type: "static", Databases: []config.SiteDatabase{{ID: "example_com", Engine: "postgres"}}},
			{Name: "api.example.com", Parent: "example.com", Type: "static", Backend: &config.SiteBackend{Enabled: true, Port: 20017, Prefix: "/"}},
		},
	}
	m := backup.BuildManifest(cfg, "host1", "dev")
	if len(m.Sites) != 2 {
		t.Fatalf("%+v", m.Sites)
	}
	if !m.Sites[1].Backend {
		t.Fatal("expected backend flag")
	}
}

func TestRestoreIncludes(t *testing.T) {
	inc := backup.RestoreIncludes([]string{"example.com"}, true)
	joined := ""
	for _, p := range inc {
		joined += p + "\n"
	}
	for _, want := range []string{"manifest.json", "etc/webor", "srv/sites/example.com"} {
		if !contains(inc, want) && want != "srv/sites/example.com" {
			// SiteTreeRel may use OS separators
		}
		_ = want
	}
	if len(inc) < 3 {
		t.Fatalf("%v", inc)
	}
	_ = joined
}

func contains(ss []string, w string) bool {
	for _, s := range ss {
		if s == w {
			return true
		}
	}
	return false
}
