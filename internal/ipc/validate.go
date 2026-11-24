package ipc

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/panelenv"
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
	if a.CPUQuota != "" {
		if err := config.ValidateCPUQuota(a.CPUQuota); err != nil {
			return err
		}
	}
	return nil
}

func validateMemoryMax(v string) error {
	return config.ValidateSizeString(v)
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

// ValidateDeployAppZipArgs validates DeployAppZip arguments.
func ValidateDeployAppZipArgs(a DeployAppZipArgs) error {
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
	domain := strings.ToLower(strings.TrimSpace(a.Domain))
	allowedPrefixes := []string{
		filepath.Join(sitesRoot, domain, "tmp") + string(filepath.Separator),
		filepath.Join("/var/lib/webor/uploads", domain) + string(filepath.Separator),
		filepath.Join(sitesRoot, domain) + string(filepath.Separator),
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(p, prefix) {
			return nil
		}
	}
	return fmt.Errorf("zip_path %q not under allowed directories", p)
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
	if a.CPUQuota != "" {
		if err := config.ValidateCPUQuota(a.CPUQuota); err != nil {
			return err
		}
	}
	return nil
}

// ValidateApplySiteLimitsArgs validates ApplySiteLimits arguments.
func ValidateApplySiteLimitsArgs(a ApplySiteLimitsArgs) error {
	if err := ValidateDomain(a.Site.Name); err != nil {
		return err
	}
	switch a.Site.Type {
	case config.SiteTypeStatic, config.SiteTypeApp, config.SiteTypeContainer:
	default:
		return fmt.Errorf("invalid site type %q", a.Site.Type)
	}
	return config.ValidateLimits(a.Site.Limits)
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

func validateTaskJob(domain, job string) error {
	if err := ValidateDomain(domain); err != nil {
		return err
	}
	if err := config.ValidateTaskName(job); err != nil {
		return fmt.Errorf("job: %w", err)
	}
	return nil
}

// ValidateInstallCronJobArgs validates InstallCronJob.
func ValidateInstallCronJobArgs(a InstallCronJobArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.Exec) == "" {
		return fmt.Errorf("exec is required")
	}
	if strings.Contains(a.Exec, "\n") || strings.Contains(a.Exec, "\r") {
		return fmt.Errorf("invalid exec")
	}
	if s := strings.TrimSpace(a.Schedule); s != "" {
		fields := strings.Fields(s)
		if len(fields) != 5 {
			return fmt.Errorf("schedule must have 5 cron fields")
		}
	}
	return nil
}

// ValidateRemoveCronJobArgs validates RemoveCronJob.
func ValidateRemoveCronJobArgs(a RemoveCronJobArgs) error {
	return validateTaskJob(a.Domain, a.Job)
}

// ValidateEnableCronTimerArgs validates EnableCronTimer.
func ValidateEnableCronTimerArgs(a EnableCronTimerArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.Schedule) == "" {
		return fmt.Errorf("schedule is required")
	}
	if strings.TrimSpace(a.Exec) == "" {
		return fmt.Errorf("exec is required")
	}
	fields := strings.Fields(strings.TrimSpace(a.Schedule))
	if len(fields) != 5 {
		return fmt.Errorf("schedule must have 5 cron fields")
	}
	return nil
}

// ValidateDisableCronTimerArgs validates DisableCronTimer.
func ValidateDisableCronTimerArgs(a DisableCronTimerArgs) error {
	return validateTaskJob(a.Domain, a.Job)
}

// ValidateDeployCronBinaryArgs validates DeployCronBinary.
func ValidateDeployCronBinaryArgs(a DeployCronBinaryArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.BinaryPath) == "" {
		return fmt.Errorf("binary_path is required")
	}
	return nil
}

// ValidateDeployCronZipArgs validates DeployCronZip.
func ValidateDeployCronZipArgs(a DeployCronZipArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.ZipPath) == "" {
		return fmt.Errorf("zip_path is required")
	}
	return nil
}

// ValidateListCronDirArgs validates ListCronDir.
func ValidateListCronDirArgs(a ListCronDirArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.Contains(a.Path, "..") {
		return fmt.Errorf("path must not contain ..")
	}
	return nil
}

// ValidateInstallJobArgs validates InstallJob.
func ValidateInstallJobArgs(a InstallJobArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.Exec) == "" {
		return fmt.Errorf("exec is required")
	}
	if strings.Contains(a.Exec, "\n") || strings.Contains(a.Exec, "\r") {
		return fmt.Errorf("invalid exec")
	}
	return nil
}

// ValidateRemoveJobArgs validates RemoveJob.
func ValidateRemoveJobArgs(a RemoveJobArgs) error {
	return validateTaskJob(a.Domain, a.Job)
}

// ValidateJobStatusArgs validates JobStatus / Start / Stop / Restart.
func ValidateJobStatusArgs(a JobStatusArgs) error {
	return validateTaskJob(a.Domain, a.Job)
}

