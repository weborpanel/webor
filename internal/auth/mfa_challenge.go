package auth

import (
	"database/sql"
	"fmt"
	"time"
)

const DefaultMFAChallengeTTL = 5 * time.Minute

// createMFAChallenge stores a short-lived challenge and returns the plaintext token.
func (a *Account) createMFAChallenge(ip, userAgent string) (plain string, err error) {
	plain, hash, err := NewToken(32)
	if err != nil {
		return "", err
	}
	id, _, err := NewToken(16)
	if err != nil {
		return "", err
	}
	now := a.now()
	exp := now.Add(DefaultMFAChallengeTTL)
	_, err = a.DB.Exec(
		`INSERT INTO mfa_challenges (id, token_hash, created_at, expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?)`,
		id, hash,
		now.Format(time.RFC3339Nano),
		exp.Format(time.RFC3339Nano),
		ip, truncate(userAgent, 256),
	)
	if err != nil {
		return "", err
	}
	return plain, nil
}

// consumeMFAChallenge validates a challenge token, deletes it, and returns nil on success.
func (a *Account) consumeMFAChallenge(plain string) error {
	if plain == "" {
		return fmt.Errorf("invalid or expired MFA challenge")
	}
	hash := HashToken(plain)
	row := a.DB.QueryRow(
		`SELECT id, expires_at FROM mfa_challenges WHERE token_hash = ?`,
		hash,
	)
	var id, exp string
	if err := row.Scan(&id, &exp); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid or expired MFA challenge")
		}
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, exp)
	if err != nil {
		expiresAt, _ = time.Parse(time.RFC3339, exp)
	}
	if a.now().After(expiresAt) {
		_, _ = a.DB.Exec(`DELETE FROM mfa_challenges WHERE id = ?`, id)
		return fmt.Errorf("invalid or expired MFA challenge")
	}
	_, err = a.DB.Exec(`DELETE FROM mfa_challenges WHERE id = ?`, id)
	return err
}

// peekMFAChallenge reports whether a challenge token is still valid without consuming it.
func (a *Account) peekMFAChallenge(plain string) error {
	if plain == "" {
		return fmt.Errorf("invalid or expired MFA challenge")
	}
	hash := HashToken(plain)
	row := a.DB.QueryRow(
		`SELECT expires_at FROM mfa_challenges WHERE token_hash = ?`,
		hash,
	)
	var exp string
	if err := row.Scan(&exp); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid or expired MFA challenge")
		}
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, exp)
	if err != nil {
		expiresAt, _ = time.Parse(time.RFC3339, exp)
	}
	if a.now().After(expiresAt) {
		_, _ = a.DB.Exec(`DELETE FROM mfa_challenges WHERE token_hash = ?`, hash)
		return fmt.Errorf("invalid or expired MFA challenge")
	}
	return nil
}
