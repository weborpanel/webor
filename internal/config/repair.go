package config

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
)

const sqliteCorrupt = 11 // SQLITE_CORRUPT

// IsCorrupt reports whether err is SQLite SQLITE_CORRUPT (disk image malformed).
func IsCorrupt(err error) bool {
	if err == nil {
		return false
	}
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqliteCorrupt {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "malformed") || strings.Contains(msg, "sqlite_corrupt")
}

// RepairStateDB attempts to fix a corrupted /var/lib/webor/state.db.
// Callers must stop webor and webor-helper first so nothing holds the file open.
//
// Strategy:
//  1. Soft: if the main file passes integrity_check, drop orphan -wal/-shm and reopen.
//  2. Hard: sqlite3 .recover into a new DB (preserves what can be salvaged).
//  3. Last resort: move aside and create a fresh empty DB (auth/audit/metrics reset).
func RepairStateDB(stateDir string) (string, error) {
	if stateDir == "" {
		stateDir = DefaultStateDir
	}
	dbPath := filepath.Join(stateDir, "state.db")
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", err
	}

	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		db, err := OpenDB(stateDir)
		if err != nil {
			return "", err
		}
		_ = db.Close()
		return "created new empty state.db (file was missing)", nil
	}

	// Soft path: main file still consistent — checkpoint WAL and reopen.
	if ok := integrityOK(dbPath); ok {
		if err := checkpointWAL(dbPath); err != nil {
			// Stale/broken sidecars: drop them and retry once.
			_ = os.Remove(walPath)
			_ = os.Remove(shmPath)
		}
		db, err := OpenDB(stateDir)
		if err != nil {
			return "", fmt.Errorf("reopen after soft repair: %w", err)
		}
		_ = db.Close()
		return "state.db integrity OK; WAL checkpointed", nil
	}

	stamp := time.Now().UTC().Format("20060102T150405Z")
	backup := dbPath + ".corrupt." + stamp
	if err := os.Rename(dbPath, backup); err != nil {
		return "", fmt.Errorf("quarantine corrupt db: %w", err)
	}
	_ = moveAside(walPath, backup+"-wal")
	_ = moveAside(shmPath, backup+"-shm")

	if err := recoverWithSQLite3(backup, dbPath); err == nil {
		if ok := integrityOK(dbPath); ok {
			db, err := OpenDB(stateDir)
			if err != nil {
				return "", fmt.Errorf("recovered file failed migrate: %w", err)
			}
			_ = db.Close()
			shareStateWithPanel(stateDir, dbPath)
			return fmt.Sprintf("recovered state.db via sqlite3 .recover (backup %s)", filepath.Base(backup)), nil
		}
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	}

	// Fresh DB — panel will need first-run setup again if admin_users was lost.
	db, err := OpenDB(stateDir)
	if err != nil {
		// Put backup back so the operator is not worse off.
		_ = os.Rename(backup, dbPath)
		return "", fmt.Errorf("create fresh state.db: %w (restored quarantine)", err)
	}
	_ = db.Close()
	shareStateWithPanel(stateDir, dbPath)
	return fmt.Sprintf("created fresh state.db; corrupt copy kept as %s (re-login / first-run setup may be required)", filepath.Base(backup)), nil
}

func moveAside(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

func integrityOK(dbPath string) bool {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(result), "ok")
}

func checkpointWAL(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

func recoverWithSQLite3(corruptPath, destPath string) error {
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		return fmt.Errorf("sqlite3 CLI not found")
	}
	recoverCmd := exec.Command(bin, corruptPath, ".recover")
	stdout, err := recoverCmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := recoverCmd.StderrPipe()
	if err != nil {
		return err
	}
	loadCmd := exec.Command(bin, destPath)
	stdin, err := loadCmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := recoverCmd.Start(); err != nil {
		return err
	}
	if err := loadCmd.Start(); err != nil {
		_ = recoverCmd.Process.Kill()
		return err
	}
	_, copyErr := io.Copy(stdin, stdout)
	_ = stdin.Close()
	errBuf, _ := io.ReadAll(stderr)
	waitRecover := recoverCmd.Wait()
	waitLoad := loadCmd.Wait()
	if copyErr != nil {
		_ = os.Remove(destPath)
		return copyErr
	}
	if waitRecover != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("sqlite3 .recover: %w (%s)", waitRecover, strings.TrimSpace(string(errBuf)))
	}
	if waitLoad != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("sqlite3 load recovered SQL: %w", waitLoad)
	}
	return nil
}
