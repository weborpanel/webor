// Package deploy extracts and activates site releases.
package deploy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/webooza/webooza/internal/sites"
)

// AppBinaryOptions controls copying a binary into a new app release.
type AppBinaryOptions struct {
	BinaryName string // default "server"
	Now        time.Time
}

// InstallAppBinary copies srcBinary into a new releases/<id>/<binaryName> directory.
// It does not activate the release.
func InstallAppBinary(domain, srcBinary string, opt AppBinaryOptions) (releaseID string, dest string, err error) {
	if opt.BinaryName == "" {
		opt.BinaryName = sites.DefaultAppBinary
	}
	st, err := os.Stat(srcBinary)
	if err != nil {
		return "", "", err
	}
	if st.IsDir() {
		return "", "", fmt.Errorf("source binary is a directory")
	}

	id := sites.NewReleaseID(opt.Now)
	relDir := sites.Release(domain, id)
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		return "", "", err
	}
	dest = filepath.Join(relDir, opt.BinaryName)
	if err := copyFile(srcBinary, dest, 0o755); err != nil {
		_ = os.RemoveAll(relDir)
		return "", "", err
	}
	return id, dest, nil
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
