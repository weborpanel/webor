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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/unitfiles"
)

func TestApplyDryRunNoWrite(t *testing.T) {
	latest := "0.2.0"
	tarName := "webor-linux-amd64.tar.gz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/VERSION"):
			_, _ = io.WriteString(w, latest+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	binDir := t.TempDir()
	res, err := Apply(context.Background(), ApplyOptions{
		BaseURL: srv.URL,
		Channel: "beta",
		Current: "0.1.0",
		DryRun:  true,
		BinDir:  binDir,
		Client:  srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.To != latest || !res.DryRun {
		t.Fatalf("unexpected result: %+v", res)
	}
	entries, _ := os.ReadDir(binDir)
	if len(entries) != 0 {
		t.Fatalf("dry-run wrote files: %v", entries)
	}
	_ = tarName
}

func TestApplyChecksumMismatch(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "webor-linux-amd64.tar.gz")
	if err := os.WriteFile(tarPath, []byte("not-a-real-tarball"), 0o644); err != nil {
		t.Fatal(err)
	}
	sums := filepath.Join(tmp, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte("0000000000000000000000000000000000000000000000000000000000000000  webor-linux-amd64.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifySHA256(sums, tarPath, "webor-linux-amd64.tar.gz")
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestApplyChecksumOK(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "webor-linux-amd64.tar.gz")
	payload := []byte("hello-webor")
	if err := os.WriteFile(tarPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	sums := filepath.Join(tmp, "SHA256SUMS")
	line := hex.EncodeToString(sum[:]) + "  webor-linux-amd64.tar.gz\n"
	if err := os.WriteFile(sums, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(sums, tarPath, "webor-linux-amd64.tar.gz"); err != nil {
		t.Fatal(err)
	}
}

func TestSaveLoadChannelConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.env")
	if err := os.WriteFile(path, []byte("WBR_PANEL_PORT=1234\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := SaveChannelConfig(path, "beta", DefaultBaseURL); err != nil {
		t.Fatal(err)
	}
	cfg := LoadChannelConfig(path)
	if cfg.Channel != "beta" {
		t.Fatalf("channel=%q", cfg.Channel)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("base=%q", cfg.BaseURL)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "WBR_PANEL_PORT=1234") {
		t.Fatalf("lost existing keys: %s", raw)
	}
}

func TestLoadChannelConfigInfersBeta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.env")
	if err := os.WriteFile(path, []byte("WBR_PANEL_PORT=1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	cfg := LoadChannelConfigFor(path, "0.1.0-beta.3")
	if cfg.Channel != "beta" {
		t.Fatalf("channel=%q want beta (inferred)", cfg.Channel)
	}
}

func TestChannelAssetURL(t *testing.T) {
	gh := DefaultBaseURL
	got := ChannelAssetURL(gh, "beta", "VERSION")
	want := gh + "/beta/VERSION"
	if got != want {
		t.Fatalf("gh: got %q want %q", got, want)
	}
	got = ChannelAssetURL(LegacyS3BaseURL, "beta", "VERSION")
	want = LegacyS3BaseURL + "/downloads/beta/VERSION"
	if got != want {
		t.Fatalf("s3: got %q want %q", got, want)
	}
}

func TestMigrateLegacyBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.env")
	if err := os.WriteFile(path, []byte("WBR_CHANNEL=beta\nWBR_BASE_URL="+LegacyS3BaseURL+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyBaseURL(path, "beta"); err != nil {
		t.Fatal(err)
	}
	cfg := LoadChannelConfig(path)
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("base=%q", cfg.BaseURL)
	}
	if err := MigrateLegacyBaseURL(path, "beta"); err != nil {
		t.Fatal(err)
	}
}

func TestLoadChannelConfigFromProcessEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.env")
	t.Setenv("WBR_CHANNEL", "beta")
	cfg := LoadChannelConfigFor(path, "0.1.0")
	if cfg.Channel != "beta" {
		t.Fatalf("channel=%q want beta from process env", cfg.Channel)
	}
}

func TestExtractAndFindRoot(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "rel.tar.gz")
	if err := writeMiniReleaseTar(tarPath); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(tarPath, dest); err != nil {
		t.Fatal(err)
	}
	root, err := findReleaseRoot(dest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "webor")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallUnitsFromReleaseRoot(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "webor-linux-amd64")
	installDir := filepath.Join(root, "install")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("[Unit]\nDescription=webor-caddy test\n[Service]\nReadWritePaths=/var/lib/caddy /srv/sites\n")
	if err := os.WriteFile(filepath.Join(installDir, "webor-caddy.service"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "webor.service"), []byte("[Unit]\nDescription=webor\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "webor-helper.service"), []byte("[Unit]\nDescription=helper\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmp, "systemd")
	changed, err := unitfiles.InstallFromDir(installDir, unitfiles.InstallOptions{
		DestDir:          dest,
		ForceCaddy:       true,
		SkipDaemonReload: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected units to install")
	}
	got, err := os.ReadFile(filepath.Join(dest, "webor-caddy.service"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "/srv/sites") {
		t.Fatalf("missing /srv/sites: %s", got)
	}
}

func writeMiniReleaseTar(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	files := map[string]string{
		"webor-linux-amd64/bin/webor":        "#!/bin/sh\necho webor\n",
		"webor-linux-amd64/bin/webor-helper": "#!/bin/sh\necho helper\n",
		"webor-linux-amd64/bin/wbr":          "#!/bin/sh\necho wbr\n",
	}
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.WriteString(tw, body); err != nil {
			return err
		}
	}
	return nil
}

func TestCheckNewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "0.2.0")
	}))
	t.Cleanup(srv.Close)
	st, err := Check(context.Background(), CheckOptions{
		BaseURL: srv.URL,
		Channel: "stable",
		Current: "0.1.0",
		Arch:    "amd64",
		Client:  srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !st.Newer || st.Latest != "0.2.0" {
		t.Fatalf("%+v", st)
	}
}
