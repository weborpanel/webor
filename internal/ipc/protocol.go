// Package ipc defines the typed JSON protocol between webooza and webooza-helper.
package ipc

import "encoding/json"

// DefaultSocketPath is the production Unix socket for helper IPC.
const DefaultSocketPath = "/run/webooza/helper.sock"

// Operation names for the Milestone 0 allowlist surface.
const (
	OpCreateSiteUser   = "CreateSiteUser"
	OpWriteUnitFile    = "WriteUnitFile"
	OpReloadCaddy      = "ReloadCaddy"
	OpRequestApproval  = "RequestApproval"
	OpApprove          = "Approve" // console / root only
	OpDangerousDemo    = "DangerousDemo"
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
