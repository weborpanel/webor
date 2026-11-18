// Package ipc defines the typed JSON protocol between webooza and webooza-helper.
package ipc

import "encoding/json"

// DefaultSocketPath is the production Unix socket for helper IPC.
const DefaultSocketPath = "/run/webooza/helper.sock"

// Operation names for the helper allowlist surface.
const (
	OpCreateSiteUser  = "CreateSiteUser"
	OpDeleteSiteUser  = "DeleteSiteUser"
	OpWritePublicFile = "WritePublicFile"
	OpDeployStaticZip = "DeployStaticZip"
	OpDeployAppBinary = "DeployAppBinary"
	OpDeployAppGit    = "DeployAppGit"
	OpRunBuild        = "RunBuild"
	OpWriteUnitFile   = "WriteUnitFile"
	OpReloadCaddy     = "ReloadCaddy"
	OpRunCronNow      = "RunCronNow"
	OpCronStatus      = "CronStatus"
	OpRequestApproval = "RequestApproval"
	OpApprove         = "Approve" // console / root only
	OpDangerousDemo   = "DangerousDemo"
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

// DeleteSiteUserArgs removes the per-site Linux user.
// DeleteData=true also removes /srv/sites/<domain> and requires ApprovalID.
type DeleteSiteUserArgs struct {
	Domain     string `json:"domain"`
	DeleteData bool   `json:"delete_data"`
	ApprovalID string `json:"approval_id,omitempty"`
}

// WritePublicFileArgs writes a file under /srv/sites/<domain>/public/.
type WritePublicFileArgs struct {
	Domain  string `json:"domain"`
	Path    string `json:"path"` // relative path under public/, e.g. index.html
	Content string `json:"content"`
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

// DangerousDemoArgs is a Spike C stand-in for a destructive op.
type DangerousDemoArgs struct {
	Domain     string `json:"domain"`
	ApprovalID string `json:"approval_id"`
}
