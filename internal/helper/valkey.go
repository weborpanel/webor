package helper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/valkey"
)

// ValkeyStatusResult reports whether Valkey/Redis is usable.
type ValkeyStatusResult struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Message   string `json:"message,omitempty"`
	Binary    string `json:"binary,omitempty"`
}

// EnsureSiteCacheResult is returned after creating/reconciling a site cache ACL.
type EnsureSiteCacheResult struct {
	Engine     string `json:"engine"`
	Username   string `json:"username"`
	Prefix     string `json:"prefix"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	SecretPath string `json:"secret_path,omitempty"`
}

// ValkeyStatus checks for valkey/redis-server and service.
func (o *Ops) ValkeyStatus() (*ValkeyStatusResult, error) {
	res := &ValkeyStatusResult{}
	bin := o.valkeyCLI()
	if bin == "" {
		res.Message = "valkey-cli/redis-cli not found"
		return res, nil
	}
	res.Binary = bin
	res.Installed = true
	for _, unit := range []string{"valkey", "valkey-server", "redis", "redis-server"} {
		if out, err := exec.Command("systemctl", "is-active", unit).CombinedOutput(); err == nil {
			if strings.TrimSpace(string(out)) == "active" {
				res.Active = true
				res.Message = unit + " active"
				return res, nil
			}
		}
	}
	res.Message = "valkey/redis installed but not active"
	return res, nil
}

func (o *Ops) valkeyCLI() string {
	for _, name := range []string{"valkey-cli", "redis-cli"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// InstallValkey installs and enables Valkey (falls back to redis-server).
func (o *Ops) InstallValkey() error {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("apt-get not found (InstallValkey supports Debian/Ubuntu)")
	}
	update := exec.Command("apt-get", "update", "-qq")
	update.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if out, err := update.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get update: %w (%s)", err, truncateOut(out))
	}
	installPkg := func(pkg string) error {
		cmd := exec.Command("apt-get", "install", "-y", pkg)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("apt-get install %s: %w (%s)", pkg, err, truncateOut(out))
		}
		return nil
	}
	if err := installPkg("valkey"); err != nil {
		if err2 := installPkg("redis-server"); err2 != nil {
			return fmt.Errorf("install valkey failed (%v); redis-server failed (%v)", err, err2)
		}
	}
	// Prefer real unit names; "valkey"/"redis" are aliases and `enable` refuses them.
	for _, unit := range []string{"valkey-server", "redis-server"} {
		_ = exec.Command("systemctl", "enable", "--now", unit).Run()
	}
	st, err := o.ValkeyStatus()
	if err != nil {
		return err
	}
	if !st.Installed || !st.Active {
		return fmt.Errorf("valkey install finished but service not active: %s", st.Message)
	}
	return nil
}

// EnsureSiteCache creates/reconciles a per-site Valkey ACL user.
func (o *Ops) EnsureSiteCache(domain, engine, stateDir string) (*EnsureSiteCacheResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine != "" && engine != valkey.Engine && engine != "redis" {
		return nil, fmt.Errorf("unsupported cache engine %q", engine)
	}
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	st, err := o.ValkeyStatus()
	if err != nil {
		return nil, err
	}
	if !st.Installed {
		return nil, fmt.Errorf("valkey not installed")
	}
	if !st.Active {
		for _, unit := range []string{"valkey", "valkey-server", "redis-server", "redis"} {
			_ = exec.Command("systemctl", "start", unit).Run()
		}
		st, _ = o.ValkeyStatus()
		if st == nil || !st.Active {
			return nil, fmt.Errorf("valkey not active")
		}
	}

	user := valkey.Username(domain)
	prefix := valkey.KeyPrefix(domain)
	secretPath := valkey.SecretPath(stateDir, domain)
	password, err := randomPassword(24)
	if err != nil {
		return nil, err
	}

	if err := o.valkeyACLSet(user, password, prefix); err != nil {
		return nil, fmt.Errorf("acl setuser: %w", err)
	}
	_ = o.valkeyArgs("ACL", "SAVE")

	sec := valkey.Secret{
		User:     user,
		Password: password,
		Host:     valkey.DefaultTCPHost,
		Port:     valkey.DefaultPort,
		Prefix:   prefix,
	}
	if err := valkey.WriteSecret(secretPath, sec); err != nil {
		return nil, err
	}
	_ = chownWebor(secretPath)
	_ = chownWebor(filepath.Dir(secretPath))

	return &EnsureSiteCacheResult{
		Engine:     valkey.Engine,
		Username:   user,
		Prefix:     prefix,
		Host:       valkey.DefaultTCPHost,
		Port:       valkey.DefaultPort,
		SecretPath: secretPath,
	}, nil
}

// DropSiteCache removes the site ACL user and secret.
func (o *Ops) DropSiteCache(domain, stateDir string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	user := valkey.Username(domain)
	secretPath := valkey.SecretPath(stateDir, domain)
	st, err := o.ValkeyStatus()
	if err != nil {
		return err
	}
	if st.Installed && st.Active {
		_ = o.valkeyArgs("ACL", "DELUSER", user)
		_ = o.valkeyArgs("ACL", "SAVE")
	}
	return valkey.RemoveSecret(secretPath)
}

// RotateCachePassword regenerates the Valkey ACL password for a site.
func (o *Ops) RotateCachePassword(domain, stateDir string) (*EnsureSiteCacheResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	st, err := o.ValkeyStatus()
	if err != nil {
		return nil, err
	}
	if !st.Installed || !st.Active {
		return nil, fmt.Errorf("valkey not active")
	}
	user := valkey.Username(domain)
	prefix := valkey.KeyPrefix(domain)
	secretPath := valkey.SecretPath(stateDir, domain)
	password, err := randomPassword(24)
	if err != nil {
		return nil, err
	}
	if err := o.valkeyACLSet(user, password, prefix); err != nil {
		return nil, fmt.Errorf("acl rotate: %w", err)
	}
	_ = o.valkeyArgs("ACL", "SAVE")
	sec := valkey.Secret{
		User:     user,
		Password: password,
		Host:     valkey.DefaultTCPHost,
		Port:     valkey.DefaultPort,
		Prefix:   prefix,
	}
	if err := valkey.WriteSecret(secretPath, sec); err != nil {
		return nil, err
	}
	_ = chownWebor(secretPath)
	return &EnsureSiteCacheResult{
		Engine:     valkey.Engine,
		Username:   user,
		Prefix:     prefix,
		Host:       valkey.DefaultTCPHost,
		Port:       valkey.DefaultPort,
		SecretPath: secretPath,
	}, nil
}

func (o *Ops) valkeyACLSet(user, password, prefix string) error {
	return o.valkeyArgs(
		"ACL", "SETUSER", user, "on",
		">"+password,
		"~"+prefix+"*",
		"+@all", "-@admin", "-@dangerous",
	)
}

func (o *Ops) valkeyArgs(args ...string) error {
	bin := o.valkeyCLI()
	if bin == "" {
		return fmt.Errorf("valkey-cli/redis-cli not found")
	}
	c := exec.Command(bin, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", bin, args[:min(2, len(args))], err, truncateOut(out))
	}
	return nil
}
