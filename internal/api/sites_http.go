package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/dnscheck"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/reconcile"
)

// DefaultConfigPath is the production desired-state file.
const DefaultConfigPath = "/etc/webor/config.yaml"

// sitesMu serializes config.yaml read-modify-write from the panel.
var sitesMu sync.Mutex

func (s *Server) configPath() string {
	if s.ConfigPath != "" {
		return s.ConfigPath
	}
	return DefaultConfigPath
}

func (s *Server) ops() *helper.Ops {
	if s.Ops != nil {
		return s.Ops
	}
	return helper.DefaultOps()
}

// siteFS returns IPC-backed Ops when HelperSocket is set (production panel user).
func (s *Server) siteFS() helper.SiteFS {
	if s.HelperSocket != "" {
		return &ipc.SiteClient{Socket: s.HelperSocket}
	}
	return s.ops()
}

// MountSites registers /api/sites routes (auth required).
func (s *Server) MountSites(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites", s.handleListSites)
	mux.HandleFunc("POST /api/sites", s.handleAddSite)
	mux.HandleFunc("DELETE /api/sites/{name}", s.handleDeleteSite)
	mux.HandleFunc("PUT /api/sites/{name}/files/{path...}", s.handlePutSiteFile)
	mux.HandleFunc("GET /api/sites/{name}/dns", s.handleSiteDNS)
	mux.HandleFunc("POST /api/sites/{name}/https", s.handleSiteHTTPS)
	mux.HandleFunc("GET /api/sites/{name}/logs", s.handleSiteLogs)
	mux.HandleFunc("GET /api/sites/{name}/log-files", s.handleListSiteLogFiles)
	mux.HandleFunc("GET /api/sites/{name}/log-files/{file}", s.handleReadSiteLogFile)
	mux.HandleFunc("GET /api/system/log-files", s.handleListHostLogFiles)
	mux.HandleFunc("GET /api/system/log-files/{file}", s.handleReadHostLogFile)
	mux.HandleFunc("GET /api/sites/{name}/resources", s.handleSiteResources)
	mux.HandleFunc("POST /api/sites/{name}/deploy", s.handleDeployBinary)
	mux.HandleFunc("POST /api/sites/{name}/deploy/git", s.handleDeployGit)
	mux.HandleFunc("POST /api/sites/{name}/deploy-zip", s.handleDeployAppZip)
	s.mountFSRoutes(mux)
	s.mountDatabaseRoutes(mux)
	s.mountCacheRoutes(mux)
	s.mountBackendRoutes(mux)
	s.mountCronRoutes(mux)
	s.mountProcessRoutes(mux)
	s.mountRuntimeRoutes(mux)
	s.mountTokenRoutes(mux)
	s.mountLimitsRoutes(mux)
	s.mountDBAdminer(mux)
}

// requireAuth requires an admin session cookie. Deploy tokens are rejected
// (they must use requireSiteScope / requireSiteToken on site routes).
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.sessionOK(r) == nil {
		s.writeErr(w, 401, "unauthorized")
		return false
	}
	return true
}

// requireSiteToken allows a session or any valid deploy token bound to site.
func (s *Server) requireSiteToken(w http.ResponseWriter, r *http.Request, site string) bool {
	if s.sessionOK(r) != nil {
		return true
	}
	tok := s.deployTokenOK(r)
	if tok == nil {
		s.writeErr(w, 401, "unauthorized")
		return false
	}
	if !strings.EqualFold(tok.Site, site) {
		s.writeErr(w, 403, "token not valid for this site")
		return false
	}
	return true
}

// requireSiteScope allows a session or a deploy token for site with the given scope.
func (s *Server) requireSiteScope(w http.ResponseWriter, r *http.Request, site, scope string) bool {
	if s.sessionOK(r) != nil {
		return true
	}
	tok := s.deployTokenOK(r)
	if tok == nil {
		s.writeErr(w, 401, "unauthorized")
		return false
	}
	if !strings.EqualFold(tok.Site, site) {
		s.writeErr(w, 403, "token not valid for this site")
		return false
	}
	if !tok.HasScope(scope) {
		s.writeErr(w, 403, "token missing scope: "+scope)
		return false
	}
	return true
}

