package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DefaultTTL is how long a pending approval remains usable.
const DefaultTTL = 5 * time.Minute

// Approval is a single-use, argument-bound step-up grant.
type Approval struct {
	ID         string
	Operation  string
	ArgsJSON   string
	ArgsHash   string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	UsedAt     *time.Time
	ApprovedBy string
}

// Store persists approvals in SQLite (helper-owned verification).
type Store struct {
	DB  *sql.DB
	TTL time.Duration
	Now func() time.Time
}

func (s *Store) ttl() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return DefaultTTL
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// CanonicalArgs returns stable JSON for binding (compact, key-sorted via map round-trip).
func CanonicalArgs(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", err
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ArgsHash hashes canonical args JSON.
func ArgsHash(argsJSON string) string {
	sum := sha256.Sum256([]byte(argsJSON))
	return hex.EncodeToString(sum[:])
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Create inserts a pending approval (not yet console-approved).
func (s *Store) Create(operation string, args any) (*Approval, error) {
	op := strings.TrimSpace(operation)
	if op == "" {
		return nil, fmt.Errorf("operation is required")
	}
	argsJSON, err := CanonicalArgs(args)
	if err != nil {
		return nil, fmt.Errorf("args: %w", err)
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now := s.now()
	exp := now.Add(s.ttl())
	_, err = s.DB.Exec(
		`INSERT INTO approvals (id, operation, args_json, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, op, argsJSON, now.Format(time.RFC3339Nano), exp.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	return &Approval{
		ID:        id,
		Operation: op,
		ArgsJSON:  argsJSON,
		ArgsHash:  ArgsHash(argsJSON),
		CreatedAt: now,
		ExpiresAt: exp,
	}, nil
}

// Approve marks a pending approval as console-approved. Idempotent if already approved and unused.
func (s *Store) Approve(id, approvedBy string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("approval id is required")
	}
	if strings.TrimSpace(approvedBy) == "" {
		approvedBy = "console"
	}
	a, err := s.Get(id)
	if err != nil {
		return err
	}
	if a.UsedAt != nil {
		return fmt.Errorf("approval already used")
	}
	if s.now().After(a.ExpiresAt) {
		return fmt.Errorf("approval expired")
	}
	if a.ApprovedBy != "" {
		return nil
	}
	_, err = s.DB.Exec(`UPDATE approvals SET approved_by = ? WHERE id = ? AND approved_by IS NULL`, approvedBy, id)
	return err
}

// Consume verifies an approved, unused, unexpired approval bound to op+args, then marks it used.
func (s *Store) Consume(id, operation string, args any) error {
	a, err := s.Get(id)
	if err != nil {
		return err
	}
	if a.ApprovedBy == "" {
		return fmt.Errorf("approval not console-approved")
	}
	if a.UsedAt != nil {
		return fmt.Errorf("approval already used")
	}
	if s.now().After(a.ExpiresAt) {
		return fmt.Errorf("approval expired")
	}
	if a.Operation != operation {
		return fmt.Errorf("approval operation mismatch")
	}
	argsJSON, err := CanonicalArgs(args)
	if err != nil {
		return err
	}
	if a.ArgsJSON != argsJSON {
		return fmt.Errorf("approval args mismatch")
	}
	now := s.now().Format(time.RFC3339Nano)
	res, err := s.DB.Exec(
		`UPDATE approvals SET used_at = ? WHERE id = ? AND used_at IS NULL AND approved_by IS NOT NULL`,
		now, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("approval consume raced or invalid")
	}
	return nil
}

// ListPending returns unused, unexpired approvals (newest first).
func (s *Store) ListPending() ([]Approval, error) {
	now := s.now().Format(time.RFC3339Nano)
	rows, err := s.DB.Query(`
SELECT id, operation, args_json, created_at, expires_at, used_at, COALESCE(approved_by, '')
FROM approvals
WHERE used_at IS NULL AND expires_at > ?
ORDER BY created_at DESC
LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		var a Approval
		var created, expires string
		var used sql.NullString
		if err := rows.Scan(&a.ID, &a.Operation, &a.ArgsJSON, &created, &expires, &used, &a.ApprovedBy); err != nil {
			return nil, err
		}
		if a.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			a.CreatedAt, _ = time.Parse(time.RFC3339, created)
		}
		if a.ExpiresAt, err = time.Parse(time.RFC3339Nano, expires); err != nil {
			a.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
		}
		if used.Valid && used.String != "" {
			t, err := time.Parse(time.RFC3339Nano, used.String)
			if err != nil {
				t, _ = time.Parse(time.RFC3339, used.String)
			}
			a.UsedAt = &t
		}
		a.ArgsHash = ArgsHash(a.ArgsJSON)
		out = append(out, a)
	}
	return out, rows.Err()
}

// Get loads one approval by id.
func (s *Store) Get(id string) (*Approval, error) {
	row := s.DB.QueryRow(
		`SELECT id, operation, args_json, created_at, expires_at, used_at, COALESCE(approved_by, '') FROM approvals WHERE id = ?`,
		id,
	)
	var a Approval
	var created, expires string
	var used sql.NullString
	if err := row.Scan(&a.ID, &a.Operation, &a.ArgsJSON, &created, &expires, &used, &a.ApprovedBy); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("approval not found")
		}
		return nil, err
	}
	var err error
	if a.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		a.CreatedAt, _ = time.Parse(time.RFC3339, created)
	}
	if a.ExpiresAt, err = time.Parse(time.RFC3339Nano, expires); err != nil {
		a.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	}
	if used.Valid && used.String != "" {
		t, err := time.Parse(time.RFC3339Nano, used.String)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, used.String)
		}
		a.UsedAt = &t
	}
	a.ArgsHash = ArgsHash(a.ArgsJSON)
	return &a, nil
}
