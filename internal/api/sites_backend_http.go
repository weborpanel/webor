package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/ports"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func (s *Server) mountBackendRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/backends", s.handleListBackends)
	mux.HandleFunc("POST /api/sites/{name}/backends", s.handleCreateBackend)
	mux.HandleFunc("GET /api/sites/{name}/backends/{id}", s.handleGetBackendByID)
	mux.HandleFunc("PATCH /api/sites/{name}/backends/{id}", s.handlePatchBackendByID)
	mux.HandleFunc("DELETE /api/sites/{name}/backends/{id}", s.handleDeleteBackendByID)
	mux.HandleFunc("POST /api/sites/{name}/backends/{id}/deploy", s.handleDeployBackendByID)
	mux.HandleFunc("POST /api/sites/{name}/backends/{id}/deploy-zip", s.handleDeployBackendZipByID)
	mux.HandleFunc("POST /api/sites/{name}/backends/{id}/restart", s.handleRestartBackendByID)
	mux.HandleFunc("GET /api/sites/{name}/backends/{id}/fs", s.handleListBackendFS)

	// Legacy single-backend aliases (work when exactly one backend exists, or create the first).
	mux.HandleFunc("GET /api/sites/{name}/backend", s.handleGetBackendLegacy)
	mux.HandleFunc("POST /api/sites/{name}/backend", s.handleAttachBackendLegacy)
	mux.HandleFunc("PATCH /api/sites/{name}/backend", s.handlePatchBackendLegacy)
	mux.HandleFunc("DELETE /api/sites/{name}/backend", s.handleDetachBackendLegacy)
	mux.HandleFunc("POST /api/sites/{name}/backend/deploy", s.handleDeployBackendLegacy)
	mux.HandleFunc("POST /api/sites/{name}/backend/deploy-zip", s.handleDeployBackendZipLegacy)
}

func backendView(domain string, b *config.SiteBackend, active bool) map[string]any {
	if b == nil || !b.Enabled {
		return map[string]any{"enabled": false, "active": false}
	}
	config.NormalizeBackend(b)
	return map[string]any{
		"enabled":   true,
		"id":        b.ID,
		"prefix":    b.Prefix,
		"paths":     b.BackendPathPrefixes(),
		"websocket": b.WebSocket,
		"exec":      b.Exec,
		"port":      b.Port,
		"listen":    fmt.Sprintf("127.0.0.1:%d", b.Port),
		"active":    active,
		"dir":       "apis/" + b.ID,
		"unit":      systemd.HTTPBackendUnitName(domain, b.ID),
		"runtime":   runtimeView(b.Runtime),
	}
}

func (s *Server) handleListBackends(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	list := make([]map[string]any, 0, len(site.Backends))
	for i := range site.Backends {
		b := site.Backends[i]
		active := s.httpBackendActive(name, b.ID)
		list = append(list, backendView(name, &b, active))
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "backends": list})
}

