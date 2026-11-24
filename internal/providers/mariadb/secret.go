package mariadb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/weborpanel/webor/internal/providers/dbengine"
)

// ROSecret is the panel browse credential for MariaDB.
type ROSecret struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

func (s ROSecret) DSN() string {
	return s.DSNAs(s.User)
}

// DSNAs returns a MySQL DSN using user (e.g. AppRole) and this secret's password.
func (s ROSecret) DSNAs(user string) string {
	host := s.Host
	if host == "" {
		host = DefaultTCPHost
	}
	port := s.Port
	if port == 0 {
		port = DefaultPort
	}
	if user == "" {
		user = s.User
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", user, s.Password, host, port, s.Database)
}

func (s ROSecret) PasswordURL() string {
	host := s.Host
	if host == "" {
		host = DefaultTCPHost
	}
	port := s.Port
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("mysql://%s:%s@%s:%d/%s", s.User, s.Password, host, port, s.Database)
}

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

func RemoveSecret(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
