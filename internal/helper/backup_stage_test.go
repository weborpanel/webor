package helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/sites"
)

func TestStageSitesExcludesLogsByDefault(t *testing.T) {
	root := t.TempDir()
	sites.RootDir = root
	t.Cleanup(func() { sites.RootDir = "/srv/sites" })

	domain := "ex.com"
	siteRoot := sites.Root(domain)
	_ = os.MkdirAll(filepath.Join(siteRoot, "public"), 0o755)
	_ = os.WriteFile(filepath.Join(siteRoot, "public", "index.html"), []byte("ok"), 0o644)
	logs := sites.Logs(domain)
	_ = os.MkdirAll(logs, 0o755)
	_ = os.WriteFile(filepath.Join(logs, "access.log"), []byte("secret"), 0o644)

	host := t.TempDir()
	logging.HostDir = host
	t.Cleanup(func() { logging.HostDir = "/var/log/webor" })
	_ = os.WriteFile(filepath.Join(host, "backup.log"), []byte("host"), 0o644)

	cfg := &config.Config{
		Version: 1,
		Sites:   []config.Site{{Name: domain, Type: config.SiteTypeStatic}},
		Backup:  &config.Backup{Enabled: true, IncludeLogs: false},
	}
	staging := t.TempDir()
	if err := stageSites(staging, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(staging, "srv", "sites", domain, "public", "index.html")); err != nil {
		t.Fatal("expected public staged")
	}
	if _, err := os.Stat(filepath.Join(staging, "srv", "sites", domain, "logs", "access.log")); !os.IsNotExist(err) {
		t.Fatal("logs should be excluded by default")
	}

	cfg.Backup.IncludeLogs = true
	staging2 := t.TempDir()
	if err := stageSites(staging2, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(staging2, "srv", "sites", domain, "logs", "access.log")); err != nil {
		t.Fatal("logs should be included when include_logs")
	}
	if _, err := os.Stat(filepath.Join(staging2, "logs", "host", "backup.log")); err != nil {
		t.Fatal("host logs should be staged when include_logs")
	}
}
