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

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/ports"
	"github.com/weborpanel/webor/internal/sites"
)

func (s *Server) mountBackendRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/backend", s.handleGetBackend)
	mux.HandleFunc("POST /api/sites/{name}/backend", s.handleAttachBackend)
	mux.HandleFunc("PATCH /api/sites/{name}/backend", s.handlePatchBackend)
	mux.HandleFunc("DELETE /api/sites/{name}/backend", s.handleDetachBackend)
	mux.HandleFunc("POST /api/sites/{name}/backend/deploy", s.handleDeployBackend)
}

func backendView(b *config.SiteBackend, active bool) map[string]any {
	if b == nil || !b.Enabled {
		return map[string]any{"enabled": false, "active": false}
	}
	config.NormalizeBackend(b)
	return map[string]any{
		"enabled":   true,
		"prefix":    b.Prefix,
		"paths":     b.BackendPathPrefixes(),
		"websocket": b.WebSocket,
		"exec":      b.Exec,
		"port":      b.Port,
		"listen":    fmt.Sprintf("127.0.0.1:%d", b.Port),
		"active":    active,
	}
}

func (s *Server) handleGetBackend(w http.ResponseWriter, r *http.Request) {
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
	active := false
	if site.Backend != nil && site.Backend.Enabled {
		active = s.httpBackendActive(name)
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"backend": backendView(site.Backend, active),
	})
}

func (s *Server) handleAttachBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Prefix    string   `json:"prefix"`
		Paths     []string `json:"paths"`
		WebSocket *bool    `json:"websocket"`
		Exec      string   `json:"exec"`
		Port      int      `json:"port"` // rejected if set
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
	if site.Backend != nil && site.Backend.Enabled {
		s.writeErr(w, 409, "backend already attached")
		return
	}

	ws := true
	if body.WebSocket != nil {
		ws = *body.WebSocket
	}
	port, err := ports.Allocate(cfg, name)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	b := &config.SiteBackend{
		Enabled:   true,
		Prefix:    body.Prefix,
		Paths:     body.Paths,
		WebSocket: ws,
		Exec:      body.Exec,
		Port:      port,
	}
	config.NormalizeBackend(b)
	if err := config.ValidateSiteBackend(config.SiteTypeStatic, b); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	cfg.Sites[idx].Backend = b
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if _, err := s.ensureHTTPBackend(cfg.Sites[idx]); err != nil {
		// Keep config; unit may start after deploy.
		_ = err
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	active := s.httpBackendActive(name)
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"backend": backendView(b, active),
	})
}

func (s *Server) handlePatchBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Prefix    *string  `json:"prefix"`
		Paths     []string `json:"paths"`
		PathsSet  bool     `json:"-"`
		WebSocket *bool    `json:"websocket"`
		Exec      *string  `json:"exec"`
		Port      int      `json:"port"`
	}
	dec := json.NewDecoder(r.Body)
	raw := map[string]json.RawMessage{}
	if err := dec.Decode(&raw); err != nil && err != io.EOF {
		s.writeErr(w, 400, "invalid json")
		return
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
	if body.Port != 0 {
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
	if site.Backend == nil || !site.Backend.Enabled {
		s.writeErr(w, 404, "backend not attached")
		return
	}
	b := *site.Backend
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
	config.NormalizeBackend(&b)
	if err := config.ValidateSiteBackend(config.SiteTypeStatic, &b); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	cfg.Sites[idx].Backend = &b
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if _, err := s.ensureHTTPBackend(cfg.Sites[idx]); err != nil {
		_ = err
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	active := s.httpBackendActive(name)
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"backend": backendView(&b, active),
	})
}

func (s *Server) handleDetachBackend(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))

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
	if site.Backend == nil || !site.Backend.Enabled {
		s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(nil, false)})
		return
	}
	_ = s.removeHTTPBackend(name)
	cfg.Sites[idx].Backend = nil
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "backend": backendView(nil, false)})
}

func (s *Server) handleDeployBackend(w http.ResponseWriter, r *http.Request) {
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
	if site.Type != config.SiteTypeStatic || site.Backend == nil || !site.Backend.Enabled {
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

	dir := filepath.Join(s.uploadsRoot(), name)
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

	res, err := s.deployHTTPBackend(name, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	active := s.httpBackendActive(name)
	s.writeJSON(w, 200, map[string]any{
		"ok":         true,
		"release_id": res.ReleaseID,
		"binary":     res.Binary,
		"active":     active,
	})
}

func (s *Server) ensureHTTPBackend(site config.Site) (*helper.EnsureHTTPBackendResult, error) {
	if site.Backend == nil {
		return nil, fmt.Errorf("backend not set")
	}
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.EnsureHTTPBackendArgs{
			Domain:    site.Name,
			Prefix:    site.Backend.Prefix,
			Paths:     site.Backend.Paths,
			WebSocket: site.Backend.WebSocket,
			Exec:      site.Backend.Exec,
			Port:      site.Backend.Port,
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
	return s.ops().EnsureHTTPBackend(site)
}

func (s *Server) removeHTTPBackend(domain string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RemoveHTTPBackendArgs{Domain: domain})
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
	return s.ops().RemoveHTTPBackend(domain)
}

func (s *Server) deployHTTPBackend(domain, binaryPath string) (*helper.DeployAppBinaryResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployHTTPBackendArgs{Domain: domain, BinaryPath: binaryPath})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeployHTTPBackend, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.DeployAppBinaryResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.ops().DeployHTTPBackend(ctx, domain, binaryPath)
}

func (s *Server) httpBackendActive(domain string) bool {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.HTTPBackendStatusArgs{Domain: domain})
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
	return s.ops().HTTPBackendActive(domain)
}
