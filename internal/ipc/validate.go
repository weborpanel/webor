package ipc

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	sitesRoot    = "/srv/sites"
	maxUnitBytes = 256 * 1024
	maxUnitName  = 128
	maxDomainLen = 253
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

// ValidateDeleteSiteUserArgs validates DeleteSiteUser arguments.
func ValidateDeleteSiteUserArgs(a DeleteSiteUserArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	if a.DeleteData && strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required when delete_data is true")
	}
	return nil
}

// ValidateRunBuildArgs validates RunBuild arguments.
func ValidateRunBuildArgs(a RunBuildArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	cmd := strings.TrimSpace(a.Command)
	if cmd == "" {
		return fmt.Errorf("command is required")
	}
	if len(cmd) > 8*1024 {
		return fmt.Errorf("command too long")
	}
	if a.TimeoutSec < 0 || a.TimeoutSec > 3600 {
		return fmt.Errorf("timeout_sec out of range")
	}
	if a.MemoryMax != "" {
		if err := validateMemoryMax(a.MemoryMax); err != nil {
			return err
		}
	}
	return nil
}

func validateMemoryMax(v string) error {
	v = strings.TrimSpace(v)
	ok, _ := regexp.MatchString(`(?i)^[0-9]+[KMG]?$`, v)
	if !ok {
		return fmt.Errorf("invalid memory_max %q", v)
	}
	return nil
}

// ValidateWritePublicFileArgs validates WritePublicFile arguments.
func ValidateWritePublicFileArgs(a WritePublicFileArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	p := strings.TrimSpace(a.Path)
	if p == "" {
		return fmt.Errorf("path is required")
	}
	if strings.Contains(p, "..") || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return fmt.Errorf("invalid path")
	}
	full := filepath.Join(sitesRoot, strings.ToLower(a.Domain), "public", p)
	if err := ValidateSitePath(a.Domain, full); err != nil {
		return err
	}
	if len(a.Content) > 5*1024*1024 {
		return fmt.Errorf("content exceeds 5 MiB")
	}
	return nil
}

// ValidateDeployStaticZipArgs validates DeployStaticZip arguments.
func ValidateDeployStaticZipArgs(a DeployStaticZipArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	p := filepath.Clean(strings.TrimSpace(a.ZipPath))
	if p == "" || p == "." {
		return fmt.Errorf("zip_path is required")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("zip_path must be absolute")
	}
	if strings.ToLower(filepath.Ext(p)) != ".zip" {
		return fmt.Errorf("zip_path must end in .zip")
	}
	domain := strings.ToLower(a.Domain)
	allowedPrefixes := []string{
		filepath.Join(sitesRoot, domain, "tmp") + string(filepath.Separator),
		filepath.Join("/var/lib/webooza/uploads", domain) + string(filepath.Separator),
	}
	ok := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(p, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("zip_path %q not under allowed upload directories", p)
	}
	return nil
}

// ValidateDeployAppBinaryArgs validates DeployAppBinary arguments.
func ValidateDeployAppBinaryArgs(a DeployAppBinaryArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	p := filepath.Clean(strings.TrimSpace(a.BinaryPath))
	if p == "" || p == "." {
		return fmt.Errorf("binary_path is required")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("binary_path must be absolute")
	}
	domain := strings.ToLower(strings.TrimSpace(a.Domain))
	allowedPrefixes := []string{
		filepath.Join(sitesRoot, domain, "tmp") + string(filepath.Separator),
		filepath.Join("/var/lib/webooza/uploads", domain) + string(filepath.Separator),
		filepath.Join(sitesRoot, domain) + string(filepath.Separator),
	}
	ok := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(p, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("binary_path %q not under allowed directories", p)
	}
	return nil
}

var gitURLRE = regexp.MustCompile(`^(https://|http://|git@|ssh://)`)

// ValidateDeployAppGitArgs validates DeployAppGit arguments.
func ValidateDeployAppGitArgs(a DeployAppGitArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	url := strings.TrimSpace(a.GitURL)
	if url == "" {
		return fmt.Errorf("git_url is required")
	}
	if len(url) > 2048 {
		return fmt.Errorf("git_url too long")
	}
	if strings.ContainsAny(url, " \t\n\r;$`|&;<>(){}\\\"'") {
		return fmt.Errorf("git_url contains invalid characters")
	}
	if !gitURLRE.MatchString(url) {
		return fmt.Errorf("git_url must start with https://, http://, git@, or ssh://")
	}
	ref := strings.TrimSpace(a.GitRef)
	if strings.ContainsAny(ref, " \t\n\r;$`|&;<>(){}\\\"'") {
		return fmt.Errorf("git_ref contains invalid characters")
	}
	art := strings.TrimSpace(a.Artifact)
	if art != "" {
		if strings.Contains(art, "..") || strings.HasPrefix(art, "/") || strings.Contains(art, `\`) {
			return fmt.Errorf("invalid artifact path")
		}
	}
	cmd := strings.TrimSpace(a.BuildCommand)
	if len(cmd) > 8*1024 {
		return fmt.Errorf("build_command too long")
	}
	if a.TimeoutSec < 0 || a.TimeoutSec > 3600 {
		return fmt.Errorf("timeout_sec out of range")
	}
	if a.MemoryMax != "" {
		if err := validateMemoryMax(a.MemoryMax); err != nil {
			return err
		}
	}
	return nil
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

// ValidateRunCronNowArgs validates RunCronNow arguments.
func ValidateRunCronNowArgs(a RunCronNowArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	job := strings.TrimSpace(a.Job)
	if job == "" {
		return fmt.Errorf("job is required")
	}
	if len(job) > 64 {
		return fmt.Errorf("job name too long")
	}
	if strings.ContainsAny(job, "/\\\n\r") {
		return fmt.Errorf("invalid job name")
	}
	return nil
}

// ValidateCronStatusArgs validates CronStatus arguments.
func ValidateCronStatusArgs(a CronStatusArgs) error {
	return ValidateRunCronNowArgs(RunCronNowArgs{Domain: a.Domain, Job: a.Job})
}

// ValidateJournalTailArgs validates JournalTail arguments.
func ValidateJournalTailArgs(a JournalTailArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	sel := strings.TrimSpace(a.Unit)
	if sel == "" {
		sel = "app"
	}
	switch {
	case sel == "app", sel == "backend":
	case strings.HasPrefix(sel, "cron:"), strings.HasPrefix(sel, "job:"):
		name := strings.TrimSpace(sel[strings.IndexByte(sel, ':')+1:])
		if name == "" || len(name) > 64 || strings.ContainsAny(name, "/\\\n\r") {
			return fmt.Errorf("invalid unit selector")
		}
	default:
		return fmt.Errorf("unit must be app|backend|cron:<name>|job:<name>")
	}
	if a.Lines < 0 || a.Lines > 500 {
		return fmt.Errorf("lines out of range")
	}
	return nil
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
