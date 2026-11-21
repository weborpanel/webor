// Package postgres holds naming and status helpers for managed Postgres.
package postgres

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/sites"
)

const (
	// Engine is the config.yaml value for a postgres SiteDatabase.
	Engine = "postgres"
	// DefaultUnixSocketDir is the Debian/Ubuntu Postgres socket directory.
	DefaultUnixSocketDir = "/var/run/postgresql"
	// DefaultTCPHost is used for panel read-only (password) connections.
	DefaultTCPHost = "127.0.0.1"
	// DefaultPort is the Postgres listen port.
	DefaultPort = 5432
)

// DBName returns the database name for a parent site + db id.
func DBName(domain, id string) string {
	return dbengine.DBName(domain, id)
}

// PeerRole returns the peer-auth role / Linux username for a site.
func PeerRole(domain string) string {
	return sites.Username(domain)
}

// AppRole is the password login role for apps (fixed username).
func AppRole(domain, id string) string {
	return dbengine.AppUser(domain, id)
}

// ReadOnlyRole returns the panel browse role for a site database.
func ReadOnlyRole(domain, id string) string {
	return "webor_ro_" + dbengine.AppUser(domain, id)
}

// PeerDSN is the libpq keyword DSN for Unix peer auth as the site user.
func PeerDSN(domain, id string) string {
	db := DBName(domain, id)
	peer := PeerRole(domain)
	return fmt.Sprintf("dbname=%s user=%s host=%s", db, peer, DefaultUnixSocketDir)
}

// PeerURL is a postgresql URL form for documentation.
func PeerURL(domain, id string) string {
	db := DBName(domain, id)
	return fmt.Sprintf("postgresql:///%s?host=%s", db, DefaultUnixSocketDir)
}

// PasswordURL is a TCP URL for apps using the fixed app role + password.
func PasswordURL(domain, id, password string) string {
	user := AppRole(domain, id)
	db := DBName(domain, id)
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", user, password, DefaultTCPHost, DefaultPort, db)
}

// SecretFileName returns the basename for the RO secret JSON file.
func SecretFileName(domain, id string) string {
	return dbengine.SecretFileName(domain, id, Engine)
}

// LegacySecretFileName is the pre-multi-DB secret basename.
func LegacySecretFileName(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, "/", "_")
	return d + ".pg.ro.json"
}

// SecretPath returns the preferred secret path; callers may fall back to LegacySecretPath.
func SecretPath(stateDir, domain, id string) string {
	return filepath.Join(SecretsDir(stateDir), SecretFileName(domain, id))
}

// LegacySecretPath is the old single-DB secret location.
func LegacySecretPath(stateDir, domain string) string {
	return filepath.Join(SecretsDir(stateDir), LegacySecretFileName(domain))
}

// SecretsDir is stateDir/secrets.
func SecretsDir(stateDir string) string {
	return filepath.Join(stateDir, "secrets")
}
