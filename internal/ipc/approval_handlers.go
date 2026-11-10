package ipc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/webooza/webooza/internal/auth"
)

func makeRequestApproval(store *auth.Store) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
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
		a, err := store.Create(args.Operation, bound)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("approval_id=%s expires=%s", a.ID, a.ExpiresAt.UTC().Format(time.RFC3339)),
		}
	}
}

func makeApprove(store *auth.Store) HandlerFunc {
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
		if err := store.Approve(args.ID, by); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "approved " + args.ID}
	}
}

func makeDangerousDemo(store *auth.Store) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
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
		// Bind consume to the destructive args only (not the approval id itself).
		bound := map[string]string{"domain": args.Domain}
		if err := store.Consume(args.ApprovalID, OpDangerousDemo, bound); err != nil {
			return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("dangerous demo executed for %s (spike noop)", args.Domain),
		}
	}
}
