package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func (s *Server) mountProcessRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/processes", s.handleListProcesses)
	mux.HandleFunc("POST /api/sites/{name}/processes", s.handleCreateProcess)
	mux.HandleFunc("GET /api/sites/{name}/processes/{job}", s.handleGetProcess)
	mux.HandleFunc("PATCH /api/sites/{name}/processes/{job}", s.handlePatchProcess)
	mux.HandleFunc("DELETE /api/sites/{name}/processes/{job}", s.handleDeleteProcess)
	mux.HandleFunc("POST /api/sites/{name}/processes/{job}/deploy", s.handleDeployProcessBinary)
	mux.HandleFunc("POST /api/sites/{name}/processes/{job}/deploy-zip", s.handleDeployProcessZip)
	mux.HandleFunc("GET /api/sites/{name}/processes/{job}/fs", s.handleListProcessFS)
	mux.HandleFunc("GET /api/sites/{name}/processes/{job}/status", s.handleProcessStatus)
	mux.HandleFunc("POST /api/sites/{name}/processes/{job}/start", s.handleStartProcess)
	mux.HandleFunc("POST /api/sites/{name}/processes/{job}/stop", s.handleStopProcess)
	mux.HandleFunc("POST /api/sites/{name}/processes/{job}/restart", s.handleRestartProcess)
}

func processView(domain string, job config.Job, st *helper.JobStatusResult) map[string]any {
	v := map[string]any{
		"name":    job.Name,
		"exec":    job.Exec,
		"dir":     "processes/" + strings.ToLower(strings.TrimSpace(job.Name)),
		"unit":    systemd.JobServiceUnitName(domain, job.Name),
		"runtime": runtimeView(job.Runtime),
	}
	if st != nil {
		v["active"] = st.Active
		v["failed"] = st.Failed
	}
	return v
}

func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
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
	list := make([]map[string]any, 0, len(site.Jobs))
	for _, job := range site.Jobs {
		st, _ := s.jobStatus(name, job.Name)
		list = append(list, processView(name, job, st))
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "processes": list})
}

