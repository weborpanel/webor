// Package ipc defines the typed JSON protocol between webor and webor-helper.
package ipc

import (
	"encoding/json"

	"github.com/weborpanel/webor/internal/config"
)

// DefaultSocketPath is the production Unix socket for helper IPC.
const DefaultSocketPath = "/run/webor/helper.sock"

// Operation names for the helper allowlist surface.
const (
	OpCreateSiteUser  = "CreateSiteUser"
	OpDeleteSiteUser  = "DeleteSiteUser"
	OpWritePublicFile       = "WritePublicFile"
	OpListPublicDir         = "ListPublicDir"
	OpMkdirPublic           = "MkdirPublic"
	OpRenamePublic          = "RenamePublic"
	OpDeletePublicPath      = "DeletePublicPath"
	OpReadPublicFile        = "ReadPublicFile"
	OpExtractPublicArchive  = "ExtractPublicArchive"
	OpDeployStaticZip       = "DeployStaticZip"
	OpDeployAppBinary = "DeployAppBinary"
	OpDeployAppZip    = "DeployAppZip"
	OpDeployAppGit    = "DeployAppGit"
	OpRunBuild        = "RunBuild"
	OpWriteUnitFile   = "WriteUnitFile"
	OpReloadCaddy     = "ReloadCaddy"
	OpRunCronNow      = "RunCronNow"
	OpCronStatus      = "CronStatus"
	OpInstallCronJob  = "InstallCronJob"
	OpRemoveCronJob   = "RemoveCronJob"
	OpEnableCronTimer = "EnableCronTimer"
	OpDisableCronTimer = "DisableCronTimer"
	OpDeployCronBinary = "DeployCronBinary"
	OpDeployCronZip    = "DeployCronZip"
	OpListCronDir      = "ListCronDir"
	OpInstallJob       = "InstallJob"
	OpRemoveJob        = "RemoveJob"
	OpJobStatus        = "JobStatus"
	OpStartJob         = "StartJob"
	OpStopJob          = "StopJob"
	OpRestartJob       = "RestartJob"
	OpDeployProcessBinary = "DeployProcessBinary"
	OpDeployProcessZip    = "DeployProcessZip"
	OpListProcessDir      = "ListProcessDir"
	OpJournalTail     = "JournalTail"
	OpRequestApproval = "RequestApproval"
	OpApprove         = "Approve" // console / root only
	OpListApprovals   = "ListApprovals"
	OpGetApproval     = "GetApproval" // panel may poll status by id
	OpDangerousDemo   = "DangerousDemo"
	OpInstallPostgres        = "InstallPostgres"
	OpInstallMariaDB         = "InstallMariaDB"
	OpInstallValkey          = "InstallValkey"
	OpEnsureSiteDatabase     = "EnsureSiteDatabase"
	OpDropSiteDatabase       = "DropSiteDatabase"
	OpRotateDatabasePassword = "RotateDatabasePassword"
	OpEnsureSiteCache        = "EnsureSiteCache"
	OpDropSiteCache          = "DropSiteCache"
	OpRotateCachePassword    = "RotateCachePassword"
	OpPostgresStatus         = "PostgresStatus"
	OpMariaDBStatus          = "MariaDBStatus"
	OpValkeyStatus           = "ValkeyStatus"
	OpEnsureHTTPBackend      = "EnsureHTTPBackend"
	OpRemoveHTTPBackend      = "RemoveHTTPBackend"
	OpDeployHTTPBackend      = "DeployHTTPBackend"
	OpDeployHTTPBackendZip   = "DeployHTTPBackendZip"
	OpHTTPBackendStatus      = "HTTPBackendStatus"
	OpRestartHTTPBackend     = "RestartHTTPBackend"
	OpListAPIDir             = "ListAPIDir"
	OpBackupConfigure        = "BackupConfigure"
	OpBackupRun              = "BackupRun"
	OpBackupList             = "BackupList"
	OpBackupForget           = "BackupForget"
	OpBackupRestore          = "BackupRestore"
	OpBackupStatus           = "BackupStatus"
	OpEnsureBackupSchedule   = "EnsureBackupSchedule"
	OpEnsureNodeRuntime      = "EnsureNodeRuntime"
	OpApplySiteLimits        = "ApplySiteLimits"
	OpCheckUpdate            = "CheckUpdate"
	OpApplyUpdate            = "ApplyUpdate" // approval-gated when called from panel path
)

