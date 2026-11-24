package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/update"
	"github.com/weborpanel/webor/internal/version"
)

// MountUpdate registers /api/update/* routes (auth required).
func (s *Server) MountUpdate(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/update/status", s.handleUpdateStatus)
	mux.HandleFunc("POST /api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /api/update/request", s.handleUpdateRequest)
}

func (s *Server) updateChannelConfig() update.ChannelConfig {
	return update.LoadChannelConfig("")
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	cfg := s.updateChannelConfig()
	s.writeJSON(w, 200, map[string]any{
		"ok":      true,
		"current": version.Version,
		"channel": cfg.Channel,
		"base_url": cfg.BaseURL,
	})
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	cfg := s.updateChannelConfig()
	var body struct {
		Channel string `json:"channel"`
		BaseURL string `json:"base_url"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	ch := strings.TrimSpace(body.Channel)
	if ch == "" {
		ch = cfg.Channel
	}
	base := strings.TrimSpace(body.BaseURL)
	if base == "" {
		base = cfg.BaseURL
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	st, err := update.Check(ctx, update.CheckOptions{Channel: ch, BaseURL: base})
	if err != nil {
		s.writeErr(w, 502, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":     true,
		"status": st,
		"cli":    "sudo wbr update apply -channel " + st.Channel,
	})
}

func (s *Server) handleUpdateRequest(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if s.HelperSocket == "" {
		s.writeErr(w, 503, "helper socket not configured")
		return
	}
	cfg := s.updateChannelConfig()
	var body struct {
		Channel string `json:"channel"`
		BaseURL string `json:"base_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	ch := strings.TrimSpace(body.Channel)
	if ch == "" {
		ch = cfg.Channel
	}
	base := strings.TrimSpace(body.BaseURL)
	if base == "" {
		base = cfg.BaseURL
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	st, err := update.Check(ctx, update.CheckOptions{Channel: ch, BaseURL: base})
	if err != nil {
		s.writeErr(w, 502, err.Error())
		return
	}
	if !st.Newer {
		s.writeErr(w, 400, "no newer version on channel "+st.Channel+" (latest "+st.Latest+")")
		return
	}
	bound, _ := json.Marshal(map[string]any{
		"channel":  st.Channel,
		"version":  st.Latest,
		"base_url": st.BaseURL,
	})
	raw, _ := json.Marshal(ipc.RequestApprovalArgs{Operation: ipc.OpApplyUpdate, Args: bound})
	resp, err := ipc.Call(s.HelperSocket, ipc.Request{
		ID: "panel", Operation: ipc.OpRequestApproval, Args: raw,
	})
	if err != nil {
		s.writeErr(w, 502, err.Error())
		return
	}
	if !resp.OK {
		s.writeErr(w, 400, resp.Error)
		return
	}
	id := ""
	for _, part := range strings.Fields(resp.Message) {
		if strings.HasPrefix(part, "approval_id=") {
			id = strings.TrimPrefix(part, "approval_id=")
		}
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":          true,
		"approval_id": id,
		"message":     resp.Message,
		"status":      st,
		"next": []string{
			"sudo wbr approve " + id,
			"sudo wbr update apply -channel " + st.Channel + " -approval-id " + id,
		},
	})
}
