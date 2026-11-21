// Package sites holds shared site path and username helpers.
package sites

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// RootDir is the on-disk root for all sites. Tests may override it.
var RootDir = "/srv/sites"

var nonUser = regexp.MustCompile(`[^a-z0-9_]`)

// Username derives a Linux username from a domain (max 32 chars).
func Username(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.ReplaceAll(d, ".", "_")
	d = strings.ReplaceAll(d, "-", "_")
	d = nonUser.ReplaceAllString(d, "_")
	d = strings.Trim(d, "_")
	if d == "" {
		d = "site"
	}
	if !unicode.IsLetter(rune(d[0])) {
		d = "s_" + d
	}
	name := "wbr_" + d
	if len(name) > 32 {
		name = name[:32]
		name = strings.TrimRight(name, "_")
	}
	return name
}

// Root returns /srv/sites/<domain>.
func Root(domain string) string {
	return filepath.Join(RootDir, strings.ToLower(strings.TrimSpace(domain)))
}

// Public returns the live document root path Caddy serves (/srv/sites/<domain>/public).
// On disk this is a symlink to the active release directory.
func Public(domain string) string {
	return filepath.Join(Root(domain), "public")
}

// ReleasesDir returns /srv/sites/<domain>/releases.
func ReleasesDir(domain string) string {
	return filepath.Join(Root(domain), "releases")
}

// Release returns /srv/sites/<domain>/releases/<id>.
func Release(domain, id string) string {
	return filepath.Join(ReleasesDir(domain), id)
}

// Current returns the current-release symlink path.
func Current(domain string) string {
	return filepath.Join(Root(domain), "current")
}

// Logs returns /srv/sites/<domain>/logs.
func Logs(domain string) string {
	return filepath.Join(Root(domain), "logs")
}

// Tmp returns /srv/sites/<domain>/tmp.
func Tmp(domain string) string {
	return filepath.Join(Root(domain), "tmp")
}

// BinDir returns /srv/sites/<domain>/bin.
func BinDir(domain string) string {
	return filepath.Join(Root(domain), "bin")
}

// RuntimeDir is the parent for per-site runtime sockets.
// Tests may override it.
var RuntimeDir = "/run/webor/sites"

// SiteRuntime returns /run/webor/sites/<domain>.
func SiteRuntime(domain string) string {
	return filepath.Join(RuntimeDir, strings.ToLower(strings.TrimSpace(domain)))
}

// AppSocket returns the public Unix socket path Caddy reverse-proxies to.
func AppSocket(domain string) string {
	return filepath.Join(SiteRuntime(domain), "app.sock")
}

// AppBackendSocket returns the private backend socket used with systemd-socket-proxyd.
func AppBackendSocket(domain string) string {
	return filepath.Join(SiteRuntime(domain), "backend.sock")
}
