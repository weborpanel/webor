package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/weborpanel/webor/internal/deploy"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
)

const (
	maxPublicFileBytes    = 32 * 1024 * 1024
	maxPublicArchiveBytes = 100 * 1024 * 1024
	// Panel text editor cap (textarea + JSON round-trip).
	maxPublicEditBytes = 1 * 1024 * 1024
)

func (s *Server) mountFSRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sites/{name}/fs", s.handleFSList)
	mux.HandleFunc("DELETE /api/sites/{name}/fs", s.handleFSDelete)
	mux.HandleFunc("POST /api/sites/{name}/fs/mkdir", s.handleFSMkdir)
	mux.HandleFunc("POST /api/sites/{name}/fs/rename", s.handleFSRename)
	mux.HandleFunc("GET /api/sites/{name}/fs/raw", s.handleFSRaw)
	mux.HandleFunc("GET /api/sites/{name}/fs/content", s.handleFSContentGet)
	mux.HandleFunc("PUT /api/sites/{name}/fs/content", s.handleFSContentPut)
	mux.HandleFunc("POST /api/sites/{name}/fs/upload", s.handleFSUpload)
	mux.HandleFunc("POST /api/sites/{name}/fs/extract", s.handleFSExtract)
}

func (s *Server) requireSiteName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return "", false
	}
	sitesMu.Lock()
	_, site, err := s.findSite(name)
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return "", false
	}
	if site == nil {
		s.writeErr(w, 404, "site not found")
		return "", false
	}
	return name, true
}

func (s *Server) handleFSList(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	path := r.URL.Query().Get("path")
	entries, err := s.listPublicDir(name, path)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": path, "entries": entries})
}

func (s *Server) handleFSDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	path := r.URL.Query().Get("path")
	recursive := r.URL.Query().Get("recursive") == "1" || r.URL.Query().Get("recursive") == "true"
	if err := s.deletePublicPath(name, path, recursive); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleFSMkdir(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.mkdirPublic(name, body.Path); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": body.Path})
}

func (s *Server) handleFSRename(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.renamePublic(name, body.From, body.To); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "from": body.From, "to": body.To})
}

func (s *Server) handleFSRaw(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	path := r.URL.Query().Get("path")
	fname, data, err := s.readPublicFile(name, path)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, fname))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

func (s *Server) handleFSContentGet(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.writeErr(w, 400, "path is required")
		return
	}
	fname, data, err := s.readPublicFile(name, path)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	if len(data) > maxPublicEditBytes {
		s.writeErr(w, 400, fmt.Sprintf("file exceeds editor limit of %d bytes — download instead", maxPublicEditBytes))
		return
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		s.writeErr(w, 400, "binary file — download instead of editing")
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"path":    path,
		"name":    fname,
		"content": string(data),
		"size":    len(data),
	})
}

func (s *Server) handleFSContentPut(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxPublicEditBytes+4096)).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid JSON body")
		return
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		s.writeErr(w, 400, "path is required")
		return
	}
	data := []byte(body.Content)
	if len(data) > maxPublicEditBytes {
		s.writeErr(w, 400, fmt.Sprintf("content exceeds editor limit of %d bytes", maxPublicEditBytes))
		return
	}
	if err := s.writePublicFile(name, path, data); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": path, "size": len(data)})
}

func (s *Server) handleFSUpload(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	if err := r.ParseMultipartForm(maxPublicFileBytes + 1024); err != nil {
		s.writeErr(w, 400, "invalid multipart form")
		return
	}
	dir := strings.TrimSpace(r.FormValue("path"))
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 400, "file is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPublicFileBytes+1))
	if err != nil {
		s.writeErr(w, 400, "read upload")
		return
	}
	if len(data) > maxPublicFileBytes {
		s.writeErr(w, 400, fmt.Sprintf("file exceeds %d bytes", maxPublicFileBytes))
		return
	}
	base := filepath.Base(hdr.Filename)
	rel := base
	if dir != "" && dir != "." {
		rel = filepath.ToSlash(filepath.Join(dir, base))
	}
	if err := s.writePublicFile(name, rel, data); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": rel, "size": len(data)})
}