func (s *Server) loadOrEmptyConfig() (*config.Config, error) {
	path := s.configPath()
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return &config.Config{Version: 1}, nil
	}
	return nil, err
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request) {
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
	type dbView struct {
		ID     string `json:"id"`
		Engine string `json:"engine"`
	}
	type backendViewJSON struct {
		Enabled   bool     `json:"enabled"`
		ID        string   `json:"id,omitempty"`
		Prefix    string   `json:"prefix,omitempty"`
		Paths     []string `json:"paths,omitempty"`
		WebSocket bool     `json:"websocket,omitempty"`
		Port      int      `json:"port,omitempty"`
		Exec      string   `json:"exec,omitempty"`
	}
	type runtimeViewJSON struct {
		Kind    string `json:"kind,omitempty"`
		Version string `json:"version,omitempty"`
		Command string `json:"command,omitempty"`
	}
	type siteView struct {
		Name           string           `json:"name"`
		Type           string           `json:"type"`
		Root           string           `json:"root"`
		Parent         string           `json:"parent,omitempty"`
		Exec           string           `json:"exec,omitempty"`
		Database       string           `json:"database,omitempty"` // legacy: first engine
		Databases      []dbView         `json:"databases,omitempty"`
		Cache          string           `json:"cache,omitempty"`
		HTTPS          bool             `json:"https,omitempty"`
		Backend        *backendViewJSON  `json:"backend,omitempty"` // first backend (compat)
		Backends       []backendViewJSON `json:"backends,omitempty"`
		Runtime        *runtimeViewJSON  `json:"runtime,omitempty"`
		SubdomainCount int              `json:"subdomain_count,omitempty"`
		SocketProxy    bool             `json:"socket_proxy,omitempty"`
		Cron           []string         `json:"cron,omitempty"`
		Jobs           []string         `json:"jobs,omitempty"`
	}
	out := make([]siteView, 0, len(cfg.Sites))
	for _, site := range cfg.Sites {
		root := site.Root
		if root == "" {
			root = "public"
		}
		v := siteView{
			Name:        site.Name,
			Type:        site.Type,
			Root:        root,
			Parent:      site.Parent,
			Exec:        site.Exec,
			Cache:       site.Cache,
			HTTPS:       site.HTTPS,
			SocketProxy: site.SocketProxy,
		}
		if site.Runtime != nil {
			config.NormalizeRuntime(site.Runtime)
			v.Runtime = &runtimeViewJSON{
				Kind:    site.Runtime.Kind,
				Version: site.Runtime.Version,
				Command: site.Runtime.Command,
			}
		}
		for _, d := range site.Databases {
			v.Databases = append(v.Databases, dbView{ID: d.ID, Engine: d.Engine})
		}
		if len(site.Databases) > 0 {
			v.Database = site.Databases[0].Engine
		}
		if strings.TrimSpace(site.Parent) == "" {
			v.SubdomainCount = len(cfg.ChildrenOf(site.Name))
		}
		for i := range site.Backends {
			b := site.Backends[i]
			if !b.Enabled {
				continue
			}
			config.NormalizeBackend(&b)
			row := backendViewJSON{
				Enabled:   true,
				ID:        b.ID,
				Prefix:    b.Prefix,
				Paths:     b.BackendPathPrefixes(),
				WebSocket: b.WebSocket,
				Port:      b.Port,
				Exec:      b.Exec,
			}
			v.Backends = append(v.Backends, row)
			if v.Backend == nil {
				cp := row
				v.Backend = &cp
			}
		}
		for _, c := range site.Cron {
			v.Cron = append(v.Cron, c.Name)
		}
		for _, j := range site.Jobs {
			v.Jobs = append(v.Jobs, j.Name)
		}
		out = append(out, v)
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "sites": out})
}

