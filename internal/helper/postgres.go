package helper

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/distro"
	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/postgres"
)

// PostgresStatusResult reports whether a Postgres cluster is usable.
type PostgresStatusResult struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Message   string `json:"message,omitempty"`
}

// EnsureSiteDatabaseResult is returned after creating/reconciling a site DB.
type EnsureSiteDatabaseResult struct {
	Engine     string `json:"engine"`
	ID         string `json:"id,omitempty"`
	Database   string `json:"database"`
	PeerRole   string `json:"peer_role,omitempty"`
	AppRole    string `json:"app_role,omitempty"`
	ReadOnly   string `json:"readonly_role,omitempty"`
	PeerDSN    string `json:"peer_dsn,omitempty"`
	PeerURL    string `json:"peer_url,omitempty"`
	SecretPath string `json:"secret_path,omitempty"`
	Path       string `json:"path,omitempty"` // sqlite file
}

// PostgresStatus checks for a local Postgres install and running service.
func (o *Ops) PostgresStatus() (*PostgresStatusResult, error) {
	res := &PostgresStatusResult{}
	if _, err := exec.LookPath("psql"); err != nil {
		// RHEL may have server without client on PATH yet; treat postgres unit as installed.
		if !postgresUnitActive() && !postgresUnitLoaded() {
			res.Message = "psql not found"
			return res, nil
		}
	}
	res.Installed = true
	if unit, ok := postgresActiveUnit(); ok {
		res.Active = true
		res.Message = unit + " active"
		return res, nil
	}
	res.Message = "postgresql installed but not active"
	return res, nil
}

func postgresUnitCandidates() []string {
	return []string{
		"postgresql",
		"postgresql.service",
		"postgresql-16",
		"postgresql-15",
		"postgresql-14",
		"postgresql-13",
	}
}

func postgresUnitActive() bool {
	_, ok := postgresActiveUnit()
	return ok
}

func postgresActiveUnit() (string, bool) {
	for _, unit := range postgresUnitCandidates() {
		if out, err := exec.Command("systemctl", "is-active", unit).CombinedOutput(); err == nil {
			if strings.TrimSpace(string(out)) == "active" {
				return unit, true
			}
		}
	}
	return "", false
}

func postgresUnitLoaded() bool {
	for _, unit := range postgresUnitCandidates() {
		if out, err := exec.Command("systemctl", "show", "-p", "LoadState", "--value", unit).CombinedOutput(); err == nil {
			if strings.TrimSpace(string(out)) == "loaded" {
				return true
			}
		}
	}
	return false
}

// InstallPostgres installs and enables PostgreSQL (Debian/Ubuntu or Alma/RHEL). Requires root.
func (o *Ops) InstallPostgres() error {
	info := distro.Detect()
	switch {
	case info.IsRHEL() || distro.LookManager() == "dnf" || distro.LookManager() == "yum":
		if err := distro.InstallPackages("postgresql-server", "postgresql"); err != nil {
			return err
		}
		// Idempotent init; ignore "already initialized" style errors.
		_ = exec.Command("postgresql-setup", "--initdb").Run()
		_ = exec.Command("postgresql-setup", "initdb").Run()
	case info.IsDebian() || distro.LookManager() == "apt-get":
		if err := distro.InstallPackages("postgresql", "postgresql-client"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("InstallPostgres needs apt-get or dnf (Debian/Ubuntu or AlmaLinux/RHEL)")
	}
	if err := distro.RunCommands([][]string{{"systemctl", "enable", "--now", "postgresql"}}); err != nil {
		// Try versioned units on EL.
		var last error = err
		for _, unit := range []string{"postgresql-16", "postgresql-15", "postgresql-14"} {
			if e := distro.RunCommands([][]string{{"systemctl", "enable", "--now", unit}}); e == nil {
				last = nil
				break
			} else {
				last = e
			}
		}
		if last != nil {
			return last
		}
	}
	st, err := o.PostgresStatus()
	if err != nil {
		return err
	}
	if !st.Installed || !st.Active {
		return fmt.Errorf("postgres install finished but service not active: %s", st.Message)
	}
	return nil
}

// EnsureSiteDatabase creates/reconciles a site database for the given engine.
// Blank id becomes dbengine.DefaultID(domain).
func (o *Ops) EnsureSiteDatabase(domain, engine, id, stateDir string) (*EnsureSiteDatabaseResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	id = dbengine.NormalizeID(domain, id)
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "sqlite":
		return o.ensureSQLite(domain, id)
	case "mariadb":
		return o.ensureMariaDB(domain, id, stateDir)
	case "postgres", "":
		return o.ensurePostgres(domain, id, stateDir)
	default:
		return nil, fmt.Errorf("unsupported engine %q", engine)
	}
}

