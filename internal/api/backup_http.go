package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/reconcile"
)

// MountBackup registers /api/backup* routes.
func (s *Server) MountBackup(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/backup", s.handleBackupGet)
	mux.HandleFunc("PUT /api/backup", s.handleBackupPut)
	mux.HandleFunc("POST /api/backup/run", s.handleBackupRun)
	mux.HandleFunc("GET /api/backup/snapshots", s.handleBackupSnapshots)
	mux.HandleFunc("POST /api/backup/restore", s.handleBackupRestore)
	mux.HandleFunc("POST /api/backup/forget", s.handleBackupForget)
}

func (s *Server) handleBackupGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	status, _ := s.backupStatus()
	view := map[string]any{
		"ok":               true,
		"enabled":          false,
		"configured":       status != nil && status.Configured,
		"restic_installed": status != nil && status.ResticInstalled,
		"hint":             "",
		"repository":       "",
		"schedule":         "",
		"s3":               map[string]any{},
		"retention":        map[string]any{"keep_daily": 7, "keep_weekly": 4},
	}
	if status != nil {
		view["hint"] = status.Hint
		view["repository"] = status.Repository
	}
	if cfg.Backup != nil {
		b := cfg.Backup
		config.NormalizeBackup(b)
		view["enabled"] = b.Enabled
		view["schedule"] = b.Schedule
		view["s3"] = map[string]any{
			"endpoint": b.S3.Endpoint,
			"region":   b.S3.Region,
			"bucket":   b.S3.Bucket,
			"prefix":   b.S3.Prefix,
		}
		view["retention"] = map[string]any{
			"keep_daily":  b.Retention.KeepDaily,
			"keep_weekly": b.Retention.KeepWeekly,
		}
		if b.Enabled {
			view["repository"] = b.ResticRepositoryURL()
		}
	}
	s.writeJSON(w, 200, view)
}

func (s *Server) handleBackupPut(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		Endpoint       string `json:"endpoint"`
		Region         string `json:"region"`
		Bucket         string `json:"bucket"`
		Prefix         string `json:"prefix"`
		Schedule       string `json:"schedule"`
		KeepDaily      int    `json:"keep_daily"`
		KeepWeekly     int    `json:"keep_weekly"`
		AccessKey      string `json:"access_key"`
		SecretKey      string `json:"secret_key"`
		ResticPassword string `json:"restic_password"`
		Repository     string `json:"repository"`
		ApprovalID     string `json:"approval_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		s.writeErr(w, 400, "invalid json")
		return
	}
	args := ipc.BackupConfigureArgs{
		ConfigPath:     s.configPath(),
		Endpoint:       body.Endpoint,
		Region:         body.Region,
		Bucket:         body.Bucket,
		Prefix:         body.Prefix,
		Schedule:       body.Schedule,
		KeepDaily:      body.KeepDaily,
		KeepWeekly:     body.KeepWeekly,
		AccessKey:      body.AccessKey,
		SecretKey:      body.SecretKey,
		ResticPassword: body.ResticPassword,
		Repository:     body.Repository,
		ApprovalID:     body.ApprovalID,
	}
	if args.ApprovalID == "" {
		s.writeJSON(w, 403, map[string]any{
			"ok":                 false,
			"error":              "root approval required to configure backups",
			"approval_operation": ipc.OpBackupConfigure,
			"hint":               "Request approval, then: sudo wbr approve <id>",
		})
		return
	}
	raw, _ := json.Marshal(args)
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpBackupConfigure, Args: raw})
		if err != nil {
			s.writeErr(w, 502, err.Error())
			return
		}
		if !resp.OK {
			s.writeJSON(w, 400, map[string]any{
				"ok": false, "error": resp.Error,
				"approval_operation": ipc.OpBackupConfigure,
			})
			return
		}
		var res helper.BackupConfigureResult
		_ = json.Unmarshal([]byte(resp.Message), &res)
		s.writeJSON(w, 200, map[string]any{"ok": true, "repository": res.Repository, "configured": res.Configured})
		return
	}
	b := config.Backup{
		Enabled:  true,
		Schedule: body.Schedule,
		S3: config.BackupS3{
			Endpoint: body.Endpoint, Region: body.Region, Bucket: body.Bucket, Prefix: body.Prefix,
		},
		Retention:  config.BackupRetention{KeepDaily: body.KeepDaily, KeepWeekly: body.KeepWeekly},
		Repository: body.Repository,
	}
	res, err := s.ops().BackupConfigure(s.configPath(), b, body.AccessKey, body.SecretKey, body.ResticPassword, body.Region)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "repository": res.Repository, "configured": res.Configured})
}

func (s *Server) handleBackupRun(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	raw, _ := json.Marshal(ipc.BackupRunArgs{ConfigPath: s.configPath()})
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpBackupRun, Args: raw})
		if err != nil {
			s.writeErr(w, 502, err.Error())
			return
		}
		if !resp.OK {
			s.writeErr(w, 400, resp.Error)
			return
		}
		var res helper.BackupRunResult
		_ = json.Unmarshal([]byte(resp.Message), &res)
		s.writeJSON(w, 200, map[string]any{"ok": true, "snapshot_id": res.SnapshotID, "sites": res.Sites})
		return
	}
	s.writeErr(w, 503, "helper socket required for backup run")
}

func (s *Server) handleBackupSnapshots(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	raw, _ := json.Marshal(ipc.BackupListArgs{ConfigPath: s.configPath()})
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpBackupList, Args: raw})
		if err != nil {
			s.writeErr(w, 502, err.Error())
			return
		}
		if !resp.OK {
			s.writeErr(w, 400, resp.Error)
			return
		}
		var res helper.BackupListResult
		_ = json.Unmarshal([]byte(resp.Message), &res)
		s.writeJSON(w, 200, map[string]any{"ok": true, "snapshots": res.Snapshots})
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "snapshots": []any{}})
}

func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		SnapshotID   string   `json:"snapshot_id"`
		Sites        []string `json:"sites"`
		IncludePanel bool     `json:"include_panel"`
		ApprovalID   string   `json:"approval_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	if strings.TrimSpace(body.ApprovalID) == "" {
		s.writeJSON(w, 403, map[string]any{
			"ok":                 false,
			"error":              "root approval required to restore a backup",
			"approval_operation": ipc.OpBackupRestore,
			"hint":               "Request approval, then: sudo wbr approve <id>",
		})
		return
	}
	args := ipc.BackupRestoreArgs{
		ConfigPath:   s.configPath(),
		StateDir:     s.stateDir(),
		SnapshotID:   body.SnapshotID,
		Sites:        body.Sites,
		IncludePanel: body.IncludePanel,
		ApprovalID:   body.ApprovalID,
	}
	raw, _ := json.Marshal(args)
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpBackupRestore, Args: raw})
		if err != nil {
			s.writeErr(w, 502, err.Error())
			return
		}
		if !resp.OK {
			s.writeJSON(w, 400, map[string]any{
				"ok": false, "error": resp.Error,
				"approval_operation": ipc.OpBackupRestore,
			})
			return
		}
		// Files/config are on disk; bring live routing + units back to match.
		out := map[string]any{"ok": true}
		if warn := s.reconcileAfterBackupRestore(); warn != "" {
			out["warning"] = warn
		}
		s.writeJSON(w, 200, out)
		return
	}
	s.writeErr(w, 503, "helper socket required for restore")
}