func (s *Server) handleAddSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	if body.Type == "" {
		body.Type = config.SiteTypeStatic
	}
	site := config.Site{
		Name: body.Name,
		Type: body.Type,
		Root: "public",
		Limits: config.Limits{
			DiskQuota: config.DefaultDiskQuota,
		},
	}
	if body.Type == config.SiteTypeApp {
		site.Root = ""
		site.Exec = "./bin/server"
		site.SocketActivation = true
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	for _, existing := range cfg.Sites {
		if strings.EqualFold(existing.Name, site.Name) {
			s.writeErr(w, 409, "site already exists")
			return
		}
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Panel.Domain), strings.ToLower(strings.TrimSpace(site.Name))) {
		s.writeErr(w, 409, "hostname is reserved for the panel")
		return
	}
	cfg.Sites = append(cfg.Sites, site)
	if err := cfg.Validate(); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	warnings := []string{}
	if err := s.siteFS().CreateSiteUser(site.Name); err != nil {
		warnings = append(warnings, "CreateSiteUser: "+err.Error())
	} else if err := s.applyCaddy(cfg); err != nil {
		warnings = append(warnings, "caddy reload: "+err.Error())
	}
	out := map[string]any{"ok": true, "name": site.Name, "root": "public"}
	if len(warnings) > 0 {
		out["warning"] = strings.Join(warnings, "; ")
	}
	s.writeJSON(w, 200, out)
}

// caddyApplyOptions builds Caddy listen/TLS options from panel flags + site https: markers.
func (s *Server) caddyApplyOptions(cfg *config.Config) reconcile.StaticApplyOptions {
	opt := reconcile.StaticApplyOptions{
		CaddyConfigPath: s.CaddyConfigPath,
		HTTPListen:      s.HTTPListen,
		DisableHTTPS:    s.DisableHTTPS,
		PanelUpstream:   s.panelUpstreamDial(),
		SeedIndex:       true,
	}
	if opt.CaddyConfigPath == "" {
		opt.CaddyConfigPath = "/etc/webor/caddy.json"
	}
	if opt.HTTPListen == "" {
		opt.HTTPListen = ":8080"
	}
	// Lab / forced off stays HTTP. Otherwise enable HTTPS when any site opted in after DNS green.
	anyHTTPS := false
	if cfg != nil {
		for _, site := range cfg.Sites {
			if site.HTTPS {
				anyHTTPS = true
				break
			}
		}
	}
	if s.DisableHTTPS || !anyHTTPS {
		opt.DisableHTTPS = true
	} else {
		opt.DisableHTTPS = false
		if opt.HTTPListen == ":8080" {
			opt.HTTPListen = ":443"
		}
	}
	return opt
}

func (s *Server) applyCaddy(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return reconcile.ApplyStaticSites(ctx, cfg, s.siteFS(), s.caddyApplyOptions(cfg))
}

