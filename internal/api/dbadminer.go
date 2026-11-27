package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/dbadminer/dbadminer"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/mariadb"
	"github.com/weborpanel/webor/internal/providers/postgres"
)

const (
	adminerAccessRO   = "ro"
	adminerAccessFull = "full"
)

type adminerEntry struct {
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	ID     string `json:"id"`
	Engine string `json:"engine"`
	Browse string `json:"browse"`
	Edit   string `json:"edit"`
}

type adminerMount struct {
	entry adminerEntry
	ro    http.Handler
	full  http.Handler
}

type globalDBAdminer struct {
	mu      sync.Mutex
	dbs     []*sql.DB
	bySlug  map[string]*adminerMount
	entries []adminerEntry
	hash    string
}

func (s *Server) getGlobalDBA() *globalDBAdminer {
	s.dbAdminerOnce.Do(func() {
		s.globalDBA = &globalDBAdminer{}
	})
	return s.globalDBA
}

func (s *Server) rebuildDBAdminer() {
	s.getGlobalDBA().mu.Lock()
	defer s.getGlobalDBA().mu.Unlock()
	s.rebuildDBAdminerLocked()
}

func (s *Server) rebuildDBAdminerLocked() {
	g := s.globalDBA
	for _, db := range g.dbs {
		_ = db.Close()
	}
	g.dbs = nil
	g.bySlug = nil
	g.entries = nil
	g.hash = ""

	cfg, err := s.loadOrEmptyConfig()
	if err != nil || cfg == nil {
		return
	}
	s.preparePostgresAdminer(cfg)
	bySlug := make(map[string]*adminerMount)
	var entries []adminerEntry
	var parts []string
	for _, site := range cfg.Sites {
		config.NormalizeSiteDatabases(&site)
		if strings.TrimSpace(site.Parent) != "" {
			continue // children inherit; DBs live on parent
		}
		for _, db := range site.Databases {
			slug := dbengine.AdminerSlug(site.Name, db.ID)
			roDB, err := s.openSiteDB(site.Name, db.Engine, db.ID, adminerAccessRO)
			if err != nil {
				continue
			}
			fullDB, err := s.openSiteDB(site.Name, db.Engine, db.ID, adminerAccessFull)
			if err != nil {
				_ = roDB.Close()
				continue
			}
			roCfg := dbadminer.DefaultConfig()
			roCfg.Access = dbadminer.AccessReadOnly
			roEx, err := dbadminer.New(roDB, roCfg)
			if err != nil {
				_ = roDB.Close()
				_ = fullDB.Close()
				continue
			}
			fullCfg := dbadminer.DefaultConfig()
			fullCfg.Access = dbadminer.AccessFull
			fullEx, err := dbadminer.New(fullDB, fullCfg)
			if err != nil {
				_ = roDB.Close()
				_ = fullDB.Close()
				continue
			}
			entry := adminerEntry{
				Slug:   slug,
				Domain: site.Name,
				ID:     dbengine.NormalizeID(site.Name, db.ID),
				Engine: db.Engine,
				Browse: dbengine.AdminerBrowsePath(site.Name, db.ID),
				Edit:   dbengine.AdminerEditPath(site.Name, db.ID),
			}
			bySlug[slug] = &adminerMount{
				entry: entry,
				ro:    roEx.Handler(),
				full:  fullEx.Handler(),
			}
			entries = append(entries, entry)
			parts = append(parts, slug+":"+db.Engine)
			g.dbs = append(g.dbs, roDB, fullDB)
		}
	}
	if len(bySlug) == 0 {
		return
	}
	g.hash = strings.Join(parts, ",")
	g.bySlug = bySlug
	g.entries = entries
}

func (s *Server) openSiteDB(domain, engine, id, access string) (*sql.DB, error) {
	id = dbengine.NormalizeID(domain, id)
	full := access == adminerAccessFull
	switch engine {
	case dbengine.Postgres:
		sec, err := postgres.ReadSecret(postgres.ResolveSecretPath(s.stateDir(), domain, id))
		if err != nil {
			return nil, err
		}
		dsn := sec.DSN()
		if full {
			dsn = sec.DSNAs(postgres.AppRole(domain, id))
		}
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return nil, err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	case dbengine.MariaDB:
		sec, err := mariadb.ReadSecret(mariadb.ResolveSecretPath(s.stateDir(), domain, id))
		if err != nil {
			return nil, err
		}
		dsn := sec.DSN()
		if full {
			dsn = sec.DSNAs(mariadb.AppRole(domain, id))
		}
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	case dbengine.SQLite:
		path := dbengine.SQLitePath(domain, id)
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
		dsn := dbengine.SQLiteDSN(domain, id)
		if full {
			dsn = dbengine.SQLiteRWDSN(domain, id)
		}
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported engine %q", engine)
	}
}