func (o *Ops) ensurePostgres(domain, id, stateDir string) (*EnsureSiteDatabaseResult, error) {
	st, err := o.PostgresStatus()
	if err != nil {
		return nil, err
	}
	if !st.Installed {
		return nil, fmt.Errorf("postgres not installed")
	}
	if !st.Active {
		_ = exec.Command("systemctl", "start", "postgresql").Run()
		st, _ = o.PostgresStatus()
		if st == nil || !st.Active {
			return nil, fmt.Errorf("postgres not active")
		}
	}

	if err := o.EnsurePostgresPanelAuth(nil, stateDir); err != nil {
		return nil, fmt.Errorf("postgres panel auth: %w", err)
	}
	if err := o.migrateDoubledPostgres(domain, id, stateDir); err != nil {
		return nil, err
	}

	dbName := postgres.DBName(domain, id)
	peer := postgres.PeerRole(domain)
	app := postgres.AppRole(domain, id)
	ro := postgres.ReadOnlyRole(domain, id)
	secretPath := postgres.SecretPath(stateDir, domain, id)

	password, err := randomPassword(24)
	if err != nil {
		return nil, err
	}

	// Peer role always exists (Linux site user).
	sqlPeer := fmt.Sprintf(`
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %s) THEN
    EXECUTE format('CREATE ROLE %%I LOGIN', %s);
  END IF;
END$$;
SELECT 'ok';
`, pgQuote(peer), pgQuote(peer))
	if err := o.psql(sqlPeer); err != nil {
		return nil, fmt.Errorf("create peer role: %w", err)
	}

	// App role (may equal peer for default id) gets a password for TCP / env.
	if app != peer {
		sqlApp := fmt.Sprintf(`
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %s) THEN
    EXECUTE format('CREATE ROLE %%I LOGIN PASSWORD %%L', %s, %s);
  ELSE
    EXECUTE format('ALTER ROLE %%I WITH LOGIN PASSWORD %%L', %s, %s);
  END IF;
END$$;
`, pgQuote(app), pgQuote(app), pgQuote(password), pgQuote(app), pgQuote(password))
		if err := o.psql(sqlApp); err != nil {
			return nil, fmt.Errorf("create app role: %w", err)
		}
	} else {
		sqlPass := fmt.Sprintf(`ALTER ROLE %s WITH LOGIN PASSWORD %s;`, pgIdent(peer), pgQuote(password))
		if err := o.psql(sqlPass); err != nil {
			return nil, fmt.Errorf("set peer password: %w", err)
		}
	}

	sqlDB := fmt.Sprintf(`
SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_database WHERE datname = %s)
  THEN 'exists' ELSE 'missing' END;
`, pgQuote(dbName))
	out, err := o.psqlOut(sqlDB)
	if err != nil {
		return nil, fmt.Errorf("check database: %w", err)
	}
	if !strings.Contains(out, "exists") {
		if err := o.psql(fmt.Sprintf("CREATE DATABASE %s OWNER %s;", pgIdent(dbName), pgIdent(app))); err != nil {
			return nil, fmt.Errorf("create database: %w", err)
		}
	}

	sqlRO := fmt.Sprintf(`
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %s) THEN
    EXECUTE format('CREATE ROLE %%I LOGIN PASSWORD %%L', %s, %s);
  ELSE
    EXECUTE format('ALTER ROLE %%I WITH LOGIN PASSWORD %%L', %s, %s);
  END IF;
END$$;
REVOKE ALL ON DATABASE %s FROM PUBLIC;
GRANT CONNECT ON DATABASE %s TO %s;
GRANT CONNECT ON DATABASE %s TO %s;
GRANT CONNECT ON DATABASE %s TO %s;
`, pgQuote(ro), pgQuote(ro), pgQuote(password), pgQuote(ro), pgQuote(password),
		pgIdent(dbName), pgIdent(dbName), pgIdent(peer), pgIdent(dbName), pgIdent(app), pgIdent(dbName), pgIdent(ro))
	if err := o.psql(sqlRO); err != nil {
		return nil, fmt.Errorf("create readonly role: %w", err)
	}
	sqlGrants := fmt.Sprintf(`
GRANT USAGE ON SCHEMA public TO %s;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO %s;
ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA public GRANT SELECT ON TABLES TO %s;
ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA public GRANT SELECT ON SEQUENCES TO %s;
`, pgIdent(ro), pgIdent(ro), pgIdent(ro), pgIdent(app), pgIdent(ro), pgIdent(app), pgIdent(ro))
	if err := o.psqlDB(dbName, sqlGrants); err != nil {
		return nil, fmt.Errorf("grant readonly: %w", err)
	}

	sec := postgres.ROSecret{
		User:     ro,
		Password: password,
		Database: dbName,
		Host:     postgres.DefaultTCPHost,
		Port:     postgres.DefaultPort,
	}
	if err := postgres.WriteSecret(secretPath, sec); err != nil {
		return nil, fmt.Errorf("write secret: %w", err)
	}
	_ = chownWebor(secretPath)
	_ = chownWebor(filepath.Dir(secretPath))

	return &EnsureSiteDatabaseResult{
		Engine:     "postgres",
		ID:         id,
		Database:   dbName,
		PeerRole:   peer,
		AppRole:    app,
		ReadOnly:   ro,
		PeerDSN:    postgres.PeerDSN(domain, id),
		PeerURL:    postgres.PeerURL(domain, id),
		SecretPath: secretPath,
	}, nil
}

