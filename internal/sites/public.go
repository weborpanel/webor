package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsurePublicDir creates a real public/ document root (Virtualmin-style),
// unless public already exists as a directory or symlink.
func EnsurePublicDir(domain string) error {
	pub := Public(domain)
	if _, err := os.Lstat(pub); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(pub, 0o755); err != nil {
		return err
	}
	return os.Chmod(pub, 0o755)
}

// ResolvePublicPath joins a relative path under public/ and rejects escapes.
func ResolvePublicPath(domain, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("path is required")
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path escapes public root")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path escapes public root")
	}
	root := Public(domain)
	full := filepath.Join(root, clean)
	rootClean := filepath.Clean(root) + string(filepath.Separator)
	if full != filepath.Clean(root) && !strings.HasPrefix(filepath.Clean(full)+string(filepath.Separator), rootClean) && filepath.Clean(full) != filepath.Clean(root) {
		// file path: allow files under root
		if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(root)+string(filepath.Separator)) {
			return "", fmt.Errorf("path escapes public root")
		}
	}
	return full, nil
}