// ValidateDeployProcessBinaryArgs validates DeployProcessBinary.
func ValidateDeployProcessBinaryArgs(a DeployProcessBinaryArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.BinaryPath) == "" {
		return fmt.Errorf("binary_path is required")
	}
	return nil
}

// ValidateDeployProcessZipArgs validates DeployProcessZip.
func ValidateDeployProcessZipArgs(a DeployProcessZipArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.TrimSpace(a.ZipPath) == "" {
		return fmt.Errorf("zip_path is required")
	}
	return nil
}

// ValidateListProcessDirArgs validates ListProcessDir.
func ValidateListProcessDirArgs(a ListProcessDirArgs) error {
	if err := validateTaskJob(a.Domain, a.Job); err != nil {
		return err
	}
	if strings.Contains(a.Path, "..") {
		return fmt.Errorf("path must not contain ..")
	}
	return nil
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
	case sel == "apis":
		if len(a.APIIds) == 0 {
			return fmt.Errorf("api_ids required when unit is apis")
		}
		for _, id := range a.APIIds {
			if err := config.ValidateBackendID(id); err != nil {
				return fmt.Errorf("invalid api_ids entry")
			}
		}
	case strings.HasPrefix(sel, "api:"):
		id := strings.TrimSpace(sel[strings.IndexByte(sel, ':')+1:])
		if err := config.ValidateBackendID(id); err != nil {
			return fmt.Errorf("invalid unit selector")
		}
	case strings.HasPrefix(sel, "cron:"), strings.HasPrefix(sel, "job:"):
		name := strings.TrimSpace(sel[strings.IndexByte(sel, ':')+1:])
		if name == "" || len(name) > 64 || strings.ContainsAny(name, "/\\\n\r") {
			return fmt.Errorf("invalid unit selector")
		}
	default:
		return fmt.Errorf("unit must be app|backend|apis|api:<id>|cron:<name>|job:<name>")
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
	id := strings.ToLower(strings.TrimSpace(a.ID))
	if id == "" {
		id = "api"
	}
	if err := config.ValidateBackendID(id); err != nil {
		return fmt.Errorf("id: %w", err)
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
		execPath = "./server"
	}
	if strings.Contains(execPath, "..") {
		return fmt.Errorf("invalid exec path")
	}
	if err := config.ValidateBackendRuntime(a.Runtime); err != nil {
		return err
	}
	return nil
}

// ValidateRemoveHTTPBackendArgs validates RemoveHTTPBackend arguments.
func ValidateRemoveHTTPBackendArgs(a RemoveHTTPBackendArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(a.ID))
	if id == "" {
		id = "api"
	}
	return config.ValidateBackendID(id)
}

// ValidateDeployHTTPBackendArgs validates DeployHTTPBackend arguments.
func ValidateDeployHTTPBackendArgs(a DeployHTTPBackendArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(a.ID))
	if id == "" {
		id = "api"
	}
	if err := config.ValidateBackendID(id); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	return ValidateDeployAppBinaryArgs(DeployAppBinaryArgs{Domain: a.Domain, BinaryPath: a.BinaryPath})
}

// ValidateDeployHTTPBackendZipArgs validates DeployHTTPBackendZip arguments.
func ValidateDeployHTTPBackendZipArgs(a DeployHTTPBackendZipArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(a.ID))
	if id == "" {
		id = "api"
	}
	if err := config.ValidateBackendID(id); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	p := strings.TrimSpace(a.ZipPath)
	if p == "" {
		return fmt.Errorf("zip_path is required")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("zip_path must be absolute")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("invalid zip_path")
	}
	return nil
}

// ValidateHTTPBackendStatusArgs validates HTTPBackendStatus arguments.
func ValidateHTTPBackendStatusArgs(a HTTPBackendStatusArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(a.ID))
	if id == "" {
		id = "api"
	}
	return config.ValidateBackendID(id)
}

// ValidateListAPIDirArgs validates ListAPIDir arguments.
func ValidateListAPIDirArgs(a ListAPIDirArgs) error {
	if err := ValidateDomain(a.Domain); err != nil {
		return err
	}
	if err := config.ValidateBackendID(a.ID); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	if strings.Contains(a.Path, "..") {
		return fmt.Errorf("path must not contain ..")
	}
	return nil
}

func defaultBackupConfigPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/etc/webor/config.yaml"
	}
	if !strings.HasPrefix(p, "/etc/webor/") && p != "/etc/webor/config.yaml" {
		return ""
	}
	return p
}

