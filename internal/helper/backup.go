package helper

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/backup"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/distro"
	"github.com/weborpanel/webor/internal/providers/restic"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
	"github.com/weborpanel/webor/internal/version"
)

func resticInstallHint() string {
	return distro.Detect().InstallHintRestic()
}

const (
	defaultBackupEnvPath      = restic.EnvFile
	defaultBackupStagingRoot  = "/var/lib/webor/backup-staging"
	defaultRestoreStagingRoot = "/var/lib/webor/restore-staging"
	defaultConfigPath         = "/etc/webor/config.yaml"
)

func (o *Ops) backupEnvPath() string {
	if o != nil && o.BackupEnvPath != "" {
		return o.BackupEnvPath
	}
	return defaultBackupEnvPath
}

func (o *Ops) backupStagingRoot() string {
	if o != nil && o.BackupStagingRoot != "" {
		return o.BackupStagingRoot
	}
	return defaultBackupStagingRoot
}

func (o *Ops) restoreStagingRoot() string {
	if o != nil && o.RestoreStagingRoot != "" {
		return o.RestoreStagingRoot
	}
	return defaultRestoreStagingRoot
}

// BackupConfigureArgs are written into config + backup.env.
type BackupConfigureResult struct {
	Repository string `json:"repository"`
	Configured bool   `json:"configured"`
}

// BackupRunResult is returned after a successful full backup.
type BackupRunResult struct {
	SnapshotID string `json:"snapshot_id"`
	Sites      int    `json:"sites"`
}

// BackupListResult wraps snapshots for the panel.
type BackupListResult struct {
	Snapshots []BackupSnapshotView `json:"snapshots"`
}

// BackupSnapshotView is a panel-friendly snapshot row.
type BackupSnapshotView struct {
	ID        string               `json:"id"`
	ShortID   string               `json:"short_id"`
	Time      time.Time            `json:"time"`
	Hostname  string               `json:"hostname"`
	Sites     []backup.ManifestSite `json:"sites,omitempty"`
}

// BackupStatusResult reports tool + credential readiness.
type BackupStatusResult struct {
	ResticInstalled bool   `json:"restic_installed"`
	Configured      bool   `json:"configured"`
	Repository      string `json:"repository,omitempty"`
	Hint            string `json:"hint,omitempty"`
}

// BackupConfigure writes S3 settings into config.yaml and credentials into backup.env,
// initializes the restic repo, and ensures the backup timer when scheduled.
func (o *Ops) resticOK() bool {
	if o != nil && o.ResticBin != "" {
		if st, err := os.Stat(o.ResticBin); err == nil && !st.IsDir() {
			return true
		}
	}
	return restic.Available()
}

