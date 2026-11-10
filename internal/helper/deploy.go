package helper

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/webooza/webooza/internal/deploy"
	"github.com/webooza/webooza/internal/sites"
)

// DeployStaticZip extracts a zip into the site's public/ document root.
// zipPath must be under the site tmp dir or /var/lib/webooza/uploads/.
func (o *Ops) DeployStaticZip(domain, zipPath string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if err := validateDeployZipPath(domain, zipPath); err != nil {
		return "", err
	}
	if err := sites.EnsurePublicDir(domain); err != nil {
		return "", err
	}
	pub := sites.Public(domain)
	if err := deploy.ExtractZip(zipPath, pub); err != nil {
		return "", err
	}
	return pub, nil
}

func validateDeployZipPath(domain, zipPath string) error {
	clean := filepath.Clean(zipPath)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("zip_path must be absolute")
	}
	allowed := []string{
		sites.Tmp(domain) + string(filepath.Separator),
		filepath.Join("/var/lib/webooza/uploads", domain) + string(filepath.Separator),
	}
	ok := false
	for _, prefix := range allowed {
		if strings.HasPrefix(clean, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("zip_path %q not under allowed upload directories", clean)
	}
	if strings.ToLower(filepath.Ext(clean)) != ".zip" {
		return fmt.Errorf("zip_path must end in .zip")
	}
	return nil
}
