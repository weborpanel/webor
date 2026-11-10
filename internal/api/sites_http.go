package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/helper"
)

// DefaultConfigPath is the production desired-state file.
const DefaultConfigPath = "/etc/webooza/config.yaml"

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

// MountSites registers /api/sites routes (auth required).
func (s *Server) MountSites(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites", s.handleListSites)
	mux.HandleFunc("POST /api/sites", s.handleAddSite)
	mux.HandleFunc("DELETE /api/sites/{name}", s.handleDeleteSite)
	mux.HandleFunc("PUT /api/sites/{name}/files/{path...}", s.handlePutSiteFile)
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
		Name string `json:"name"`
		Type string `json:"type"`
		Root string `json:"root"`
	}
	out := make([]siteView, 0, len(cfg.Sites))
	for _, site := range cfg.Sites {
		root := site.Root
		if root == "" {
			root = "public"
		}
		out = append(out, siteView{Name: site.Name, Type: site.Type, Root: root})
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
	// Best-effort: create Linux user + public/ when the panel can (lab/root).
	if err := s.ops().CreateSiteUser(site.Name); err != nil {
		s.writeJSON(w, 200, map[string]any{
			"ok":      true,
			"name":    site.Name,
			"warning": "saved config but CreateSiteUser failed: " + err.Error(),
		})
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "name": site.Name, "root": "public"})
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
	cfg.Sites = filtered
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
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
	if err := s.ops().WritePublicFile(name, rel, data); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": rel})
}
