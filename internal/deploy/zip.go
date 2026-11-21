// Package deploy extracts and activates static site releases.
package deploy

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip unpacks zipPath into destDir. Entries may not escape destDir.
// If the archive has a single top-level directory, its contents are lifted
// into destDir so `public/`-style zips and flat zips both work.
func ExtractZip(zipPath, destDir string) error {
	return ExtractZipOpts(zipPath, destDir, ExtractOptions{FlattenSingleTopDir: true})
}

// ExtractZipOpts is ExtractZip with controllable flatten behaviour and bomb limits.
func ExtractZipOpts(zipPath, destDir string, opt ExtractOptions) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	if len(r.File) > MaxArchiveEntries {
		return fmt.Errorf("archive has too many entries (>%d)", MaxArchiveEntries)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	destClean := filepath.Clean(destDir) + string(filepath.Separator)
	var total int64
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("zip entry escapes destination: %q", f.Name)
		}
		if filepath.IsAbs(name) {
			return fmt.Errorf("zip entry absolute path: %q", f.Name)
		}
		target := filepath.Join(destDir, name)
		if !strings.HasPrefix(filepath.Clean(target)+string(filepath.Separator), destClean) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("zip entry escapes destination: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > MaxArchiveUncompressed {
			return fmt.Errorf("archive uncompressed size exceeds %d bytes", MaxArchiveUncompressed)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(f, target); err != nil {
			return err
		}
	}

	if opt.FlattenSingleTopDir {
		return flattenSingleTopDir(destDir)
	}
	return nil
}

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	mode &^= os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// flattenSingleTopDir moves contents up when the zip had one root folder.
func flattenSingleTopDir(destDir string) error {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}
	top := filepath.Join(destDir, entries[0].Name())
	inner, err := os.ReadDir(top)
	if err != nil {
		return err
	}
	for _, e := range inner {
		src := filepath.Join(top, e.Name())
		dst := filepath.Join(destDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	return os.Remove(top)
}
