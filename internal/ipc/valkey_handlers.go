package ipc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/valkey"
)

func makeInstallValkey(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args InstallValkeyArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateInstallValkeyArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		actor := fmt.Sprintf("uid=%d", cred.UID)
		bound := map[string]any{}
		if err := store.Consume(args.ApprovalID, OpInstallValkey, bound); err != nil {
			_ = log.Append(actor, OpInstallValkey, bound, "approval: "+err.Error())
			return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
		}
		if err := ops.InstallValkey(); err != nil {
			_ = log.Append(actor, OpInstallValkey, bound, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpInstallValkey, bound, "ok")
		return Response{ID: req.ID, OK: true, Message: "valkey installed and enabled"}
	}
}

func makeEnsureSiteCache(ops *helper.Ops, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args EnsureSiteCacheArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnsureSiteCacheArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		engine := strings.ToLower(strings.TrimSpace(args.Engine))
		if engine == "" {
			engine = valkey.Engine
		}
		actor := fmt.Sprintf("uid=%d", cred.UID)
		res, err := ops.EnsureSiteCache(domain, engine, args.StateDir)
		if err != nil {
			_ = log.Append(actor, OpEnsureSiteCache, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpEnsureSiteCache, map[string]any{"domain": domain}, "ok")
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeDropSiteCache(ops *helper.Ops, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args DropSiteCacheArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDropSiteCacheArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		actor := fmt.Sprintf("uid=%d", cred.UID)
		if err := ops.DropSiteCache(domain, args.StateDir); err != nil {
			_ = log.Append(actor, OpDropSiteCache, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpDropSiteCache, map[string]any{"domain": domain}, "ok")
		return Response{ID: req.ID, OK: true, Message: "dropped site cache for " + domain}
	}
}

func makeRotateCachePassword(ops *helper.Ops, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args RotateCachePasswordArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRotateCachePasswordArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		actor := fmt.Sprintf("uid=%d", cred.UID)
		res, err := ops.RotateCachePassword(domain, args.StateDir)
		if err != nil {
			_ = log.Append(actor, OpRotateCachePassword, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpRotateCachePassword, map[string]any{"domain": domain}, "ok")
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeValkeyStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		res, err := ops.ValkeyStatus()
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
