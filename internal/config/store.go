package config

import (
	"database/sql"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"
)

// DefaultStateDir is the production runtime state directory.
const DefaultStateDir = "/var/lib/webor"

// OpenDB opens (or creates) the SQLite state database under stateDir and
// applies pending migrations.
func OpenDB(stateDir string) (*sql.DB, error) {
	if stateDir == "" {
		stateDir = DefaultStateDir
	}
	// 0755: site users must traverse /var/lib/webor to exec shared Node under
	// runtimes/. Secrets and state.db stay mode-restricted on the files/dirs
	// themselves — never rely on locking down the state root.
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	// MkdirAll does not change the mode of an existing directory — re-assert.
	_ = os.Chmod(stateDir, 0o755)
	dbPath := filepath.Join(stateDir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	// busy_timeout: panel + helper both open state.db; without it concurrent writers
	// can surface SQLITE_BUSY and, under hard kills, contribute to WAL/corruption pain.
	if _, err := db.Exec(`
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Helper runs as root and opens first; panel runs as webor and needs RW.
	shareStateWithPanel(stateDir, dbPath)
	return db, nil
}

func shareStateWithPanel(stateDir, dbPath string) {
	if os.Geteuid() != 0 {
		return
	}
	u, err := user.Lookup("webor")
	if err != nil {
		return
	}
	uid, err1 := strconv.Atoi(u.Uid)
	gid, err2 := strconv.Atoi(u.Gid)
	if err1 != nil || err2 != nil {
		return
	}
	// Must stay other-executable: cron/app units run as wbr_* and exec
	// /var/lib/webor/runtimes/node/... . 0770 here caused systemd 203/EXEC
	// after every helper restart (recreate-cron temporarily fixed it via
	// prepareNodeRuntimeAccess).
	_ = os.Chmod(stateDir, 0o755)
	_ = os.Chown(stateDir, uid, gid)
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		_ = os.Chmod(p, 0o660)
		_ = os.Chown(p, uid, gid)
	}
}
