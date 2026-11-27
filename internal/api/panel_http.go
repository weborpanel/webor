package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/dnscheck"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/panelenv"
	"github.com/weborpanel/webor/internal/providers/caddy"
)

// MountPanel registers /api/panel routes (auth required).
func (s *Server) MountPanel(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/panel", s.handleGetPanel)
	mux.HandleFunc("PUT /api/panel/domain", s.handlePutPanelDomain)
	mux.HandleFunc("PUT /api/panel/timezone", s.handlePutPanelTimezone)
	mux.HandleFunc("DELETE /api/panel/domain", s.handleDeletePanelDomain)
	mux.HandleFunc("POST /api/panel/listen", s.handlePostPanelListen)
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

func (s *Server) panelListenHost() string {
	addr := strings.TrimSpace(s.PanelListen)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	if strings.HasPrefix(addr, ":") {
		return ""
	}
	return addr
}

func (s *Server) panelPublicListen() bool {
	host := s.panelListenHost()
	return host == "" || host == "0.0.0.0" || host == "::" || host == "[::]"
}

// setPanelListenHost rewrites panel.env and restarts webor via helper (or PanelEnvPath in tests).
func (s *Server) setPanelListenHost(host string) (listenAddr string, err error) {
	if err := panelenv.ValidateListenHost(host); err != nil {
		return "", err
	}
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.SetPanelListenHostArgs{Host: host})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpSetPanelListenHost, Args: raw,
		})
		if err != nil {
			return "", err
		}
		if !resp.OK {
			return "", fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			Listen string `json:"listen"`
		}
		_ = json.Unmarshal([]byte(resp.Message), &out)
		if out.Listen != "" {
			return out.Listen, nil
		}
		return host, nil
	}
	envPath := strings.TrimSpace(s.PanelEnvPath)
	if envPath == "" {
		return "", fmt.Errorf("helper socket not configured")
	}
	return panelenv.SetListenHost(envPath, host)
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
	host := s.panelListenHost()
	s.writeJSON(w, 200, map[string]any{
		"ok":               true,
		"domain":           cfg.Panel.Domain,
		"port":             port,
		"listen":           s.PanelListen,
		"listen_host":      host,
		"public_listen":    s.panelPublicListen(),
		"parent_site":      cfg.PanelParentSite(),
		"upstream":         s.panelUpstreamDial(),
		"timezone":         config.EffectiveTimezone(cfg),
		"timezone_config":  strings.TrimSpace(cfg.Panel.Timezone),
		"timezone_host":    config.HostTimezone(),
	})
}

func (s *Server) handlePutPanelTimezone(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	tz := strings.TrimSpace(body.Timezone)
	if tz != "" {
		if err := config.ValidateTimezone(tz); err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	cfg.Panel.Timezone = tz
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}

	// Refresh cron + backup timers so OnCalendar picks up the new zone.
	for _, site := range cfg.Sites {
		for _, job := range site.Cron {
			if strings.TrimSpace(job.Schedule) == "" {
				continue
			}
			_ = s.installCronJob(site, job)
		}
	}
	_ = s.ensureBackupSchedule()

	s.writeJSON(w, 200, map[string]any{
		"ok":              true,
		"timezone":        config.EffectiveTimezone(cfg),
		"timezone_config": tz,
		"timezone_host":   config.HostTimezone(),
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
	warnings := []string{}
	// Restore public high-port fallback so operators can reach the panel by IP again.
	if _, err := s.setPanelListenHost(panelenv.HostPublic); err != nil {
		warnings = append(warnings, "restore public listen: "+err.Error())
	}
	out := map[string]any{"ok": true, "domain": "", "public_listen": true}
	if len(warnings) > 0 {
		out["warning"] = strings.Join(warnings, "; ")
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) handlePostPanelListen(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		Host string `json:"host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	host := strings.TrimSpace(body.Host)
	// Panel UI may only lock to localhost. Re-exposing 0.0.0.0 is Disable or root CLI.
	if host != panelenv.HostLocal {
		s.writeErr(w, 400, "host must be "+panelenv.HostLocal+" (use Disable panel access or wbr panel listen to restore public bind)")
		return
	}

	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if strings.TrimSpace(cfg.Panel.Domain) == "" {
		s.writeErr(w, 400, "panel domain is not set; enable Panel access first")
		return
	}

	listenAddr, err := s.setPanelListenHost(host)
	if err != nil {
		s.writeErr(w, 502, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":            true,
		"listen":        listenAddr,
		"listen_host":   host,
		"public_listen": false,
		"domain":        cfg.Panel.Domain,
		"restarting":    true,
	})
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

func (s *Server) ensureBackupSchedule() error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.EnsureBackupScheduleArgs{ConfigPath: s.configPath()})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpEnsureBackupSchedule, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	cfg, err := config.Load(s.configPath())
	if err != nil {
		return err
	}
	return s.ops().EnsureBackupSchedule(cfg)
}
