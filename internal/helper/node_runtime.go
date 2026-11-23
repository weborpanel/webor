package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	wbrruntime "github.com/weborpanel/webor/internal/runtime"
)

// DefaultNodePins are offline fallbacks when nodejs.org index is unreachable.
var DefaultNodePins = map[string]string{
	"20": "20.18.1",
	"22": "22.14.0",
	"24": "24.4.1",
}

// EnsureNodeRuntimeResult is returned after a Node install is ready.
type EnsureNodeRuntimeResult struct {
	Version string `json:"version"`
	Binary  string `json:"binary"`
	BinDir  string `json:"bin_dir"`
}

// EnsureNodeRuntime downloads (if needed) an official Node build for version
// ("22" or "22.14.0") and returns the resolved full version + paths.
func (o *Ops) EnsureNodeRuntime(ctx context.Context, version string) (*EnsureNodeRuntimeResult, error) {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	full, err := o.resolveNodeVersion(ctx, version)
	if err != nil {
		return nil, err
	}
	bin := wbrruntime.NodeBinary(full)
	if st, err := os.Stat(bin); err == nil && !st.IsDir() {
		major := strings.Split(full, ".")[0]
		_ = wbrruntime.AliasMajor(major, full)
		_ = prepareNodeRuntimeAccess(full)
		return &EnsureNodeRuntimeResult{Version: full, Binary: bin, BinDir: wbrruntime.BinDir(full)}, nil
	}

	arch, err := wbrruntime.NodeArch()
	if err != nil {
		return nil, err
	}
	goos := wbrruntime.NodeOS()
	if goos != "linux" {
		// Allow lab on non-linux only if already installed; otherwise require linux.
		return nil, fmt.Errorf("Node runtime downloads are supported on linux (got %s)", goos)
	}
	url := wbrruntime.DistURL(full, goos, arch)
	if err := o.downloadAndExtractNode(ctx, url, full); err != nil {
		return nil, err
	}
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return nil, fmt.Errorf("node binary missing after extract: %s", bin)
	}
	major := strings.Split(full, ".")[0]
	_ = wbrruntime.AliasMajor(major, full)
	_ = prepareNodeRuntimeAccess(full)
	return &EnsureNodeRuntimeResult{Version: full, Binary: bin, BinDir: wbrruntime.BinDir(full)}, nil
}

// prepareNodeRuntimeAccess makes shared Node installs usable by site users under
// ProtectSystem=strict + SELinux Enforcing (Alma/RHEL).
// - /var/lib/webor must be other-traversable (not 0750) so wbr_* can reach runtimes/
// - Node binaries extracted under var_lib_t must be relabeled bin_t for systemd EXEC
func prepareNodeRuntimeAccess(fullVersion string) error {
	stateDir := filepath.Dir(filepath.Dir(wbrruntime.NodeRoot)) // /var/lib/webor
	for _, dir := range []string{stateDir, filepath.Dir(wbrruntime.NodeRoot), wbrruntime.NodeRoot, wbrruntime.DirForVersion(fullVersion)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		_ = os.Chmod(dir, 0o755)
	}
	bin := wbrruntime.NodeBinary(fullVersion)
	_ = os.Chmod(bin, 0o755)
	if !selinuxEnforcing() {
		return nil
	}
	binDir := wbrruntime.BinDir(fullVersion)
	_ = exec.Command("chcon", "-R", "-t", "bin_t", binDir).Run()
	libDir := filepath.Join(wbrruntime.DirForVersion(fullVersion), "lib")
	if st, err := os.Stat(libDir); err == nil && st.IsDir() {
		_ = exec.Command("chcon", "-R", "-t", "lib_t", libDir).Run()
	}
	return nil
}

func selinuxEnforcing() bool {
	b, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == "1"
}

func (o *Ops) resolveNodeVersion(ctx context.Context, version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) >= 3 {
		return version, nil
	}
	// Major (or major.minor): try alias, then pin, then index.json
	if len(parts) == 1 {
		if full, err := wbrruntime.ReadMajorAlias(parts[0]); err == nil && full != "" {
			return full, nil
		}
	}
	if full, ok := DefaultNodePins[parts[0]]; ok && len(parts) == 1 {
		// Prefer live index when possible
		if live, err := latestNodeFromIndex(ctx, parts[0]); err == nil && live != "" {
			return live, nil
		}
		return full, nil
	}
	if live, err := latestNodeFromIndex(ctx, parts[0]); err == nil && live != "" {
		return live, nil
	}
	if full, ok := DefaultNodePins[parts[0]]; ok {
		return full, nil
	}
	return "", fmt.Errorf("cannot resolve Node version %q", version)
}

type nodeIndexEntry struct {
	Version string   `json:"version"`
	LTS     any      `json:"lts"`
	Files   []string `json:"files"`
}

func latestNodeFromIndex(ctx context.Context, major string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://nodejs.org/dist/index.json", nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("nodejs index HTTP %d", res.StatusCode)
	}
	var entries []nodeIndexEntry
	if err := json.NewDecoder(res.Body).Decode(&entries); err != nil {
		return "", err
	}
	prefix := "v" + major + "."
	for _, e := range entries {
		if strings.HasPrefix(e.Version, prefix) {
			return strings.TrimPrefix(e.Version, "v"), nil
		}
	}
	return "", fmt.Errorf("no Node %s in index", major)
}

func (o *Ops) downloadAndExtractNode(ctx context.Context, url, fullVersion string) error {
	if err := os.MkdirAll(wbrruntime.NodeRoot, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(wbrruntime.NodeRoot, "dl-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "node.tar.xz")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("download Node: HTTP %d (%s)", res.StatusCode, url)
	}
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()

	cmd := exec.CommandContext(ctx, "tar", "-xJf", tarPath, "-C", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract Node: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	// Tarball contains node-vX.Y.Z-linux-x64/
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	var extracted string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "node-v") {
			extracted = filepath.Join(tmpDir, e.Name())
			break
		}
	}
	if extracted == "" {
		return fmt.Errorf("extracted Node directory not found")
	}
	dest := wbrruntime.DirForVersion(fullVersion)
	_ = os.RemoveAll(dest)
	if err := os.Rename(extracted, dest); err != nil {
		// cross-device fallback
		if err := copyDir(extracted, dest); err != nil {
			return err
		}
	}
	return nil
}
