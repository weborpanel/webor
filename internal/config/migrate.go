package config

import (
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "001_init",
		sql: `
CREATE TABLE audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  actor TEXT NOT NULL,
  operation TEXT NOT NULL,
  args_json TEXT NOT NULL DEFAULT '{}',
  result TEXT NOT NULL DEFAULT '',
  prev_hash TEXT NOT NULL DEFAULT '',
  hash TEXT NOT NULL,
  UNIQUE (hash)
);

CREATE TABLE approvals (
  id TEXT PRIMARY KEY,
  operation TEXT NOT NULL,
  args_json TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  expires_at TEXT NOT NULL,
  used_at TEXT,
  approved_by TEXT
);
`,
	},
	{
		version: 2,
		name:    "002_auth",
		sql: `
CREATE TABLE admin_users (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  totp_secret TEXT NOT NULL DEFAULT '',
  totp_confirmed INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE setup_tokens (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  token_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  used_at TEXT
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  expires_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  ip TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT ''
);

CREATE TABLE login_attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT NOT NULL,
  attempted_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  ok INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX login_attempts_key_ts ON login_attempts (key, attempted_at);
`,
	},
}

// Migrate applies all pending schema migrations in order.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var exists int
		err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, m.version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}
		if exists > 0 {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (?, ?)`,
			m.version, m.name,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