func (o *Ops) BackupConfigure(cfgPath string, b config.Backup, accessKey, secretKey, resticPassword, region string) (*BackupConfigureResult, error) {
	if !o.resticOK() {
		return nil, fmt.Errorf("restic not installed; run: %s", resticInstallHint())
	}
	b.Enabled = true
	config.NormalizeBackup(&b)
	if err := config.ValidateBackup(&b); err != nil {
		return nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if cfg == nil {
		cfg = &config.Config{Version: 1}
	}
	cfg.Backup = &b
	if err := config.Save(cfgPath, cfg); err != nil {
		return nil, err
	}

	env := map[string]string{}
	if accessKey != "" {
		env["AWS_ACCESS_KEY_ID"] = accessKey
	}
	if secretKey != "" {
		env["AWS_SECRET_ACCESS_KEY"] = secretKey
	}
	if region == "" {
		region = b.S3.Region
	}
	if region != "" {
		env["AWS_DEFAULT_REGION"] = region
	}
	if resticPassword != "" {
		env["RESTIC_PASSWORD"] = resticPassword
	}
	// Merge with existing so password-only updates work.
	envPath := o.backupEnvPath()
	if prev, err := restic.LoadEnvFile(envPath); err == nil {
		for k, v := range prev {
			if _, ok := env[k]; !ok && v != "" {
				env[k] = v
			}
		}
	}
	if env["RESTIC_PASSWORD"] == "" {
		return nil, fmt.Errorf("RESTIC_PASSWORD is required")
	}
	if strings.TrimSpace(b.Repository) == "" {
		if env["AWS_ACCESS_KEY_ID"] == "" || env["AWS_SECRET_ACCESS_KEY"] == "" {
			return nil, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
		}
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		return nil, err
	}
	if err := restic.WriteEnvFile(envPath, env); err != nil {
		return nil, err
	}

	client, err := o.resticClient(cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := client.Init(ctx); err != nil {
		return nil, fmt.Errorf("restic init: %w", err)
	}
	// Timer sync requires root + systemd; ignore failures in lab/dev.
	_ = o.EnsureBackupSchedule(cfg)
	return &BackupConfigureResult{Repository: b.ResticRepositoryURL(), Configured: true}, nil
}

// BackupStatus reports whether restic and credentials are ready.
func (o *Ops) BackupStatus(cfgPath string) (*BackupStatusResult, error) {
	res := &BackupStatusResult{ResticInstalled: o.resticOK()}
	if !res.ResticInstalled {
		res.Hint = resticInstallHint()
		return res, nil
	}
	cfg, err := config.Load(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if cfg != nil && cfg.Backup != nil && cfg.Backup.Enabled {
		res.Repository = cfg.Backup.ResticRepositoryURL()
	}
	if _, err := os.Stat(o.backupEnvPath()); err == nil {
		env, _ := restic.LoadEnvFile(o.backupEnvPath())
		if env["RESTIC_PASSWORD"] != "" {
			res.Configured = true
		}
	}
	if !res.Configured {
		res.Hint = "configure S3 credentials and restic password in the Backups panel"
	}
	return res, nil
}

// BackupRun performs a full dump+stage+restic backup.
func (o *Ops) BackupRun(ctx context.Context, cfgPath string) (*BackupRunResult, error) {
	if !o.resticOK() {
		return nil, fmt.Errorf("restic not installed; run: %s", resticInstallHint())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	if cfg.Backup == nil || !cfg.Backup.Enabled {
		return nil, fmt.Errorf("backup not configured")
	}
	client, err := o.resticClient(cfg)
	if err != nil {
		return nil, err
	}

	runID := time.Now().UTC().Format("20060102T150405Z")
	staging := filepath.Join(o.backupStagingRoot(), runID)
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	host, _ := os.Hostname()
	man := backup.BuildManifest(cfg, host, version.Version)
	if err := backup.WriteManifest(staging, man); err != nil {
		return nil, err
	}
	if err := stagePanel(staging, cfgPath); err != nil {
		return nil, err
	}
	if err := stageSites(staging, cfg); err != nil {
		return nil, err
	}
	if err := stageDumps(ctx, staging, cfg); err != nil {
		return nil, err
	}

	snapID, err := client.Backup(ctx, staging, []string{"webor", "hostname=" + host})
	if err != nil {
		return nil, err
	}
	config.NormalizeBackup(cfg.Backup)
	_ = client.ForgetPrune(ctx, cfg.Backup.Retention.KeepDaily, cfg.Backup.Retention.KeepWeekly)

	return &BackupRunResult{SnapshotID: snapID, Sites: len(cfg.Sites)}, nil
}

// BackupList returns restic snapshots (manifest sites filled when dumpable).
func (o *Ops) BackupList(ctx context.Context, cfgPath string) (*BackupListResult, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	client, err := o.resticClient(cfg)
	if err != nil {
		return nil, err
	}
	snaps, err := client.Snapshots(ctx)
	if err != nil {
		return nil, err
	}
	out := &BackupListResult{}
	for _, s := range snaps {
		view := BackupSnapshotView{
			ID:       s.ID,
			ShortID:  s.ShortID,
			Time:     s.Time,
			Hostname: s.Hostname,
		}
		// Best-effort: restore only manifest for site list (skip on failure).
		if sites := o.snapshotManifestSites(ctx, client, s.ID); sites != nil {
			view.Sites = sites
		}
		out.Snapshots = append(out.Snapshots, view)
	}
	return out, nil
}

func (o *Ops) snapshotManifestSites(ctx context.Context, client *restic.Client, snapID string) []backup.ManifestSite {
	tmp, err := os.MkdirTemp("", "webor-man-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmp)
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := client.Restore(ctx2, snapID, tmp, []string{"manifest.json"}); err != nil {
		return nil
	}
	m, _, err := backup.FindManifest(tmp)
	if err != nil {
		return nil
	}
	return m.Sites
}

// BackupForget prunes according to retention (approval-gated at IPC).
func (o *Ops) BackupForget(ctx context.Context, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	client, err := o.resticClient(cfg)
	if err != nil {
		return err
	}
	config.NormalizeBackup(cfg.Backup)
	return client.ForgetPrune(ctx, cfg.Backup.Retention.KeepDaily, cfg.Backup.Retention.KeepWeekly)
}

// BackupRestoreArgs controls selective restore.
type BackupRestoreArgs struct {
	SnapshotID   string
	Sites        []string
	IncludePanel bool
	ConfigPath   string
	StateDir     string
}

// BackupRestore restores selected sites (and optional panel) from a snapshot.
func (o *Ops) BackupRestore(ctx context.Context, args BackupRestoreArgs) error {
	if !o.resticOK() {
		return fmt.Errorf("restic not installed; run: %s", resticInstallHint())
	}
	cfgPath := args.ConfigPath
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}
	cfg, err := config.Load(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{Version: 1}
	}
	client, err := o.resticClient(cfg)
	if err != nil {
		return err
	}

	runID := time.Now().UTC().Format("20060102T150405Z")
	staging := filepath.Join(o.restoreStagingRoot(), runID)
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	includes := backup.RestoreIncludes(args.Sites, args.IncludePanel)
	if err := client.Restore(ctx, args.SnapshotID, staging, includes); err != nil {
		return err
	}
	_, snapRoot, err := backup.FindManifest(staging)
	if err != nil {
		// Still try staging itself
		snapRoot = staging
	}

	restoredCfgPath := filepath.Join(snapRoot, "etc", "webor", "config.yaml")
	var snapCfg *config.Config
	if raw, err := os.ReadFile(restoredCfgPath); err == nil {
		snapCfg, _ = config.Parse(raw)
	}

	for _, name := range args.Sites {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		src := filepath.Join(snapRoot, backup.SiteTreeRel(name))
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("site %s missing in snapshot: %w", name, err)
		}
		if err := o.CreateSiteUser(name); err != nil {
			// Lab/tests may lack root; still restore files into the site tree.
			_ = os.MkdirAll(sites.Root(name), 0o755)
		}
		dst := sites.Root(name)
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("restore site files %s: %w", name, err)
		}
		u := sites.Username(name)
		_ = exec.Command("chown", "-R", u+":"+u, dst).Run()

		if snapCfg != nil {
			if site, _, ok := snapCfg.FindSite(name); ok {
				mergeSite(cfg, site)
				// Import SQL dumps when present
				for _, db := range site.Databases {
					id := config.NormalizeDBID(site.Name, db.ID)
					engine := strings.ToLower(db.Engine)
					dump := dumpPath(snapRoot, engine, id, name)
					stateDir := args.StateDir
					if stateDir == "" {
						stateDir = config.DefaultStateDir
					}
					if _, err := o.EnsureSiteDatabase(name, engine, id, stateDir); err != nil {
						return fmt.Errorf("ensure db %s: %w", id, err)
					}
					if dump != "" {
						if err := importDump(engine, id, dump); err != nil {
							return fmt.Errorf("import dump %s: %w", id, err)
						}
					}
				}
				for _, b := range site.EnabledBackends() {
					if b.HasNodeRuntime() && b.Runtime != nil && strings.TrimSpace(b.Runtime.Command) == "" {
						if _, err := o.EnsureNodeRuntime(ctx, b.Runtime.Version); err != nil {
							return fmt.Errorf("ensure node for backend %s/%s: %w", name, b.ID, err)
						}
					}
					if _, err := o.EnsureHTTPBackend(site, b); err != nil {
						// Unit may start after files land; keep going but surface later via reconcile.
						_ = err
					}
				}
				if site.Type == config.SiteTypeApp && site.HasNodeRuntime() && site.Runtime != nil &&
					strings.TrimSpace(site.Runtime.Command) == "" {
					if _, err := o.EnsureNodeRuntime(ctx, site.Runtime.Version); err != nil {
						return fmt.Errorf("ensure node for app %s: %w", name, err)
					}
				}
			}
		}
	}

	if args.IncludePanel {
		srcState := filepath.Join(snapRoot, "var", "lib", "webor")
		dstState := args.StateDir
		if dstState == "" {
			dstState = config.DefaultStateDir
		}
		if st, err := os.Stat(srcState); err == nil && st.IsDir() {
			// Copy SQLite files only — avoid clobbering live helper sockets.
			_ = filepath.Walk(srcState, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				if !strings.HasSuffix(info.Name(), ".sqlite") && !strings.HasSuffix(info.Name(), ".db") {
					return nil
				}
				rel, _ := filepath.Rel(srcState, path)
				dest := filepath.Join(dstState, rel)
				_ = os.MkdirAll(filepath.Dir(dest), 0o755)
				return copyFile(path, dest)
			})
		}
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	return nil
}

// EnsureBackupSchedule writes/enables or removes webor-backup timer units.
func (o *Ops) EnsureBackupSchedule(cfg *config.Config) error {
	unit, timer, bodyUnit, bodyTimer, err := systemd.RenderBackupUnits(cfg)
	if err != nil {
		return err
	}
	if bodyUnit == "" {
		// disable
		_ = o.SystemctlStop(timer)
		_ = exec.Command("systemctl", "disable", timer).Run()
		_ = os.Remove(filepath.Join("/etc/systemd/system", unit))
		_ = os.Remove(filepath.Join("/etc/systemd/system", timer))
		_ = o.SystemctlDaemonReload()
		return nil
	}
	if err := o.WriteUnitFile("webor", unit, bodyUnit); err != nil {
		return err
	}
	if err := o.WriteUnitFile("webor", timer, bodyTimer); err != nil {
		return err
	}
	_ = o.SystemctlDaemonReload()
	cmd := exec.Command("systemctl", "enable", "--now", timer)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enable %s: %w (%s)", timer, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (o *Ops) resticClient(cfg *config.Config) (*restic.Client, error) {
	if cfg == nil || cfg.Backup == nil {
		return nil, fmt.Errorf("backup not configured")
	}
	envPath := o.backupEnvPath()
	env, err := restic.LoadEnvFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w (configure backups first)", envPath, err)
	}
	if env["RESTIC_CACHE_DIR"] == "" {
		env["RESTIC_CACHE_DIR"] = filepath.Join(filepath.Dir(envPath), "restic-cache")
		_ = os.MkdirAll(env["RESTIC_CACHE_DIR"], 0o700)
	}
	return &restic.Client{
		Bin:        o.ResticBin,
		Repository: cfg.Backup.ResticRepositoryURL(),
		Env:        env,
	}, nil
}

func stagePanel(staging, cfgPath string) error {
	etc := filepath.Join(staging, "etc", "webor")
	if err := os.MkdirAll(etc, 0o755); err != nil {
		return err
	}
	if err := copyFile(cfgPath, filepath.Join(etc, "config.yaml")); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = copyFile("/etc/webor/panel.env", filepath.Join(etc, "panel.env"))

	stateSrc := config.DefaultStateDir
	stateDst := filepath.Join(staging, "var", "lib", "webor")
	if err := os.MkdirAll(stateDst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(stateSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == "backup-staging" || name == "restore-staging" || name == "uploads" {
			continue
		}
		src := filepath.Join(stateSrc, name)
		dst := filepath.Join(stateDst, name)
		if e.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func stageSites(staging string, cfg *config.Config) error {
	for _, s := range cfg.Sites {
		src := sites.Root(s.Name)
		dst := filepath.Join(staging, backup.SiteTreeRel(s.Name))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("stage site %s: %w", s.Name, err)
		}
	}
	return nil
}

func stageDumps(ctx context.Context, staging string, cfg *config.Config) error {
	_ = os.MkdirAll(filepath.Join(staging, "dumps", "postgres"), 0o755)
	_ = os.MkdirAll(filepath.Join(staging, "dumps", "mariadb"), 0o755)
	_ = os.MkdirAll(filepath.Join(staging, "dumps", "sqlite"), 0o755)
	_ = os.MkdirAll(filepath.Join(staging, "dumps", "valkey"), 0o755)

	for _, s := range cfg.Sites {
		for _, db := range s.Databases {
			id := config.NormalizeDBID(s.Name, db.ID)
			engine := strings.ToLower(strings.TrimSpace(db.Engine))
			switch engine {
			case "postgres", "postgresql":
				out := filepath.Join(staging, "dumps", "postgres", id+".sql.gz")
				plain := strings.TrimSuffix(out, ".gz")
				c := exec.CommandContext(ctx, "pg_dump", "--no-owner", "-d", id, "-f", plain)
				if err := c.Run(); err == nil {
					_ = gzipFile(plain, out)
					_ = os.Remove(plain)
				}
			case "mariadb", "mysql":
				out := filepath.Join(staging, "dumps", "mariadb", id+".sql.gz")
				plain := strings.TrimSuffix(out, ".gz")
				c := exec.CommandContext(ctx, "mariadb-dump", id)
				f, err := os.Create(plain)
				if err == nil {
					c.Stdout = f
					_ = c.Run()
					_ = f.Close()
					_ = gzipFile(plain, out)
					_ = os.Remove(plain)
				}
			case "sqlite":
				src := filepath.Join(sites.Root(s.Name), "data", id+".sqlite")
				dstDir := filepath.Join(staging, "dumps", "sqlite", s.Name)
				_ = os.MkdirAll(dstDir, 0o755)
				_ = copyFile(src, filepath.Join(dstDir, id+".sqlite"))
			}
		}
	}
	// Valkey RDB if present
	for _, cand := range []string{"/var/lib/valkey/dump.rdb", "/var/lib/redis/dump.rdb"} {
		if _, err := os.Stat(cand); err == nil {
			_ = copyFile(cand, filepath.Join(staging, "dumps", "valkey", "dump.rdb"))
			break
		}
	}
	return nil
}

func dumpPath(snapRoot, engine, id, domain string) string {
	switch engine {
	case "postgres", "postgresql":
		p := filepath.Join(snapRoot, "dumps", "postgres", id+".sql.gz")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		p2 := filepath.Join(snapRoot, "dumps", "postgres", id+".sql")
		if _, err := os.Stat(p2); err == nil {
			return p2
		}
	case "mariadb", "mysql":
		p := filepath.Join(snapRoot, "dumps", "mariadb", id+".sql.gz")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	case "sqlite":
		p := filepath.Join(snapRoot, "dumps", "sqlite", domain, id+".sqlite")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func importDump(engine, id, path string) error {
	switch engine {
	case "postgres", "postgresql":
		if strings.HasSuffix(path, ".gz") {
			cmd := exec.Command("bash", "-c", fmt.Sprintf("gzip -dc %s | psql -d %s -v ON_ERROR_STOP=1", shellQuote(path), shellQuote(id)))
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
			}
			return nil
		}
		cmd := exec.Command("psql", "-d", id, "-f", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
		}
	case "mariadb", "mysql":
		if strings.HasSuffix(path, ".gz") {
			cmd := exec.Command("bash", "-c", fmt.Sprintf("gzip -dc %s | mariadb %s", shellQuote(path), shellQuote(id)))
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
			}
			return nil
		}
		cmd := exec.Command("mariadb", id, "-e", "source "+path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
		}
	case "sqlite":
		// already copied with site tree; dump copy is redundant
	}
	return nil
}

func mergeSite(cfg *config.Config, site config.Site) {
	for i, s := range cfg.Sites {
		if strings.EqualFold(s.Name, site.Name) {
			cfg.Sites[i] = site
			return
		}
	}
	cfg.Sites = append(cfg.Sites, site)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		return copyFile(path, target)
	})
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	cmd := exec.Command("gzip", "-c")
	cmd.Stdin = in
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	cmd.Stdout = out
	return cmd.Run()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
