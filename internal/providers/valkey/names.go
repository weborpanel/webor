// Package valkey holds naming and secret paths for shared Valkey/Redis cache.
package valkey

import (
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/sites"
)

const (
	Engine         = "valkey"
	DefaultTCPHost = "127.0.0.1"
	DefaultPort    = 6379
)

// Username is the fixed ACL username for a parent site.
func Username(domain string) string {
	return sites.Username(domain)
}

// KeyPrefix is the ACL-restricted key namespace (e.g. wbr_example_com:).
func KeyPrefix(domain string) string {
	return Username(domain) + ":"
}

// SecretFileName is secrets/<domain>.valkey.json.
func SecretFileName(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, "/", "_")
	return d + ".valkey.json"
}

func SecretsDir(stateDir string) string {
	return filepath.Join(stateDir, "secrets")
}

func SecretPath(stateDir, domain string) string {
	return filepath.Join(SecretsDir(stateDir), SecretFileName(domain))
}
