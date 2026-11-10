package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/dnscheck"
)

func (s *Server) resolver() dnscheck.Resolver {
	if s.DNS != nil {
		return s.DNS
	}
	return &dnscheck.System{}
}

func (s *Server) handleSiteDNS(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
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
	var aliases []string
	for _, site := range cfg.Sites {
		if strings.EqualFold(site.Name, name) {
			found = true
			aliases = site.Aliases
			name = strings.ToLower(site.Name)
			break
		}
	}
	if !found {
		s.writeErr(w, 404, "site not found")
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

	hosts := append([]string{name}, aliases...)
	reports := make([]*dnscheck.Report, 0, len(hosts))
	allOK := true
	for _, host := range hosts {
		rep, err := dnscheck.CheckAuthoritative(ctx, s.resolver(), dnscheck.Expect{
			Host:       host,
			A:          expectedA,
			AAAA:       expectedAAAA,
			RequireCAA: requireCAA,
		})
		if err != nil {
			s.writeErr(w, 500, err.Error())
			return
		}
		reports = append(reports, rep)
		if !rep.OK {
			allOK = false
		}
	}

	s.writeJSON(w, 200, map[string]any{
		"ok":                   true,
		"can_issue_certificate": allOK && dnscheck.CanIssueCertificate(reports[0]),
		"expected_a":           expectedA,
		"reports":              reports,
	})
}
