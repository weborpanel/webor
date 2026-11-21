package ipc

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/weborpanel/webor/internal/config"
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
	for _, child := range a.CascadeDomains {
		if strings.TrimSpace(child) == "" {
			continue
		}
		if err := ValidateDomain(child); err != nil {
			return fmt.Errorf("cascade_domains: %w", err)
		}
	}
	for _, db := range a.DropDatabases {
		if err := validateEngine(db.Engine); err != nil {
			return fmt.Errorf("drop_databases: %w", err)
		}
		if err := validateDBID(a.Domain, db.ID); err != nil {
			return fmt.Errorf("drop_databases: %w", err)
		}
	}
	return validateStateDir(a.StateDir)
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
	if len(a.Content) > 32*1024*1024 {
		return fmt.Errorf("content exceeds 32 MiB")
	}
	return nil
}

func validatePublicRelPath(domain, rel string, allowEmpty bool) error {
	if err := ValidateDomain(domain); err != nil {
		return err
	}
	p := strings.TrimSpace(rel)
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimPrefix(p, "/")
	if p == "" || p == "." {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("path is required")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("invalid path")
	}
	full := filepath.Join(sitesRoot, strings.ToLower(domain), "public", p)
	return ValidateSitePath(domain, full)
}

// ValidateListPublicDirArgs validates ListPublicDir arguments.
func ValidateListPublicDirArgs(a ListPublicDirArgs) error {
	return validatePublicRelPath(a.Domain, a.Path, true)
}

// ValidateMkdirPublicArgs validates MkdirPublic arguments.
func ValidateMkdirPublicArgs(a MkdirPublicArgs) error {
	return validatePublicRelPath(a.Domain, a.Path, false)
}

// ValidateRenamePublicArgs validates RenamePublic arguments.
func ValidateRenamePublicArgs(a RenamePublicArgs) error {
	if err := validatePublicRelPath(a.Domain, a.From, false); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	if err := validatePublicRelPath(a.Domain, a.To, false); err != nil {
		return fmt.Errorf("to: %w", err)
	}
	return nil
}

// ValidateDeletePublicPathArgs validates DeletePublicPath arguments.
func ValidateDeletePublicPathArgs(a DeletePublicPathArgs) error {
	return validatePublicRelPath(a.Domain, a.Path, false)
}

// ValidateReadPublicFileArgs validates ReadPublicFile arguments.
func ValidateReadPublicFileArgs(a ReadPublicFileArgs) error {
	return validatePublicRelPath(a.Domain, a.Path, false)
}

// ValidateExtractPublicArchiveArgs validates ExtractPublicArchive arguments.
func ValidateExtractPublicArchiveArgs(a ExtractPublicArchiveArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	if err := validatePublicRelPath(a.Domain, a.Dest, true); err != nil {
		return fmt.Errorf("dest: %w", err)
	}
	p := filepath.Clean(strings.TrimSpace(a.ArchivePath))
	if p == "" || p == "." {
		return fmt.Errorf("archive_path is required")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("archive_path must be absolute")
	}
	lower := strings.ToLower(p)
	okExt := strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
	if !okExt {
		return fmt.Errorf("archive_path must be .zip, .tar, .tar.gz or .tgz")
	}
	domain := strings.ToLower(a.Domain)
	allowedPrefixes := []string{
		filepath.Join(sitesRoot, domain, "tmp") + string(filepath.Separator),
		filepath.Join("/var/lib/webor/uploads", domain) + string(filepath.Separator),
	}
	ok := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(p, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("archive_path %q not under allowed upload directories", p)
	}
	switch strings.ToLower(strings.TrimSpace(a.Format)) {
	case "", "zip", "tar", "tar.gz", "tgz":
		return nil
	default:
		return fmt.Errorf("unsupported format %q", a.Format)
	}
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
		filepath.Join("/var/lib/webor/uploads", domain) + string(filepath.Separator),
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
		filepath.Join("/var/lib/webor/uploads", domain) + string(filepath.Separator),
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
	allowedPrefixes := []string{"/etc/webor/", "/var/lib/webor/"}
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

// ValidateInstallPostgresArgs requires an approval id.
func ValidateInstallPostgresArgs(a InstallPostgresArgs) error {
	if strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required")
	}
	return nil
}

// ValidateInstallMariaDBArgs requires an approval id.
func ValidateInstallMariaDBArgs(a InstallMariaDBArgs) error {
	if strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required")
	}
	return nil
}

// ValidateInstallValkeyArgs requires an approval id.
func ValidateInstallValkeyArgs(a InstallValkeyArgs) error {
	if strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required")
	}
	return nil
}

