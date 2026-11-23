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
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func (s *Server) mountCronRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/cron", s.handleListCron)
	mux.HandleFunc("POST /api/sites/{name}/cron", s.handleCreateCron)
	mux.HandleFunc("GET /api/sites/{name}/cron/{job}", s.handleGetCron)
	mux.HandleFunc("PATCH /api/sites/{name}/cron/{job}", s.handlePatchCron)
	mux.HandleFunc("DELETE /api/sites/{name}/cron/{job}", s.handleDeleteCron)
	mux.HandleFunc("POST /api/sites/{name}/cron/{job}/deploy", s.handleDeployCronBinary)
	mux.HandleFunc("POST /api/sites/{name}/cron/{job}/deploy-zip", s.handleDeployCronZip)
	mux.HandleFunc("GET /api/sites/{name}/cron/{job}/fs", s.handleListCronFS)
	mux.HandleFunc("GET /api/sites/{name}/cron/{job}/status", s.handleCronStatus)
	mux.HandleFunc("POST /api/sites/{name}/cron/{job}/run", s.handleRunCron)
	mux.HandleFunc("POST /api/sites/{name}/cron/{job}/enable", s.handleEnableCron)
	mux.HandleFunc("POST /api/sites/{name}/cron/{job}/disable", s.handleDisableCron)
}

func cronView(domain string, job config.CronJob, st *helper.CronStatusResult) map[string]any {
	v := map[string]any{
		"name":     job.Name,
		"schedule": job.Schedule,
		"exec":     job.Exec,
		"dir":      "crons/" + strings.ToLower(strings.TrimSpace(job.Name)),
		"unit":     systemd.CronServiceUnitName(domain, job.Name),
		"timer":    systemd.CronTimerUnitName(domain, job.Name),
	}
	if st != nil {
		v["timer_active"] = st.TimerActive
		v["last_result"] = st.LastResult
		v["last_trigger"] = st.LastTrigger
		v["next_elapse"] = st.NextElapse
		v["failed"] = st.Failed
	}
	return v
}

func (s *Server) handleListCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	list := make([]map[string]any, 0, len(site.Cron))
	for _, job := range site.Cron {
		st, _ := s.cronStatus(name, job.Name)
		list = append(list, cronView(name, job, st))
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "cron": list})
}

func (s *Server) handleCreateCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		Exec     string `json:"exec"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	jobName := strings.TrimSpace(body.Name)
	if err := config.ValidateTaskName(jobName); err != nil {
		s.writeErr(w, 400, "name: "+err.Error())
		return
	}
	execPath := strings.TrimSpace(body.Exec)
	if execPath == "" {
		execPath = "./crons/" + strings.ToLower(jobName) + "/" + sites.DefaultAppBinary
	}
	schedule := strings.TrimSpace(body.Schedule)
	if schedule != "" {
		if _, err := systemd.CronToOnCalendar(schedule); err != nil {
			s.writeErr(w, 400, "schedule: "+err.Error())
			return
		}
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	if _, _, exists := site.FindCron(jobName); exists {
		s.writeErr(w, 409, "cron job already exists")
		return
	}
	job := config.CronJob{Name: jobName, Schedule: schedule, Exec: execPath}
	cfg.Sites[idx].Cron = append(cfg.Sites[idx].Cron, job)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	// Best-effort unit install (may fail before binary exists or without systemd).
	_ = s.installCronJob(cfg.Sites[idx], job)
	st, _ := s.cronStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "cron": cronView(name, job, st)})
}

func (s *Server) handleGetCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	job, _, ok := site.FindCron(jobName)
	if !ok {
		s.writeErr(w, 404, "cron job not found")
		return
	}
	st, _ := s.cronStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "cron": cronView(name, job, st)})
}

func (s *Server) handlePatchCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	var body struct {
		Schedule *string `json:"schedule"`
		Exec     *string `json:"exec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	job, jIdx, ok := site.FindCron(jobName)
	if !ok {
		s.writeErr(w, 404, "cron job not found")
		return
	}
	if body.Schedule != nil {
		sched := strings.TrimSpace(*body.Schedule)
		if sched != "" {
			if _, err := systemd.CronToOnCalendar(sched); err != nil {
				s.writeErr(w, 400, "schedule: "+err.Error())
				return
			}
		}
		job.Schedule = sched
	}
	if body.Exec != nil {
		execPath := strings.TrimSpace(*body.Exec)
		if execPath == "" {
			s.writeErr(w, 400, "exec is required")
			return
		}
		job.Exec = execPath
	}
	cfg.Sites[idx].Cron[jIdx] = job
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if job.Schedule == "" {
		_ = s.disableCronTimer(name, job.Name)
	}
	if err := s.installCronJob(cfg.Sites[idx], job); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.cronStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "cron": cronView(name, job, st)})
}

func (s *Server) handleDeleteCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))

	sitesMu.Lock()
	defer sitesMu.Unlock()
	cfg, err := s.loadOrEmptyConfig()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, idx, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	_, jIdx, ok := site.FindCron(jobName)
	if !ok {
		s.writeErr(w, 404, "cron job not found")
		return
	}
	cfg.Sites[idx].Cron = append(site.Cron[:jIdx], site.Cron[jIdx+1:]...)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	_ = s.removeCronJob(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleCronStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	st, err := s.cronStatus(name, jobName)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleRunCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	if err := s.runCronNow(name, jobName); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.cronStatus(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleEnableCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, ok := cfg.FindSite(name)
	if !ok {
		s.writeErr(w, 404, "site not found")
		return
	}
	job, _, ok := site.FindCron(jobName)
	if !ok {
		s.writeErr(w, 404, "cron job not found")
		return
	}
	if strings.TrimSpace(job.Schedule) == "" {
		s.writeErr(w, 400, "set a schedule before enabling the timer")
		return
	}
	if err := s.enableCronTimer(site, job); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.cronStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "cron": cronView(name, job, st)})
}

