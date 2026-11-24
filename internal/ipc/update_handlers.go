package ipc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/update"
)

func makeCheckUpdate(_ *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args CheckUpdateArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateCheckUpdateArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		cfg := update.LoadChannelConfig("") // infers beta from running version when unset
		ch := strings.TrimSpace(args.Channel)
		if ch == "" {
			ch = cfg.Channel
		}
		base := strings.TrimSpace(args.BaseURL)
		if base == "" {
			base = cfg.BaseURL
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		st, err := update.Check(ctx, update.CheckOptions{Channel: ch, BaseURL: base})
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(st)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeApplyUpdate(_ *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args ApplyUpdateArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateApplyUpdateArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ch := strings.ToLower(strings.TrimSpace(args.Channel))
		base := strings.TrimRight(strings.TrimSpace(args.BaseURL), "/")
		if base == "" {
			base = update.DefaultBaseURL
		}
		ver := strings.TrimPrefix(strings.TrimSpace(args.Version), "v")
		bound := map[string]any{
			"channel":  ch,
			"version":  ver,
			"base_url": base,
		}
		actor := "panel"
		if cred.UID == 0 {
			actor = "root"
		}
		// Non-root callers (panel) must present a consumed approval. Root (wbr) may omit.
		if cred.UID != 0 {
			if store == nil {
				return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
			}
			if strings.TrimSpace(args.ApprovalID) == "" {
				return Response{ID: req.ID, OK: false, Error: "approval_id is required"}
			}
			if err := store.Consume(args.ApprovalID, OpApplyUpdate, bound); err != nil {
				if log != nil {
					_ = log.Append(actor, OpApplyUpdate, bound, "approval: "+err.Error())
				}
				return Response{ID: req.ID, OK: false, Error: err.Error()}
			}
		} else if strings.TrimSpace(args.ApprovalID) != "" && store != nil {
			// Optional: root can still consume a pending approval so the UI request is marked used.
			if err := store.Consume(args.ApprovalID, OpApplyUpdate, bound); err != nil {
				if log != nil {
					_ = log.Append(actor, OpApplyUpdate, bound, "approval: "+err.Error())
				}
				return Response{ID: req.ID, OK: false, Error: err.Error()}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		res, err := update.Apply(ctx, update.ApplyOptions{
			Channel:         ch,
			BaseURL:         base,
			ExpectedVersion: ver,
			Force:           args.Force,
			SaveChannel:     args.SaveChannel,
		})
		if err != nil {
			if log != nil {
				_ = log.Append(actor, OpApplyUpdate, bound, err.Error())
			}
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if log != nil {
			_ = log.Append(actor, OpApplyUpdate, bound, "ok")
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
