package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureAPIsDir creates the per-backend API working directory.
func EnsureAPIsDir(domain, id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return fmt.Errorf("backend id is required")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("invalid backend id")
	}
	dir := APIs(domain, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.Chmod(dir, 0o755)
}

// ResolveAPIsRel joins a relative path under apis/<id>/ and rejects escapes.
// Empty rel is the API root itself.
func ResolveAPIsRel(domain, id, rel string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return "", fmt.Errorf("backend id is required")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid backend id")
	}
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	root := APIs(domain, id)
	if rel == "" || rel == "." {
		return filepath.Clean(root), nil
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path escapes api root")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path escapes api root")
	}
	full := filepath.Join(root, clean)
	rootClean := filepath.Clean(root)
	fullClean := filepath.Clean(full)
	if fullClean != rootClean && !strings.HasPrefix(fullClean, rootClean+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes api root")
	}
	return fullClean, nil
}