// reconcileAfterBackupRestore reloads saved config into Caddy and re-ensures
// app/backend units so HTTPS listeners and site routes match the snapshot.
func (s *Server) reconcileAfterBackupRestore() string {
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		return "reload config: " + err.Error()
	}

	var warnings []string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	hasApp := false
	for _, site := range cfg.Sites {
		if site.Type == config.SiteTypeApp {
			hasApp = true
			if site.HasNodeRuntime() && site.Runtime != nil && strings.TrimSpace(site.Runtime.Command) == "" {
				if err := s.ensureNodeRuntime(site.Runtime.Version); err != nil {
					warnings = append(warnings, site.Name+": node: "+err.Error())
				}
			}
		}
		for _, b := range site.EnabledBackends() {
			if b.HasNodeRuntime() && b.Runtime != nil && strings.TrimSpace(b.Runtime.Command) == "" {
				if err := s.ensureNodeRuntime(b.Runtime.Version); err != nil {
					warnings = append(warnings, site.Name+"/"+b.ID+": node: "+err.Error())
				}
			}
			if _, err := s.ensureHTTPBackend(site, b); err != nil {
				warnings = append(warnings, site.Name+"/"+b.ID+": backend: "+err.Error())
			}
		}
	}

	caddyOpt := s.caddyApplyOptions(cfg)
	if hasApp {
		if err := reconcile.ApplyAppSites(ctx, cfg, s.ops(), reconcile.AppApplyOptions{
			CaddyConfigPath: caddyOpt.CaddyConfigPath,
			HTTPListen:      caddyOpt.HTTPListen,
			DisableHTTPS:    caddyOpt.DisableHTTPS,
			PanelUpstream:   caddyOpt.PanelUpstream,
		}); err != nil {
			warnings = append(warnings, "app units: "+err.Error())
		}
	}

	if err := s.applyCaddy(cfg); err != nil {
		warnings = append(warnings, "caddy: "+err.Error())
	}
	return strings.Join(warnings, "; ")
}

func (s *Server) handleBackupForget(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		ApprovalID string `json:"approval_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ApprovalID == "" {
		s.writeJSON(w, 403, map[string]any{
			"ok": false, "error": "root approval required",
			"approval_operation": ipc.OpBackupForget,
		})
		return
	}
	raw, _ := json.Marshal(ipc.BackupForgetArgs{ConfigPath: s.configPath(), ApprovalID: body.ApprovalID})
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpBackupForget, Args: raw})
		if err != nil {
			s.writeErr(w, 502, err.Error())
			return
		}
		if !resp.OK {
			s.writeErr(w, 400, resp.Error)
			return
		}
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) backupStatus() (*helper.BackupStatusResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.BackupStatusArgs{ConfigPath: s.configPath()})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpBackupStatus, Args: raw})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return &helper.BackupStatusResult{Hint: resp.Error}, nil
		}
		var res helper.BackupStatusResult
		_ = json.Unmarshal([]byte(resp.Message), &res)
		return &res, nil
	}
	return s.ops().BackupStatus(s.configPath())
}
