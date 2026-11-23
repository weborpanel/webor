package helper_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/backup"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/restic"
	"github.com/weborpanel/webor/internal/sites"
)

func resticBin(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "bin", "restic"),
		"restic",
	}
	// resolve relative to module root when tests run from package dir
	if wd, err := os.Getwd(); err == nil {
		candidates = append([]string{
			filepath.Join(wd, "..", "..", "bin", "restic"),
		}, candidates...)
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Skip("restic not available")
	return ""
}

func TestBackupRunRestoreLocalRestic(t *testing.T) {
	bin := resticBin(t)
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	envFile := filepath.Join(tmp, "backup.env")
	cfgPath := filepath.Join(tmp, "config.yaml")
	sitesRoot := filepath.Join(tmp, "srv", "sites")
	stateDir := filepath.Join(tmp, "var", "lib", "webor")
	_ = os.MkdirAll(filepath.Join(sitesRoot, "lab.example.com", "public"), 0o755)
	_ = os.WriteFile(filepath.Join(sitesRoot, "lab.example.com", "public", "index.html"), []byte("hello-backup"), 0o644)
	_ = os.MkdirAll(stateDir, 0o755)

	prevRoot := sites.RootDir
	sites.RootDir = sitesRoot
	t.Cleanup(func() { sites.RootDir = prevRoot })

	cfg := &config.Config{
		Version: 1,
		Backup: &config.Backup{
			Enabled:    true,
			Repository: repo,
			Retention:  config.BackupRetention{KeepDaily: 7, KeepWeekly: 4},
		},
		Sites: []config.Site{
			{Name: "lab.example.com", Type: config.SiteTypeStatic, Root: "public"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := restic.WriteEnvFile(envFile, map[string]string{
		"RESTIC_PASSWORD":  "test-password-lab",
		"RESTIC_CACHE_DIR": filepath.Join(tmp, "restic-cache"),
	}); err != nil {
		t.Fatal(err)
	}

	ops := &helper.Ops{
		BackupEnvPath:      envFile,
		BackupStagingRoot:  filepath.Join(tmp, "backup-staging"),
		RestoreStagingRoot: filepath.Join(tmp, "restore-staging"),
		ResticBin:          bin,
	}

	// init via configure with repository override
	_, err := ops.BackupConfigure(cfgPath, *cfg.Backup, "", "", "test-password-lab", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	run, err := ops.BackupRun(ctx, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if run.Sites != 1 {
		t.Fatalf("sites=%d", run.Sites)
	}

	list, err := ops.BackupList(ctx, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Snapshots) < 1 {
		t.Fatal("expected snapshot")
	}

	// wipe site and restore
	_ = os.RemoveAll(filepath.Join(sitesRoot, "lab.example.com"))
	snapID := list.Snapshots[0].ID
	if err := ops.BackupRestore(ctx, helper.BackupRestoreArgs{
		SnapshotID:   snapID,
		Sites:        []string{"lab.example.com"},
		IncludePanel: false,
		ConfigPath:   cfgPath,
		StateDir:     stateDir,
	}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(sitesRoot, "lab.example.com", "public", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello-backup" {
		t.Fatalf("got %q", body)
	}
	_ = backup.SiteTreeRel("lab.example.com")
}
