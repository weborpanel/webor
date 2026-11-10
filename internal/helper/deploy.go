package helper

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/webooza/webooza/internal/deploy"
	"github.com/webooza/webooza/internal/sites"
)

// DeployStaticZip extracts zipPath into a new release for domain and activates it.
// zipPath must be under the site tmp dir or /var/lib/webooza/uploads/.
func (o *Ops) DeployStaticZip(domain, zipPath string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if err := validateDeployZipPath(domain, zipPath); err != nil {
		return "", err
	}
	id, err := deploy.StaticZip(domain, zipPath, deploy.StaticZipOptions{})
	if err != nil {
		return "", err
	}
	return id, nil
}

func validateDeployZipPath(domain, zipPath string) error {
	clean := filepath.Clean(zipPath)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("zip_path must be absolute")
	}
	allowed := []string{
		sites.Tmp(domain) + string(filepath.Separator),
		filepath.Join("/var/lib/webooza/uploads", domain) + string(filepath.Separator),
		"/var/lib/webooza/uploads/",
	}
	ok := false
	for _, prefix := range allowed {
		if strings.HasPrefix(clean, prefix) {
			ok = true
			break
		}
	}
	// Also allow the uploads dir file directly under domain subdir exact file.
	if !ok {
		return fmt.Errorf("zip_path %q not under allowed upload directories", clean)
	}
	if strings.ToLower(filepath.Ext(clean)) != ".zip" {
		return fmt.Errorf("zip_path must end in .zip")
	}
	return nil
}
