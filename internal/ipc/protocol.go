// Package ipc defines the typed JSON protocol between webor and webor-helper.
package ipc

import "encoding/json"

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
	OpDeployAppGit    = "DeployAppGit"
	OpRunBuild        = "RunBuild"
	OpWriteUnitFile   = "WriteUnitFile"
	OpReloadCaddy     = "ReloadCaddy"
	OpRunCronNow      = "RunCronNow"
	OpCronStatus      = "CronStatus"
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

// DeployAppGitArgs clones/fetches, builds, and deploys an app artifact.
type DeployAppGitArgs struct {
	Domain       string `json:"domain"`
	GitURL       string `json:"git_url"`
	GitRef       string `json:"git_ref,omitempty"`
	BuildCommand string `json:"build_command,omitempty"`
	Artifact     string `json:"artifact,omitempty"` // relative to src/, default bin/server
	MemoryMax    string `json:"memory_max,omitempty"`
	TimeoutSec   int    `json:"timeout_sec,omitempty"`
}

// RunBuildArgs runs a build command as the site user with resource limits.
type RunBuildArgs struct {
	Domain     string `json:"domain"`
	Command    string `json:"command"`
	MemoryMax  string `json:"memory_max,omitempty"`
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

// JournalTailArgs tails a site unit journal (app|backend|cron:<n>|job:<n>).
type JournalTailArgs struct {
	Domain   string `json:"domain"`
	Unit     string `json:"unit"` // selector
	Lines    int    `json:"lines,omitempty"`
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
