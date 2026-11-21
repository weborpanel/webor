package deploy

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Archive bomb / size limits for extract.
const (
	MaxArchiveEntries      = 10_000
	MaxArchiveUncompressed = 500 * 1024 * 1024 // 500 MiB
)

// ExtractOptions controls archive unpack behaviour.
type ExtractOptions struct {
	// FlattenSingleTopDir lifts a lone top-level directory into dest (zip default).
	FlattenSingleTopDir bool
}

// DetectArchiveFormat returns zip|tar|tar.gz from the file name, or "".
func DetectArchiveFormat(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	default:
		return ""
	}
}

// ExtractArchive unpacks archivePath into destDir using format (zip|tar|tar.gz).
func ExtractArchive(archivePath, destDir, format string, opt ExtractOptions) error {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = DetectArchiveFormat(archivePath)
	}
	switch format {
	case "zip":
		return ExtractZipOpts(archivePath, destDir, opt)
	case "tar":
		return ExtractTar(archivePath, destDir, opt)
	case "tar.gz", "tgz":
		return ExtractTarGz(archivePath, destDir, opt)
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

// ExtractTar unpacks a plain tar into destDir.
func ExtractTar(tarPath, destDir string, opt ExtractOptions) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar: %w", err)
	}
	defer f.Close()
	if err := extractTarReader(tar.NewReader(f), destDir); err != nil {
		return err
	}
	if opt.FlattenSingleTopDir {
		return flattenSingleTopDir(destDir)
	}
	return nil
}

// ExtractTarGz unpacks a gzip-compressed tar into destDir.
func ExtractTarGz(path, destDir string, opt ExtractOptions) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	if err := extractTarReader(tar.NewReader(gz), destDir); err != nil {
		return err
	}
	if opt.FlattenSingleTopDir {
		return flattenSingleTopDir(destDir)
	}
	return nil
}

func extractTarReader(tr *tar.Reader, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destClean := filepath.Clean(destDir) + string(filepath.Separator)
	var entries int
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		entries++
		if entries > MaxArchiveEntries {
			return fmt.Errorf("archive has too many entries (>%d)", MaxArchiveEntries)
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("tar entry escapes destination: %q", hdr.Name)
		}
		if filepath.IsAbs(name) {
			return fmt.Errorf("tar entry absolute path: %q", hdr.Name)
		}
		// Drop drive-like or nested absolute forms.
		if strings.Contains(name, ":") {
			return fmt.Errorf("tar entry invalid path: %q", hdr.Name)
		}
		target := filepath.Join(destDir, name)
		if !strings.HasPrefix(filepath.Clean(target)+string(filepath.Separator), destClean) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("tar entry escapes destination: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			total += hdr.Size
			if total > MaxArchiveUncompressed {
				return fmt.Errorf("archive uncompressed size exceeds %d bytes", MaxArchiveUncompressed)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := hdr.FileInfo().Mode().Perm()
			if mode == 0 {
				mode = 0o644
			}
			// Never create setuid/setgid/sticky from archives.
			mode &^= os.ModeSetuid | os.ModeSetgid | os.ModeSticky
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			n, err := io.Copy(out, io.LimitReader(tr, hdr.Size+1))
			closeErr := out.Close()
			if err != nil {
				return err
			}
			if n > hdr.Size {
				return fmt.Errorf("tar entry larger than header size: %q", hdr.Name)
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("tar symlinks/hardlinks are not allowed: %q", hdr.Name)
		default:
			// Skip other types (char devices, etc.).
			continue
		}
	}
	return nil
}
