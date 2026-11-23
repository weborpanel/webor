package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/reconcile"
)

func (s *Server) mountRuntimeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/runtime", s.handleGetRuntime)
	mux.HandleFunc("PUT /api/sites/{name}/runtime", s.handlePutRuntime)
	mux.HandleFunc("GET /api/runtimes/node", s.handleListNodeRuntimes)
}

func (s *Server) handleListNodeRuntimes(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	// Curated majors for the UI; patch resolved on ensure.
	s.writeJSON(w, 200, map[string]any{
		"ok": true,
		"versions": []map[string]string{
			{"id": "20", "label": "Node 20 (LTS)"},
			{"id": "22", "label": "Node 22 (LTS)"},
			{"id": "24", "label": "Node 24"},
		},
		"hint": "Prefer CI-built Linux artifacts; runtime only provides node/npm on PATH.",
	})
}

func (s *Server) handleGetRuntime(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if !s.requireSiteScope(w, r, name, auth.ScopeRuntime) {
		return
	}
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"runtime": runtimeView(site.Runtime),
		"exec":    site.Exec,
	})
}

func runtimeView(r *config.SiteRuntime) map[string]any {
	if r == nil {
		return map[string]any{"kind": ""}
	}
	config.NormalizeRuntime(r)
	return map[string]any{
		"kind":    r.Kind,
		"version": r.Version,
		"command": r.Command,
	}
}

func (s *Server) handlePutRuntime(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if !s.requireSiteScope(w, r, name, auth.ScopeRuntime) {
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		Version string `json:"version"`
		Command string `json:"command"`
		Exec    string `json:"exec"`
		Clear   bool   `json:"clear"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
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
	if site.Type != config.SiteTypeApp {
		s.writeErr(w, 400, "runtime is only for app sites")
		return
	}

	if body.Clear || strings.TrimSpace(body.Kind) == "" {
		cfg.Sites[idx].Runtime = nil
	} else {
		rt := &config.SiteRuntime{Kind: body.Kind, Version: body.Version, Command: body.Command}
		if err := config.ValidateSiteRuntime(config.SiteTypeApp, rt); err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
		if rt.Kind == config.RuntimeKindNode && rt.Command == "" {
			if err := s.ensureNodeRuntime(rt.Version); err != nil {
				s.writeErr(w, 400, err.Error())
				return
			}
		}
		cfg.Sites[idx].Runtime = rt
	}
	if strings.TrimSpace(body.Exec) != "" {
		cfg.Sites[idx].Exec = strings.TrimSpace(body.Exec)
	}
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = reconcile.ApplyAppSites(ctx, cfg, s.ops(), reconcile.AppApplyOptions{
		CaddyConfigPath: s.CaddyConfigPath,
		HTTPListen:      s.HTTPListen,
		DisableHTTPS:    s.DisableHTTPS,
		PanelUpstream:   s.panelUpstreamDial(),
	})

	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"runtime": runtimeView(cfg.Sites[idx].Runtime),
		"exec":    cfg.Sites[idx].Exec,
	})
}

func (s *Server) ensureNodeRuntime(version string) error {
	raw, _ := json.Marshal(ipc.EnsureNodeRuntimeArgs{Version: version})
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpEnsureNodeRuntime, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_, err := s.ops().EnsureNodeRuntime(ctx, version)
	return err
}
