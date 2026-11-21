package helper

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/sites"
)

func (o *Ops) ensureSQLite(domain, id string) (*EnsureSiteDatabaseResult, error) {
	path := dbengine.SQLitePath(domain, id)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir sqlite dir: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
		if err != nil {
			return nil, err
		}
		_ = f.Close()
	}
	username := sites.Username(domain)
	uid, gid := -1, -1
	if u, err := user.Lookup(username); err == nil {
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	}
	// Prefer webor group so the panel can open mode=ro without world-readable files.
	if g, err := user.LookupGroup("webor"); err == nil {
		if wg, err := strconv.Atoi(g.Gid); err == nil {
			gid = wg
		}
	}
	if uid >= 0 && gid >= 0 {
		_ = os.Chown(dir, uid, gid)
		_ = os.Chown(path, uid, gid)
		_ = os.Chmod(dir, 0o750)
		_ = os.Chmod(path, 0o640)
	}
	return &EnsureSiteDatabaseResult{
		Engine:   dbengine.SQLite,
		ID:       id,
		Database: filepath.Base(path),
		Path:     path,
		PeerDSN:  dbengine.SQLiteDSN(domain, id),
		PeerURL:  "sqlite://" + path,
	}, nil
}

func (o *Ops) dropSQLite(domain, id string) error {
	path := dbengine.SQLitePath(domain, id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Best-effort remove empty data dir (only when no other files remain).
	_ = os.Remove(filepath.Dir(path))
	return nil
}
