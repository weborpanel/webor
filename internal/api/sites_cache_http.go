package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/distro"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/providers/valkey"
)

func (s *Server) mountCacheRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/cache", s.handleGetCache)
	mux.HandleFunc("POST /api/sites/{name}/cache", s.handleAttachCache)
	mux.HandleFunc("DELETE /api/sites/{name}/cache", s.handleDetachCache)
	mux.HandleFunc("POST /api/sites/{name}/cache/rotate-password", s.handleRotateCachePassword)
}

func (s *Server) handleGetCache(w http.ResponseWriter, r *http.Request) {
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
	site, ok := findSite(cfg, name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	parentName := name
	inherited := false
	if p := strings.TrimSpace(site.Parent); p != "" {
		inherited = true
		parentName = strings.ToLower(p)
		parent, ok := findSite(cfg, parentName)
		if !ok {
			s.writeErr(w, 500, "parent site missing")
			return
		}
		site = parent
	}
	st, _ := s.valkeyStatus()
	info := distro.Detect()
	engine := strings.ToLower(strings.TrimSpace(site.Cache))
	out := map[string]any{
		"ok":            true,
		"parent":        parentName,
		"inherited":     inherited,
		"attached":      engine != "",
		"engine":        engine,
		"valkey":        st,
		"engines":       []string{valkey.Engine},
		"install_hint":  info.InstallHintValkey(),
		"start_hint":    info.StartHintValkey(),
		"os_family":     string(info.Family),
	}
	if engine != "" {
		view := s.cacheView(parentName, false)
		for k, v := range view {
			out[k] = v
		}
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) cacheView(domain string, withPassword bool) map[string]any {
	out := map[string]any{
		"username": valkey.Username(domain),
		"prefix":   valkey.KeyPrefix(domain),
		"host":     valkey.DefaultTCPHost,
		"port":     valkey.DefaultPort,
	}
	secretPath := valkey.SecretPath(s.stateDir(), domain)
	_, err := os.Stat(secretPath)
	out["secret_ready"] = err == nil
	if withPassword && err == nil {
		if sec, err := valkey.ReadSecret(secretPath); err == nil {
			out["password"] = sec.Password
			out["url"] = sec.URL()
		}
	}
	return out
}

func (s *Server) handleAttachCache(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Engine string `json:"engine"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	engine := strings.ToLower(strings.TrimSpace(body.Engine))
	if engine == "" || engine == "redis" {
		engine = valkey.Engine
	}
	if engine != valkey.Engine {
		s.writeErr(w, 400, "unsupported cache engine")
		return
	}

	st, err := s.valkeyStatus()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if st == nil || !st.Installed {
		// Package install stays out of the panel approval flow: an operator
		// installs Valkey/Redis as root, then returns here to Attach.
		info := distro.Detect()
		s.writeJSON(w, 409, map[string]any{
			"ok":               false,
			"error":            "valkey not installed",
			"install_required": true,
			"hint":             "Log in as root and install Valkey (or Redis), then try Attach again:\n" + info.InstallHintValkey(),
		})
		return
	}
	if !st.Active {
		info := distro.Detect()
		s.writeJSON(w, 409, map[string]any{
			"ok":               false,
			"error":            "valkey installed but not running",
			"install_required": true,
			"hint":             "Start the cache as root, then try Attach again:\n" + info.StartHintValkey(),
		})
		return
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	_, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if strings.TrimSpace(cfg.Sites[idx].Parent) != "" {
		s.writeErr(w, 400, "child sites cannot declare cache (use parent)")
		return
	}
	res, err := s.ensureSiteCache(name, engine)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	cfg.Sites[idx].Cache = engine
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	out := s.cacheView(name, true)
	out["ok"] = true
	out["engine"] = engine
	out["attached"] = true
	if res != nil {
		out["username"] = res.Username
		out["prefix"] = res.Prefix
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) handleDetachCache(w http.ResponseWriter, r *http.Request) {
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
	_, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if strings.TrimSpace(cfg.Sites[idx].Parent) != "" {
		s.writeErr(w, 400, "detach cache on the parent site")
		return
	}
	if strings.TrimSpace(cfg.Sites[idx].Cache) == "" {
		s.writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	if err := s.dropSiteCache(name); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	cfg.Sites[idx].Cache = ""
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleRotateCachePassword(w http.ResponseWriter, r *http.Request) {
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
	site, ok := findSite(cfg, name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if strings.TrimSpace(site.Parent) != "" {
		s.writeErr(w, 400, "rotate cache password on the parent site")
		return
	}
	if strings.TrimSpace(site.Cache) == "" {
		s.writeErr(w, 404, "no cache attached")
		return
	}
	res, err := s.rotateCachePassword(name)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	out := s.cacheView(name, true)
	out["ok"] = true
	if res != nil {
		out["username"] = res.Username
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) valkeyStatus() (*helper.ValkeyStatusResult, error) {
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpValkeyStatus, Args: json.RawMessage(`{}`),
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var st helper.ValkeyStatusResult
		if err := json.Unmarshal([]byte(resp.Message), &st); err != nil {
			return nil, err
		}
		return &st, nil
	}
	return s.ops().ValkeyStatus()
}

func (s *Server) ensureSiteCache(domain, engine string) (*helper.EnsureSiteCacheResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.EnsureSiteCacheArgs{
			Domain: domain, Engine: engine, StateDir: s.stateDir(),
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpEnsureSiteCache, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.EnsureSiteCacheResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	return s.ops().EnsureSiteCache(domain, engine, s.stateDir())
}

func (s *Server) dropSiteCache(domain string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DropSiteCacheArgs{Domain: domain, StateDir: s.stateDir()})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDropSiteCache, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().DropSiteCache(domain, s.stateDir())
}

func (s *Server) rotateCachePassword(domain string) (*helper.EnsureSiteCacheResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RotateCachePasswordArgs{Domain: domain, StateDir: s.stateDir()})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpRotateCachePassword, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.EnsureSiteCacheResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	return s.ops().RotateCachePassword(domain, s.stateDir())
}
