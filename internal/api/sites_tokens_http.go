package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
)

func (s *Server) mountTokenRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/tokens", s.handleListDeployTokens)
	mux.HandleFunc("POST /api/sites/{name}/tokens", s.handleCreateDeployToken)
	mux.HandleFunc("DELETE /api/sites/{name}/tokens/{id}", s.handleRevokeDeployToken)
	mux.HandleFunc("GET /api/sites/{name}/ci", s.handleSiteCI)
}

func (s *Server) handleListDeployTokens(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	list, err := s.Account.ListDeployTokens(name)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if list == nil {
		list = []auth.DeployTokenInfo{}
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "tokens": list})
}

func (s *Server) handleCreateDeployToken(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	var body struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresIn int      `json:"expires_in_days"` // 0 = no expiry
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		s.writeErr(w, 400, "invalid json")
		return
	}
	scopes := body.Scopes
	if len(scopes) == 0 {
		scopes = auth.AllDeployScopes
	}
	var exp *time.Time
	if body.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(body.ExpiresIn) * 24 * time.Hour)
		exp = &t
	}
	plain, info, err := s.Account.CreateDeployToken(name, body.Name, scopes, exp)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":     true,
		"token":  plain, // shown once
		"record": info,
	})
}

func (s *Server) handleRevokeDeployToken(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.TrimSpace(r.PathValue("id"))
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	if id == "" {
		s.writeErr(w, 400, "token id required")
		return
	}
	if err := s.Account.RevokeDeployToken(name, id); err != nil {
		s.writeErr(w, 404, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

// handleSiteCI is the Action preflight probe: site must exist; never creates.
func (s *Server) handleSiteCI(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
		return
	}
	if !s.requireSiteToken(w, r, name) {
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
		s.writeErr(w, 404, "site not found: "+name)
		return
	}

	type backendCI struct {
		ID     string `json:"id"`
		Prefix string `json:"prefix,omitempty"`
		Exec   string `json:"exec,omitempty"`
	}
	var backends []backendCI
	for i := range site.Backends {
		b := site.Backends[i]
		if !b.Enabled {
			continue
		}
		config.NormalizeBackend(&b)
		backends = append(backends, backendCI{ID: b.ID, Prefix: b.Prefix, Exec: b.Exec})
	}
	if backends == nil {
		backends = []backendCI{}
	}

	suggested := "static"
	if site.Type == config.SiteTypeApp {
		suggested = "binary"
		if site.HasNodeRuntime() {
			suggested = "node"
		}
	}

	s.writeJSON(w, 200, map[string]any{
		"ok":             true,
		"name":           site.Name,
		"type":           site.Type,
		"exec":           site.Exec,
		"runtime":        runtimeView(site.Runtime),
		"backends":       backends,
		"suggested_kind": suggested,
	})
}
