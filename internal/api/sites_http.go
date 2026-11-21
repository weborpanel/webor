package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/reconcile"
)

// DefaultConfigPath is the production desired-state file.
const DefaultConfigPath = "/etc/webor/config.yaml"

// sitesMu serializes config.yaml read-modify-write from the panel.
var sitesMu sync.Mutex

func (s *Server) configPath() string {
	if s.ConfigPath != "" {
		return s.ConfigPath
	}
	return DefaultConfigPath
}

func (s *Server) ops() *helper.Ops {
	if s.Ops != nil {
		return s.Ops
	}
	return helper.DefaultOps()
}

// siteFS returns IPC-backed Ops when HelperSocket is set (production panel user).
func (s *Server) siteFS() helper.SiteFS {
	if s.HelperSocket != "" {
		return &ipc.SiteClient{Socket: s.HelperSocket}
	}
	return s.ops()
}

// MountSites registers /api/sites routes (auth required).
func (s *Server) MountSites(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites", s.handleListSites)
	mux.HandleFunc("POST /api/sites", s.handleAddSite)
	mux.HandleFunc("DELETE /api/sites/{name}", s.handleDeleteSite)
	mux.HandleFunc("PUT /api/sites/{name}/files/{path...}", s.handlePutSiteFile)
	mux.HandleFunc("GET /api/sites/{name}/dns", s.handleSiteDNS)
	mux.HandleFunc("GET /api/sites/{name}/logs", s.handleSiteLogs)
	mux.HandleFunc("GET /api/sites/{name}/resources", s.handleSiteResources)
	mux.HandleFunc("POST /api/sites/{name}/deploy", s.handleDeployBinary)
	mux.HandleFunc("POST /api/sites/{name}/deploy/git", s.handleDeployGit)
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.sessionOK(r) == nil {
		s.writeErr(w, 401, "unauthorized")
		return false
	}
	return true
}

func (s *Server) loadOrEmptyConfig() (*config.Config, error) {
	path := s.configPath()
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return &config.Config{Version: 1}, nil
	}
	return nil, err
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	type siteView struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		Root        string   `json:"root"`
		SocketProxy bool     `json:"socket_proxy,omitempty"`
		Cron        []string `json:"cron,omitempty"`
		Jobs        []string `json:"jobs,omitempty"`
	}
	out := make([]siteView, 0, len(cfg.Sites))
	for _, site := range cfg.Sites {
		root := site.Root
		if root == "" {
			root = "public"
		}
		v := siteView{
			Name:        site.Name,
			Type:        site.Type,
			Root:        root,
			SocketProxy: site.SocketProxy,
		}
		for _, c := range site.Cron {
			v.Cron = append(v.Cron, c.Name)
		}
		for _, j := range site.Jobs {
			v.Jobs = append(v.Jobs, j.Name)
		}
		out = append(out, v)
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "sites": out})
}

func (s *Server) handleAddSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	if body.Type == "" {
		body.Type = config.SiteTypeStatic
	}
	site := config.Site{
		Name: body.Name,
		Type: body.Type,
		Root: "public",
	}
	if body.Type == config.SiteTypeApp {
		site.Root = ""
		site.Exec = "./bin/server"
		site.SocketActivation = true
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	for _, existing := range cfg.Sites {
		if strings.EqualFold(existing.Name, site.Name) {
			s.writeErr(w, 409, "site already exists")
			return
		}
	}
	cfg.Sites = append(cfg.Sites, site)
	if err := cfg.Validate(); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	warnings := []string{}
	if err := s.siteFS().CreateSiteUser(site.Name); err != nil {
		warnings = append(warnings, "CreateSiteUser: "+err.Error())
	} else if err := s.applyCaddy(cfg); err != nil {
		warnings = append(warnings, "caddy reload: "+err.Error())
	}
	out := map[string]any{"ok": true, "name": site.Name, "root": "public"}
	if len(warnings) > 0 {
		out["warning"] = strings.Join(warnings, "; ")
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) applyCaddy(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opt := reconcile.StaticApplyOptions{
		CaddyConfigPath: s.CaddyConfigPath,
		HTTPListen:      s.HTTPListen,
		DisableHTTPS:    s.DisableHTTPS,
		SeedIndex:       true,
	}
	if opt.CaddyConfigPath == "" {
		opt.CaddyConfigPath = "/etc/webor/caddy.json"
	}
	if opt.HTTPListen == "" {
		opt.HTTPListen = ":8080"
	}
	// Default lab behaviour: HTTPS off until DNS gating enables it.
	if !s.DisableHTTPS && len(s.ExpectedA) == 0 {
		opt.DisableHTTPS = true
	}
	return reconcile.ApplyStaticSites(ctx, cfg, s.siteFS(), opt)
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
		return
	}
	deleteData := r.URL.Query().Get("delete_data") == "1" || r.URL.Query().Get("delete_data") == "true"
	approvalID := strings.TrimSpace(r.URL.Query().Get("approval_id"))

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	filtered := cfg.Sites[:0]
	found := false
	for _, site := range cfg.Sites {
		if strings.EqualFold(site.Name, name) {
			found = true
			continue
		}
		filtered = append(filtered, site)
	}
	if !found {
		s.writeErr(w, 404, "site not found")
		return
	}

	if deleteData {
		if approvalID == "" {
			s.writeJSON(w, 403, map[string]any{
				"ok":    false,
				"error": "delete_data requires step-up approval",
				"hint":  "POST /api/approvals with operation DeleteSiteUser, then: wbr approve <id>",
			})
			return
		}
		if err := s.deleteSiteUser(name, true, approvalID); err != nil {
			s.writeErr(w, 403, err.Error())
			return
		}
	}

	cfg.Sites = filtered
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if len(cfg.Sites) > 0 {
		_ = s.applyCaddy(cfg)
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "delete_data": deleteData})
}

func (s *Server) deleteSiteUser(domain string, deleteData bool, approvalID string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeleteSiteUserArgs{
			Domain:     domain,
			DeleteData: deleteData,
			ApprovalID: approvalID,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeleteSiteUser, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	if deleteData {
		return fmt.Errorf("delete_data requires helper IPC (set -helper-socket)")
	}
	return s.ops().DeleteSiteUser(domain, false)
}

func (s *Server) handlePutSiteFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	rel := strings.TrimSpace(r.PathValue("path"))
	if name == "" || rel == "" {
		s.writeErr(w, 400, "name and path are required")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 5*1024*1024+1))
	if err != nil {
		s.writeErr(w, 400, "read body")
		return
	}
	if len(data) > 5*1024*1024 {
		s.writeErr(w, 400, "file exceeds 5 MiB")
		return
	}
	if err := s.siteFS().WritePublicFile(name, rel, data); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": rel})
}
