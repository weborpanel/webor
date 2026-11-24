package auth

import (
	"fmt"
	"strings"
	"time"
)

const (
	loginWindow   = 15 * time.Minute
	loginMaxFails = 10
)

// LoginOutcome is the result of BeginLogin: either a session cookie value or an MFA challenge.
type LoginOutcome struct {
	SessionToken string
	MFAToken     string
}

// MFARequired reports whether the client must complete TOTP via CompleteMFA.
func (o LoginOutcome) MFARequired() bool {
	return o.MFAToken != ""
}

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

// BeginLogin authenticates email/password. When TOTP is confirmed and totpCode is
// empty it returns an MFA challenge token (no session). When totpCode is provided
// (one-shot / scripts) it validates TOTP and issues a session.
func (a *Account) BeginLogin(email, password, totpCode, ip, userAgent string) (LoginOutcome, error) {
	emailNorm, err := normalizeEmail(email)
	if err != nil {
		return LoginOutcome{}, err
	}
	key := ip + "|" + emailNorm
	if err := a.AllowLogin(key); err != nil {
		return LoginOutcome{}, err
	}
	adm, err := a.GetAdmin()
	if err != nil {
		_ = a.RecordLoginAttempt(key, false)
		return LoginOutcome{}, fmt.Errorf("invalid credentials")
	}
	if adm.Email != emailNorm || !CheckPassword(adm.PasswordHash, password) {
		_ = a.RecordLoginAttempt(key, false)
		return LoginOutcome{}, fmt.Errorf("invalid credentials")
	}
	if adm.TOTPConfirmed {
		totpCode = strings.TrimSpace(totpCode)
		if totpCode == "" {
			mfaToken, err := a.createMFAChallenge(ip, userAgent)
			if err != nil {
				return LoginOutcome{}, err
			}
			return LoginOutcome{MFAToken: mfaToken}, nil
		}
		if !ValidateTOTP(adm.TOTPSecret, totpCode) {
			_ = a.RecordLoginAttempt(key, false)
			return LoginOutcome{}, fmt.Errorf("invalid credentials")
		}
	}
	plain, _, err := a.CreateSession(ip, userAgent)
	if err != nil {
		return LoginOutcome{}, err
	}
	_ = a.RecordLoginAttempt(key, true)
	return LoginOutcome{SessionToken: plain}, nil
}

// CompleteMFA finishes a stepped login after BeginLogin returned an MFA challenge.
// Wrong codes keep the challenge alive until it expires; rate limits still apply.
func (a *Account) CompleteMFA(mfaToken, totpCode, ip, userAgent string) (sessionToken string, err error) {
	if err := a.peekMFAChallenge(mfaToken); err != nil {
		return "", err
	}
	adm, err := a.GetAdmin()
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if !adm.TOTPConfirmed {
		_ = a.consumeMFAChallenge(mfaToken)
		return "", fmt.Errorf("MFA is not enabled")
	}
	key := ip + "|" + adm.Email
	if err := a.AllowLogin(key); err != nil {
		return "", err
	}
	totpCode = strings.TrimSpace(totpCode)
	if !ValidateTOTP(adm.TOTPSecret, totpCode) {
		_ = a.RecordLoginAttempt(key, false)
		return "", fmt.Errorf("invalid authenticator code")
	}
	if err := a.consumeMFAChallenge(mfaToken); err != nil {
		return "", err
	}
	plain, _, err := a.CreateSession(ip, userAgent)
	if err != nil {
		return "", err
	}
	_ = a.RecordLoginAttempt(key, true)
	return plain, nil
}

// Login authenticates email/password/TOTP in one shot and returns a session cookie value.
// Prefer BeginLogin + CompleteMFA for interactive UIs.
func (a *Account) Login(email, password, totpCode, ip, userAgent string) (sessionToken string, err error) {
	out, err := a.BeginLogin(email, password, totpCode, ip, userAgent)
	if err != nil {
		return "", err
	}
	if out.MFARequired() {
		return "", fmt.Errorf("invalid credentials")
	}
	return out.SessionToken, nil
}
