package ipc

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/providers/dbengine"
)

func makeRequestApproval(store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		var args RequestApprovalArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if args.Operation == "" {
			return Response{ID: req.ID, OK: false, Error: "operation is required"}
		}
		var bound any
		if len(args.Args) > 0 {
			if err := json.Unmarshal(args.Args, &bound); err != nil {
				return Response{ID: req.ID, OK: false, Error: "invalid nested args: " + err.Error()}
			}
		} else {
			bound = map[string]any{}
		}
		bound = canonicalizeApprovalArgs(args.Operation, bound)
		a, err := store.Create(args.Operation, bound)
		actor := fmt.Sprintf("uid=%d", cred.UID)
		if err != nil {
			_ = log.Append(actor, OpRequestApproval, map[string]any{"operation": args.Operation, "args": bound}, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpRequestApproval, map[string]any{
			"approval_id": a.ID,
			"operation":   a.Operation,
			"args":        bound,
		}, "ok")
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("approval_id=%s expires=%s", a.ID, a.ExpiresAt.UTC().Format(time.RFC3339)),
		}
	}
}

func makeApprove(store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		if cred.UID != 0 {
			return Response{ID: req.ID, OK: false, Error: "Approve requires root (uid 0)"}
		}
		var args ApproveArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		by := args.ApprovedBy
		if by == "" {
			by = fmt.Sprintf("uid=%d pid=%d", cred.UID, cred.PID)
		}
		// Load first so the CLI/panel can show what was granted.
		prior, getErr := store.Get(args.ID)
		if err := store.Approve(args.ID, by); err != nil {
			_ = log.Append(by, OpApprove, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(by, OpApprove, args, "ok")
		msg := "approved " + args.ID
		if getErr == nil && prior != nil {
			msg = fmt.Sprintf("approved %s operation=%s args=%s", args.ID, prior.Operation, prior.ArgsJSON)
		}
		return Response{ID: req.ID, OK: true, Message: msg}
	}
}

func makeGetApproval(store *auth.Store) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		var args GetApprovalArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return Response{ID: req.ID, OK: false, Error: "id is required"}
		}
		a, err := store.Get(id)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		status := "pending"
		expired := time.Now().UTC().After(a.ExpiresAt)
		if a.UsedAt != nil {
			status = "used"
		} else if a.ApprovedBy != "" {
			status = "approved"
		} else if expired {
			status = "expired"
		}
		out := map[string]any{
			"id":          a.ID,
			"operation":   a.Operation,
			"args_json":   a.ArgsJSON,
			"status":      status,
			"approved":    a.ApprovedBy != "",
			"approved_by": a.ApprovedBy,
			"used":        a.UsedAt != nil,
			"expired":     expired,
			"expires_at":  a.ExpiresAt.UTC().Format(time.RFC3339),
			"created_at":  a.CreatedAt.UTC().Format(time.RFC3339),
		}
		if a.UsedAt != nil {
			out["used_at"] = a.UsedAt.UTC().Format(time.RFC3339)
		}
		raw, _ := json.Marshal(out)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeListApprovals(store *auth.Store) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		if cred.UID != 0 {
			return Response{ID: req.ID, OK: false, Error: "ListApprovals requires root (uid 0)"}
		}
		list, err := store.ListPending()
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		type row struct {
			ID         string `json:"id"`
			Operation  string `json:"operation"`
			ArgsJSON   string `json:"args_json"`
			ExpiresAt  string `json:"expires_at"`
			ApprovedBy string `json:"approved_by,omitempty"`
		}
		out := make([]row, 0, len(list))
		for _, a := range list {
			out = append(out, row{
				ID:         a.ID,
				Operation:  a.Operation,
				ArgsJSON:   a.ArgsJSON,
				ExpiresAt:  a.ExpiresAt.UTC().Format(time.RFC3339),
				ApprovedBy: a.ApprovedBy,
			})
		}
		raw, _ := json.Marshal(out)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

// canonicalizeApprovalArgs normalizes args so Create and Consume bind the same JSON
// (e.g. blank database id → domain default slug).
func canonicalizeApprovalArgs(operation string, bound any) any {
	m, ok := bound.(map[string]any)
	if !ok || m == nil {
		return bound
	}
	switch operation {
	case OpEnsureSiteDatabase, OpDropSiteDatabase:
		domain, _ := m["domain"].(string)
		engine, _ := m["engine"].(string)
		id, _ := m["id"].(string)
		domain = strings.ToLower(strings.TrimSpace(domain))
		engine = strings.ToLower(strings.TrimSpace(engine))
		id = dbengine.NormalizeID(domain, id)
		return map[string]any{"domain": domain, "engine": engine, "id": id}
	case OpApplyUpdate:
		ch, _ := m["channel"].(string)
		ver, _ := m["version"].(string)
		base, _ := m["base_url"].(string)
		ch = strings.ToLower(strings.TrimSpace(ch))
		ver = strings.TrimPrefix(strings.TrimSpace(ver), "v")
		base = strings.TrimRight(strings.TrimSpace(base), "/")
		if base == "" {
			base = "https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor"
		}
		return map[string]any{"channel": ch, "version": ver, "base_url": base}
	default:
		return bound
	}
}

func makeDangerousDemo(store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		var args DangerousDemoArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDomain(args.Domain); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if args.ApprovalID == "" {
			return Response{ID: req.ID, OK: false, Error: "approval_id is required"}
		}
		actor := fmt.Sprintf("uid=%d", cred.UID)
		// Bind consume to the destructive args only (not the approval id itself).
		bound := map[string]string{"domain": args.Domain}
		if err := store.Consume(args.ApprovalID, OpDangerousDemo, bound); err != nil {
			_ = log.Append(actor, OpDangerousDemo, bound, "approval: "+err.Error())
			return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
		}
		_ = log.Append(actor, OpDangerousDemo, bound, "ok")
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("dangerous demo executed for %s (spike noop)", args.Domain),
		}
	}
}
