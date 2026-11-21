package valkey

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Secret is the panel/app credential for a site Valkey ACL user.
type Secret struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Prefix   string `json:"prefix"`
}

func (s Secret) URL() string {
	host := s.Host
	if host == "" {
		host = DefaultTCPHost
	}
	port := s.Port
	if port == 0 {
		port = DefaultPort
	}
	if s.User == "" {
		return fmt.Sprintf("redis://:%s@%s:%d/0", s.Password, host, port)
	}
	return fmt.Sprintf("redis://%s:%s@%s:%d/0", s.User, s.Password, host, port)
}

func WriteSecret(path string, sec Secret) error {
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

func ReadSecret(path string) (Secret, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Secret{}, err
	}
	var sec Secret
	if err := json.Unmarshal(raw, &sec); err != nil {
		return Secret{}, err
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