// ValidateBackupConfigureArgs validates BackupConfigure.
func ValidateBackupConfigureArgs(a BackupConfigureArgs) error {
	if strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required")
	}
	if defaultBackupConfigPath(a.ConfigPath) == "" {
		return fmt.Errorf("invalid config_path")
	}
	if strings.TrimSpace(a.Repository) != "" {
		return nil
	}
	if strings.TrimSpace(a.Bucket) == "" {
		return fmt.Errorf("bucket is required")
	}
	if strings.TrimSpace(a.Endpoint) == "" {
		return fmt.Errorf("endpoint is required")
	}
	return nil
}

// ValidateBackupRunArgs validates BackupRun.
func ValidateBackupRunArgs(a BackupRunArgs) error {
	if defaultBackupConfigPath(a.ConfigPath) == "" {
		return fmt.Errorf("invalid config_path")
	}
	return nil
}

// ValidateBackupListArgs validates BackupList.
func ValidateBackupListArgs(a BackupListArgs) error {
	return ValidateBackupRunArgs(BackupRunArgs{ConfigPath: a.ConfigPath})
}

// ValidateBackupForgetArgs validates BackupForget.
func ValidateBackupForgetArgs(a BackupForgetArgs) error {
	if strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required")
	}
	return ValidateBackupRunArgs(BackupRunArgs{ConfigPath: a.ConfigPath})
}

// ValidateBackupRestoreArgs validates BackupRestore.
func ValidateBackupRestoreArgs(a BackupRestoreArgs) error {
	if strings.TrimSpace(a.ApprovalID) == "" {
		return fmt.Errorf("approval_id is required")
	}
	if strings.TrimSpace(a.SnapshotID) == "" {
		return fmt.Errorf("snapshot_id is required")
	}
	if len(a.Sites) == 0 && !a.IncludePanel {
		return fmt.Errorf("select at least one site or include_panel")
	}
	for _, s := range a.Sites {
		if err := ValidateDomain(s); err != nil {
			return fmt.Errorf("sites: %w", err)
		}
	}
	if defaultBackupConfigPath(a.ConfigPath) == "" {
		return fmt.Errorf("invalid config_path")
	}
	sd := strings.TrimSpace(a.StateDir)
	if sd != "" && sd != "/var/lib/webor" && !strings.HasPrefix(sd, "/var/lib/webor/") {
		return fmt.Errorf("invalid state_dir")
	}
	return nil
}

// ValidateBackupStatusArgs validates BackupStatus.
func ValidateBackupStatusArgs(a BackupStatusArgs) error {
	return ValidateBackupRunArgs(BackupRunArgs{ConfigPath: a.ConfigPath})
}

// ValidateEnsureBackupScheduleArgs validates EnsureBackupSchedule.
func ValidateEnsureBackupScheduleArgs(a EnsureBackupScheduleArgs) error {
	return ValidateBackupRunArgs(BackupRunArgs{ConfigPath: a.ConfigPath})
}

// ValidateEnsureNodeRuntimeArgs validates EnsureNodeRuntime.
func ValidateEnsureNodeRuntimeArgs(a EnsureNodeRuntimeArgs) error {
	v := strings.TrimSpace(strings.TrimPrefix(a.Version, "v"))
	if v == "" {
		return fmt.Errorf("version is required")
	}
	if err := config.ValidateSiteRuntime(config.SiteTypeApp, &config.SiteRuntime{Kind: config.RuntimeKindNode, Version: v}); err != nil {
		return err
	}
	return nil
}

// ValidateCheckUpdateArgs validates CheckUpdate.
func ValidateCheckUpdateArgs(a CheckUpdateArgs) error {
	ch := strings.TrimSpace(a.Channel)
	if ch == "" {
		return nil
	}
	switch strings.ToLower(ch) {
	case "stable", "beta":
		return nil
	default:
		return fmt.Errorf("unsupported channel %q", ch)
	}
}

// ValidateApplyUpdateArgs validates ApplyUpdate.
func ValidateApplyUpdateArgs(a ApplyUpdateArgs) error {
	ch := strings.ToLower(strings.TrimSpace(a.Channel))
	switch ch {
	case "stable", "beta":
	default:
		return fmt.Errorf("unsupported channel %q", a.Channel)
	}
	if strings.TrimSpace(strings.TrimPrefix(a.Version, "v")) == "" {
		return fmt.Errorf("version is required")
	}
	return nil
}

// ValidateSetPanelListenHostArgs validates SetPanelListenHost.
func ValidateSetPanelListenHostArgs(a SetPanelListenHostArgs) error {
	if err := panelenv.ValidateListenHost(a.Host); err != nil {
		return err
	}
	envPath := strings.TrimSpace(a.EnvPath)
	if envPath == "" {
		return nil
	}
	if !filepath.IsAbs(envPath) {
		return fmt.Errorf("env_path must be absolute")
	}
	// Only allow the production path or temp paths under /tmp for tests.
	if envPath != panelenv.DefaultEnvPath && !strings.HasPrefix(envPath, "/tmp/") {
		return fmt.Errorf("env_path not allowed")
	}
	return nil
}
