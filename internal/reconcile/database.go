package reconcile

import (
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/dbengine"
)

// DatabaseApplyOptions controls per-site database reconcile.
type DatabaseApplyOptions struct {
	StateDir string
	ID       string
}

// EnsureSiteDatabase reconciles one configured database on a parent site.
// Prefer EnsureSiteDatabases for multi-DB sites.
func EnsureSiteDatabase(ops *helper.Ops, site config.Site, opt DatabaseApplyOptions) (*helper.EnsureSiteDatabaseResult, error) {
	if ops == nil {
		ops = helper.DefaultOps()
	}
	config.NormalizeSiteDatabases(&site)
	engine := ""
	id := opt.ID
	if len(site.Databases) > 0 {
		if id == "" {
			id = site.Databases[0].ID
			engine = site.Databases[0].Engine
		} else {
			db, ok := site.FindDatabase(id)
			if !ok {
				return nil, fmt.Errorf("database id %q not configured", id)
			}
			engine = db.Engine
			id = db.ID
		}
	} else {
		engine = strings.ToLower(strings.TrimSpace(site.Database))
	}
	if engine == "" {
		return nil, nil
	}
	if _, err := dbengine.Normalize(engine); err != nil {
		return nil, err
	}
	stateDir := opt.StateDir
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	return ops.EnsureSiteDatabase(site.Name, engine, id, stateDir)
}

// EnsureSiteDatabases ensures every database listed on the parent site.
func EnsureSiteDatabases(ops *helper.Ops, site config.Site, opt DatabaseApplyOptions) error {
	config.NormalizeSiteDatabases(&site)
	for _, db := range site.Databases {
		if _, err := EnsureSiteDatabase(ops, site, DatabaseApplyOptions{StateDir: opt.StateDir, ID: db.ID}); err != nil {
			return err
		}
	}
	return nil
}

// DropSiteDatabase removes database resources for a site engine + id.
func DropSiteDatabase(ops *helper.Ops, domain, engine, id string, opt DatabaseApplyOptions) error {
	if ops == nil {
		ops = helper.DefaultOps()
	}
	if engine == "" {
		return fmt.Errorf("engine is required")
	}
	stateDir := opt.StateDir
	if stateDir == "" {
		stateDir = config.DefaultStateDir
	}
	return ops.DropSiteDatabase(domain, engine, id, stateDir)
}