// DropSiteDatabase removes the site database for the given engine.
func (o *Ops) DropSiteDatabase(domain, engine, id, stateDir string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	id = dbengine.NormalizeID(domain, id)
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "sqlite":
		return o.dropSQLite(domain, id)
	case "mariadb":
		return o.dropMariaDB(domain, id, stateDir)
	case "postgres", "":
		return o.dropPostgres(domain, id, stateDir)
	default:
		return fmt.Errorf("unsupported engine %q", engine)
	}
}

func (o *Ops) dropPostgres(domain, id, stateDir string) error {
	dbName := postgres.DBName(domain, id)
	peer := postgres.PeerRole(domain)
	app := postgres.AppRole(domain, id)
	ro := postgres.ReadOnlyRole(domain, id)
	secretPath := postgres.ResolveSecretPath(stateDir, domain, id)
	legacyPath := postgres.LegacySecretPath(stateDir, domain)

	st, err := o.PostgresStatus()
	if err != nil {
		return err
	}
	if st.Installed && st.Active {
		_ = o.psql(fmt.Sprintf(`
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid();
`, pgQuote(dbName)))
		_ = o.psql(fmt.Sprintf("DROP DATABASE IF EXISTS %s;", pgIdent(dbName)))
		_ = o.psql(fmt.Sprintf("DROP ROLE IF EXISTS %s;", pgIdent(ro)))
		if app != peer {
			_ = o.psql(fmt.Sprintf("DROP ROLE IF EXISTS %s;", pgIdent(app)))
		}
		legacyDB := postgres.LegacyDBName(domain, id)
		legacyApp := postgres.LegacyAppRole(domain, id)
		legacyRO := postgres.LegacyReadOnlyRole(domain, id)
		if legacyDB != dbName {
			_ = o.psql(fmt.Sprintf(`
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid();
`, pgQuote(legacyDB)))
			_ = o.psql(fmt.Sprintf("DROP DATABASE IF EXISTS %s;", pgIdent(legacyDB)))
			_ = o.psql(fmt.Sprintf("DROP ROLE IF EXISTS %s;", pgIdent(legacyRO)))
			if legacyApp != peer && legacyApp != app {
				_ = o.psql(fmt.Sprintf("DROP ROLE IF EXISTS %s;", pgIdent(legacyApp)))
			}
		}
		// Do not drop peer role — it is the Linux site user shared across DBs.
	}
	_ = postgres.RemoveSecret(secretPath)
	if dbengine.IsDefaultID(domain, id) {
		_ = postgres.RemoveSecret(legacyPath)
	}
	return nil
}

// RotateDatabasePassword regenerates the password for a site database and rewrites the secret.
func (o *Ops) RotateDatabasePassword(domain, engine, id, stateDir string) (*EnsureSiteDatabaseResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	id = dbengine.NormalizeID(domain, id)
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	engine, err := dbengine.Normalize(engine)
	if err != nil {
		return nil, err
	}
	switch engine {
	case dbengine.SQLite:
		return nil, fmt.Errorf("sqlite has no password to rotate")
	case dbengine.MariaDB:
		return o.rotateMariaDBPassword(domain, id, stateDir)
	case dbengine.Postgres:
		return o.rotatePostgresPassword(domain, id, stateDir)
	default:
		return nil, fmt.Errorf("unsupported engine %q", engine)
	}
}

