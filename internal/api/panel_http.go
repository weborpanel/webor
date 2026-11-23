package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/dnscheck"
	"github.com/weborpanel/webor/internal/providers/caddy"
)

// MountPanel registers /api/panel routes (auth required).
func (s *Server) MountPanel(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/panel", s.handleGetPanel)
	mux.HandleFunc("PUT /api/panel/domain", s.handlePutPanelDomain)
	mux.HandleFunc("DELETE /api/panel/domain", s.handleDeletePanelDomain)
	mux.HandleFunc("GET /api/panel/dns", s.handlePanelDNS)
}

func (s *Server) panelUpstreamDial() string {
	if dial, err := caddy.PanelDial(s.PanelListen, 0); err == nil {
		return dial
	}
	// Fallback to panel.port from config. Caller may already hold sitesMu;
	// load without locking (best-effort; Save/Validate still serialized elsewhere).
	cfg, err := s.loadOrEmptyConfig()
	if err == nil && cfg.Panel.Port > 0 {
		if dial, err := caddy.PanelDial("", cfg.Panel.Port); err == nil {
			return dial
		}
	}
	return ""
}

func (s *Server) panelPort() int {
	if s.PanelListen != "" {
		if _, p, err := net.SplitHostPort(s.PanelListen); err == nil {
			if n, err := strconv.Atoi(p); err == nil {
				return n
			}
		}
		if strings.HasPrefix(s.PanelListen, ":") {
			if n, err := strconv.Atoi(strings.TrimPrefix(s.PanelListen, ":")); err == nil {
				return n
			}
		}
	}
	return 0
}

func (s *Server) handleGetPanel(w http.ResponseWriter, r *http.Request) {
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
	port := cfg.Panel.Port
	if p := s.panelPort(); p > 0 {
		port = p
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":          true,
		"domain":      cfg.Panel.Domain,
		"port":        port,
		"listen":      s.PanelListen,
		"parent_site": cfg.PanelParentSite(),
		"upstream":    s.panelUpstreamDial(),
	})
}

func (s *Server) handlePutPanelDomain(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		Parent string `json:"parent"`
		Label  string `json:"label"`
		Domain string `json:"domain"`
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

	parent := strings.ToLower(strings.TrimSpace(body.Parent))
	label := strings.ToLower(strings.TrimSpace(body.Label))
	domain := strings.ToLower(strings.TrimSpace(body.Domain))

	switch {
	case parent != "" && label != "":
		domain, err = config.BuildPanelDomain(label, parent)
		if err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
	case domain != "":
		tmp := *cfg
		tmp.Panel.Domain = domain
		parent = strings.ToLower(tmp.PanelParentSite())
		if parent == "" {
			s.writeErr(w, 400, "domain must be a subdomain of an existing apex site")
			return
		}
		suffix := "." + parent
		if !strings.HasSuffix(domain, suffix) || domain == parent {
			s.writeErr(w, 400, "domain must be a subdomain of an existing apex site")
			return
		}
		label = strings.TrimSuffix(domain, suffix)
		if _, err := config.BuildPanelDomain(label, parent); err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
	default:
		s.writeErr(w, 400, "parent and label are required (or pass domain)")
		return
	}

	parentSite, _, ok := cfg.FindSite(parent)
	if !ok {
		s.writeErr(w, 404, "parent site not found")
		return
	}
	if strings.TrimSpace(parentSite.Parent) != "" {
		s.writeErr(w, 400, "parent must be an apex site")
		return
	}
	if name, taken := cfg.HostnameTaken(domain); taken {
		s.writeErr(w, 409, "hostname is already used by site "+name)
		return
	}

	cfg.Panel.Domain = domain
	if p := s.panelPort(); p > 0 {
		cfg.Panel.Port = p
	}
	if err := cfg.Validate(); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	warnings := []string{}
	if err := s.applyCaddy(cfg); err != nil {
		warnings = append(warnings, "caddy reload: "+err.Error())
	}
	out := map[string]any{
		"ok":          true,
		"domain":      cfg.Panel.Domain,
		"port":        cfg.Panel.Port,
		"parent_site": cfg.PanelParentSite(),
		"label":       label,
	}
	if len(warnings) > 0 {
		out["warning"] = strings.Join(warnings, "; ")
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) handleDeletePanelDomain(w http.ResponseWriter, r *http.Request) {
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
	cfg.Panel.Domain = ""
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	// Best-effort: may fail when no sites remain to render.
	_ = s.applyCaddy(cfg)
	s.writeJSON(w, 200, map[string]any{"ok": true, "domain": ""})
}

func (s *Server) handlePanelDNS(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	domain := strings.ToLower(strings.TrimSpace(cfg.Panel.Domain))
	if domain == "" {
		s.writeErr(w, 400, "panel domain is not set")
		return
	}

	expectedA := append([]string{}, s.ExpectedA...)
	if q := strings.TrimSpace(r.URL.Query().Get("a")); q != "" {
		expectedA = strings.Split(q, ",")
	}
	expectedAAAA := append([]string{}, s.ExpectedAAAA...)
	requireCAA := r.URL.Query().Get("caa") != "0"

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	rep, err := dnscheck.CheckAuthoritative(ctx, s.resolver(), dnscheck.Expect{
		Host:       domain,
		A:          expectedA,
		AAAA:       expectedAAAA,
		RequireCAA: requireCAA,
	})
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":                    true,
		"domain":                domain,
		"parent_site":           cfg.PanelParentSite(),
		"can_issue_certificate": rep.OK && dnscheck.CanIssueCertificate(rep),
		"expected_a":            expectedA,
		"reports":               []*dnscheck.Report{rep},
	})
}
