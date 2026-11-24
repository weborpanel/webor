package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShareStateWithPanelKeepsStateDirWorldTraversable(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to exercise shareStateWithPanel chmod path")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Simulate the old lock-down that broke Node cron after helper restart.
	if err := os.Chmod(dir, 0o770); err != nil {
		t.Fatal(err)
	}

	shareStateWithPanel(dir, dbPath)

	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o005 == 0 {
		t.Fatalf("state dir mode=%o want other-executable (site users traverse to runtimes/)", st.Mode().Perm())
	}
	dbSt, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if dbSt.Mode().Perm()&0o077 != 0 {
		// 0660 is fine; reject world-readable/writable DB.
		if dbSt.Mode().Perm()&0o006 != 0 {
			t.Fatalf("state.db mode=%o must not be world-accessible", dbSt.Mode().Perm())
		}
	}
}

func TestOpenDBLeavesStateDirTraversable(t *testing.T) {
	dir := t.TempDir()
	// Pretend an older helper locked the dir before OpenDB runs again.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(filepath.Join(dir, "state.db")); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o005 == 0 {
		t.Fatalf("after OpenDB, state dir mode=%o want other-executable", st.Mode().Perm())
	}
}
