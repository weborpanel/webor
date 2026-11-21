package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/weborpanel/webor/internal/ipc"
)

// MountApprovals registers panel routes that request step-up approvals via the helper.
func (s *Server) MountApprovals(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/approvals", s.handleRequestApproval)
	mux.HandleFunc("GET /api/approvals/{id}", s.handleGetApproval)
}

func (s *Server) handleRequestApproval(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if s.HelperSocket == "" {
		s.writeErr(w, 503, "helper socket not configured")
		return
	}
	var body struct {
		Operation string          `json:"operation"`
		Args      json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	body.Operation = strings.TrimSpace(body.Operation)
	if body.Operation == "" {
		s.writeErr(w, 400, "operation is required")
		return
	}
	if len(body.Args) == 0 {
		body.Args = json.RawMessage(`{}`)
	}
	raw, _ := json.Marshal(ipc.RequestApprovalArgs{Operation: body.Operation, Args: body.Args})
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
	// Message format: approval_id=... expires=...
	id := ""
	expires := ""
	for _, part := range strings.Fields(resp.Message) {
		if strings.HasPrefix(part, "approval_id=") {
			id = strings.TrimPrefix(part, "approval_id=")
		}
		if strings.HasPrefix(part, "expires=") {
			expires = strings.TrimPrefix(part, "expires=")
		}
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":          true,
		"approval_id": id,
		"expires":     expires,
		"operation":   body.Operation,
		"args":        json.RawMessage(body.Args),
		"message":     resp.Message,
		"hint":        "sudo wbr approve " + id,
	})
}

func (s *Server) handleGetApproval(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		s.writeErr(w, 400, "id is required")
		return
	}
	if s.HelperSocket == "" {
		s.writeErr(w, 503, "helper socket not configured")
		return
	}
	raw, _ := json.Marshal(ipc.GetApprovalArgs{ID: id})
	resp, err := ipc.Call(s.HelperSocket, ipc.Request{
		ID: "panel", Operation: ipc.OpGetApproval, Args: raw,
	})
	if err != nil {
		s.writeErr(w, 502, err.Error())
		return
	}
	if !resp.OK {
		status := 400
		if strings.Contains(resp.Error, "not found") {
			status = 404
		}
		s.writeErr(w, status, resp.Error)
		return
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(resp.Message), &payload); err != nil {
		s.writeErr(w, 500, fmt.Sprintf("decode approval: %v", err))
		return
	}
	payload["ok"] = true
	s.writeJSON(w, 200, payload)
}
