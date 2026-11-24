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
	{
		version: 3,
		name:    "003_metrics",
		sql: `
CREATE TABLE metrics_host (
  res INTEGER NOT NULL,
  ts INTEGER NOT NULL,
  cpu_pct REAL NOT NULL DEFAULT 0,
  cpu_max REAL NOT NULL DEFAULT 0,
  mem_used INTEGER NOT NULL DEFAULT 0,
  mem_total INTEGER NOT NULL DEFAULT 0,
  load1 REAL NOT NULL DEFAULT 0,
  disk_used INTEGER NOT NULL DEFAULT 0,
  disk_total INTEGER NOT NULL DEFAULT 0,
  net_rx REAL NOT NULL DEFAULT 0,
  net_tx REAL NOT NULL DEFAULT 0,
  PRIMARY KEY (res, ts)
);

CREATE TABLE metrics_site (
  res INTEGER NOT NULL,
  ts INTEGER NOT NULL,
  site TEXT NOT NULL,
  cpu_pct REAL NOT NULL DEFAULT 0,
  cpu_max REAL NOT NULL DEFAULT 0,
  mem_bytes INTEGER NOT NULL DEFAULT 0,
  disk_bytes INTEGER NOT NULL DEFAULT 0,
  active INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (site, res, ts)
);

CREATE INDEX metrics_site_res_ts ON metrics_site (res, ts);
`,
	},
	{
		version: 4,
		name:    "004_deploy_tokens",
		sql: `
CREATE TABLE deploy_tokens (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  site TEXT NOT NULL,
  scopes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  last_used_at TEXT,
  expires_at TEXT,
  revoked_at TEXT
);

CREATE INDEX deploy_tokens_site ON deploy_tokens (site);
CREATE INDEX deploy_tokens_hash ON deploy_tokens (token_hash);
`,
	},
	{
		version: 5,
		name:    "005_mfa_challenges",
		sql: `
CREATE TABLE mfa_challenges (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  expires_at TEXT NOT NULL,
  ip TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT ''
);

CREATE INDEX mfa_challenges_hash ON mfa_challenges (token_hash);
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
