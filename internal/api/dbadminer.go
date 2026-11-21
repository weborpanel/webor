package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/dbadminer/dbadminer"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/mariadb"
	"github.com/weborpanel/webor/internal/providers/postgres"
)

type openedExplorer struct {
	name string
	db   *sql.DB
	ex   *dbadminer.Explorer
}

type globalDBAdminer struct {
	mu   sync.Mutex
	dbs  []*sql.DB
	h    http.Handler
	hash string
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
	g.h = nil
	g.hash = ""

	cfg, err := s.loadOrEmptyConfig()
	if err != nil || cfg == nil {
		return
	}
	var items []openedExplorer
	var parts []string
	for _, site := range cfg.Sites {
		config.NormalizeSiteDatabases(&site)
		if strings.TrimSpace(site.Parent) != "" {
			continue // children inherit; DBs live on parent
		}
		for _, db := range site.Databases {
			slug := dbengine.AdminerSlug(site.Name, db.ID)
			opened, err := s.openSiteDB(site.Name, db.Engine, db.ID)
			if err != nil {
				continue
			}
			dcfg := dbadminer.DefaultConfig()
			dcfg.Access = dbadminer.AccessReadOnly
			ex, err := dbadminer.New(opened, dcfg)
			if err != nil {
				_ = opened.Close()
				continue
			}
			items = append(items, openedExplorer{name: slug, db: opened, ex: ex})
			parts = append(parts, slug+":"+db.Engine)
			g.dbs = append(g.dbs, opened)
		}
	}
	if len(items) == 0 {
		return
	}
	g.hash = strings.Join(parts, ",")
	g.h = multiDBHandler(items)
}

func multiDBHandler(items []openedExplorer) http.Handler {
	mux := http.NewServeMux()
	dbadminer.MountUI(mux)
	items[0].ex.MountAPI(mux, "/api")
	if items[0].name != "" {
		mux.HandleFunc("GET /api/databases", func(w http.ResponseWriter, r *http.Request) {
			list := make([]dbadminer.DatabaseInfo, 0, len(items))
			for _, item := range items {
				list = append(list, item.ex.DatabaseInfo(item.name))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]any{"databases": list})
		})
		for _, item := range items {
			item.ex.MountAPI(mux, "/api/databases/"+item.name)
		}
	}
	return items[0].ex.Wrap(mux)
}

func (s *Server) openSiteDB(domain, engine, id string) (*sql.DB, error) {
	id = dbengine.NormalizeID(domain, id)
	switch engine {
	case dbengine.Postgres:
		sec, err := postgres.ReadSecret(postgres.ResolveSecretPath(s.stateDir(), domain, id))
		if err != nil {
			return nil, err
		}
		db, err := sql.Open("pgx", sec.DSN())
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
		db, err := sql.Open("mysql", sec.DSN())
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
		db, err := sql.Open("sqlite", dbengine.SQLiteDSN(domain, id))
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

func (s *Server) mountDBAdminer(mux *http.ServeMux) {
	mux.Handle("GET /db", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/db/", http.StatusFound)
	}))
	mux.Handle("/db/{path...}", http.HandlerFunc(s.handleGlobalDBAdminer))
}

func (s *Server) handleGlobalDBAdminer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	g := s.getGlobalDBA()
	g.mu.Lock()
	if g.h == nil {
		s.rebuildDBAdminerLocked()
	}
	h := g.h
	g.mu.Unlock()
	if h == nil {
		s.writeErr(w, 404, "no databases attached yet")
		return
	}

	r.Host = "127.0.0.1:8787"
	r.Header.Del("Origin")
	pathRest := r.PathValue("path")
	if pathRest == "" {
		r.URL.Path = "/"
	} else {
		r.URL.Path = "/" + pathRest
	}
	r.RequestURI = r.URL.RequestURI()
	h.ServeHTTP(w, r)
}
