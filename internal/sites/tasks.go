package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CronsRoot returns /srv/sites/<domain>/crons.
func CronsRoot(domain string) string {
	return filepath.Join(Root(domain), "crons")
}

// CronDir returns /srv/sites/<domain>/crons/<job>.
func CronDir(domain, job string) string {
	return filepath.Join(CronsRoot(domain), strings.ToLower(strings.TrimSpace(job)))
}

// ProcessesRoot returns /srv/sites/<domain>/processes.
func ProcessesRoot(domain string) string {
	return filepath.Join(Root(domain), "processes")
}

// ProcessDir returns /srv/sites/<domain>/processes/<job>.
func ProcessDir(domain, job string) string {
	return filepath.Join(ProcessesRoot(domain), strings.ToLower(strings.TrimSpace(job)))
}

func validateTaskJobID(job string) error {
	job = strings.ToLower(strings.TrimSpace(job))
	if job == "" {
		return fmt.Errorf("job name is required")
	}
	if strings.Contains(job, "..") || strings.ContainsAny(job, `/\`) {
		return fmt.Errorf("invalid job name")
	}
	return nil
}

// EnsureCronDir creates crons/<job>/ under the site root.
func EnsureCronDir(domain, job string) error {
	if err := validateTaskJobID(job); err != nil {
		return err
	}
	dir := CronDir(domain, job)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.Chmod(dir, 0o755)
}

// EnsureProcessDir creates processes/<job>/ under the site root.
func EnsureProcessDir(domain, job string) error {
	if err := validateTaskJobID(job); err != nil {
		return err
	}
	dir := ProcessDir(domain, job)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.Chmod(dir, 0o755)
}

// ResolveCronRel joins a relative path under crons/<job>/ and rejects escapes.
func ResolveCronRel(domain, job, rel string) (string, error) {
	return resolveTaskRel(CronDir(domain, job), job, rel, "cron")
}

// ResolveProcessRel joins a relative path under processes/<job>/ and rejects escapes.
func ResolveProcessRel(domain, job, rel string) (string, error) {
	return resolveTaskRel(ProcessDir(domain, job), job, rel, "process")
}

func resolveTaskRel(root, job, rel, kind string) (string, error) {
	if err := validateTaskJobID(job); err != nil {
		return "", err
	}
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	root = filepath.Clean(root)
	if rel == "" || rel == "." {
		return root, nil
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path escapes %s root", kind)
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path escapes %s root", kind)
	}
	full := filepath.Clean(filepath.Join(root, clean))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes %s root", kind)
	}
	return full, nil
}