// findMount resolves a /db/ segment to a mount. The canonical key is AdminerSlug,
// which is the instance id when that id already includes the hostname. The raw
// instance id and the older doubled hostname prefix are accepted as well.
func (g *globalDBAdminer) findMount(segment string) *adminerMount {
	segment = strings.ToLower(strings.Trim(strings.TrimSpace(segment), "/"))
	if segment == "" || g == nil || g.bySlug == nil {
		return nil
	}
	if m := g.bySlug[segment]; m != nil {
		return m
	}
	var match *adminerMount
	seen := map[*adminerMount]struct{}{}
	for _, m := range g.bySlug {
		if m == nil {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		if !dbengine.MatchAdminerSegment(m.entry.Domain, m.entry.ID, segment) {
			continue
		}
		if match != nil {
			return nil
		}
		match = m
	}
	return match
}

// preparePostgresAdminer opens localhost password auth and renames doubled
// Postgres database names before DBAdminer tries to connect. A failure here
// leaves SQLite mounts working.
func (s *Server) preparePostgresAdminer(cfg *config.Config) {
	if cfg == nil {
		return
	}
	var dbs []helper.PostgresAuthDB
	for _, site := range cfg.Sites {
		if strings.TrimSpace(site.Parent) != "" {
			continue
		}
		for _, db := range site.Databases {
			if !strings.EqualFold(strings.TrimSpace(db.Engine), dbengine.Postgres) {
				continue
			}
			dbs = append(dbs, helper.PostgresAuthDB{Domain: site.Name, ID: db.ID})
		}
	}
	if len(dbs) == 0 {
		return
	}
	_ = s.ensurePostgresPanelAuth(dbs)
}

func (s *Server) ensurePostgresPanelAuth(dbs []helper.PostgresAuthDB) error {
	args := ipc.EnsurePostgresAuthArgs{StateDir: s.stateDir()}
	for _, db := range dbs {
		args.DBs = append(args.DBs, ipc.PostgresAuthDB{Domain: db.Domain, ID: db.ID})
	}
	if s.HelperSocket != "" {
		raw, err := json.Marshal(args)
		if err != nil {
			return err
		}
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpEnsurePostgresAuth, Args: raw,
		})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().EnsurePostgresPanelAuth(dbs, s.stateDir())
}

func (s *Server) mountDBAdminer(mux *http.ServeMux) {
	mux.Handle("GET /db", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/db/", http.StatusFound)
	}))
	mux.Handle("/db/{path...}", http.HandlerFunc(s.handleDBAdminer))
}

// parseAdminerPath splits /db/{path...} into slug, access mode, and explorer-relative rest.
func parseAdminerPath(pathRest string) (slug, access, rest string, ok bool) {
	pathRest = strings.Trim(pathRest, "/")
	if pathRest == "" {
		return "", "", "", false
	}
	parts := strings.Split(pathRest, "/")
	slug = parts[0]
	if slug == "" || slug == "edit" {
		return "", "", "", false
	}
	access = adminerAccessRO
	restParts := parts[1:]
	if len(restParts) > 0 && restParts[0] == "edit" {
		access = adminerAccessFull
		restParts = restParts[1:]
	}
	return slug, access, strings.Join(restParts, "/"), true
}

func (s *Server) handleDBAdminer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	pathRest := r.PathValue("path")
	if strings.Trim(pathRest, "/") == "" {
		s.serveDBAdminerIndex(w, r)
		return
	}
	slug, access, rest, ok := parseAdminerPath(pathRest)
	if !ok {
		s.writeErr(w, 404, "database not found")
		return
	}

	g := s.getGlobalDBA()
	g.mu.Lock()
	if g.bySlug == nil {
		s.rebuildDBAdminerLocked()
	}
	m := g.findMount(slug)
	var h http.Handler
	if m != nil {
		if access == adminerAccessFull {
			h = m.full
		} else {
			h = m.ro
		}
	}
	g.mu.Unlock()
	if h == nil {
		s.writeErr(w, 404, "database not found")
		return
	}

	r.Host = "127.0.0.1:8787"
	r.Header.Del("Origin")
	if rest == "" {
		r.URL.Path = "/"
	} else {
		r.URL.Path = "/" + rest
	}
	r.RequestURI = r.URL.RequestURI()
	h.ServeHTTP(w, r)
}

func (s *Server) serveDBAdminerIndex(w http.ResponseWriter, r *http.Request) {
	g := s.getGlobalDBA()
	g.mu.Lock()
	if g.bySlug == nil {
		s.rebuildDBAdminerLocked()
	}
	entries := append([]adminerEntry(nil), g.entries...)
	g.mu.Unlock()

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "databases": entries})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Databases</title>`)
	b.WriteString(`<style>
body{font-family:system-ui,sans-serif;margin:2rem;line-height:1.45;color:#1a1a1a;background:#fafafa}
h1{font-size:1.25rem;margin:0 0 1rem}
ul{list-style:none;padding:0;margin:0;max-width:40rem}
li{display:flex;flex-wrap:wrap;gap:.75rem;align-items:baseline;padding:.65rem 0;border-bottom:1px solid #e5e5e5}
.meta{flex:1;min-width:10rem} .meta strong{display:block} .meta span{color:#666;font-size:.875rem}
a{color:#0b57d0;text-decoration:none} a:hover{text-decoration:underline}
.empty{color:#666}
</style></head><body><h1>Databases</h1>`)
	if len(entries) == 0 {
		b.WriteString(`<p class="empty">No databases attached yet.</p>`)
	} else {
		b.WriteString(`<ul>`)
		for _, e := range entries {
			b.WriteString(`<li><div class="meta"><strong>`)
			b.WriteString(html.EscapeString(e.Slug))
			b.WriteString(`</strong><span>`)
			b.WriteString(html.EscapeString(e.Domain))
			b.WriteString(` · `)
			b.WriteString(html.EscapeString(e.Engine))
			b.WriteString(`</span></div>`)
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(e.Browse))
			b.WriteString(`">Browse</a>`)
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(e.Edit))
			b.WriteString(`">Edit</a></li>`)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</body></html>`)
	_, _ = w.Write([]byte(b.String()))
}
