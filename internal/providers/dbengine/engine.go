// Package dbengine names supported site database engines and shared paths.
package dbengine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/weborpanel/webor/internal/sites"
)

const (
	SQLite   = "sqlite"
	Postgres = "postgres"
	MariaDB  = "mariadb"
)

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

// Normalize returns a canonical engine name or an error.
func Normalize(engine string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(engine))
	switch e {
	case SQLite, Postgres, MariaDB:
		return e, nil
	case "":
		return "", fmt.Errorf("engine is required")
	default:
		return "", fmt.Errorf("unsupported engine %q (sqlite|postgres|mariadb)", engine)
	}
}

// NeedsApproval reports whether add/remove requires step-up approval.
func NeedsApproval(engine string) bool {
	e, err := Normalize(engine)
	if err != nil {
		return true
	}
	return e == Postgres || e == MariaDB
}

// Slug is a sanitized id fragment for a domain (e.g. example.com → example_com).
func Slug(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = slugRE.ReplaceAllString(d, "_")
	d = strings.Trim(d, "_")
	if d == "" {
		d = "site"
	}
	return d
}

// DefaultID is the domain-derived default database id.
func DefaultID(domain string) string { return Slug(domain) }

// NormalizeID returns id or DefaultID(domain) when blank.
func NormalizeID(domain, id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return DefaultID(domain)
	}
	return id
}

// IsDefaultID reports whether id is the domain default.
func IsDefaultID(domain, id string) bool {
	return NormalizeID(domain, id) == DefaultID(domain)
}

// AppUser is the fixed DB username for a parent domain + db id.
func AppUser(domain, id string) string {
	base := sites.Username(domain)
	id = NormalizeID(domain, id)
	if IsDefaultID(domain, id) {
		return base
	}
	name := base + "_" + id
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "_")
	}
	return name
}

// DBName is the postgres/mariadb database name (same as AppUser).
func DBName(domain, id string) string { return AppUser(domain, id) }

// AdminerSlug is the path segment for per-DB Adminer mounts.
func AdminerSlug(domain, id string) string {
	id = NormalizeID(domain, id)
	if IsDefaultID(domain, id) {
		return id
	}
	return Slug(domain) + "_" + id
}

// AdminerBrowsePath is the session-gated read-only explorer URL for a DB.
func AdminerBrowsePath(domain, id string) string {
	return "/db/" + AdminerSlug(domain, id) + "/"
}

// AdminerEditPath is the session-gated full-access explorer URL for a DB.
func AdminerEditPath(domain, id string) string {
	return "/db/" + AdminerSlug(domain, id) + "/edit/"
}

// SQLitePath is the on-disk SQLite file for a site database id.
func SQLitePath(domain, id string) string {
	id = NormalizeID(domain, id)
	if IsDefaultID(domain, id) {
		return filepath.Join(sites.Root(domain), "data", "app.db")
	}
	return filepath.Join(sites.Root(domain), "data", id+".db")
}

// SQLiteDSN is a read-only modernc/sqlite DSN for Adminer browse.
func SQLiteDSN(domain, id string) string {
	return fmt.Sprintf("file:%s?mode=ro", SQLitePath(domain, id))
}

// SQLiteRWDSN is a read-write modernc/sqlite DSN for Adminer edit.
func SQLiteRWDSN(domain, id string) string {
	return fmt.Sprintf("file:%s", SQLitePath(domain, id))
}

// SecretFileName returns secrets/<parent>.<id>.<engine>.ro.json basename.
func SecretFileName(domain, id, engine string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, "/", "_")
	id = NormalizeID(domain, id)
	engine = strings.ToLower(strings.TrimSpace(engine))
	return fmt.Sprintf("%s.%s.%s.ro.json", d, id, engine)
}
