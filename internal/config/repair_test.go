package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestIsCorrupt(t *testing.T) {
	if config.IsCorrupt(nil) {
		t.Fatal("nil")
	}
	if !config.IsCorrupt(errString("database disk image is malformed (11)")) {
		t.Fatal("expected malformed detected")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestRepairStateDBHealthy(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	msg, err := config.RepairStateDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" {
		t.Fatal("empty message")
	}
	if _, err := os.Stat(filepath.Join(dir, "state.db")); err != nil {
		t.Fatal(err)
	}
}

func TestRepairStateDBMissing(t *testing.T) {
	dir := t.TempDir()
	msg, err := config.RepairStateDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" {
		t.Fatal("empty message")
	}
}

func TestRepairStateDBCorruptFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	msg, err := config.RepairStateDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" {
		t.Fatal("empty message")
	}
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
}
