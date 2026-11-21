package postgres

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/weborpanel/webor/internal/providers/dbengine"
)

// ROSecret is the on-disk credential for panel DBAdminer browse / app env.
type ROSecret struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// DSN returns a libpq keyword DSN for the read-only role.
func (s ROSecret) DSN() string {
	host := s.Host
	if host == "" {
		host = DefaultTCPHost
	}
	port := s.Port
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, s.User, s.Password, s.Database)
}

// PasswordURL returns a postgresql URL with the secret password.
func (s ROSecret) PasswordURL() string {
	host := s.Host
	if host == "" {
		host = DefaultTCPHost
	}
	port := s.Port
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", s.User, s.Password, host, port, s.Database)
}

// ResolveSecretPath returns the multi-DB path, or legacy path if that exists and new does not.
func ResolveSecretPath(stateDir, domain, id string) string {
	p := SecretPath(stateDir, domain, id)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if dbengine.IsDefaultID(domain, id) {
		legacy := LegacySecretPath(stateDir, domain)
		if _, err := os.Stat(legacy); err == nil {
			return legacy
		}
	}
	return p
}

// WriteSecret writes the RO secret with mode 0600.
func WriteSecret(path string, sec ROSecret) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(sec, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadSecret loads an RO secret from disk.
func ReadSecret(path string) (ROSecret, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ROSecret{}, err
	}
	var sec ROSecret
	if err := json.Unmarshal(raw, &sec); err != nil {
		return ROSecret{}, err
	}
	return sec, nil
}

// RemoveSecret deletes the RO secret if present.
func RemoveSecret(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