// Request is a single typed helper operation. Arguments are never shell strings.
type Request struct {
	ID        string          `json:"id"`
	Operation string          `json:"operation"`
	Args      json.RawMessage `json:"args"`
}

// Response is the helper reply for one request.
type Response struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// CreateSiteUserArgs creates the per-site Linux user and directories.
type CreateSiteUserArgs struct {
	Domain string `json:"domain"`
}

// DropDatabaseRef names one parent-owned database to drop during a site purge.
type DropDatabaseRef struct {
	ID     string `json:"id"`
	Engine string `json:"engine"`
}

// DeleteSiteUserArgs removes the per-site Linux user.
// DeleteData=true also removes /srv/sites/<domain> and requires ApprovalID.
// CascadeDomains and DropDatabases are applied in the same approved purge
// (subdomain site trees + parent databases).
type DeleteSiteUserArgs struct {
	Domain          string            `json:"domain"`
	DeleteData      bool              `json:"delete_data"`
	ApprovalID      string            `json:"approval_id,omitempty"`
	CascadeDomains  []string          `json:"cascade_domains,omitempty"`
	DropDatabases   []DropDatabaseRef `json:"drop_databases,omitempty"`
	StateDir        string            `json:"state_dir,omitempty"`
}

// WritePublicFileArgs writes a file under /srv/sites/<domain>/public/.
type WritePublicFileArgs struct {
	Domain  string `json:"domain"`
	Path    string `json:"path"` // relative path under public/, e.g. index.html
	Content string `json:"content"`
}

