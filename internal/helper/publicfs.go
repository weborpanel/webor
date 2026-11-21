package helper

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/deploy"
	"github.com/weborpanel/webor/internal/sites"
)

// PublicFSEntry is one listing row under public/.
type PublicFSEntry struct {
	Name  string `json:"name"`
	Type  string `json:"type"` // file | dir
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"` // unix seconds
	Mode  string `json:"mode,omitempty"`
}

const maxPublicReadBytes = 32 * 1024 * 1024

// ListPublicDir lists a directory under the site public root (rel "" = root).
func (o *Ops) ListPublicDir(domain, rel string) ([]PublicFSEntry, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if err := sites.EnsurePublicDir(domain); err != nil {
		return nil, err
	}
	dir, err := sites.ResolvePublicRel(domain, rel)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]PublicFSEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		typ := "file"
		if e.IsDir() {
			typ = "dir"
		}
		out = append(out, PublicFSEntry{
			Name:  e.Name(),
			Type:  typ,
			Size:  info.Size(),
			Mtime: info.ModTime().Unix(),
			Mode:  info.Mode().Perm().String(),
		})
	}
	return out, nil
}

// MkdirPublic creates a directory under public/.
func (o *Ops) MkdirPublic(domain, rel string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	full, err := sites.ResolvePublicPath(domain, rel)
	if err != nil {
		return err
	}
	if err := sites.EnsurePublicDir(domain); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return err
	}
	return os.Chmod(full, 0o755)
}

// RenamePublic renames/moves within public/.
func (o *Ops) RenamePublic(domain, from, to string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	src, err := sites.ResolvePublicPath(domain, from)
	if err != nil {
		return fmt.Errorf("from: %w", err)
	}
	dst, err := sites.ResolvePublicPath(domain, to)
	if err != nil {
		return fmt.Errorf("to: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// DeletePublicPath removes a file or directory under public/.
func (o *Ops) DeletePublicPath(domain, rel string, recursive bool) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	full, err := sites.ResolvePublicPath(domain, rel)
	if err != nil {
		return err
	}
	root := filepath.Clean(sites.Public(domain))
	if filepath.Clean(full) == root {
		return fmt.Errorf("cannot delete public root")
	}
	st, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if recursive {
			return os.RemoveAll(full)
		}
		return os.Remove(full) // fails if non-empty
	}
	return os.Remove(full)
}

// ReadPublicFile returns file bytes (capped) under public/.
func (o *Ops) ReadPublicFile(domain, rel string) (name string, data []byte, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	full, err := sites.ResolvePublicPath(domain, rel)
	if err != nil {
		return "", nil, err
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", nil, err
	}
	if st.IsDir() {
		return "", nil, fmt.Errorf("is a directory")
	}
	if st.Size() > maxPublicReadBytes {
		return "", nil, fmt.Errorf("file exceeds %d bytes", maxPublicReadBytes)
	}
	data, err = os.ReadFile(full)
	if err != nil {
		return "", nil, err
	}
	return filepath.Base(full), data, nil
}

// ReadPublicFileBase64 is ReadPublicFile encoded for IPC Message transport.
func (o *Ops) ReadPublicFileBase64(domain, rel string) (name string, b64 string, size int, err error) {
	name, data, err := o.ReadPublicFile(domain, rel)
	if err != nil {
		return "", "", 0, err
	}
	return name, base64.StdEncoding.EncodeToString(data), len(data), nil
}

// ExtractPublicArchive extracts a staged archive into destRel under public/.
// archivePath must be under site tmp or /var/lib/webor/uploads/<domain>/.
func (o *Ops) ExtractPublicArchive(domain, archivePath, destRel, format string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if err := validateArchivePath(domain, archivePath); err != nil {
		return "", err
	}
	if format == "" {
		format = deploy.DetectArchiveFormat(archivePath)
	}
	if format == "" {
		return "", fmt.Errorf("could not detect archive format")
	}
	if err := sites.EnsurePublicDir(domain); err != nil {
		return "", err
	}
	dest, err := sites.ResolvePublicRel(domain, destRel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	st, err := os.Stat(dest)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("extract destination is not a directory")
	}
	pubRoot := filepath.Clean(sites.Public(domain))
	flatten := filepath.Clean(dest) == pubRoot
	if err := deploy.ExtractArchive(archivePath, dest, format, deploy.ExtractOptions{
		FlattenSingleTopDir: flatten,
	}); err != nil {
		return "", err
	}
	return dest, nil
}

func validateArchivePath(domain, archivePath string) error {
	clean := filepath.Clean(archivePath)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("archive_path must be absolute")
	}
	allowed := []string{
		sites.Tmp(domain) + string(filepath.Separator),
		filepath.Join("/var/lib/webor/uploads", domain) + string(filepath.Separator),
	}
	ok := false
	for _, prefix := range allowed {
		if strings.HasPrefix(clean, prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("archive_path %q not under allowed upload directories", clean)
	}
	format := deploy.DetectArchiveFormat(clean)
	if format == "" {
		return fmt.Errorf("archive_path must be .zip, .tar, .tar.gz or .tgz")
	}
	return nil
}

// WritePublicFileUnder writes data to a relative path that may include directories.
func (o *Ops) WritePublicFileUnder(domain, dirRel, fileName string, r io.Reader, maxBytes int64) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "" || fileName == "." || fileName == ".." {
		return "", fmt.Errorf("file name is required")
	}
	rel := fileName
	if strings.TrimSpace(dirRel) != "" && dirRel != "." {
		rel = filepath.ToSlash(filepath.Join(dirRel, fileName))
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxBytes)
	}
	if err := o.WritePublicFile(domain, rel, data); err != nil {
		return "", err
	}
	return rel, nil
}
