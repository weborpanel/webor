package ipc

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	sitesRoot     = "/srv/sites"
	maxUnitBytes  = 256 * 1024
	maxUnitName   = 128
	maxDomainLen  = 253
)

var (
	domainRE   = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	unitNameRE = regexp.MustCompile(`^[A-Za-z0-9:_.@+-]+\.(service|socket|timer|path)$`)
)

// ValidateDomain checks domain syntax for helper arguments.
func ValidateDomain(domain string) error {
	d := strings.TrimSpace(domain)
	if d == "" {
		return fmt.Errorf("domain is required")
	}
	if len(d) > maxDomainLen {
		return fmt.Errorf("domain too long")
	}
	if !domainRE.MatchString(d) {
		return fmt.Errorf("invalid domain %q", domain)
	}
	return nil
}

// ValidateSitePath ensures path is under /srv/sites/<domain>/ (or equals that root).
func ValidateSitePath(domain, path string) error {
	if err := ValidateDomain(domain); err != nil {
		return err
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("path must be absolute")
	}
	root := filepath.Join(sitesRoot, strings.ToLower(domain))
	if clean != root && !strings.HasPrefix(clean, root+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes site root %q", clean, root)
	}
	return nil
}

// ValidateUnitName checks a systemd unit basename.
func ValidateUnitName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("unit name is required")
	}
	if len(n) > maxUnitName {
		return fmt.Errorf("unit name too long")
	}
	if strings.Contains(n, "/") || strings.Contains(n, `\`) {
		return fmt.Errorf("unit name must be a basename")
	}
	if !unitNameRE.MatchString(n) {
		return fmt.Errorf("invalid unit name %q", name)
	}
	return nil
}

// ValidateUnitContent enforces size and UTF-8 limits on unit file bodies.
func ValidateUnitContent(content string) error {
	if content == "" {
		return fmt.Errorf("unit content is required")
	}
	if len(content) > maxUnitBytes {
		return fmt.Errorf("unit content exceeds %d bytes", maxUnitBytes)
	}
	if !utf8.ValidString(content) {
		return fmt.Errorf("unit content must be valid UTF-8")
	}
	return nil
}

// ValidateCreateSiteUserArgs validates CreateSiteUser arguments.
func ValidateCreateSiteUserArgs(a CreateSiteUserArgs) error {
	return ValidateDomain(a.Domain)
}

// ValidateWriteUnitFileArgs validates WriteUnitFile arguments.
func ValidateWriteUnitFileArgs(a WriteUnitFileArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	if err := ValidateUnitName(a.Unit); err != nil {
		return err
	}
	return ValidateUnitContent(a.Content)
}

// ValidateReloadCaddyArgs validates ReloadCaddy arguments.
func ValidateReloadCaddyArgs(a ReloadCaddyArgs) error {
	p := filepath.Clean(strings.TrimSpace(a.ConfigPath))
	if p == "" || p == "." {
		return fmt.Errorf("config_path is required")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("config_path must be absolute")
	}
	// Confine Caddy config to known panel/config locations for M0 surface.
	allowedPrefixes := []string{"/etc/webooza/", "/var/lib/webooza/"}
	ok := false
	for _, prefix := range allowedPrefixes {
		if p == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(p, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("config_path %q not under allowed directories", p)
	}
	return nil
}
