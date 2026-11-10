// Package sites holds shared site path and username helpers.
package sites

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// RootDir is the on-disk root for all sites.
const RootDir = "/srv/sites"

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
	name := "wbz_" + d
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
