package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/sites"
)

const maxDeployBinaryBytes = 64 * 1024 * 1024

// uploadsRoot returns the panel upload staging root (per-domain subdirs).
func (s *Server) uploadsRoot() string {
	if s.UploadsDir != "" {
		return s.UploadsDir
	}
	return "/var/lib/webor/uploads"
}

func (s *Server) findSite(name string) (*config.Config, *config.Site, error) {
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		return nil, nil, err
	}
	for i := range cfg.Sites {
		if strings.EqualFold(cfg.Sites[i].Name, name) {
			return cfg, &cfg.Sites[i], nil
		}
	}
	return cfg, nil, nil
}

func (s *Server) handleDeployBinary(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
		return
	}
	if !s.requireSiteScope(w, r, name, auth.ScopeDeploy) {
		return
	}

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	if site.Type != config.SiteTypeApp {
		s.writeErr(w, 400, "deploy binary is only for type app")
		return
	}

	if err := r.ParseMultipartForm(maxDeployBinaryBytes + 1024); err != nil {
		s.writeErr(w, 400, "invalid multipart form")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 400, "file is required")
		return
	}
	defer file.Close()

	dir := filepath.Join(s.uploadsRoot(), name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeErr(w, 500, "upload dir: "+err.Error())
		return
	}
	staged := filepath.Join(dir, sites.DefaultAppBinary)
	tmp := staged + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	n, err := io.Copy(out, io.LimitReader(file, maxDeployBinaryBytes+1))
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "read upload")
		return
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, closeErr.Error())
		return
	}
	if n > maxDeployBinaryBytes {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "file exceeds 64 MiB")
		return
	}
	if err := os.Rename(tmp, staged); err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, err.Error())
		return
	}
	_ = hdr // filename reserved for future audit log

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	res, err := s.ops().DeployAppBinary(ctx, name, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":         true,
		"release_id": res.ReleaseID,
		"binary":     res.Binary,
	})
}

func (s *Server) handleDeployAppZip(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
		return
	}
	if !s.requireSiteScope(w, r, name, auth.ScopeDeploy) {
		return
	}

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	if site.Type != config.SiteTypeApp {
		s.writeErr(w, 400, "deploy-zip is only for type app")
		return
	}

	if err := r.ParseMultipartForm(maxDeployBinaryBytes + 1024); err != nil {
		s.writeErr(w, 400, "invalid multipart form")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 400, "file is required")
		return
	}
	defer file.Close()
	fname := ""
	if hdr != nil {
		fname = hdr.Filename
	}
	if !strings.HasSuffix(strings.ToLower(fname), ".zip") {
		s.writeErr(w, 400, "file must be a .zip archive")
		return
	}

	dir := filepath.Join(s.uploadsRoot(), name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeErr(w, 500, "upload dir: "+err.Error())
		return
	}
	staged := filepath.Join(dir, "app-bundle.zip")
	tmp := staged + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	n, err := io.Copy(out, io.LimitReader(file, maxDeployBinaryBytes+1))
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "read upload")
		return
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, closeErr.Error())
		return
	}
	if n > maxDeployBinaryBytes {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "file exceeds 64 MiB")
		return
	}
	if err := os.Rename(tmp, staged); err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, err.Error())
		return
	}

	res, err := s.deployAppZip(name, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":   true,
		"root": res.Root,
	})
}

func (s *Server) deployAppZip(domain, zipPath string) (*helper.DeployAppZipResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployAppZipArgs{Domain: domain, ZipPath: zipPath})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpDeployAppZip, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var res helper.DeployAppZipResult
		if err := json.Unmarshal([]byte(resp.Message), &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.ops().DeployAppZip(ctx, domain, zipPath)
}

func (s *Server) handleDeployGit(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "name is required")
		return
	}
	if !s.requireSiteScope(w, r, name, auth.ScopeDeploy) {
		return
	}

	var body struct {
		GitURL       string `json:"git_url"`
		GitRef       string `json:"git_ref"`
		BuildCommand string `json:"build_command"`
		Artifact     string `json:"artifact"`
		MemoryMax    string `json:"memory_max"`
		TimeoutSec   int    `json:"timeout_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	if strings.TrimSpace(body.GitURL) == "" {
		s.writeErr(w, 400, "git_url is required")
		return
	}

	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return
	}
	if site.Type != config.SiteTypeApp {
		s.writeErr(w, 400, "deploy git is only for type app")
		return
	}

	timeout := 65 * time.Minute
	if body.TimeoutSec > 0 {
		timeout = time.Duration(body.TimeoutSec+30) * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	mem := body.MemoryMax
	if strings.TrimSpace(mem) == "" {
		mem = site.Limits.MemoryMax
	}
	cpu := site.Limits.CPUQuota
	res, err := s.ops().DeployAppGit(ctx, name, body.GitURL, body.GitRef, body.BuildCommand, body.Artifact, mem, cpu, body.TimeoutSec)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":         true,
		"release_id": res.ReleaseID,
		"binary":     res.Binary,
	})
}