func (o *Ops) rotatePostgresPassword(domain, id, stateDir string) (*EnsureSiteDatabaseResult, error) {
	st, err := o.PostgresStatus()
	if err != nil {
		return nil, err
	}
	if !st.Installed || !st.Active {
		return nil, fmt.Errorf("postgres not active")
	}
	if err := o.migrateDoubledPostgres(domain, id, stateDir); err != nil {
		return nil, err
	}
	dbName := postgres.DBName(domain, id)
	peer := postgres.PeerRole(domain)
	app := postgres.AppRole(domain, id)
	ro := postgres.ReadOnlyRole(domain, id)
	secretPath := postgres.SecretPath(stateDir, domain, id)

	password, err := randomPassword(24)
	if err != nil {
		return nil, err
	}
	if err := o.psql(fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s;", pgIdent(app), pgQuote(password))); err != nil {
		return nil, fmt.Errorf("alter app role: %w", err)
	}
	if err := o.psql(fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s;", pgIdent(ro), pgQuote(password))); err != nil {
		return nil, fmt.Errorf("alter readonly role: %w", err)
	}
	sec := postgres.ROSecret{
		User:     ro,
		Password: password,
		Database: dbName,
		Host:     postgres.DefaultTCPHost,
		Port:     postgres.DefaultPort,
	}
	if err := postgres.WriteSecret(secretPath, sec); err != nil {
		return nil, err
	}
	_ = chownWebor(secretPath)
	return &EnsureSiteDatabaseResult{
		Engine:     "postgres",
		ID:         id,
		Database:   dbName,
		PeerRole:   peer,
		AppRole:    app,
		ReadOnly:   ro,
		PeerDSN:    postgres.PeerDSN(domain, id),
		PeerURL:    postgres.PeerURL(domain, id),
		SecretPath: secretPath,
	}, nil
}

// PostgresAuthDB is one parent database the panel wants reachable over localhost.
type PostgresAuthDB struct {
	Domain string
	ID     string
}

// EnsurePostgresPanelAuth lets the panel process log into site databases.
// Distros that ship ident/peer for 127.0.0.1 reject the password DSN DBAdminer
// uses, so Browse/Edit never mount and the UI reports database not found.
// Existing doubled names (wbr_<host>_<host>_postgres_1) are renamed when dbs
// is non-empty.
func (o *Ops) EnsurePostgresPanelAuth(dbs []PostgresAuthDB, stateDir string) error {
	st, err := o.PostgresStatus()
	if err != nil || st == nil || !st.Installed || !st.Active {
		return nil
	}
	if err := o.ensurePostgresListen(); err != nil {
		return err
	}
	if err := o.ensurePostgresHBA(); err != nil {
		return err
	}
	for _, db := range dbs {
		if err := o.migrateDoubledPostgres(db.Domain, db.ID, stateDir); err != nil {
			return err
		}
	}
	return nil
}

func (o *Ops) ensurePostgresListen() error {
	out, err := o.psqlOut("SHOW listen_addresses;")
	if err != nil {
		return err
	}
	v := strings.Trim(strings.TrimSpace(out), "'")
	if v != "" {
		return nil
	}
	if err := o.psql(`ALTER SYSTEM SET listen_addresses = 'localhost';`); err != nil {
		return err
	}
	if unit, ok := postgresActiveUnit(); ok {
		if err := exec.Command("systemctl", "restart", unit).Run(); err != nil {
			return fmt.Errorf("restart %s: %w", unit, err)
		}
	}
	return nil
}

