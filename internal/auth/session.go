package auth

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	// SessionCookie is the HTTP cookie name for the panel session.
	SessionCookie = "webor_session"
	// DefaultSessionTTL is how long a session remains valid.
	DefaultSessionTTL = 12 * time.Hour
)

// Session is an authenticated panel session.
type Session struct {
	ID        string
	ExpiresAt time.Time
}

// CreateSession issues a new session and returns the plaintext cookie value.
func (a *Account) CreateSession(ip, userAgent string) (plain string, sess *Session, err error) {
	plain, hash, err := NewToken(32)
	if err != nil {
		return "", nil, err
	}
	id, _, err := NewToken(16)
	if err != nil {
		return "", nil, err
	}
	now := a.now()
	exp := now.Add(DefaultSessionTTL)
	_, err = a.DB.Exec(
		`INSERT INTO sessions (id, token_hash, created_at, expires_at, last_seen_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, hash,
		now.Format(time.RFC3339Nano),
		exp.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		ip, truncate(userAgent, 256),
	)
	if err != nil {
		return "", nil, err
	}
	return plain, &Session{ID: id, ExpiresAt: exp}, nil
}

// LookupSession validates a presented cookie token.
func (a *Account) LookupSession(plain string) (*Session, error) {
	if plain == "" {
		return nil, fmt.Errorf("no session")
	}
	hash := HashToken(plain)
	row := a.DB.QueryRow(
		`SELECT id, expires_at FROM sessions WHERE token_hash = ?`,
		hash,
	)
	var s Session
	var exp string
	if err := row.Scan(&s.ID, &exp); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid session")
		}
		return nil, err
	}
	var err error
	if s.ExpiresAt, err = time.Parse(time.RFC3339Nano, exp); err != nil {
		s.ExpiresAt, _ = time.Parse(time.RFC3339, exp)
	}
	if a.now().After(s.ExpiresAt) {
		_ = a.DeleteSession(s.ID)
		return nil, fmt.Errorf("session expired")
	}
	_, _ = a.DB.Exec(
		`UPDATE sessions SET last_seen_at = ? WHERE id = ?`,
		a.now().Format(time.RFC3339Nano), s.ID,
	)
	return &s, nil
}

// DeleteSession removes one session by id.
func (a *Account) DeleteSession(id string) error {
	_, err := a.DB.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteSessionByToken removes the session matching a cookie value.
func (a *Account) DeleteSessionByToken(plain string) error {
	_, err := a.DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, HashToken(plain))
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
