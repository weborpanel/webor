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
	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/mariadb"
	"github.com/weborpanel/webor/internal/providers/postgres"
	"github.com/weborpanel/webor/internal/reconcile"
)

func (s *Server) stateDir() string {
	if s.StateDir != "" {
		return s.StateDir
	}
	return config.DefaultStateDir
}

func (s *Server) mountDatabaseRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/databases", s.handleListDatabases)
	mux.HandleFunc("POST /api/sites/{name}/databases", s.handleAddDatabase)
	mux.HandleFunc("DELETE /api/sites/{name}/databases/{id}", s.handleDeleteDatabase)
	mux.HandleFunc("POST /api/sites/{name}/databases/{id}/rotate-password", s.handleRotateDatabasePassword)
	mux.HandleFunc("POST /api/sites/{name}/subdomains", s.handleAddSubdomain)
	mux.HandleFunc("DELETE /api/sites/{name}/subdomains/{child}", s.handleDeleteSubdomain)
	// Legacy singular routes (first/default DB).
	mux.HandleFunc("GET /api/sites/{name}/database", s.handleGetDatabaseLegacy)
	mux.HandleFunc("POST /api/sites/{name}/database", s.handleAttachDatabaseLegacy)
	mux.HandleFunc("DELETE /api/sites/{name}/database", s.handleDetachDatabaseLegacy)
}

func (s *Server) handleListDatabases(w http.ResponseWriter, r *http.Request) {
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
	pgSt, _ := s.postgresStatus()
	mariaSt, _ := s.mariaDBStatus()
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
	list := make([]map[string]any, 0, len(site.Databases))
	for _, db := range site.Databases {
		// Never return passwords on list — only on create / rotate responses.
		list = append(list, s.databaseView(parentName, db.ID, db.Engine, false))
	}
	children := cfg.ChildrenOf(parentName)
	childNames := make([]string, 0, len(children))
	for _, c := range children {
		childNames = append(childNames, c.Name)
	}
	info := distro.Detect()
	s.writeJSON(w, 200, map[string]any{
		"ok":                 true,
		"parent":             parentName,
		"inherited":          inherited,
		"default_id":         config.DefaultDBID(parentName),
		"databases":          list,
		"subdomains":         childNames,
		"browse_path":        "/db/",
		"postgres":           pgSt,
		"mariadb":            mariaSt,
		"engines":            []string{dbengine.SQLite, dbengine.Postgres, dbengine.MariaDB},
		"postgres_install_hint": info.InstallHintPostgres(),
		"postgres_start_hint":   info.StartHintPostgres(),
		"mariadb_install_hint":  info.InstallHintMariaDB(),
		"mariadb_start_hint":    info.StartHintMariaDB(),
		"os_family":             string(info.Family),
	})
}

func (s *Server) databaseView(domain, id, engine string, withSecrets bool) map[string]any {
	id = dbengine.NormalizeID(domain, id)
	out := map[string]any{
		"id":     id,
		"engine": engine,
	}
	switch engine {
	case dbengine.Postgres:
		app := postgres.AppRole(domain, id)
		ro := postgres.ReadOnlyRole(domain, id)
		out["database"] = postgres.DBName(domain, id)
		out["username"] = app
		out["peer_role"] = postgres.PeerRole(domain)
		out["readonly_role"] = ro
		out["peer_dsn"] = postgres.PeerDSN(domain, id)
		out["peer_url"] = postgres.PeerURL(domain, id)
		secretPath := postgres.ResolveSecretPath(s.stateDir(), domain, id)
		_, secretErr := os.Stat(secretPath)
		out["secret_ready"] = secretErr == nil
		if withSecrets && secretErr == nil {
			if sec, err := postgres.ReadSecret(secretPath); err == nil {
				out["password"] = sec.Password
			}
		}
	case dbengine.MariaDB:
		app := mariadb.AppRole(domain, id)
		ro := mariadb.ReadOnlyRole(domain, id)
		out["database"] = mariadb.DBName(domain, id)
		out["username"] = app
		out["peer_role"] = mariadb.PeerRole(domain)
		out["readonly_role"] = ro
		out["peer_dsn"] = mariadb.PeerDSN(domain, id)
		out["peer_url"] = mariadb.PeerURL(domain, id)
		secretPath := mariadb.ResolveSecretPath(s.stateDir(), domain, id)
		_, secretErr := os.Stat(secretPath)
		out["secret_ready"] = secretErr == nil
		if withSecrets && secretErr == nil {
			if sec, err := mariadb.ReadSecret(secretPath); err == nil {
				out["password"] = sec.Password
			}
		}
	case dbengine.SQLite:
		path := dbengine.SQLitePath(domain, id)
		_, err := os.Stat(path)
		out["database"] = filepathBase(path)
		out["path"] = path
		out["peer_dsn"] = dbengine.SQLiteDSN(domain, id)
		out["peer_url"] = "sqlite://" + path
		out["secret_ready"] = err == nil
	}
	return out
}

