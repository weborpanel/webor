package auth

import (
	"database/sql"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

// Admin is the single panel administrator.
type Admin struct {
	Email         string
	PasswordHash  string
	TOTPSecret    string
	TOTPConfirmed bool
	CreatedAt     time.Time
}

// Account wraps admin, setup, session, and login-rate helpers on one DB.
type Account struct {
	DB  *sql.DB
	Now func() time.Time
}

func (a *Account) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now().UTC()
}

// HasAdmin reports whether the first-run admin exists.
func (a *Account) HasAdmin() (bool, error) {
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(1) FROM admin_users WHERE id = 1`).Scan(&n)
	return n > 0, err
}

// GetAdmin loads the admin row.
func (a *Account) GetAdmin() (*Admin, error) {
	row := a.DB.QueryRow(`SELECT email, password_hash, totp_secret, totp_confirmed, created_at FROM admin_users WHERE id = 1`)
	var adm Admin
	var confirmed int
	var created string
	if err := row.Scan(&adm.Email, &adm.PasswordHash, &adm.TOTPSecret, &confirmed, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin not configured")
		}
		return nil, err
	}
	adm.TOTPConfirmed = confirmed == 1
	adm.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &adm, nil
}

// SeedSetupToken inserts a known plaintext setup token when none exists yet.
// Used by the installer so the token can be printed outside the panel journal.
func (a *Account) SeedSetupToken(plain string) (seeded bool, err error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return false, nil
	}
	has, err := a.HasAdmin()
	if err != nil {
		return false, err
	}
	if has {
		return false, nil
	}
	var existing string
	err = a.DB.QueryRow(`SELECT token_hash FROM setup_tokens WHERE id = 1`).Scan(&existing)
	if err == nil {
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	_, err = a.DB.Exec(
		`INSERT INTO setup_tokens (id, token_hash, created_at) VALUES (1, ?, ?)`,
		HashToken(plain), a.now().Format(time.RFC3339Nano),
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

// EnsureSetupToken creates a setup token if none exists and no admin is present.
// Returns the plaintext token when newly created; empty string if already issued/used.
func (a *Account) EnsureSetupToken() (plain string, created bool, err error) {
	has, err := a.HasAdmin()
	if err != nil {
		return "", false, err
	}
	if has {
		return "", false, nil
	}
	var existing string
	var used sql.NullString
	err = a.DB.QueryRow(`SELECT token_hash, used_at FROM setup_tokens WHERE id = 1`).Scan(&existing, &used)
	if err == nil {
		return "", false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}
	plain, hash, err := NewSetupToken()
	if err != nil {
		return "", false, err
	}
	_, err = a.DB.Exec(
		`INSERT INTO setup_tokens (id, token_hash, created_at) VALUES (1, ?, ?)`,
		hash, a.now().Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", false, err
	}
	return plain, true, nil
}

// CompleteSetup consumes a valid setup token and creates the admin account.
func (a *Account) CompleteSetup(setupToken, email, password string) error {
	has, err := a.HasAdmin()
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("admin already configured")
	}
	email, err = normalizeEmail(email)
	if err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	var tokenHash string
	var used sql.NullString
	err = a.DB.QueryRow(`SELECT token_hash, used_at FROM setup_tokens WHERE id = 1`).Scan(&tokenHash, &used)
	if err == sql.ErrNoRows {
		return fmt.Errorf("setup token not issued")
	}
	if err != nil {
		return err
	}
	if used.Valid && used.String != "" {
		return fmt.Errorf("setup token already used")
	}
	if HashToken(setupToken) != tokenHash {
		return fmt.Errorf("invalid setup token")
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := a.now().Format(time.RFC3339Nano)
	if _, err := tx.Exec(
		`INSERT INTO admin_users (id, email, password_hash, created_at) VALUES (1, ?, ?, ?)`,
		email, hash, now,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE setup_tokens SET used_at = ? WHERE id = 1`, now); err != nil {
		return err
	}
	return tx.Commit()
}

// BeginTOTPEnrollment stores an unconfirmed TOTP secret and returns the otpauth URL.
func (a *Account) BeginTOTPEnrollment(issuer string) (secret, url string, err error) {
	adm, err := a.GetAdmin()
	if err != nil {
		return "", "", err
	}
	if adm.TOTPConfirmed {
		return "", "", fmt.Errorf("TOTP already confirmed")
	}
	secret, url, err = GenerateTOTPSecret(issuer, adm.Email)
	if err != nil {
		return "", "", err
	}
	_, err = a.DB.Exec(`UPDATE admin_users SET totp_secret = ?, totp_confirmed = 0 WHERE id = 1`, secret)
	return secret, url, err
}

// ConfirmTOTP marks TOTP as required after a valid code.
func (a *Account) ConfirmTOTP(code string) error {
	adm, err := a.GetAdmin()
	if err != nil {
		return err
	}
	if adm.TOTPSecret == "" {
		return fmt.Errorf("TOTP not started")
	}
	if !ValidateTOTP(adm.TOTPSecret, code) {
		return fmt.Errorf("invalid TOTP code")
	}
	_, err = a.DB.Exec(`UPDATE admin_users SET totp_confirmed = 1 WHERE id = 1`)
	return err
}

// ResetTOTP clears two-factor enrollment so password-only login works again.
// Intended for root break-glass recovery (wbr recover totp). Also drops all
// sessions and login failure history so the admin can sign in immediately.
func (a *Account) ResetTOTP() error {
	has, err := a.HasAdmin()
	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("admin not configured")
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE admin_users SET totp_secret = '', totp_confirmed = 0 WHERE id = 1`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM login_attempts`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mfa_challenges`); err != nil {
		return err
	}
	return tx.Commit()
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", fmt.Errorf("email is required")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", fmt.Errorf("invalid email")
	}
	return email, nil
}