// ListPublicDirArgs lists one directory under public/ (Path empty = root).
type ListPublicDirArgs struct {
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// MkdirPublicArgs creates a directory under public/.
type MkdirPublicArgs struct {
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// RenamePublicArgs renames within public/.
type RenamePublicArgs struct {
	Domain string `json:"domain"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// DeletePublicPathArgs deletes a file or directory under public/.
type DeletePublicPathArgs struct {
	Domain    string `json:"domain"`
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

// ReadPublicFileArgs reads a file under public/ (base64 in response Message JSON).
type ReadPublicFileArgs struct {
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// ExtractPublicArchiveArgs extracts a staged archive into dest under public/.
type ExtractPublicArchiveArgs struct {
	Domain      string `json:"domain"`
	ArchivePath string `json:"archive_path"`
	Dest        string `json:"dest"`   // relative under public/, empty = root
	Format      string `json:"format"` // zip|tar|tar.gz; empty = detect
}

// DeployStaticZipArgs extracts a zip into the site's public/ directory.
type DeployStaticZipArgs struct {
	Domain  string `json:"domain"`
	ZipPath string `json:"zip_path"`
}

// DeployAppBinaryArgs installs a binary into a new app release and activates it.
type DeployAppBinaryArgs struct {
	Domain     string `json:"domain"`
	BinaryPath string `json:"binary_path"`
}

// DeployAppZipArgs extracts a zip into the app site root and restarts the unit.
type DeployAppZipArgs struct {
	Domain  string `json:"domain"`
	ZipPath string `json:"zip_path"`
}

// DeployAppGitArgs clones/fetches, builds, and deploys an app artifact.
type DeployAppGitArgs struct {
	Domain       string `json:"domain"`
	GitURL       string `json:"git_url"`
	GitRef       string `json:"git_ref,omitempty"`
	BuildCommand string `json:"build_command,omitempty"`
	Artifact     string `json:"artifact,omitempty"` // relative to src/, default bin/server
	MemoryMax    string `json:"memory_max,omitempty"`
	CPUQuota     string `json:"cpu_quota,omitempty"`
	TimeoutSec   int    `json:"timeout_sec,omitempty"`
}

// RunBuildArgs runs a build command as the site user with resource limits.
type RunBuildArgs struct {
	Domain     string `json:"domain"`
	Command    string `json:"command"`
	MemoryMax  string `json:"memory_max,omitempty"`
	CPUQuota   string `json:"cpu_quota,omitempty"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

// WriteUnitFileArgs writes a systemd unit under the site's scope.
type WriteUnitFileArgs struct {
	Domain  string `json:"domain"`
	Unit    string `json:"unit"`    // basename, e.g. site.service
	Content string `json:"content"` // unit file body
}

// ReloadCaddyArgs asks the helper to validate and reload Caddy.
type ReloadCaddyArgs struct {
	ConfigPath string `json:"config_path"`
}

// RunCronNowArgs starts a site cron oneshot service immediately.
type RunCronNowArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// CronStatusArgs fetches timer/service status for a site cron job.
type CronStatusArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// InstallCronJobArgs installs a cron oneshot (+ optional timer).
type InstallCronJobArgs struct {
	Domain   string              `json:"domain"`
	Job      string              `json:"job"`
	Schedule string              `json:"schedule,omitempty"`
	Exec     string              `json:"exec"`
	Runtime  *config.SiteRuntime `json:"runtime,omitempty"`
	Limits   config.Limits       `json:"limits,omitempty"`
}

// RemoveCronJobArgs removes cron service + timer units.
type RemoveCronJobArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// EnableCronTimerArgs enables the timer for a cron job (schedule required).
type EnableCronTimerArgs struct {
	Domain   string              `json:"domain"`
	Job      string              `json:"job"`
	Schedule string              `json:"schedule"`
	Exec     string              `json:"exec"`
	Runtime  *config.SiteRuntime `json:"runtime,omitempty"`
	Limits   config.Limits       `json:"limits,omitempty"`
}

// DisableCronTimerArgs disables the cron timer only.
type DisableCronTimerArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// DeployCronBinaryArgs installs a binary into crons/<job>/server.
type DeployCronBinaryArgs struct {
	Domain     string `json:"domain"`
	Job        string `json:"job"`
	BinaryPath string `json:"binary_path"`
}

// DeployCronZipArgs extracts a zip into crons/<job>/.
type DeployCronZipArgs struct {
	Domain  string `json:"domain"`
	Job     string `json:"job"`
	ZipPath string `json:"zip_path"`
}

// ListCronDirArgs lists under crons/<job>/.
type ListCronDirArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
	Path   string `json:"path,omitempty"`
}

// InstallJobArgs installs a long-running process unit.
type InstallJobArgs struct {
	Domain  string              `json:"domain"`
	Job     string              `json:"job"`
	Exec    string              `json:"exec"`
	Runtime *config.SiteRuntime `json:"runtime,omitempty"`
	Start   bool                `json:"start,omitempty"`
	Limits  config.Limits       `json:"limits,omitempty"`
}

// RemoveJobArgs removes a long-running process unit.
type RemoveJobArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// JobStatusArgs / StartJobArgs / StopJobArgs / RestartJobArgs share domain+job.
type JobStatusArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// StartJobArgs starts a process unit.
type StartJobArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// StopJobArgs stops a process unit.
type StopJobArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// RestartJobArgs restarts a process unit.
type RestartJobArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
}

// DeployProcessBinaryArgs installs a binary into processes/<job>/server.
type DeployProcessBinaryArgs struct {
	Domain     string `json:"domain"`
	Job        string `json:"job"`
	BinaryPath string `json:"binary_path"`
}

// DeployProcessZipArgs extracts a zip into processes/<job>/.
type DeployProcessZipArgs struct {
	Domain  string `json:"domain"`
	Job     string `json:"job"`
	ZipPath string `json:"zip_path"`
}

// ListProcessDirArgs lists under processes/<job>/.
type ListProcessDirArgs struct {
	Domain string `json:"domain"`
	Job    string `json:"job"`
	Path   string `json:"path,omitempty"`
}

// JournalTailArgs tails a site unit journal (app|backend|apis|api:<id>|cron:<n>|job:<n>).
type JournalTailArgs struct {
	Domain string   `json:"domain"`
	Unit   string   `json:"unit"` // selector
	Lines  int      `json:"lines,omitempty"`
	APIIds []string `json:"api_ids,omitempty"` // required when Unit is "apis"
}

// RequestApprovalArgs creates a pending step-up approval bound to an operation.
type RequestApprovalArgs struct {
	Operation string          `json:"operation"`
	Args      json.RawMessage `json:"args"`
}

// ApproveArgs marks a pending approval as console-approved (root peercred).
type ApproveArgs struct {
	ID         string `json:"id"`
	ApprovedBy string `json:"approved_by,omitempty"`
}

// ListApprovalsArgs is empty; pending approvals are returned in Message as JSON.
type ListApprovalsArgs struct{}

// GetApprovalArgs loads one approval by id (panel poll / CLI preview).
type GetApprovalArgs struct {
	ID string `json:"id"`
}

// DangerousDemoArgs is a Spike C stand-in for a destructive op.
type DangerousDemoArgs struct {
	Domain     string `json:"domain"`
	ApprovalID string `json:"approval_id"`
}

// InstallPostgresArgs installs the Postgres packages (approval required).
type InstallPostgresArgs struct {
	ApprovalID string `json:"approval_id"`
}

// InstallMariaDBArgs installs MariaDB packages (approval required).
type InstallMariaDBArgs struct {
	ApprovalID string `json:"approval_id"`
}

// InstallValkeyArgs installs Valkey/Redis packages (approval required).
type InstallValkeyArgs struct {
	ApprovalID string `json:"approval_id"`
}

// EnsureSiteDatabaseArgs creates/reconciles a per-site database.
// ApprovalID is required for postgres/mariadb. Blank ID becomes slug(domain).
type EnsureSiteDatabaseArgs struct {
	Domain     string `json:"domain"`
	Engine     string `json:"engine"`
	ID         string `json:"id,omitempty"`
	StateDir   string `json:"state_dir,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"`
}

// DropSiteDatabaseArgs removes a per-site database.
// ApprovalID is required for postgres/mariadb.
type DropSiteDatabaseArgs struct {
	Domain     string `json:"domain"`
	Engine     string `json:"engine"`
	ID         string `json:"id,omitempty"`
	StateDir   string `json:"state_dir,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"`
}

// RotateDatabasePasswordArgs regenerates the password for a site database.
type RotateDatabasePasswordArgs struct {
	Domain   string `json:"domain"`
	Engine   string `json:"engine"`
	ID       string `json:"id,omitempty"`
	StateDir string `json:"state_dir,omitempty"`
}

// PostgresStatusArgs is empty; status is returned in Message as JSON.
type PostgresStatusArgs struct{}

// MariaDBStatusArgs is empty; status is returned in Message as JSON.
type MariaDBStatusArgs struct{}

// ValkeyStatusArgs is empty; status is returned in Message as JSON.
type ValkeyStatusArgs struct{}

// EnsureSiteCacheArgs creates a per-site Valkey ACL user (approval if install needed separately).
type EnsureSiteCacheArgs struct {
	Domain     string `json:"domain"`
	Engine     string `json:"engine,omitempty"`
	StateDir   string `json:"state_dir,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"` // unused for ensure; install uses InstallValkey
}

// DropSiteCacheArgs removes a per-site Valkey ACL user.
type DropSiteCacheArgs struct {
	Domain   string `json:"domain"`
	StateDir string `json:"state_dir,omitempty"`
}

// RotateCachePasswordArgs regenerates the Valkey ACL password.
type RotateCachePasswordArgs struct {
	Domain   string `json:"domain"`
	StateDir string `json:"state_dir,omitempty"`
}

// EnsureHTTPBackendArgs installs/enables the loopback HTTP backend unit for a static site.
type EnsureHTTPBackendArgs struct {
	Domain    string              `json:"domain"`
	ID        string              `json:"id,omitempty"`
	Prefix    string              `json:"prefix,omitempty"`
	Paths     []string            `json:"paths,omitempty"`
	WebSocket bool                `json:"websocket,omitempty"`
	Exec      string              `json:"exec,omitempty"`
	Port      int                 `json:"port"`
	Runtime   *config.SiteRuntime `json:"runtime,omitempty"`
	Limits    config.Limits       `json:"limits,omitempty"`
}

// RemoveHTTPBackendArgs stops and removes the HTTP backend unit.
type RemoveHTTPBackendArgs struct {
	Domain string `json:"domain"`
	ID     string `json:"id,omitempty"`
}

// DeployHTTPBackendArgs deploys a binary to apis/<id>/server and restarts it.
type DeployHTTPBackendArgs struct {
	Domain     string `json:"domain"`
	ID         string `json:"id,omitempty"`
	BinaryPath string `json:"binary_path"`
}

// DeployHTTPBackendZipArgs extracts a zip into apis/<id>/ and restarts the backend.
type DeployHTTPBackendZipArgs struct {
	Domain  string `json:"domain"`
	ID      string `json:"id,omitempty"`
	ZipPath string `json:"zip_path"`
}

// HTTPBackendStatusArgs reports whether the backend unit is active.
type HTTPBackendStatusArgs struct {
	Domain string `json:"domain"`
	ID     string `json:"id,omitempty"`
}

// RestartHTTPBackendArgs restarts one HTTP backend unit.
type RestartHTTPBackendArgs struct {
	Domain string `json:"domain"`
	ID     string `json:"id,omitempty"`
}

// ListAPIDirArgs lists one directory under apis/<id>/.
type ListAPIDirArgs struct {
	Domain string `json:"domain"`
	ID     string `json:"id"`
	Path   string `json:"path,omitempty"`
}

// BackupConfigureArgs writes backup settings + credentials (approval-gated).
type BackupConfigureArgs struct {
	ConfigPath     string `json:"config_path,omitempty"`
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region,omitempty"`
	Bucket         string `json:"bucket"`
	Prefix         string `json:"prefix,omitempty"`
	Schedule       string `json:"schedule,omitempty"`
	KeepDaily      int    `json:"keep_daily,omitempty"`
	KeepWeekly     int    `json:"keep_weekly,omitempty"`
	Repository     string `json:"repository,omitempty"` // lab override
	AccessKey      string `json:"access_key,omitempty"`
	SecretKey      string `json:"secret_key,omitempty"`
	ResticPassword string `json:"restic_password,omitempty"`
	ApprovalID     string `json:"approval_id"`
}

// BackupRunArgs kicks a full backup.
type BackupRunArgs struct {
	ConfigPath string `json:"config_path,omitempty"`
}

// BackupListArgs lists snapshots.
type BackupListArgs struct {
	ConfigPath string `json:"config_path,omitempty"`
}

// BackupForgetArgs runs retention prune (approval-gated).
type BackupForgetArgs struct {
	ConfigPath string `json:"config_path,omitempty"`
	ApprovalID string `json:"approval_id"`
}

// BackupRestoreArgs selectively restores sites from a snapshot (approval-gated).
type BackupRestoreArgs struct {
	ConfigPath   string   `json:"config_path,omitempty"`
	StateDir     string   `json:"state_dir,omitempty"`
	SnapshotID   string   `json:"snapshot_id"`
	Sites        []string `json:"sites"`
	IncludePanel bool     `json:"include_panel,omitempty"`
	ApprovalID   string   `json:"approval_id"`
}

// BackupStatusArgs reports restic/credentials readiness.
type BackupStatusArgs struct {
	ConfigPath string `json:"config_path,omitempty"`
}

// EnsureBackupScheduleArgs syncs webor-backup.timer from config.
type EnsureBackupScheduleArgs struct {
	ConfigPath string `json:"config_path,omitempty"`
}

// EnsureNodeRuntimeArgs downloads/caches an official Node build.
type EnsureNodeRuntimeArgs struct {
	Version string `json:"version"`
}

// ApplySiteLimitsArgs rewrites site units after limits change in config.yaml.
type ApplySiteLimitsArgs struct {
	Site config.Site `json:"site"`
}

// CheckUpdateArgs fetches the channel VERSION and compares to the running build.
type CheckUpdateArgs struct {
	Channel string `json:"channel,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

// ApplyUpdateArgs installs a release tarball (approval-gated for non-root panel).
type ApplyUpdateArgs struct {
	Channel         string `json:"channel"`
	BaseURL         string `json:"base_url,omitempty"`
	Version         string `json:"version"` // expected VERSION from check (bound)
	Force           bool   `json:"force,omitempty"`
	SaveChannel     bool   `json:"save_channel,omitempty"`
	ApprovalID      string `json:"approval_id,omitempty"`
}
