package auth_test

import (
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
)

func TestDeployTokenCRUD(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	acct := &auth.Account{DB: db}

	plain, info, err := acct.CreateDeployToken("app.example.com", "ci", auth.AllDeployScopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" || !hasPrefix(plain, auth.DeployTokenPrefix) {
		t.Fatalf("plain=%q", plain)
	}
	if info.Site != "app.example.com" || info.ID == "" {
		t.Fatalf("info=%+v", info)
	}

	tok, err := acct.LookupDeployToken(plain)
	if err != nil || tok == nil {
		t.Fatalf("lookup: %v", err)
	}
	if !tok.HasScope(auth.ScopeDeploy) || !tok.HasScope(auth.ScopeFiles) {
		t.Fatalf("scopes=%v", tok.Scopes)
	}
	if tok.Site != "app.example.com" {
		t.Fatalf("site=%q", tok.Site)
	}

	listed, err := acct.ListDeployTokens("app.example.com")
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %v len=%d", err, len(listed))
	}

	if err := acct.RevokeDeployToken("app.example.com", info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := acct.LookupDeployToken(plain); err == nil {
		t.Fatal("expected revoked")
	}
	listed, err = acct.ListDeployTokens("app.example.com")
	if err != nil || len(listed) != 0 {
		t.Fatalf("list after revoke: %v len=%d", err, len(listed))
	}
}

func TestDeployTokenExpired(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	acct := &auth.Account{DB: db, Now: func() time.Time { return now }}
	exp := now.Add(-time.Hour)
	plain, _, err := acct.CreateDeployToken("x.test", "old", []string{auth.ScopeDeploy}, &exp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acct.LookupDeployToken(plain); err == nil {
		t.Fatal("expected expired")
	}
}

func TestDeployTokenRejectsBadScopes(t *testing.T) {
	dir := t.TempDir()
	db, err := config.OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	acct := &auth.Account{DB: db}
	if _, _, err := acct.CreateDeployToken("x.test", "n", []string{"admin"}, nil); err == nil {
		t.Fatal("expected scope rejection")
	}
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
