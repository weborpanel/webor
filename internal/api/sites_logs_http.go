package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/sites"
)

func (s *Server) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, found := cfg.FindSite(name)
	if !found {
		s.writeErr(w, 404, "site not found")
		return
	}

	sel := strings.TrimSpace(r.URL.Query().Get("unit"))
	if sel == "" {
		sel = "app"
	}
	lines := 100
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 2000 {
			s.writeErr(w, 400, "lines must be 1..2000")
			return
		}
		lines = n
	}

	var apiIDs []string
	if sel == "apis" {
		for _, b := range site.EnabledBackends() {
			apiIDs = append(apiIDs, b.ID)
		}
		if len(apiIDs) == 0 {
			s.writeErr(w, 400, "no APIs configured")
			return
		}
	}

	unit, text, err := s.siteLogTail(name, sel, lines, apiIDs)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":    true,
		"unit":  unit,
		"lines": lines,
		"text":  text,
	})
}

func (s *Server) handleListSiteLogFiles(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if _, _, found := cfg.FindSite(name); !found {
		s.writeErr(w, 404, "site not found")
		return
	}
	files, err := s.listSiteLogFiles(name)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "files": files})
}

func (s *Server) handleReadSiteLogFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	file := strings.TrimSpace(r.PathValue("file"))
	if name == "" || file == "" {
		s.writeErr(w, 400, "site name and file required")
		return
	}
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	if _, _, found := cfg.FindSite(name); !found {
		s.writeErr(w, 404, "site not found")
		return
	}
	lines := 200
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 2000 {
			s.writeErr(w, 400, "lines must be 1..2000")
			return
		}
		lines = n
	}
	info, text, err := s.readSiteLogFile(name, file, lines)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "file": info, "text": text, "lines": lines})
}

func (s *Server) handleListHostLogFiles(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	files, err := s.listHostLogFiles()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "files": files})
}

func (s *Server) handleReadHostLogFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	file := strings.TrimSpace(r.PathValue("file"))
	if file == "" {
		s.writeErr(w, 400, "file required")
		return
	}
	lines := 200
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 2000 {
			s.writeErr(w, 400, "lines must be 1..2000")
			return
		}
		lines = n
	}
	info, text, err := s.readHostLogFile(file, lines)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "file": info, "text": text, "lines": lines})
}

func (s *Server) siteLogTail(domain, selector string, lines int, apiIDs []string) (unit, text string, err error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.JournalTailArgs{
			Domain: domain,
			Unit:   selector,
			Lines:  lines,
			APIIds: apiIDs,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpJournalTail, Args: raw,
		})
		if err != nil {
			return "", "", err
		}
		if !resp.OK {
			return "", "", fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			Unit string `json:"unit"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &out); err != nil {
			msg := resp.Message
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				return msg[:i], msg[i+1:], nil
			}
			return "", "", fmt.Errorf("invalid journal response")
		}
		return out.Unit, out.Text, nil
	}
	return s.ops().SiteLogTail(domain, selector, lines, apiIDs)
}

func (s *Server) listSiteLogFiles(domain string) ([]logging.FileInfo, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ListSiteLogFilesArgs{Domain: domain})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpListSiteLogFiles, Args: raw,
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			Files []logging.FileInfo `json:"files"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &out); err != nil {
			return nil, err
		}
		return out.Files, nil
	}
	return logging.ListDir(sites.Logs(domain), false)
}

func (s *Server) readSiteLogFile(domain, file string, lines int) (logging.FileInfo, string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ReadSiteLogFileArgs{Domain: domain, File: file, Lines: lines})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpReadSiteLogFile, Args: raw,
		})
		if err != nil {
			return logging.FileInfo{}, "", err
		}
		if !resp.OK {
			return logging.FileInfo{}, "", fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			File logging.FileInfo `json:"file"`
			Text string           `json:"text"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &out); err != nil {
			return logging.FileInfo{}, "", err
		}
		return out.File, out.Text, nil
	}
	return s.ops().ReadSiteLogFile(domain, file, lines)
}

func (s *Server) listHostLogFiles() ([]logging.FileInfo, error) {
	if s.HelperSocket != "" {
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpListHostLogFiles, Args: json.RawMessage(`{}`),
		})
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			Files []logging.FileInfo `json:"files"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &out); err != nil {
			return nil, err
		}
		return out.Files, nil
	}
	return logging.ListDir(logging.HostDir, true)
}

func (s *Server) readHostLogFile(file string, lines int) (logging.FileInfo, string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ReadHostLogFileArgs{File: file, Lines: lines})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpReadHostLogFile, Args: raw,
		})
		if err != nil {
			return logging.FileInfo{}, "", err
		}
		if !resp.OK {
			return logging.FileInfo{}, "", fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			File logging.FileInfo `json:"file"`
			Text string           `json:"text"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &out); err != nil {
			return logging.FileInfo{}, "", err
		}
		return out.File, out.Text, nil
	}
	return s.ops().ReadHostLogFile(file, lines)
}
