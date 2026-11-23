// Package runtime resolves shared language runtimes (Node) on disk.
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/weborpanel/webor/internal/config"
)

// NodeRoot is the shared cache for official Node builds.
var NodeRoot = "/var/lib/webor/runtimes/node"

// NodeArch maps Go arch to Node dist arch.
func NodeArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch %s for Node runtimes", runtime.GOARCH)
	}
}

// NodeOS returns "linux" for production VPS targets.
func NodeOS() string {
	if runtime.GOOS == "linux" {
		return "linux"
	}
	return runtime.GOOS
}

// DirForVersion returns /var/lib/webor/runtimes/node/vX.Y.Z
func DirForVersion(fullVersion string) string {
	v := strings.TrimSpace(fullVersion)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return filepath.Join(NodeRoot, v)
}

// BinDir returns …/bin for a full version directory.
func BinDir(fullVersion string) string {
	return filepath.Join(DirForVersion(fullVersion), "bin")
}

// NodeBinary returns …/bin/node for a full version.
func NodeBinary(fullVersion string) string {
	return filepath.Join(BinDir(fullVersion), "node")
}

// ResolveNodeBinary picks the node executable for a site runtime.
// managedVersion is the installed full version (e.g. 22.14.0) when using version:.
func ResolveNodeBinary(r *config.SiteRuntime, managedVersion string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("runtime is nil")
	}
	config.NormalizeRuntime(r)
	if r.Kind != config.RuntimeKindNode {
		return "", fmt.Errorf("not a node runtime")
	}
	if r.Command != "" {
		return r.Command, nil
	}
	if managedVersion == "" {
		managedVersion = r.Version
	}
	bin := NodeBinary(managedVersion)
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return "", fmt.Errorf("node binary not installed at %s (ensure runtime first)", bin)
	}
	return bin, nil
}

// AliasMajor writes/updates NodeRoot/<major> → vX.Y.Z symlink.
func AliasMajor(major, fullVersion string) error {
	major = strings.TrimSpace(major)
	if major == "" {
		return fmt.Errorf("major is required")
	}
	if err := os.MkdirAll(NodeRoot, 0o755); err != nil {
		return err
	}
	link := filepath.Join(NodeRoot, major)
	target := DirForVersion(fullVersion)
	_ = os.Remove(link)
	return os.Symlink(target, link)
}

// ReadMajorAlias returns the full version directory name for a major alias, if present.
func ReadMajorAlias(major string) (string, error) {
	link := filepath.Join(NodeRoot, strings.TrimSpace(major))
	target, err := os.Readlink(link)
	if err != nil {
		return "", err
	}
	base := filepath.Base(target)
	return strings.TrimPrefix(base, "v"), nil
}

// DistTarballName is node-vX.Y.Z-linux-x64.tar.xz style.
func DistTarballName(fullVersion, goos, arch string) string {
	v := strings.TrimPrefix(fullVersion, "v")
	return fmt.Sprintf("node-v%s-%s-%s.tar.xz", v, goos, arch)
}

// DistURL builds the official download URL.
func DistURL(fullVersion, goos, arch string) string {
	v := strings.TrimPrefix(fullVersion, "v")
	return fmt.Sprintf("https://nodejs.org/dist/v%s/%s", v, DistTarballName(fullVersion, goos, arch))
}