func (s *Server) handleCreateBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		ID        string   `json:"id"`
		Prefix    string   `json:"prefix"`
		Paths     []string `json:"paths"`
		WebSocket *bool    `json:"websocket"`
		Exec      string   `json:"exec"`
		Port      int      `json:"port"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Port != 0 {
		s.writeErr(w, 400, "port is assigned automatically; do not send port")
		return
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if site.Type != config.SiteTypeStatic {
		s.writeErr(w, 400, "backend is only supported on static sites")
		return
	}
	if len(site.EnabledBackends()) >= config.MaxBackends {
		s.writeErr(w, 400, fmt.Sprintf("at most %d backends allowed", config.MaxBackends))
		return
	}

	ws := true
	if body.WebSocket != nil {
		ws = *body.WebSocket
	}
	port, err := ports.Allocate(cfg)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	b := config.SiteBackend{
		Enabled:   true,
		Prefix:    body.Prefix,
		Paths:     body.Paths,
		WebSocket: ws,
		Exec:      body.Exec,
		Port:      port,
	}
	// IDs are always auto-generated from the path; never ask the user for one.
	config.NormalizeBackend(&b)
	b.ID = config.UniqueBackendID(b.ID, func(id string) bool {
		_, _, exists := site.FindBackend(id)
		return exists
	})
	trial := append(append([]config.SiteBackend{}, site.Backends...), b)
	if err := config.ValidateSiteBackends(config.SiteTypeStatic, trial); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	cfg.Sites[idx].Backends = trial
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if _, err := s.ensureHTTPBackend(cfg.Sites[idx], b); err != nil {
		_ = err
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	active := s.httpBackendActive(name, b.ID)
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"backend": backendView(name, &b, active),
	})
}

func (s *Server) handleGetBackendByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	b, _, ok := site.FindBackend(id)
	if !ok {
		s.writeErr(w, 404, "backend not found")
		return
	}
	active := s.httpBackendActive(name, b.ID)
	s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, &b, active)})
}

func (s *Server) handlePatchBackendByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))
	patch, err := decodeBackendPatch(r)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if patch.Port != 0 {
		s.writeErr(w, 400, "port cannot be changed; detach and re-attach to get a new port")
		return
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	b, bIdx, ok := site.FindBackend(id)
	if !ok {
		s.writeErr(w, 404, "backend not found")
		return
	}
	applyBackendPatch(&b, patch)
	config.NormalizeBackend(&b)
	trial := append([]config.SiteBackend{}, site.Backends...)
	trial[bIdx] = b
	if err := config.ValidateSiteBackends(config.SiteTypeStatic, trial); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if b.HasNodeRuntime() && b.Runtime.Command == "" {
		if err := s.ensureNodeRuntime(b.Runtime.Version); err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
	}
	cfg.Sites[idx].Backends = trial
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if _, err := s.ensureHTTPBackend(cfg.Sites[idx], b); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	active := s.httpBackendActive(name, b.ID)
	s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, &b, active)})
}

func (s *Server) handleDeleteBackendByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	_, bIdx, ok := site.FindBackend(id)
	if !ok {
		s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, nil, false)})
		return
	}
	_ = s.removeHTTPBackend(name, id)
	cfg.Sites[idx].Backends = append(site.Backends[:bIdx], site.Backends[bIdx+1:]...)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, nil, false)})
}

func (s *Server) handleRestartBackendByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	b, _, ok := site.FindBackend(id)
	if !ok {
		s.writeErr(w, 404, "backend not found")
		return
	}
	if err := s.restartHTTPBackend(name, id); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	active := s.httpBackendActive(name, id)
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"backend": backendView(name, &b, active),
	})
}

func (s *Server) handleDeployBackendByID(w http.ResponseWriter, r *http.Request) {
	s.deployBackendBinary(w, r, r.PathValue("id"))
}

func (s *Server) handleDeployBackendZipByID(w http.ResponseWriter, r *http.Request) {
	s.deployBackendZip(w, r, r.PathValue("id"))
}

func (s *Server) handleListBackendFS(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))
	path := strings.TrimSpace(r.URL.Query().Get("path"))

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	if _, _, ok := site.FindBackend(id); !ok {
		s.writeErr(w, 404, "backend not found")
		return
	}
	entries, err := s.listAPIDir(name, id, path)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"path":    path,
		"entries": entries,
	})
}

// --- legacy single-backend wrappers ---

func (s *Server) legacySingleBackend(site config.Site) (config.SiteBackend, error) {
	enabled := site.EnabledBackends()
	if len(enabled) == 0 {
		return config.SiteBackend{}, fmt.Errorf("backend not attached")
	}
	if len(enabled) > 1 {
		return config.SiteBackend{}, fmt.Errorf("multiple backends configured; use /backends/{id}")
	}
	return enabled[0], nil
}

func (s *Server) handleGetBackendLegacy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	enabled := site.EnabledBackends()
	if len(enabled) == 0 {
		s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, nil, false)})
		return
	}
	if len(enabled) > 1 {
		s.writeErr(w, 409, "multiple backends configured; use GET /backends")
		return
	}
	b := enabled[0]
	active := s.httpBackendActive(name, b.ID)
	s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, &b, active)})
}

func (s *Server) handleAttachBackendLegacy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if len(site.EnabledBackends()) > 0 {
		s.writeErr(w, 409, "backend already attached; use POST /backends to add another")
		return
	}
	s.handleCreateBackend(w, r)
}

func (s *Server) handlePatchBackendLegacy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	b, err := s.legacySingleBackend(site)
	if err != nil {
		if strings.Contains(err.Error(), "multiple") {
			s.writeErr(w, 409, err.Error())
		} else {
			s.writeErr(w, 404, err.Error())
		}
		return
	}
	r.SetPathValue("id", b.ID)
	s.handlePatchBackendByID(w, r)
}

func (s *Server) handleDetachBackendLegacy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	enabled := site.EnabledBackends()
	if len(enabled) == 0 {
		s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(name, nil, false)})
		return
	}
	if len(enabled) > 1 {
		s.writeErr(w, 409, "multiple backends configured; use DELETE /backends/{id}")
		return
	}
	r.SetPathValue("id", enabled[0].ID)
	s.handleDeleteBackendByID(w, r)
}

func (s *Server) handleDeployBackendLegacy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	b, err := s.legacySingleBackend(*site)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.deployBackendBinary(w, r, b.ID)
}

func (s *Server) handleDeployBackendZipLegacy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	b, err := s.legacySingleBackend(*site)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.deployBackendZip(w, r, b.ID)
}

// --- shared deploy / helpers ---

type backendPatch struct {
	Prefix       *string
	Paths        []string
	PathsSet     bool
	WebSocket    *bool
	Exec         *string
	Port         int
	RuntimeKind  *string
	RuntimeVer   *string
	RuntimeCmd   *string
	ClearRuntime bool
	RuntimeSet   bool
}

func decodeBackendPatch(r *http.Request) (backendPatch, error) {
	var body backendPatch
	raw := map[string]json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil && err != io.EOF {
		return body, fmt.Errorf("invalid json")
	}
	if v, ok := raw["prefix"]; ok {
		var p string
		_ = json.Unmarshal(v, &p)
		body.Prefix = &p
	}
	if v, ok := raw["paths"]; ok {
		_ = json.Unmarshal(v, &body.Paths)
		body.PathsSet = true
	}
	if v, ok := raw["websocket"]; ok {
		var ws bool
		_ = json.Unmarshal(v, &ws)
		body.WebSocket = &ws
	}
	if v, ok := raw["exec"]; ok {
		var e string
		_ = json.Unmarshal(v, &e)
		body.Exec = &e
	}
	if v, ok := raw["port"]; ok {
		_ = json.Unmarshal(v, &body.Port)
	}
	if v, ok := raw["clear_runtime"]; ok {
		_ = json.Unmarshal(v, &body.ClearRuntime)
		body.RuntimeSet = true
	}
	if v, ok := raw["runtime"]; ok {
		body.RuntimeSet = true
		var rt struct {
			Kind    string `json:"kind"`
			Version string `json:"version"`
			Command string `json:"command"`
			Clear   bool   `json:"clear"`
		}
		if err := json.Unmarshal(v, &rt); err != nil {
			return body, fmt.Errorf("invalid runtime")
		}
		if rt.Clear || strings.TrimSpace(rt.Kind) == "" {
			body.ClearRuntime = true
		} else {
			body.RuntimeKind = &rt.Kind
			body.RuntimeVer = &rt.Version
			body.RuntimeCmd = &rt.Command
		}
	}
	return body, nil
}

func applyBackendPatch(b *config.SiteBackend, body backendPatch) {
	if body.Prefix != nil {
		b.Prefix = *body.Prefix
	}
	if body.PathsSet {
		b.Paths = body.Paths
	}
	if body.WebSocket != nil {
		b.WebSocket = *body.WebSocket
	}
	if body.Exec != nil {
		b.Exec = *body.Exec
	}
	if body.RuntimeSet {
		if body.ClearRuntime {
			b.Runtime = nil
		} else if body.RuntimeKind != nil {
			rt := &config.SiteRuntime{
				Kind:    *body.RuntimeKind,
				Version: "",
				Command: "",
			}
			if body.RuntimeVer != nil {
				rt.Version = *body.RuntimeVer
			}
			if body.RuntimeCmd != nil {
				rt.Command = *body.RuntimeCmd
			}
			b.Runtime = rt
		}
	}
}

func (s *Server) deployBackendBinary(w http.ResponseWriter, r *http.Request, id string) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id = strings.ToLower(strings.TrimSpace(id))
	if !s.requireSiteScope(w, r, name, auth.ScopeDeploy) {
		return
	}

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	if site.Type != config.SiteTypeStatic {
		s.writeErr(w, 400, "attach an HTTP backend before deploying")
		return
	}
	if _, _, ok := site.FindBackend(id); !ok {
		s.writeErr(w, 400, "attach an HTTP backend before deploying")
		return
	}

	if err := r.ParseMultipartForm(maxDeployBinaryBytes + 1024); err != nil {
		s.writeErr(w, 400, "invalid multipart form")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 400, "file is required")
		return
	}
	defer file.Close()

	dir := filepath.Join(s.uploadsRoot(), name, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeErr(w, 500, "upload dir: "+err.Error())
		return
	}
	staged := filepath.Join(dir, sites.DefaultAppBinary)
	tmp := staged + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	n, err := io.Copy(out, io.LimitReader(file, maxDeployBinaryBytes+1))
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "read upload")
		return
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, closeErr.Error())
		return
	}
	if n > maxDeployBinaryBytes {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "file exceeds 64 MiB")
		return
	}
	if err := os.Rename(tmp, staged); err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, err.Error())
		return
	}

	res, err := s.deployHTTPBackend(name, id, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	active := s.httpBackendActive(name, id)
	s.writeJSON(w, 200, map[string]any{
		"ok":     true,
		"binary": res.Binary,
		"id":     id,
		"active": active,
	})
}

func (s *Server) deployBackendZip(w http.ResponseWriter, r *http.Request, id string) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id = strings.ToLower(strings.TrimSpace(id))
	if !s.requireSiteScope(w, r, name, auth.ScopeDeploy) {
		return
	}

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	if site.Type != config.SiteTypeStatic {
		s.writeErr(w, 400, "attach an HTTP backend before deploying")
		return
	}
	if _, _, ok := site.FindBackend(id); !ok {
		s.writeErr(w, 400, "attach an HTTP backend before deploying")
		return
	}

	if err := r.ParseMultipartForm(maxDeployBinaryBytes + 1024); err != nil {
		s.writeErr(w, 400, "invalid multipart form")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 400, "file is required")
		return
	}
	defer file.Close()
	fname := ""
	if hdr != nil {
		fname = hdr.Filename
	}
	if !strings.HasSuffix(strings.ToLower(fname), ".zip") {
		s.writeErr(w, 400, "file must be a .zip archive")
		return
	}

	dir := filepath.Join(s.uploadsRoot(), name, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeErr(w, 500, "upload dir: "+err.Error())
		return
	}
	staged := filepath.Join(dir, "backend-bundle.zip")
	tmp := staged + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	n, err := io.Copy(out, io.LimitReader(file, maxDeployBinaryBytes+1))
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "read upload")
		return
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, closeErr.Error())
		return
	}
	if n > maxDeployBinaryBytes {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "file exceeds 64 MiB")
		return
	}
	if err := os.Rename(tmp, staged); err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, err.Error())
		return
	}

	res, err := s.deployHTTPBackendZip(name, id, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	active := s.httpBackendActive(name, id)
	s.writeJSON(w, 200, map[string]any{
		"ok":     true,
		"root":   res.Root,
		"id":     id,
		"active": active,
	})
}

func (s *Server) ensureHTTPBackend(site config.Site, backend config.SiteBackend) (*helper.EnsureHTTPBackendResult, error) {
	if !backend.Enabled {
		return nil, fmt.Errorf("backend not set")
	}
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.EnsureHTTPBackendArgs{
			Domain:    site.Name,
			ID:        backend.ID,
			Prefix:    backend.Prefix,
			Paths:     backend.Paths,
			WebSocket: backend.WebSocket,
			Exec:      backend.Exec,
			Port:      backend.Port,
			Runtime:   backend.Runtime,
			Limits:    site.Limits,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpEnsureHTTPBackend, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.EnsureHTTPBackendResult
		_ = json.Unmarshal([]byte(resp.Message), &res)
		return &res, nil
	}
	return s.ops().EnsureHTTPBackend(site, backend)
}

func (s *Server) removeHTTPBackend(domain, id string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RemoveHTTPBackendArgs{Domain: domain, ID: id})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpRemoveHTTPBackend, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RemoveHTTPBackend(domain, id)
}

func (s *Server) removeAllHTTPBackends(site config.Site) {
	for _, b := range site.EnabledBackends() {
		_ = s.removeHTTPBackend(site.Name, b.ID)
	}
}

func (s *Server) deployHTTPBackend(domain, id, binaryPath string) (*helper.DeployHTTPBackendResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployHTTPBackendArgs{Domain: domain, ID: id, BinaryPath: binaryPath})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeployHTTPBackend, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.DeployHTTPBackendResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.ops().DeployHTTPBackend(ctx, domain, id, binaryPath)
}

func (s *Server) deployHTTPBackendZip(domain, id, zipPath string) (*helper.DeployHTTPBackendZipResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployHTTPBackendZipArgs{Domain: domain, ID: id, ZipPath: zipPath})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeployHTTPBackendZip, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.DeployHTTPBackendZipResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.ops().DeployHTTPBackendZip(ctx, domain, id, zipPath)
}

func (s *Server) restartHTTPBackend(domain, id string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RestartHTTPBackendArgs{Domain: domain, ID: id})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpRestartHTTPBackend, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RestartHTTPBackend(domain, id)
}

func (s *Server) httpBackendActive(domain, id string) bool {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.HTTPBackendStatusArgs{Domain: domain, ID: id})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpHTTPBackendStatus, Args: raw,
		})
		if err != nil || !resp.OK {
			return false
		}
		var st struct {
			Active bool `json:"active"`
		}
		_ = json.Unmarshal([]byte(resp.Message), &st)
		return st.Active
	}
	return s.ops().HTTPBackendActive(domain, id)
}

func (s *Server) listAPIDir(domain, id, path string) ([]helper.PublicFSEntry, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ListAPIDirArgs{Domain: domain, ID: id, Path: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpListAPIDir, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var entries []helper.PublicFSEntry
		if err := json.Unmarshal([]byte(resp.Message), &entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
	return s.ops().ListAPIDir(domain, id, path)
}
