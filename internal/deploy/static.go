package deploy

import (
	"fmt"
	"os"
	"strings"

	"github.com/weborpanel/webor/internal/sites"
)

// StaticZipOptions controls a static zip extract into public/.
type StaticZipOptions struct {
	// SkipEnsure skips EnsurePublicDir (rare; tests).
	SkipEnsure bool
}

// StaticZip extracts a zip into the site's public/ document root.
func StaticZip(domain, zipPath string, opt StaticZipOptions) (publicDir string, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	st, err := os.Stat(zipPath)
	if err != nil {
		return "", fmt.Errorf("zip: %w", err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("zip path is a directory")
	}
	if !opt.SkipEnsure {
		if err := sites.EnsurePublicDir(domain); err != nil {
			return "", err
		}
	}
	pub := sites.Public(domain)
	if err := ExtractZip(zipPath, pub); err != nil {
		return "", err
	}
	return pub, nil
}
