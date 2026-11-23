package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/ipc"
)

func (s *Server) mountLimitsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/limits", s.handleGetSiteLimits)
	mux.HandleFunc("PUT /api/sites/{name}/limits", s.handlePutSiteLimits)
}

func (s *Server) handleGetSiteLimits(w http.ResponseWriter, r *http.Request) {
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
	host := s.hostCollector().Static()
	s.writeJSON(w, 200, map[string]any{
		"ok":     true,
		"limits": site.Limits,
		"options": map[string]any{
			"memory": config.MemoryOptionsForHost(host.MemTotal),
			"cpu":    config.CPUOptionsForHost(host.CPUCount),
			"disk":   config.DiskOptionsForPanel(),
		},
		"host": map[string]any{
			"mem_total": host.MemTotal,
			"cpu_count": host.CPUCount,
		},
		"note": "Disk presets max out at 10G in the panel; larger quotas need config.yaml.",
	})
}

func (s *Server) handlePutSiteLimits(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		MemoryMax string `json:"memory_max"`
		CPUQuota  string `json:"cpu_quota"`
		DiskQuota string `json:"disk_quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}

	host := s.hostCollector().Static()
	next := config.Limits{
		MemoryMax: strings.TrimSpace(body.MemoryMax),
		CPUQuota:  strings.TrimSpace(body.CPUQuota),
		DiskQuota: strings.TrimSpace(body.DiskQuota),
	}
	canon, err := config.CanonicalPanelLimits(next, host.MemTotal, host.CPUCount)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	next = canon

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
	// Preserve log_max (yaml-only).
	next.LogMax = cfg.Sites[idx].Limits.LogMax
	cfg.Sites[idx].Limits = next
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site := cfg.Sites[idx]
	if err := s.applySiteLimits(site); err != nil {
		s.writeErr(w, 500, "limits saved but units not updated: "+err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "limits": site.Limits})
}

func (s *Server) applySiteLimits(site config.Site) error {
	if s.HelperSocket != "" {
		raw, err := json.Marshal(ipc.ApplySiteLimitsArgs{Site: site})
		if err != nil {
			return err
		}
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpApplySiteLimits, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().ApplySiteLimits(site)
}
