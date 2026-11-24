package auth_test

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
)

func TestSetupLoginTOTP(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	acct := &auth.Account{DB: db}
	plain, created, err := acct.EnsureSetupToken()
	if err != nil || !created || plain == "" {
		t.Fatalf("setup token: created=%v plain=%q err=%v", created, plain, err)
	}
	plain2, created2, err := acct.EnsureSetupToken()
	if err != nil || created2 || plain2 != "" {
		t.Fatalf("second ensure should be noop: created=%v plain=%q err=%v", created2, plain2, err)
	}

	if err := acct.CompleteSetup(plain, "admin@example.com", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(plain, "admin@example.com", "correct-horse"); err == nil {
		t.Fatal("expected already configured")
	}

	secret, url, err := acct.BeginTOTPEnrollment("Webor")
	if err != nil || secret == "" || url == "" {
		t.Fatalf("enroll: %v secret=%q url=%q", err, secret, url)
	}
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.ConfirmTOTP(code); err != nil {
		t.Fatal(err)
	}

	bad, err := acct.Login("admin@example.com", "wrong-password", code, "127.0.0.1", "test")
	if err == nil || bad != "" {
		t.Fatal("expected bad password failure")
	}
	tok, err := acct.Login("admin@example.com", "correct-horse", code, "127.0.0.1", "test")
	if err != nil || tok == "" {
		t.Fatalf("login: %v tok=%q", err, tok)
	}
	sess, err := acct.LookupSession(tok)
	if err != nil || sess == nil {
		t.Fatalf("lookup: %v", err)
	}
	if err := acct.DeleteSessionByToken(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := acct.LookupSession(tok); err == nil {
		t.Fatal("expected invalidated session")
	}
}

func TestLoginRateLimit(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	acct := &auth.Account{DB: db}
	plain, _, err := acct.EnsureSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(plain, "a@b.co", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		_, _ = acct.Login("a@b.co", "nope-nope-no", "", "10.0.0.1", "t")
	}
	_, err = acct.Login("a@b.co", "correct-horse", "", "10.0.0.1", "t")
	if err == nil {
		t.Fatal("expected rate limit")
	}
}

func TestHashPasswordMinLength(t *testing.T) {
	if _, err := auth.HashPassword("short"); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestSteppedMFALogin(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	acct := &auth.Account{DB: db}
	plain, _, err := acct.EnsureSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(plain, "admin@example.com", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	secret, _, err := acct.BeginTOTPEnrollment("Webor")
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.ConfirmTOTP(code); err != nil {
		t.Fatal(err)
	}

	out, err := acct.BeginLogin("admin@example.com", "correct-horse", "", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if !out.MFARequired() || out.MFAToken == "" || out.SessionToken != "" {
		t.Fatalf("expected MFA challenge, got %+v", out)
	}

	if _, err := acct.CompleteMFA(out.MFAToken, "000000", "127.0.0.1", "test"); err == nil {
		t.Fatal("expected bad TOTP to fail")
	}
	// Challenge must remain usable after a bad code.
	tok, err := acct.CompleteMFA(out.MFAToken, code, "127.0.0.1", "test")
	if err != nil || tok == "" {
		t.Fatalf("complete mfa: %v tok=%q", err, tok)
	}
	if _, err := acct.LookupSession(tok); err != nil {
		t.Fatal(err)
	}
	// Single-use: challenge consumed.
	if _, err := acct.CompleteMFA(out.MFAToken, code, "127.0.0.1", "test"); err == nil {
		t.Fatal("expected consumed challenge to fail")
	}

	// One-shot still works when totp is sent with the password step.
	code2, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	out2, err := acct.BeginLogin("admin@example.com", "correct-horse", code2, "127.0.0.1", "test")
	if err != nil || out2.SessionToken == "" || out2.MFARequired() {
		t.Fatalf("one-shot: %+v err=%v", out2, err)
	}

	// Expired / unknown token.
	if _, err := acct.CompleteMFA("not-a-real-token", code2, "127.0.0.1", "test"); err == nil {
		t.Fatal("expected bad mfa token failure")
	}
}

func TestResetTOTP(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	acct := &auth.Account{DB: db}
	plain, _, err := acct.EnsureSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.CompleteSetup(plain, "admin@example.com", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	secret, _, err := acct.BeginTOTPEnrollment("Webor")
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := acct.ConfirmTOTP(code); err != nil {
		t.Fatal(err)
	}
	tok, err := acct.Login("admin@example.com", "correct-horse", code, "127.0.0.1", "test")
	if err != nil || tok == "" {
		t.Fatalf("login with totp: %v", err)
	}

	if err := acct.ResetTOTP(); err != nil {
		t.Fatal(err)
	}
	adm, err := acct.GetAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if adm.TOTPConfirmed || adm.TOTPSecret != "" {
		t.Fatalf("expected totp cleared, got confirmed=%v secret=%q", adm.TOTPConfirmed, adm.TOTPSecret)
	}
	if _, err := acct.LookupSession(tok); err == nil {
		t.Fatal("expected sessions cleared")
	}
	tok2, err := acct.Login("admin@example.com", "correct-horse", "", "127.0.0.1", "test")
	if err != nil || tok2 == "" {
		t.Fatalf("password-only login after reset: %v", err)
	}
}