func filepathBase(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return p
	}
	return p[i+1:]
}

func (s *Server) handleAddDatabase(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		ID         string `json:"id"`
		Engine     string `json:"engine"`
		ApprovalID string `json:"approval_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	engine, err := dbengine.Normalize(body.Engine)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	id := config.NormalizeDBID(name, body.ID)

	if dbengine.NeedsApproval(engine) {
		st, err := s.engineStatus(engine)
		if err != nil {
			s.writeErr(w, 500, err.Error())
			return
		}
		if !st.Installed {
			// Package install stays out of the panel approval flow (same as Valkey):
			// an operator installs as root, then returns here to Add + Ensure approval.
			s.writeJSON(w, 409, s.engineInstallRequiredPayload(engine, false))
			return
		}
		if !st.Active {
			s.writeJSON(w, 409, s.engineInstallRequiredPayload(engine, true))
			return
		}
		if body.ApprovalID == "" {
			s.writeJSON(w, 403, map[string]any{
				"ok":                 false,
				"error":              engine + " attach requires approval",
				"approval_required":  true,
				"approval_operation": ipc.OpEnsureSiteDatabase,
				"approval_args":      map[string]string{"domain": name, "engine": engine, "id": id},
				"hint":               "Root approval required to attach " + engine,
			})
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
	_, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if strings.TrimSpace(cfg.Sites[idx].Parent) != "" {
		s.writeErr(w, 400, "child sites cannot declare databases (use parent)")
		return
	}
	if _, exists := cfg.Sites[idx].FindDatabase(id); exists {
		s.writeErr(w, 409, "database id already exists")
		return
	}
	if err := s.siteFS().CreateSiteUser(name); err != nil {
		s.writeErr(w, 500, "CreateSiteUser: "+err.Error())
		return
	}
	res, err := s.ensureSiteDatabase(name, engine, id, body.ApprovalID)
	if err != nil {
		s.writeErr(w, 403, err.Error())
		return
	}
	cfg.Sites[idx].Databases = append(cfg.Sites[idx].Databases, config.SiteDatabase{ID: id, Engine: engine})
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.rebuildDBAdminer()
	out := s.databaseView(name, id, engine, true)
	out["ok"] = true
	out["browse_path"] = "/db/"
	if res != nil {
		out["database"] = res.Database
		out["peer_role"] = res.PeerRole
		out["app_role"] = res.AppRole
		out["readonly_role"] = res.ReadOnly
		out["peer_dsn"] = res.PeerDSN
		out["peer_url"] = res.PeerURL
		if res.Path != "" {
			out["path"] = res.Path
		}
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))
	var body struct {
		ApprovalID string `json:"approval_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

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
		s.writeErr(w, 400, "child sites cannot remove databases")
		return
	}
	db, found := cfg.Sites[idx].FindDatabase(id)
	if !found {
		s.writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	id = db.ID
	engine := db.Engine
	if dbengine.NeedsApproval(engine) && body.ApprovalID == "" {
		s.writeJSON(w, 403, map[string]any{
			"ok":                 false,
			"error":              engine + " remove requires approval",
			"approval_required":  true,
			"approval_operation": ipc.OpDropSiteDatabase,
			"approval_args":      map[string]string{"domain": name, "engine": engine, "id": id},
			"hint":               "Root approval required to remove " + engine,
		})
		return
	}
	if err := s.dropSiteDatabase(name, engine, id, body.ApprovalID); err != nil {
		s.writeErr(w, 403, err.Error())
		return
	}
	filtered := cfg.Sites[idx].Databases[:0]
	for _, d := range cfg.Sites[idx].Databases {
		if config.NormalizeDBID(name, d.ID) != id {
			filtered = append(filtered, d)
		}
	}
	cfg.Sites[idx].Databases = filtered
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.rebuildDBAdminer()
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleRotateDatabasePassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	id := strings.ToLower(strings.TrimSpace(r.PathValue("id")))

	sitesMu.Lock()
	defer sitesMu.Unlock()
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
	if strings.TrimSpace(site.Parent) != "" {
		s.writeErr(w, 400, "rotate passwords on the parent site")
		return
	}
	db, found := site.FindDatabase(id)
	if !found {
		s.writeErr(w, 404, "database not found")
		return
	}
	if db.Engine == dbengine.SQLite {
		s.writeErr(w, 400, "sqlite has no password to rotate")
		return
	}
	res, err := s.rotateDatabasePassword(name, db.Engine, db.ID)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	out := s.databaseView(name, db.ID, db.Engine, true)
	out["ok"] = true
	if res != nil {
		out["database"] = res.Database
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) handleAddSubdomain(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	parent := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	childName := strings.ToLower(strings.TrimSpace(body.Name))
	if childName == "" {
		s.writeErr(w, 400, "name is required")
		return
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	parentSite, _, ok := cfg.FindSite(parent)
	if !ok {
		s.writeErr(w, 404, "parent site not found")
		return
	}
	if strings.TrimSpace(parentSite.Parent) != "" {
		s.writeErr(w, 400, "cannot add subdomain under a child site")
		return
	}
	if _, _, exists := cfg.FindSite(childName); exists {
		s.writeErr(w, 409, "site already exists")
		return
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Panel.Domain), childName) {
		s.writeErr(w, 409, "hostname is reserved for the panel")
		return
	}
	child := config.Site{
		Name:   childName,
		Parent: parentSite.Name,
		Type:   config.SiteTypeStatic,
		Root:   "public",
		Limits: config.Limits{DiskQuota: config.DefaultDiskQuota},
	}
	cfg.Sites = append(cfg.Sites, child)
	if err := cfg.Validate(); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	warnings := []string{}
	if err := s.siteFS().CreateSiteUser(child.Name); err != nil {
		warnings = append(warnings, "CreateSiteUser: "+err.Error())
	} else if err := s.applyCaddy(cfg); err != nil {
		warnings = append(warnings, "caddy reload: "+err.Error())
	}
	out := map[string]any{"ok": true, "name": child.Name, "parent": parentSite.Name, "root": "public"}
	if len(warnings) > 0 {
		out["warning"] = strings.Join(warnings, "; ")
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) handleDeleteSubdomain(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	parent := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	child := strings.ToLower(strings.TrimSpace(r.PathValue("child")))
	var body struct {
		ApprovalID string `json:"approval_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(child)
	if !ok {
		s.writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	if strings.ToLower(strings.TrimSpace(site.Parent)) != parent {
		s.writeErr(w, 400, "site is not a subdomain of this parent")
		return
	}
	deleteData := false
	approvalID := strings.TrimSpace(body.ApprovalID)
	if r.URL.Query().Get("delete_data") == "1" || r.URL.Query().Get("delete_data") == "true" {
		deleteData = true
	}
	if deleteData {
		if approvalID == "" {
			s.writeJSON(w, 403, map[string]any{
				"ok":    false,
				"error": "delete_data requires step-up approval",
				"hint":  "POST /api/approvals with operation DeleteSiteUser, then: wbr approve <id>",
			})
			return
		}
		if err := s.deleteSiteUser(child, true, approvalID); err != nil {
			s.writeErr(w, 403, err.Error())
			return
		}
	} else {
		_ = s.deleteSiteUser(child, false, "")
	}
	cfg.Sites = append(cfg.Sites[:idx], cfg.Sites[idx+1:]...)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	_ = s.applyCaddy(cfg)
	s.writeJSON(w, 200, map[string]any{"ok": true, "delete_data": deleteData})
}

// --- legacy singular wrappers ---

func (s *Server) handleGetDatabaseLegacy(w http.ResponseWriter, r *http.Request) {
	s.handleListDatabases(w, r)
}

func (s *Server) handleAttachDatabaseLegacy(w http.ResponseWriter, r *http.Request) {
	s.handleAddDatabase(w, r)
}

func (s *Server) handleDetachDatabaseLegacy(w http.ResponseWriter, r *http.Request) {
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
	site, ok := findSite(cfg, name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if len(site.Databases) == 0 {
		s.writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	r.SetPathValue("id", site.Databases[0].ID)
	s.handleDeleteDatabase(w, r)
}

func findSite(cfg *config.Config, name string) (config.Site, bool) {
	s, _, ok := cfg.FindSite(name)
	return s, ok
}

type engineRuntimeStatus struct {
	Installed bool
	Active    bool
}

func (s *Server) engineStatus(engine string) (engineRuntimeStatus, error) {
	switch engine {
	case dbengine.Postgres:
		st, err := s.postgresStatus()
		if err != nil {
			return engineRuntimeStatus{}, err
		}
		if st == nil {
			return engineRuntimeStatus{}, nil
		}
		return engineRuntimeStatus{Installed: st.Installed, Active: st.Active}, nil
	case dbengine.MariaDB:
		st, err := s.mariaDBStatus()
		if err != nil {
			return engineRuntimeStatus{}, err
		}
		if st == nil {
			return engineRuntimeStatus{}, nil
		}
		return engineRuntimeStatus{Installed: st.Installed, Active: st.Active}, nil
	default:
		return engineRuntimeStatus{Installed: true, Active: true}, nil
	}
}

func (s *Server) engineInstallRequiredPayload(engine string, installedButInactive bool) map[string]any {
	info := distro.Detect()
	hint := info.InstallHintPostgres()
	start := info.StartHintPostgres()
	if engine == dbengine.MariaDB {
		hint = info.InstallHintMariaDB()
		start = info.StartHintMariaDB()
	}
	if installedButInactive {
		return map[string]any{
			"ok":               false,
			"error":            engine + " installed but not running",
			"install_required": true,
			"hint":             "Start " + engine + " as root, then try Add again:\n" + start,
		}
	}
	return map[string]any{
		"ok":               false,
		"error":            engine + " not installed",
		"install_required": true,
		"hint":             "Log in as root and install " + engine + ", then try Add again:\n" + hint,
	}
}

func (s *Server) postgresStatus() (*helper.PostgresStatusResult, error) {
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpPostgresStatus, Args: json.RawMessage(`{}`),
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var st helper.PostgresStatusResult
		if err := json.Unmarshal([]byte(resp.Message), &st); err != nil {
			return nil, err
		}
		return &st, nil
	}
	return s.ops().PostgresStatus()
}

func (s *Server) mariaDBStatus() (*helper.MariaDBStatusResult, error) {
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpMariaDBStatus, Args: json.RawMessage(`{}`),
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var st helper.MariaDBStatusResult
		if err := json.Unmarshal([]byte(resp.Message), &st); err != nil {
			return nil, err
		}
		return &st, nil
	}
	return s.ops().MariaDBStatus()
}

func (s *Server) installPostgres(approvalID string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.InstallPostgresArgs{ApprovalID: approvalID})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpInstallPostgres, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	if approvalID == "" {
		return fmt.Errorf("approval_id is required")
	}
	return s.ops().InstallPostgres()
}

func (s *Server) installMariaDB(approvalID string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.InstallMariaDBArgs{ApprovalID: approvalID})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpInstallMariaDB, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	if approvalID == "" {
		return fmt.Errorf("approval_id is required")
	}
	return s.ops().InstallMariaDB()
}

func (s *Server) ensureSiteDatabase(domain, engine, id, approvalID string) (*helper.EnsureSiteDatabaseResult, error) {
	id = dbengine.NormalizeID(domain, id)
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.EnsureSiteDatabaseArgs{
			Domain: domain, Engine: engine, ID: id, StateDir: s.stateDir(), ApprovalID: approvalID,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpEnsureSiteDatabase, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.EnsureSiteDatabaseResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	if dbengine.NeedsApproval(engine) && approvalID == "" {
		return nil, fmt.Errorf("approval_id is required")
	}
	return reconcile.EnsureSiteDatabase(s.ops(), config.Site{
		Name: domain,
		Databases: []config.SiteDatabase{{ID: id, Engine: engine}},
	}, reconcile.DatabaseApplyOptions{StateDir: s.stateDir(), ID: id})
}

func (s *Server) dropSiteDatabase(domain, engine, id, approvalID string) error {
	id = dbengine.NormalizeID(domain, id)
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DropSiteDatabaseArgs{
			Domain: domain, Engine: engine, ID: id, StateDir: s.stateDir(), ApprovalID: approvalID,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDropSiteDatabase, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	if dbengine.NeedsApproval(engine) && approvalID == "" {
		return fmt.Errorf("approval_id is required")
	}
	return reconcile.DropSiteDatabase(s.ops(), domain, engine, id, reconcile.DatabaseApplyOptions{StateDir: s.stateDir()})
}

func (s *Server) rotateDatabasePassword(domain, engine, id string) (*helper.EnsureSiteDatabaseResult, error) {
	id = dbengine.NormalizeID(domain, id)
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RotateDatabasePasswordArgs{
			Domain: domain, Engine: engine, ID: id, StateDir: s.stateDir(),
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpRotateDatabasePassword, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.EnsureSiteDatabaseResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	return s.ops().RotateDatabasePassword(domain, engine, id, s.stateDir())
}
