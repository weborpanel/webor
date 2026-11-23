package mariadb

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/sites"
)

const (
	Engine         = "mariadb"
	DefaultTCPHost = "127.0.0.1"
	DefaultPort    = 3306
)

func DBName(domain, id string) string   { return dbengine.DBName(domain, id) }
func PeerRole(domain string) string     { return sites.Username(domain) }
func AppRole(domain, id string) string  { return dbengine.AppUser(domain, id) }
func ReadOnlyRole(domain, id string) string {
	return "webor_ro_" + dbengine.AppUser(domain, id)
}

func PeerDSN(domain, id string) string {
	db := DBName(domain, id)
	peer := PeerRole(domain)
	return fmt.Sprintf("user=%s database=%s", peer, db)
}

func PeerURL(domain, id string) string {
	return fmt.Sprintf("mysql:///%s?socket=%s", DBName(domain, id), UnixSocketPath())
}

func SecretFileName(domain, id string) string {
	return dbengine.SecretFileName(domain, id, Engine)
}

func LegacySecretFileName(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, "/", "_")
	return d + ".maria.ro.json"
}

func SecretsDir(stateDir string) string {
	return filepath.Join(stateDir, "secrets")
}

func SecretPath(stateDir, domain, id string) string {
	return filepath.Join(SecretsDir(stateDir), SecretFileName(domain, id))
}

func LegacySecretPath(stateDir, domain string) string {
	return filepath.Join(SecretsDir(stateDir), LegacySecretFileName(domain))
}