// ValidateEnsureSiteCacheArgs validates EnsureSiteCache.
func ValidateEnsureSiteCacheArgs(a EnsureSiteCacheArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	eng := strings.ToLower(strings.TrimSpace(a.Engine))
	if eng != "" && eng != "valkey" && eng != "redis" {
		return fmt.Errorf("unsupported cache engine %q", a.Engine)
	}
	return validateStateDir(a.StateDir)
}

// ValidateDropSiteCacheArgs validates DropSiteCache.
func ValidateDropSiteCacheArgs(a DropSiteCacheArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	return validateStateDir(a.StateDir)
}

// ValidateRotateCachePasswordArgs validates RotateCachePassword.
func ValidateRotateCachePasswordArgs(a RotateCachePasswordArgs) error {
	return ValidateDropSiteCacheArgs(DropSiteCacheArgs{Domain: a.Domain, StateDir: a.StateDir})
}

func validateStateDir(stateDir string) error {
	if stateDir == "" {
		return nil
	}
	p := filepath.Clean(stateDir)
	if !filepath.IsAbs(p) {
		return fmt.Errorf("state_dir must be absolute")
	}
	if p != "/var/lib/webor" && !strings.HasPrefix(p, "/var/lib/webor/") && !strings.HasPrefix(p, "/tmp/") {
		return fmt.Errorf("state_dir not allowed")
	}
	return nil
}

func validateEngine(engine string) error {
	e := strings.ToLower(strings.TrimSpace(engine))
	switch e {
	case "sqlite", "postgres", "mariadb":
		return nil
	case "":
		return fmt.Errorf("engine is required")
	default:
		return fmt.Errorf("unsupported engine %q", engine)
	}
}

var dbIDRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// validateDBID allows blank (default slug) or a normalized id matching dbIDRE.
func validateDBID(domain, id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil
	}
	if !dbIDRE.MatchString(id) {
		return fmt.Errorf("invalid database id %q", id)
	}
	_ = domain
	return nil
}

// ValidateEnsureSiteDatabaseArgs validates EnsureSiteDatabase arguments.
func ValidateEnsureSiteDatabaseArgs(a EnsureSiteDatabaseArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	if err := validateEngine(a.Engine); err != nil {
		return err
	}
	if err := validateDBID(a.Domain, a.ID); err != nil {
		return err
	}
	return validateStateDir(a.StateDir)
}

// ValidateDropSiteDatabaseArgs validates DropSiteDatabase arguments.
func ValidateDropSiteDatabaseArgs(a DropSiteDatabaseArgs) error {
	return ValidateEnsureSiteDatabaseArgs(EnsureSiteDatabaseArgs{
		Domain: a.Domain, Engine: a.Engine, ID: a.ID, StateDir: a.StateDir,
	})
}

// ValidateRotateDatabasePasswordArgs validates RotateDatabasePassword arguments.
func ValidateRotateDatabasePasswordArgs(a RotateDatabasePasswordArgs) error {
	return ValidateEnsureSiteDatabaseArgs(EnsureSiteDatabaseArgs{
		Domain: a.Domain, Engine: a.Engine, ID: a.ID, StateDir: a.StateDir,
	})
}

// ValidateEnsureHTTPBackendArgs validates EnsureHTTPBackend arguments.
func ValidateEnsureHTTPBackendArgs(a EnsureHTTPBackendArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	if a.Port < config.BackendPortMin || a.Port > config.BackendPortMax {
		return fmt.Errorf("port %d out of range %d-%d", a.Port, config.BackendPortMin, config.BackendPortMax)
	}
	prefix := strings.TrimSpace(a.Prefix)
	if prefix == "" {
		prefix = "/api"
	}
	if err := config.ValidateBackendPath(prefix); err != nil {
		return fmt.Errorf("prefix: %w", err)
	}
	for i, p := range a.Paths {
		if err := config.ValidateBackendPath(p); err != nil {
			return fmt.Errorf("paths[%d]: %w", i, err)
		}
	}
	execPath := strings.TrimSpace(a.Exec)
	if execPath == "" {
		execPath = "./bin/server"
	}
	if strings.Contains(execPath, "..") {
		return fmt.Errorf("invalid exec path")
	}
	return nil
}

// ValidateRemoveHTTPBackendArgs validates RemoveHTTPBackend arguments.
func ValidateRemoveHTTPBackendArgs(a RemoveHTTPBackendArgs) error {
	return ValidateDomain(a.Domain)
}

// ValidateDeployHTTPBackendArgs validates DeployHTTPBackend arguments.
func ValidateDeployHTTPBackendArgs(a DeployHTTPBackendArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	return ValidateDeployAppBinaryArgs(DeployAppBinaryArgs{Domain: a.Domain, BinaryPath: a.BinaryPath})
}

// ValidateHTTPBackendStatusArgs validates HTTPBackendStatus arguments.
func ValidateHTTPBackendStatusArgs(a HTTPBackendStatusArgs) error {
	return ValidateDomain(a.Domain)
}
