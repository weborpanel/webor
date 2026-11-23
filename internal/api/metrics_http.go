package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/metrics"
	"github.com/weborpanel/webor/internal/version"
)

// MountMetrics registers dashboard, metrics history and activity routes.
func (s *Server) MountMetrics(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/overview", s.handleOverview)
	mux.HandleFunc("GET /api/metrics/host", s.handleHostMetrics)
	mux.HandleFunc("GET /api/sites/{name}/metrics", s.handleSiteMetrics)
	mux.HandleFunc("GET /api/audit", s.handleAudit)
}

func (s *Server) metricsStore() *metrics.Store {
	return &metrics.Store{DB: s.Account.DB}
}

func (s *Server) hostCollector() *metrics.HostCollector {
	if s.Host != nil {
		return s.Host
	}
	return &metrics.HostCollector{}
}

type overviewSite struct {
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	Parent         string          `json:"parent,omitempty"`
	HTTPS          bool            `json:"https"`
	SocketProxy    bool            `json:"socket_proxy,omitempty"`
	DatabaseCount  int             `json:"database_count"`
	Cache          bool            `json:"cache"`
	SubdomainCount int             `json:"subdomain_count,omitempty"`
	Sampled        bool            `json:"sampled"`
	Active         bool            `json:"active"`
	CPUPct         float64         `json:"cpu_pct"`
	MemBytes       int64           `json:"mem_bytes"`
	DiskBytes      int64           `json:"disk_bytes"`
	MemoryMax      string          `json:"memory_max,omitempty"`
	MemLimitBytes  int64           `json:"mem_limit_bytes,omitempty"`
	DiskQuota      string          `json:"disk_quota,omitempty"`
	DiskQuotaBytes int64           `json:"disk_quota_bytes,omitempty"`
	Spark          *metrics.Series `json:"spark,omitempty"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
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

	host := s.hostCollector().Static()
	latest, sampled := s.Sampler.LatestHost()
	if sampled {
		host.CPUPct = latest.CPUPct
		host.NetRx = latest.NetRx
		host.NetTx = latest.NetTx
	}

	now := time.Now()
	sparks, err := s.metricsStore().AllSiteSeries(metrics.SparkRange, now)
	if err != nil {
		sparks = nil
	}
	hostSpark, err := s.metricsStore().HostSeries(metrics.SparkRange, now)
	if err != nil {
		hostSpark = nil
	}

	sites := make([]overviewSite, 0, len(cfg.Sites))
	for _, site := range cfg.Sites {
		name := strings.ToLower(site.Name)
		v := overviewSite{
			Name:           site.Name,
			Type:           site.Type,
			Parent:         site.Parent,
			HTTPS:          site.HTTPS,
			SocketProxy:    site.SocketProxy,
			DatabaseCount:  len(site.Databases),
			Cache:          site.Cache != "",
			MemoryMax:      site.Limits.MemoryMax,
			MemLimitBytes:  metrics.ParseSize(site.Limits.MemoryMax),
			DiskQuota:      site.Limits.DiskQuota,
			DiskQuotaBytes: metrics.ParseSize(site.Limits.DiskQuota),
		}
		if strings.TrimSpace(site.Parent) == "" {
			v.SubdomainCount = len(cfg.ChildrenOf(site.Name))
		}
		if row, ok := s.Sampler.LatestSite(name); ok {
			v.Sampled = true
			v.Active = row.Active
			v.CPUPct = row.CPUPct
			v.MemBytes = row.MemBytes
			v.DiskBytes = row.DiskBytes
		}
		if sp, ok := sparks[name]; ok {
			v.Spark = sp
		}
		sites = append(sites, v)
	}

	s.writeJSON(w, 200, map[string]any{
		"ok":                true,
		"sampled":           sampled,
		"host":              host,
		"host_spark":        hostSpark,
		"sites":             sites,
		"pending_approvals": s.pendingApprovalCount(),
		"version":           version.Version,
		"https_disabled":    s.DisableHTTPS,
	})
}

func (s *Server) pendingApprovalCount() int {
	st := &auth.Store{DB: s.Account.DB}
	list, err := st.ListPending()
	if err != nil {
		return 0
	}
	n := 0
	for _, a := range list {
		if a.ApprovedBy == "" {
			n++
		}
	}
	return n
}

func (s *Server) handleHostMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	rng, ok := metrics.ParseRange(r.URL.Query().Get("range"))
	if !ok {
		s.writeErr(w, 400, "range must be 1h, 24h or 7d")
		return
	}
	ser, err := s.metricsStore().HostSeries(rng, time.Now())
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "metrics": ser})
}

func (s *Server) handleSiteMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	rng, ok := metrics.ParseRange(r.URL.Query().Get("range"))
	if !ok {
		s.writeErr(w, 400, "range must be 1h, 24h or 7d")
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
	ser, err := s.metricsStore().SiteSeries(name, rng, time.Now())
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":               true,
		"metrics":          ser,
		"mem_limit_bytes":  metrics.ParseSize(site.Limits.MemoryMax),
		"disk_quota_bytes": metrics.ParseSize(site.Limits.DiskQuota),
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	events, err := audit.List(s.Account.DB, before, limit)
	if err != nil {
		if config.IsCorrupt(err) {
			s.writeErr(w, 500, "activity database is corrupted (state.db). On the server: systemctl stop webor webor-helper && webor -repair-state && systemctl start webor-helper webor")
			return
		}
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "events": events})
}