func (s *Server) handleFSExtract(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	if !s.requireSiteScope(w, r, name, "files") {
		return
	}
	if _, ok := s.requireSiteName(w, r); !ok {
		return
	}
	ct := r.Header.Get("Content-Type")
	var dest, format, archivePath string
	if strings.HasPrefix(ct, "multipart/") {
		if err := r.ParseMultipartForm(maxPublicArchiveBytes + 1024); err != nil {
			s.writeErr(w, 400, "invalid multipart form")
			return
		}
		dest = strings.TrimSpace(r.FormValue("dest"))
		format = strings.TrimSpace(r.FormValue("format"))
		file, hdr, err := r.FormFile("file")
		if err != nil {
			s.writeErr(w, 400, "file is required")
			return
		}
		defer file.Close()
		if format == "" {
			format = deploy.DetectArchiveFormat(hdr.Filename)
		}
		dir := filepath.Join(s.uploadsRoot(), name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			s.writeErr(w, 500, "upload dir: "+err.Error())
			return
		}
		safeName := filepath.Base(hdr.Filename)
		if safeName == "" || safeName == "." {
			safeName = "upload.bin"
		}
		staged := filepath.Join(dir, "extract-"+safeName)
		tmp := staged + ".tmp"
		out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
		if err != nil {
			s.writeErr(w, 500, err.Error())
			return
		}
		n, err := io.Copy(out, io.LimitReader(file, maxPublicArchiveBytes+1))
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
		if n > maxPublicArchiveBytes {
			_ = os.Remove(tmp)
			s.writeErr(w, 400, fmt.Sprintf("archive exceeds %d bytes", maxPublicArchiveBytes))
			return
		}
		if err := os.Rename(tmp, staged); err != nil {
			_ = os.Remove(tmp)
			s.writeErr(w, 500, err.Error())
			return
		}
		archivePath = staged
		defer func() { _ = os.Remove(staged) }()
	} else {
		var body struct {
			Dest        string `json:"dest"`
			Format      string `json:"format"`
			ArchivePath string `json:"archive_path"`
			StagedName  string `json:"staged_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		dest = body.Dest
		format = body.Format
		archivePath = strings.TrimSpace(body.ArchivePath)
		if archivePath == "" && body.StagedName != "" {
			archivePath = filepath.Join(s.uploadsRoot(), name, filepath.Base(body.StagedName))
		}
		if archivePath == "" {
			s.writeErr(w, 400, "file or archive_path is required")
			return
		}
	}
	outDir, err := s.extractPublicArchive(name, archivePath, dest, format)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "dest": dest, "extracted_to": outDir})
}

func (s *Server) listPublicDir(domain, path string) ([]helper.PublicFSEntry, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ListPublicDirArgs{Domain: domain, Path: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpListPublicDir, Args: raw})
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
	return s.ops().ListPublicDir(domain, path)
}

func (s *Server) mkdirPublic(domain, path string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.MkdirPublicArgs{Domain: domain, Path: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpMkdirPublic, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().MkdirPublic(domain, path)
}

func (s *Server) renamePublic(domain, from, to string) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.RenamePublicArgs{Domain: domain, From: from, To: to})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpRenamePublic, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().RenamePublic(domain, from, to)
}

func (s *Server) deletePublicPath(domain, path string, recursive bool) error {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.DeletePublicPathArgs{Domain: domain, Path: path, Recursive: recursive})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpDeletePublicPath, Args: raw})
		if err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		return nil
	}
	return s.ops().DeletePublicPath(domain, path, recursive)
}

func (s *Server) readPublicFile(domain, path string) (string, []byte, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ReadPublicFileArgs{Domain: domain, Path: path})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpReadPublicFile, Args: raw})
		if err != nil {
			return "", nil, err
		}
		if !resp.OK {
			return "", nil, fmt.Errorf("%s", resp.Error)
		}
		var body struct {
			Name       string `json:"name"`
			ContentB64 string `json:"content_b64"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &body); err != nil {
			return "", nil, err
		}
		data, err := base64.StdEncoding.DecodeString(body.ContentB64)
		if err != nil {
			return "", nil, err
		}
		return body.Name, data, nil
	}
	return s.ops().ReadPublicFile(domain, path)
}

func (s *Server) writePublicFile(domain, path string, data []byte) error {
	return s.siteFS().WritePublicFile(domain, path, data)
}

func (s *Server) extractPublicArchive(domain, archivePath, dest, format string) (string, error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.ExtractPublicArchiveArgs{
			Domain: domain, ArchivePath: archivePath, Dest: dest, Format: format,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{ID: "panel", Operation: ipc.OpExtractPublicArchive, Args: raw})
		if err != nil {
			return "", err
		}
		if !resp.OK {
			return "", fmt.Errorf("%s", resp.Error)
		}
		return resp.Message, nil
	}
	return s.ops().ExtractPublicArchive(domain, archivePath, dest, format)
}