func (s *Server) handleCreateProcess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	var body struct {
		Name  string `json:"name"`
		Exec  string `json:"exec"`
		Start bool   `json:"start"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	jobName := strings.TrimSpace(body.Name)
	if err := config.ValidateTaskName(jobName); err != nil {
		s.writeErr(w, 400, "name: "+err.Error())
		return
	}
	execPath := strings.TrimSpace(body.Exec)
	if execPath == "" {
		execPath = "./processes/" + strings.ToLower(jobName) + "/" + sites.DefaultAppBinary
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
	if _, _, exists := site.FindJob(jobName); exists {
		s.writeErr(w, 409, "process already exists")
		return
	}
	job := config.Job{Name: jobName, Exec: execPath}
	cfg.Sites[idx].Jobs = append(cfg.Sites[idx].Jobs, job)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	// Best-effort unit install (may fail before binary exists or without systemd).
	_ = s.installJob(cfg.Sites[idx], job, body.Start)
	st, _ := s.jobStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "process": processView(name, job, st)})
}

func (s *Server) handleGetProcess(w http.ResponseWriter, r *http.Request) {
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
	job, _, ok := site.FindJob(jobName)
	if !ok {
		s.writeErr(w, 404, "process not found")
		return
	}
	st, _ := s.jobStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "process": processView(name, job, st)})
}

func (s *Server) handlePatchProcess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	var body struct {
		Exec    *string `json:"exec"`
		Runtime *struct {
			Kind    string `json:"kind"`
			Version string `json:"version"`
			Command string `json:"command"`
			Clear   bool   `json:"clear"`
		} `json:"runtime"`
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
	job, jIdx, ok := site.FindJob(jobName)
	if !ok {
		s.writeErr(w, 404, "process not found")
		return
	}
	if body.Exec != nil {
		execPath := strings.TrimSpace(*body.Exec)
		if execPath == "" {
			s.writeErr(w, 400, "exec is required")
			return
		}
		job.Exec = execPath
	}
	if body.Runtime != nil {
		if body.Runtime.Clear || strings.TrimSpace(body.Runtime.Kind) == "" {
			job.Runtime = nil
		} else {
			job.Runtime = &config.SiteRuntime{
				Kind:    body.Runtime.Kind,
				Version: body.Runtime.Version,
				Command: body.Runtime.Command,
			}
			if err := config.ValidateNodeRuntimeFields(job.Runtime); err != nil {
				s.writeErr(w, 400, "runtime: "+err.Error())
				return
			}
		}
	}
	if job.HasNodeRuntime() && job.Runtime.Command == "" {
		if err := s.ensureNodeRuntime(job.Runtime.Version); err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
	}
	cfg.Sites[idx].Jobs[jIdx] = job
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	active := false
	if st, err := s.jobStatus(name, job.Name); err == nil && st != nil {
		active = st.Active
	}
	if err := s.installJob(cfg.Sites[idx], job, active); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if active {
		_ = s.restartJob(name, job.Name)
	}
	st, _ := s.jobStatus(name, job.Name)
	s.writeJSON(w, 200, map[string]any{"ok": true, "process": processView(name, job, st)})
}

func (s *Server) handleDeleteProcess(w http.ResponseWriter, r *http.Request) {
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
	_, jIdx, ok := site.FindJob(jobName)
	if !ok {
		s.writeErr(w, 404, "process not found")
		return
	}
	cfg.Sites[idx].Jobs = append(site.Jobs[:jIdx], site.Jobs[jIdx+1:]...)
	if err := config.Save(s.configPath(), cfg); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	_ = s.removeJob(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleProcessStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	st, err := s.jobStatus(name, jobName)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleStartProcess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	if err := s.startJob(name, jobName); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.jobStatus(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleStopProcess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	if err := s.stopJob(name, jobName); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.jobStatus(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleRestartProcess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	if err := s.restartJob(name, jobName); err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	st, _ := s.jobStatus(name, jobName)
	s.writeJSON(w, 200, map[string]any{"ok": true, "status": st})
}

func (s *Server) handleListProcessFS(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	jobName := strings.TrimSpace(r.PathValue("job"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	entries, err := s.listProcessDir(name, jobName, path)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "entries": entries, "path": path})
}

func (s *Server) handleDeployProcessBinary(w http.ResponseWriter, r *http.Request) {
	s.deployProcessUpload(w, r, false)
}

func (s *Server) handleDeployProcessZip(w http.ResponseWriter, r *http.Request) {
	s.deployProcessUpload(w, r, true)
}

func (s *Server) deployProcessUpload(w http.ResponseWriter, r *http.Request, zip bool) {
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
	if _, _, ok := site.FindJob(jobName); !ok {
		s.writeErr(w, 404, "process not found")
		return
	}
	staged, err := s.stageUpload(w, r, name, "proc-"+jobName, zip)
	if err != nil {
		return
	}
	if zip {
		root, err := s.deployProcessZip(name, jobName, staged)
		if err != nil {
			s.writeErr(w, 400, err.Error())
			return
		}
		s.writeJSON(w, 200, map[string]any{"ok": true, "root": root, "job": jobName})
		return
	}
	dest, err := s.deployProcessBinary(name, jobName, staged)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "binary": dest, "job": jobName})
}

func (s *Server) installJob(site config.Site, job config.Job, start bool) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.InstallJobArgs{
			Domain: site.Name, Job: job.Name, Exec: job.Exec, Runtime: job.Runtime, Start: start, Limits: site.Limits,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpInstallJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	_, err := s.ops().InstallJobUnit(site, job, start)
	return err
}

func (s *Server) removeJob(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RemoveJobArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpRemoveJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RemoveJob(domain, job)
}

func (s *Server) jobStatus(domain, job string) (*helper.JobStatusResult, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.JobStatusArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpJobStatus, Args: raw})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var st helper.JobStatusResult
		if err := json.Unmarshal([]byte(resp.Message), &st); err != nil {
			return nil, err
		}
		return &st, nil
	}
	return s.ops().JobStatus(domain, job)
}

func (s *Server) startJob(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.StartJobArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpStartJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().StartJob(domain, job)
}

func (s *Server) stopJob(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.StopJobArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpStopJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().StopJob(domain, job)
}

func (s *Server) restartJob(domain, job string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RestartJobArgs{Domain: domain, Job: job})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpRestartJob, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RestartJob(domain, job)
}

func (s *Server) deployProcessBinary(domain, job, path string) (string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployProcessBinaryArgs{Domain: domain, Job: job, BinaryPath: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpDeployProcessBinary, Args: raw})
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
	return s.ops().DeployProcessBinary(ctx, domain, job, path)
}

func (s *Server) deployProcessZip(domain, job, path string) (string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeployProcessZipArgs{Domain: domain, Job: job, ZipPath: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpDeployProcessZip, Args: raw})
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
	return s.ops().DeployProcessZip(ctx, domain, job, path)
}

func (s *Server) listProcessDir(domain, job, path string) ([]helper.PublicFSEntry, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ListProcessDirArgs{Domain: domain, Job: job, Path: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpListProcessDir, Args: raw})
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
	return s.ops().ListProcessDir(domain, job, path)
}
