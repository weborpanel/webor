package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/unitfiles"
)

// ApplyOptions controls Apply.
type ApplyOptions struct {
	BaseURL      string
	Channel      string
	Current      string
	Force        bool // allow same or older version
	DryRun       bool
	BinDir       string // default /usr/local/bin
	UnitDir      string // default /etc/systemd/system; tests override
	SaveChannel  bool
	PanelEnvPath string
	Client       *http.Client
	// ExpectedVersion, if set, must match the channel VERSION (approval binding).
	ExpectedVersion string
	// SkipRestart skips systemctl restart (unit install still runs unless SkipUnits).
	SkipRestart bool
	// SkipUnits skips installing systemd units from the tarball.
	SkipUnits bool
}

// ApplyResult summarizes a successful (or dry-run) apply.
type ApplyResult struct {
	From           string `json:"from"`
	To             string `json:"to"`
	Channel        string `json:"channel"`
	DryRun         bool   `json:"dry_run"`
	Restarted      bool   `json:"restarted"`
	UnitsUpdated   bool   `json:"units_updated"`
	BinDir         string `json:"bin_dir"`
}

// Apply downloads, verifies, and installs the channel tarball.
func Apply(ctx context.Context, opt ApplyOptions) (*ApplyResult, error) {
	st, err := Check(ctx, CheckOptions{
		BaseURL: opt.BaseURL,
		Channel: opt.Channel,
		Current: opt.Current,
		Client:  opt.Client,
	})
	if err != nil {
		return nil, err
	}
	if opt.ExpectedVersion != "" && Normalize(opt.ExpectedVersion) != st.Latest {
		return nil, fmt.Errorf("channel latest %s does not match expected %s", st.Latest, Normalize(opt.ExpectedVersion))
	}
	if !opt.Force && !st.Newer {
		if st.Same {
			return nil, fmt.Errorf("already on %s (use --force to reinstall)", st.Current)
		}
		return nil, fmt.Errorf("channel latest %s is not newer than %s (use --force to install anyway)", st.Latest, st.Current)
	}

	binDir := opt.BinDir
	if binDir == "" {
		binDir = DefaultBinDir
	}

	res := &ApplyResult{
		From:      st.Current,
		To:        st.Latest,
		Channel:   st.Channel,
		DryRun:    opt.DryRun,
		BinDir:    binDir,
		Restarted: false,
	}
	if opt.DryRun {
		return res, nil
	}

	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}

	tmp, err := os.MkdirTemp("", "webor-update-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	tarName := filepath.Base(st.TarURL)
	tarPath := filepath.Join(tmp, tarName)
	if err := downloadFile(ctx, client, st.TarURL, tarPath); err != nil {
		return nil, fmt.Errorf("download tarball: %w", err)
	}
	sumsPath := filepath.Join(tmp, "SHA256SUMS")
	if err := downloadFile(ctx, client, st.SumsURL, sumsPath); err != nil {
		return nil, fmt.Errorf("download SHA256SUMS: %w", err)
	}
	if err := verifySHA256(sumsPath, tarPath, tarName); err != nil {
		return nil, err
	}

	extractDir := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return nil, err
	}
	if err := extractTarGz(tarPath, extractDir); err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	root, err := findReleaseRoot(extractDir)
	if err != nil {
		return nil, err
	}
	if err := installBins(filepath.Join(root, "bin"), binDir); err != nil {
		return nil, err
	}

	if !opt.SkipUnits {
		unitDir := opt.UnitDir
		if unitDir == "" {
			unitDir = unitfiles.DefaultUnitDir
		}
		uopt := unitfiles.InstallOptions{
			DestDir:          unitDir,
			SkipDaemonReload: opt.SkipRestart, // tests: no systemctl
			RestartCaddy:     false,           // restartUnits handles caddy after bins
		}
		installDir := filepath.Join(root, "install")
		var changed bool
		var unitErr error
		if _, err := os.Stat(filepath.Join(installDir, "webor-caddy.service")); err == nil {
			changed, unitErr = unitfiles.InstallFromDir(installDir, uopt)
		} else {
			// Older / incomplete tarball: refresh from binaries embedded at build time.
			changed, unitErr = unitfiles.Install(uopt)
		}
		if unitErr != nil {
			return nil, fmt.Errorf("install units: %w", unitErr)
		}
		res.UnitsUpdated = changed
	}

	envPath := opt.PanelEnvPath
	if envPath == "" {
		envPath = DefaultPanelEnv
	}
	if opt.SaveChannel {
		base := st.BaseURL
		if IsLegacyBaseURL(base) {
			base = DefaultBaseURL
		}
		_ = SaveChannelConfig(envPath, st.Channel, base)
	} else {
		// Always leave S3-era hosts pointing at GitHub after a successful apply.
		_ = MigrateLegacyBaseURL(envPath, st.Channel)
	}

	if opt.SkipRestart {
		return res, nil
	}
	if err := restartUnits(); err != nil {
		return nil, fmt.Errorf("bins installed but restart failed: %w", err)
	}
	res.Restarted = true
	return res, nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d (%s)", res.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, res.Body); err != nil {
		return err
	}
	return f.Close()
}

func verifySHA256(sumsPath, tarPath, tarName string) error {
	raw, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	var want string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[len(fields)-1] == tarName {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("SHA256SUMS has no entry for %s", tarName)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", tarName, got, want)
	}
	return nil
}

func extractTarGz(tarPath, dest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	destClean := filepath.Clean(dest) + string(os.PathSeparator)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		clean := filepath.Clean(target)
		if !strings.HasPrefix(clean+string(os.PathSeparator), destClean) && clean != filepath.Clean(dest) {
			return fmt.Errorf("refusing path escape: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(clean, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			_ = os.Chmod(clean, mode)
		default:
			// skip symlinks and others for safety
		}
	}
}

func findReleaseRoot(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "webor-linux-") {
			root := filepath.Join(extractDir, e.Name())
			if st, err := os.Stat(filepath.Join(root, "bin", "webor")); err == nil && !st.IsDir() {
				return root, nil
			}
		}
	}
	return "", fmt.Errorf("webor-linux-* root with bin/webor not found after extract")
}

func installBins(srcBin, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"webor", "webor-helper", "wbr"} {
		src := filepath.Join(srcBin, name)
		st, err := os.Stat(src)
		if err != nil || st.IsDir() {
			return fmt.Errorf("missing binary in release: %s", name)
		}
		dest := filepath.Join(destDir, name)
		tmp := dest + ".new"
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		_ = in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			_ = os.Remove(tmp)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}
		_ = os.Chmod(tmp, 0o755)
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	// Keep wbr on PATH for Alma sudo secure_path.
	link := "/usr/bin/wbr"
	_ = os.Remove(link)
	_ = os.Symlink(filepath.Join(destDir, "wbr"), link)
	return nil
}

func restartUnits() error {
	cmd := exec.Command("systemctl", "restart", "webor-helper", "webor")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	// Apply new ReadWritePaths (e.g. /srv/sites for access.log) when Caddy is active.
	if _, err := os.Stat(filepath.Join(unitfiles.DefaultUnitDir, "webor-caddy.service")); err == nil {
		_ = exec.Command("systemctl", "try-restart", "webor-caddy").Run()
	}
	return nil
}
