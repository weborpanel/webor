package auth

import (
	"fmt"
	"time"
)

const (
	loginWindow   = 15 * time.Minute
	loginMaxFails = 10
)

// AllowLogin checks whether key (typically IP or email+IP) is under the failure budget.
func (a *Account) AllowLogin(key string) error {
	key = truncate(key, 200)
	since := a.now().Add(-loginWindow).Format(time.RFC3339Nano)
	var fails int
	err := a.DB.QueryRow(
		`SELECT COUNT(1) FROM login_attempts WHERE key = ? AND ok = 0 AND attempted_at >= ?`,
		key, since,
	).Scan(&fails)
	if err != nil {
		return err
	}
	if fails >= loginMaxFails {
		return fmt.Errorf("too many failed login attempts; try again later")
	}
	return nil
}

// RecordLoginAttempt stores a success/failure for rate limiting.
func (a *Account) RecordLoginAttempt(key string, ok bool) error {
	key = truncate(key, 200)
	flag := 0
	if ok {
		flag = 1
	}
	_, err := a.DB.Exec(
		`INSERT INTO login_attempts (key, attempted_at, ok) VALUES (?, ?, ?)`,
		key, a.now().Format(time.RFC3339Nano), flag,
	)
	return err
}

// Login authenticates email/password/TOTP and returns a new session cookie value.
func (a *Account) Login(email, password, totpCode, ip, userAgent string) (sessionToken string, err error) {
	emailNorm, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	key := ip + "|" + emailNorm
	if err := a.AllowLogin(key); err != nil {
		return "", err
	}
	adm, err := a.GetAdmin()
	if err != nil {
		_ = a.RecordLoginAttempt(key, false)
		return "", fmt.Errorf("invalid credentials")
	}
	if adm.Email != emailNorm || !CheckPassword(adm.PasswordHash, password) {
		_ = a.RecordLoginAttempt(key, false)
		return "", fmt.Errorf("invalid credentials")
	}
	if adm.TOTPConfirmed {
		if !ValidateTOTP(adm.TOTPSecret, totpCode) {
			_ = a.RecordLoginAttempt(key, false)
			return "", fmt.Errorf("invalid credentials")
		}
	}
	plain, _, err := a.CreateSession(ip, userAgent)
	if err != nil {
		return "", err
	}
	_ = a.RecordLoginAttempt(key, true)
	return plain, nil
}
