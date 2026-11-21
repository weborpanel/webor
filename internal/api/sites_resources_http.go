package api

import (
	"net/http"
	"strings"

	"github.com/weborpanel/webor/internal/config"
)

func (s *Server) handleSiteResources(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	var site *config.Site
	for i := range cfg.Sites {
		if strings.EqualFold(cfg.Sites[i].Name, name) {
			site = &cfg.Sites[i]
			break
		}
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	res, err := s.ops().SiteResources(name, site)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "resources": res})
}