func (s *Server) handleSiteHTTPS(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Enable bool `json:"enable"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	if s.DisableHTTPS && body.Enable {
		s.writeJSON(w, 400, map[string]any{
			"ok":    false,
			"error": "panel started with HTTPS disabled (lab mode); omit -disable-https and set -expected-a on a real VPS",
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

	if body.Enable {
		if len(s.ExpectedA) == 0 {
			s.writeErr(w, 400, "set panel ExpectedA (VPS public IP) before enabling HTTPS")
			return
		}
		// Re-check DNS quickly.
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		rep, err := dnscheck.CheckAuthoritative(ctx, s.resolver(), dnscheck.Expect{
			Host:       cfg.Sites[idx].Name,
			A:          s.ExpectedA,
			AAAA:       s.ExpectedAAAA,
			RequireCAA: true,
		})
		if err != nil {
			s.writeErr(w, 500, err.Error())
			return
		}
		if !rep.OK || !dnscheck.CanIssueCertificate(rep) {
			s.writeJSON(w, 400, map[string]any{
				"ok":    false,
				"error": "DNS not ready for certificate issuance",
				"report": rep,
			})
			return
		}
	}

	cfg.Sites[idx].HTTPS = body.Enable
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if err := s.applyCaddy(cfg); err != nil {
		s.writeErr(w, 500, "caddy reload: "+err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":            true,
		"https":         body.Enable,
		"disable_https": s.DisableHTTPS,
	})
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
		return
	}
	deleteData := r.URL.Query().Get("delete_data") == "1" || r.URL.Query().Get("delete_data") == "true"
	approvalID := strings.TrimSpace(r.URL.Query().Get("approval_id"))

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, found := cfg.FindSite(name)
	if !found {
		s.writeErr(w, 404, "site not found")
		return
	}

	var removedChildren []string
	for _, child := range cfg.ChildrenOf(name) {
		removedChildren = append(removedChildren, child.Name)
	}
	var dropDBs []ipc.DropDatabaseRef
	if strings.TrimSpace(site.Parent) == "" {
		for _, db := range site.Databases {
			dropDBs = append(dropDBs, ipc.DropDatabaseRef{ID: db.ID, Engine: db.Engine})
		}
	}

	// Full wipe (files + DBs + subdomains). Production helper requires step-up approval.
	if !deleteData {
		s.writeJSON(w, 403, map[string]any{
			"ok":                 false,
			"error":              "deleting a site removes files, databases, and subdomains",
			"approval_required":  true,
			"approval_operation": ipc.OpDeleteSiteUser,
			"approval_args": map[string]any{
				"domain":      name,
				"delete_data": true,
			},
			"removed_children": removedChildren,
			"drop_databases":   dropDBs,
			"hint":             "Confirm wipe, then approve DeleteSiteUser",
		})
		return
	}
	if s.HelperSocket != "" && approvalID == "" {
		s.writeJSON(w, 403, map[string]any{
			"ok":                 false,
			"error":              "delete requires step-up approval",
			"approval_required":  true,
			"approval_operation": ipc.OpDeleteSiteUser,
			"approval_args": map[string]any{
				"domain":      name,
				"delete_data": true,
			},
			"removed_children": removedChildren,
			"drop_databases":   dropDBs,
			"hint":             "POST /api/approvals with operation DeleteSiteUser, then: wbr approve <id>",
		})
		return
	}
	hadCache := strings.TrimSpace(site.Parent) == "" && strings.TrimSpace(site.Cache) != ""
	s.removeAllHTTPBackends(site)
	for _, child := range cfg.ChildrenOf(name) {
		s.removeAllHTTPBackends(child)
	}
	if err := s.purgeSite(name, removedChildren, dropDBs, approvalID); err != nil {
		s.writeErr(w, 403, err.Error())
		return
	}
	if hadCache {
		_ = s.dropSiteCache(name)
	}

	filtered := cfg.Sites[:0]
	for _, st := range cfg.Sites {
		if strings.EqualFold(st.Name, name) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(st.Parent), name) {
			continue
		}
		filtered = append(filtered, st)
	}
	cfg.Sites = filtered
	cfg.ClearPanelDomainIfOwned(name)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.rebuildDBAdminer()
	_ = s.applyCaddy(cfg)
	s.writeJSON(w, 200, map[string]any{
		"ok":               true,
		"delete_data":      true,
		"removed_children": removedChildren,
		"drop_databases":   dropDBs,
	})
}

func (s *Server) purgeSite(domain string, children []string, dropDBs []ipc.DropDatabaseRef, approvalID string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeleteSiteUserArgs{
			Domain:         domain,
			DeleteData:     true,
			ApprovalID:     approvalID,
			CascadeDomains: children,
			DropDatabases:  dropDBs,
			StateDir:       s.stateDir(),
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeleteSiteUser, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	// Tests / no helper: best-effort local purge (may no-op if users missing).
	_ = s.ops().DeleteSiteUser(domain, true)
	for _, child := range children {
		_ = s.ops().DeleteSiteUser(child, true)
	}
	for _, db := range dropDBs {
		_ = s.ops().DropSiteDatabase(domain, db.Engine, db.ID, s.stateDir())
	}
	return nil
}

func (s *Server) deleteSiteUser(domain string, deleteData bool, approvalID string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeleteSiteUserArgs{
			Domain:     domain,
			DeleteData: deleteData,
			ApprovalID: approvalID,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeleteSiteUser, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	if deleteData {
		return fmt.Errorf("delete_data requires helper IPC (set -helper-socket)")
	}
	return s.ops().DeleteSiteUser(domain, false)
}

func (s *Server) handlePutSiteFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	rel := strings.TrimSpace(r.PathValue("path"))
	if name == "" || rel == "" {
		s.writeErr(w, 400, "name and path are required")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxPublicFileBytes+1))
	if err != nil {
		s.writeErr(w, 400, "read body")
		return
	}
	if len(data) > maxPublicFileBytes {
		s.writeErr(w, 400, "file exceeds 32 MiB")
		return
	}
	if err := s.siteFS().WritePublicFile(name, rel, data); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": rel})
}
