package audit

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Logger appends hash-chained privileged-operation events.
type Logger struct {
	DB  *sql.DB
	Now func() time.Time
}

func (l *Logger) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now().UTC()
}

// Append records one event. args may be any JSON-serializable value.
func (l *Logger) Append(actor, operation string, args any, result string) error {
	if l == nil || l.DB == nil {
		return nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return fmt.Errorf("operation is required")
	}
	argsJSON := "{}"
	if args != nil {
		raw, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("args: %w", err)
		}
		argsJSON = string(raw)
	}
	result = strings.TrimSpace(result)

	tx, err := l.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var prevHash string
	err = tx.QueryRow(`SELECT hash FROM audit_events ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	ts := l.now().Format(time.RFC3339Nano)
	payload := strings.Join([]string{prevHash, ts, actor, operation, argsJSON, result}, "\n")
	sum := sha256.Sum256([]byte(payload))
	hash := hex.EncodeToString(sum[:])
	_, err = tx.Exec(
		`INSERT INTO audit_events (ts, actor, operation, args_json, result, prev_hash, hash) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ts, actor, operation, argsJSON, result, prevHash, hash,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}
