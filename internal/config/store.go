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
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	dbPath := filepath.Join(stateDir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;`); err != nil {
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
	_ = os.Chmod(stateDir, 0o770)
	_ = os.Chown(stateDir, uid, gid)
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		_ = os.Chmod(p, 0o660)
		_ = os.Chown(p, uid, gid)
	}
}
