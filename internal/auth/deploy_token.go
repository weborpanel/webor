package auth

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Deploy token scopes (comma-separated in DB).
const (
	ScopeFiles   = "files"
	ScopeDeploy  = "deploy"
	ScopeRuntime = "runtime"
	ScopeAPI     = "api"
)

// DeployTokenPrefix is prepended to plaintext deploy tokens.
const DeployTokenPrefix = "wbr_dt_"

// AllDeployScopes is the default set when creating a CI token.
var AllDeployScopes = []string{ScopeFiles, ScopeDeploy, ScopeRuntime, ScopeAPI}

// DeployToken is a site-scoped API credential for CI.
type DeployToken struct {
	ID         string
	Name       string
	Site       string
	Scopes     []string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
}

// HasScope reports whether the token includes scope (or was issued with it).
func (t *DeployToken) HasScope(scope string) bool {
	scope = strings.TrimSpace(strings.ToLower(scope))
	for _, s := range t.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// DeployTokenInfo is the public view (never includes plaintext).
type DeployTokenInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Site       string     `json:"site"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// CreateDeployToken mints a new token for site. Plaintext is returned once.
func (a *Account) CreateDeployToken(site, name string, scopes []string, expiresAt *time.Time) (plain string, info *DeployTokenInfo, err error) {
	site = strings.ToLower(strings.TrimSpace(site))
	if site == "" {
		return "", nil, fmt.Errorf("site is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "ci"
	}
	scopes = normalizeScopes(scopes)
	if len(scopes) == 0 {
		return "", nil, fmt.Errorf("at least one scope is required")
	}

	raw, hash, err := NewToken(32)
	if err != nil {
		return "", nil, err
	}
	plain = DeployTokenPrefix + raw
	hash = HashToken(plain)

	id, _, err := NewToken(12)
	if err != nil {
		return "", nil, err
	}
	now := a.now()
	var expSQL any
	if expiresAt != nil {
		expSQL = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = a.DB.Exec(
		`INSERT INTO deploy_tokens (id, token_hash, name, site, scopes, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, hash, name, site, strings.Join(scopes, ","),
		now.Format(time.RFC3339Nano), expSQL,
	)
	if err != nil {
		return "", nil, err
	}
	info = &DeployTokenInfo{
		ID:        id,
		Name:      name,
		Site:      site,
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	return plain, info, nil
}

// ListDeployTokens returns non-revoked tokens for a site.
func (a *Account) ListDeployTokens(site string) ([]DeployTokenInfo, error) {
	site = strings.ToLower(strings.TrimSpace(site))
	rows, err := a.DB.Query(
		`SELECT id, name, site, scopes, created_at, last_used_at, expires_at
		 FROM deploy_tokens WHERE site = ? AND revoked_at IS NULL ORDER BY created_at DESC`,
		site,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeployTokenInfo
	for rows.Next() {
		info, err := scanDeployTokenInfo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *info)
	}
	return out, rows.Err()
}

// RevokeDeployToken soft-deletes a token (must match site).
func (a *Account) RevokeDeployToken(site, id string) error {
	site = strings.ToLower(strings.TrimSpace(site))
	id = strings.TrimSpace(id)
	res, err := a.DB.Exec(
		`UPDATE deploy_tokens SET revoked_at = ? WHERE id = ? AND site = ? AND revoked_at IS NULL`,
		a.now().Format(time.RFC3339Nano), id, site,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("token not found")
	}
	return nil
}

// LookupDeployToken validates a presented Bearer token.
func (a *Account) LookupDeployToken(plain string) (*DeployToken, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" || !strings.HasPrefix(plain, DeployTokenPrefix) {
		return nil, fmt.Errorf("invalid token")
	}
	hash := HashToken(plain)
	row := a.DB.QueryRow(
		`SELECT id, name, site, scopes, created_at, last_used_at, expires_at, revoked_at
		 FROM deploy_tokens WHERE token_hash = ?`,
		hash,
	)
	var t DeployToken
	var scopes, created string
	var lastUsed, expires, revoked sql.NullString
	if err := row.Scan(&t.ID, &t.Name, &t.Site, &scopes, &created, &lastUsed, &expires, &revoked); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid token")
		}
		return nil, err
	}
	if revoked.Valid && strings.TrimSpace(revoked.String) != "" {
		return nil, fmt.Errorf("token revoked")
	}
	t.Scopes = splitScopes(scopes)
	t.CreatedAt, _ = parseTime(created)
	if lastUsed.Valid && lastUsed.String != "" {
		lu, _ := parseTime(lastUsed.String)
		t.LastUsedAt = &lu
	}
	if expires.Valid && expires.String != "" {
		exp, err := parseTime(expires.String)
		if err == nil {
			t.ExpiresAt = &exp
			if a.now().After(exp) {
				return nil, fmt.Errorf("token expired")
			}
		}
	}
	_, _ = a.DB.Exec(
		`UPDATE deploy_tokens SET last_used_at = ? WHERE id = ?`,
		a.now().Format(time.RFC3339Nano), t.ID,
	)
	return &t, nil
}

func normalizeScopes(scopes []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range scopes {
		s = strings.TrimSpace(strings.ToLower(s))
		switch s {
		case ScopeFiles, ScopeDeploy, ScopeRuntime, ScopeAPI:
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

func splitScopes(s string) []string {
	parts := strings.Split(s, ",")
	return normalizeScopes(parts)
}

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDeployTokenInfo(row scannable) (*DeployTokenInfo, error) {
	var info DeployTokenInfo
	var scopes, created string
	var lastUsed, expires sql.NullString
	if err := row.Scan(&info.ID, &info.Name, &info.Site, &scopes, &created, &lastUsed, &expires); err != nil {
		return nil, err
	}
	info.Scopes = splitScopes(scopes)
	info.CreatedAt, _ = parseTime(created)
	if lastUsed.Valid && lastUsed.String != "" {
		lu, _ := parseTime(lastUsed.String)
		info.LastUsedAt = &lu
	}
	if expires.Valid && expires.String != "" {
		exp, _ := parseTime(expires.String)
		info.ExpiresAt = &exp
	}
	return &info, nil
}
