package api

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
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
	found := false
	for _, site := range cfg.Sites {
		if strings.EqualFold(site.Name, name) {
			found = true
			break
		}
	}
	if !found {
		s.writeErr(w, 404, "site not found")
		return
	}

	sel := strings.TrimSpace(r.URL.Query().Get("unit"))
	if sel == "" {
		sel = "app"
	}
	lines := 100
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 {
			s.writeErr(w, 400, "lines must be 1..500")
			return
		}
		lines = n
	}

	unit, text, err := s.ops().SiteJournalTail(name, sel, lines)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":   true,
		"unit": unit,
		"lines": lines,
		"text": text,
	})
}