func (s *Server) handleDisableCron(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	if err := s.disableCronTimer(name, jobName); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.cronStatus(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleListCronFS(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	entries, err := s.listCronDir(name, jobName, path)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "entries": entries, "path": path})
}

func (s *Server) handleDeployCronBinary(w http.ResponseWriter, r *http.Request) {
	s.deployCronUpload(w, r, false)
}

func (s *Server) handleDeployCronZip(w http.ResponseWriter, r *http.Request) {
	s.deployCronUpload(w, r, true)
}

func (s *Server) deployCronUpload(w http.ResponseWriter, r *http.Request, zip bool) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
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
	if _, _, ok := site.FindCron(jobName); !ok {
		s.writeErr(w, 404, "cron job not found")
		return
	}
	staged, err := s.stageUpload(w, r, name, "cron-"+jobName, zip)
	if err != nil {
		return
	}
	if zip {
		root, err := s.deployCronZip(name, jobName, staged)
		if err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
		s.writeJSON(w, 200, map[string]any{"ok": true, "root": root, "job": jobName})
		return
	}
	dest, err := s.deployCronBinary(name, jobName, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "binary": dest, "job": jobName})
}

func (s *Server) stageUpload(w http.ResponseWriter, r *http.Request, siteName, subdir string, zip bool) (string, error) {
	if err := r.ParseMultipartForm(maxDeployBinaryBytes + 1024); err != nil {
		s.writeErr(w, 400, "invalid multipart form")
		return "", err
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 400, "file is required")
		return "", err
	}
	defer file.Close()
	if zip {
		fname := ""
		if hdr != nil {
			fname = hdr.Filename
		}
		if !strings.HasSuffix(strings.ToLower(fname), ".zip") {
			s.writeErr(w, 400, "file must be a .zip archive")
			return "", fmt.Errorf("not zip")
		}
	}
	dir := filepath.Join(s.uploadsRoot(), siteName, subdir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeErr(w, 500, "upload dir: "+err.Error())
		return "", err
	}
	name := sites.DefaultAppBinary
	mode := os.FileMode(0o755)
	if zip {
		name = "bundle.zip"
		mode = 0o644
	}
	staged := filepath.Join(dir, name)
	tmp := staged + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return "", err
	}
	n, err := io.Copy(out, io.LimitReader(file, maxDeployBinaryBytes+1))
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "read upload")
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, closeErr.Error())
		return "", closeErr
	}
	if n > maxDeployBinaryBytes {
		_ = os.Remove(tmp)
		s.writeErr(w, 400, "file exceeds 64 MiB")
		return "", fmt.Errorf("too large")
	}
	if err := os.Rename(tmp, staged); err != nil {
		_ = os.Remove(tmp)
		s.writeErr(w, 500, err.Error())
		return "", err
	}
	return staged, nil
}

func (s *Server) installCronJob(site config.Site, job config.CronJob) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.InstallCronJobArgs{
			Domain: site.Name, Job: job.Name, Schedule: job.Schedule, Exec: job.Exec, Limits: site.Limits,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpInstallCronJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	_, err := s.ops().InstallCronJob(site, job)
	return err
}

func (s *Server) removeCronJob(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RemoveCronJobArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpRemoveCronJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RemoveCronJob(domain, job)
}

func (s *Server) enableCronTimer(site config.Site, job config.CronJob) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.EnableCronTimerArgs{
			Domain: site.Name, Job: job.Name, Schedule: job.Schedule, Exec: job.Exec, Limits: site.Limits,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpEnableCronTimer, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().EnableCronTimer(site, job)
}

func (s *Server) disableCronTimer(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DisableCronTimerArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpDisableCronTimer, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().DisableCronTimer(domain, job)
}

func (s *Server) runCronNow(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RunCronNowArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpRunCronNow, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RunCronNow(domain, job)
}

func (s *Server) cronStatus(domain, job string) (*helper.CronStatusResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.CronStatusArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpCronStatus, Args: raw})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var st helper.CronStatusResult
		if err := json.Unmarshal([]byte(resp.Message), &st); err != nil {
			return nil, err
		}
		return &st, nil
	}
	return s.ops().CronStatus(domain, job)
}

func (s *Server) deployCronBinary(domain, job, path string) (string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployCronBinaryArgs{Domain: domain, Job: job, BinaryPath: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpDeployCronBinary, Args: raw})
		if err != nil {
			return "", err
		}
		if !resp.OK {
			return "", fmt.Errorf("%s", resp.Error)
		}
		var res struct {
			Binary string `json:"binary"`
		}
		_ = json.Unmarshal([]byte(resp.Message), &res)
		return res.Binary, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.ops().DeployCronBinary(ctx, domain, job, path)
}

func (s *Server) deployCronZip(domain, job, path string) (string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployCronZipArgs{Domain: domain, Job: job, ZipPath: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpDeployCronZip, Args: raw})
		if err != nil {
			return "", err
		}
		if !resp.OK {
			return "", fmt.Errorf("%s", resp.Error)
		}
		var res struct {
			Root string `json:"root"`
		}
		_ = json.Unmarshal([]byte(resp.Message), &res)
		return res.Root, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.ops().DeployCronZip(ctx, domain, job, path)
}

func (s *Server) listCronDir(domain, job, path string) ([]helper.PublicFSEntry, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ListCronDirArgs{Domain: domain, Job: job, Path: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpListCronDir, Args: raw})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var entries []helper.PublicFSEntry
		if err := json.Unmarshal([]byte(resp.Message), &entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
	return s.ops().ListCronDir(domain, job, path)
}