func (o *Ops) ensurePostgresHBA() error {
	out, err := o.psqlOut("SHOW hba_file;")
	if err != nil {
		return err
	}
	path := strings.TrimSpace(out)
	if path == "" || strings.ContainsAny(path, "\n\r") {
		return fmt.Errorf("unexpected hba_file %q", out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	next, changed := postgres.MergePanelAuthHBA(string(raw))
	if !changed {
		return nil
	}
	mode := os.FileMode(0o640)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode()
	}
	if err := os.WriteFile(path, []byte(next), mode); err != nil {
		return err
	}
	if err := o.psql("SELECT pg_reload_conf();"); err != nil {
		return err
	}
	return nil
}

func (o *Ops) migrateDoubledPostgres(domain, id, stateDir string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	id = dbengine.NormalizeID(domain, id)
	canon := postgres.DBName(domain, id)
	legacy := postgres.LegacyDBName(domain, id)
	if legacy == canon {
		return nil
	}
	canonApp := postgres.AppRole(domain, id)
	legacyApp := postgres.LegacyAppRole(domain, id)
	canonRO := postgres.ReadOnlyRole(domain, id)
	legacyRO := postgres.LegacyReadOnlyRole(domain, id)
	peer := postgres.PeerRole(domain)

	if o.pgTrue("SELECT 1 FROM pg_database WHERE datname = "+pgQuote(legacy)) &&
		!o.pgTrue("SELECT 1 FROM pg_database WHERE datname = "+pgQuote(canon)) {
		_ = o.psql(fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid();`, pgQuote(legacy)))
		if err := o.psql(fmt.Sprintf("ALTER DATABASE %s RENAME TO %s;", pgIdent(legacy), pgIdent(canon))); err != nil {
			return fmt.Errorf("rename database: %w", err)
		}
	}
	if legacyApp != peer {
		if err := o.renameRole(legacyApp, canonApp); err != nil {
			return err
		}
	}
	if err := o.renameRole(legacyRO, canonRO); err != nil {
		return err
	}

	path := postgres.ResolveSecretPath(stateDir, domain, id)
	sec, err := postgres.ReadSecret(path)
	if err != nil {
		return nil
	}
	changed := false
	if sec.Database == legacy || sec.Database == legacyApp {
		sec.Database = canon
		changed = true
	}
	if sec.User == legacyRO {
		sec.User = canonRO
		changed = true
	}
	if !changed {
		return nil
	}
	if err := postgres.WriteSecret(path, sec); err != nil {
		return err
	}
	_ = chownWebor(path)
	return nil
}

func (o *Ops) renameRole(from, to string) error {
	if from == "" || from == to {
		return nil
	}
	if o.pgTrue("SELECT 1 FROM pg_roles WHERE rolname = "+pgQuote(from)) &&
		!o.pgTrue("SELECT 1 FROM pg_roles WHERE rolname = "+pgQuote(to)) {
		if err := o.psql(fmt.Sprintf("ALTER ROLE %s RENAME TO %s;", pgIdent(from), pgIdent(to))); err != nil {
			return fmt.Errorf("rename role: %w", err)
		}
	}
	return nil
}

func (o *Ops) pgTrue(sql string) bool {
	out, err := o.psqlOut(sql)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "1"
}

func (o *Ops) psql(sql string) error {
	_, err := o.psqlOut(sql)
	return err
}

func (o *Ops) psqlOut(sql string) (string, error) {
	cmd := exec.Command("runuser", "-u", "postgres", "--", "psql", "-v", "ON_ERROR_STOP=1", "-At", "-c", sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("sudo", "-u", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-At", "-c", sql)
		out, err = cmd.CombinedOutput()
	}
	if err != nil {
		return string(out), fmt.Errorf("psql: %w (%s)", err, truncateOut(out))
	}
	return string(out), nil
}

func (o *Ops) psqlDB(db, sql string) error {
	cmd := exec.Command("runuser", "-u", "postgres", "--", "psql", "-v", "ON_ERROR_STOP=1", "-d", db, "-At", "-c", sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("sudo", "-u", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-d", db, "-At", "-c", sql)
		out, err = cmd.CombinedOutput()
	}
	if err != nil {
		return fmt.Errorf("psql -d %s: %w (%s)", db, err, truncateOut(out))
	}
	return nil
}

func pgQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func pgIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func randomPassword(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func truncateOut(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

func chownWebor(path string) error {
	var u *user.User
	var err error
	for _, name := range []string{"webor", "webooza"} {
		u, err = user.Lookup(name)
		if err == nil {
			break
		}
	}
	if err != nil || u == nil {
		return fmt.Errorf("panel user webor/webooza not found: %w", err)
	}
	var uid, gid int
	fmt.Sscanf(u.Uid, "%d", &uid)
	fmt.Sscanf(u.Gid, "%d", &gid)
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return os.Chmod(path, 0o750)
	}
	return os.Chmod(path, 0o600)
}
