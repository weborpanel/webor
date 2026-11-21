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
// Empty rel is rejected; use ResolvePublicRel for the document root itself.
func ResolvePublicPath(domain, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("path is required")
	}
	return ResolvePublicRel(domain, rel)
}

// ResolvePublicRel is like ResolvePublicPath but allows "" / "." for the public root.
func ResolvePublicRel(domain, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	root := Public(domain)
	if rel == "" || rel == "." {
		return filepath.Clean(root), nil
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path escapes public root")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path escapes public root")
	}
	full := filepath.Join(root, clean)
	rootClean := filepath.Clean(root)
	fullClean := filepath.Clean(full)
	if fullClean != rootClean && !strings.HasPrefix(fullClean, rootClean+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes public root")
	}
	return fullClean, nil
}
