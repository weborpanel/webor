package ipc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/dbengine"
)

func makeInstallPostgres(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args InstallPostgresArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateInstallPostgresArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		actor := fmt.Sprintf("uid=%d", cred.UID)
		bound := map[string]any{}
		if err := store.Consume(args.ApprovalID, OpInstallPostgres, bound); err != nil {
			_ = log.Append(actor, OpInstallPostgres, bound, "approval: "+err.Error())
			return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
		}
		if err := ops.InstallPostgres(); err != nil {
			_ = log.Append(actor, OpInstallPostgres, bound, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpInstallPostgres, bound, "ok")
		return Response{ID: req.ID, OK: true, Message: "postgresql installed and enabled"}
	}
}

func makeInstallMariaDB(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args InstallMariaDBArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateInstallMariaDBArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if store == nil {
			return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
		}
		actor := fmt.Sprintf("uid=%d", cred.UID)
		bound := map[string]any{}
		if err := store.Consume(args.ApprovalID, OpInstallMariaDB, bound); err != nil {
			_ = log.Append(actor, OpInstallMariaDB, bound, "approval: "+err.Error())
			return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
		}
		if err := ops.InstallMariaDB(); err != nil {
			_ = log.Append(actor, OpInstallMariaDB, bound, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpInstallMariaDB, bound, "ok")
		return Response{ID: req.ID, OK: true, Message: "mariadb installed and enabled"}
	}
}

func makeEnsureSiteDatabase(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args EnsureSiteDatabaseArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnsureSiteDatabaseArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		engine := strings.ToLower(strings.TrimSpace(args.Engine))
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		id := dbengine.NormalizeID(domain, args.ID)
		actor := fmt.Sprintf("uid=%d", cred.UID)
		if dbengine.NeedsApproval(engine) {
			if store == nil {
				return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
			}
			if args.ApprovalID == "" {
				return Response{ID: req.ID, OK: false, Error: "approval_id is required"}
			}
			bound := map[string]any{
				"domain": domain,
				"engine": engine,
				"id":     id,
			}
			if err := store.Consume(args.ApprovalID, OpEnsureSiteDatabase, bound); err != nil {
				_ = log.Append(actor, OpEnsureSiteDatabase, bound, "approval: "+err.Error())
				return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
			}
		}
		res, err := ops.EnsureSiteDatabase(domain, engine, id, args.StateDir)
		if err != nil {
			_ = log.Append(actor, OpEnsureSiteDatabase, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpEnsureSiteDatabase, map[string]any{"domain": domain, "engine": engine, "id": id}, "ok")
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeDropSiteDatabase(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args DropSiteDatabaseArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDropSiteDatabaseArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		engine := strings.ToLower(strings.TrimSpace(args.Engine))
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		id := dbengine.NormalizeID(domain, args.ID)
		actor := fmt.Sprintf("uid=%d", cred.UID)
		if dbengine.NeedsApproval(engine) {
			if store == nil {
				return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
			}
			if args.ApprovalID == "" {
				return Response{ID: req.ID, OK: false, Error: "approval_id is required"}
			}
			bound := map[string]any{
				"domain": domain,
				"engine": engine,
				"id":     id,
			}
			if err := store.Consume(args.ApprovalID, OpDropSiteDatabase, bound); err != nil {
				_ = log.Append(actor, OpDropSiteDatabase, bound, "approval: "+err.Error())
				return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
			}
		}
		if err := ops.DropSiteDatabase(domain, engine, id, args.StateDir); err != nil {
			_ = log.Append(actor, OpDropSiteDatabase, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpDropSiteDatabase, map[string]any{"domain": domain, "engine": engine, "id": id}, "ok")
		return Response{ID: req.ID, OK: true, Message: "dropped site database for " + domain + "/" + id}
	}
}

func makeRotateDatabasePassword(ops *helper.Ops, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args RotateDatabasePasswordArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRotateDatabasePasswordArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		engine := strings.ToLower(strings.TrimSpace(args.Engine))
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		id := dbengine.NormalizeID(domain, args.ID)
		actor := fmt.Sprintf("uid=%d", cred.UID)
		res, err := ops.RotateDatabasePassword(domain, engine, id, args.StateDir)
		if err != nil {
			_ = log.Append(actor, OpRotateDatabasePassword, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpRotateDatabasePassword, map[string]any{"domain": domain, "engine": engine, "id": id}, "ok")
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makePostgresStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		res, err := ops.PostgresStatus()
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeMariaDBStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		res, err := ops.MariaDBStatus()
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
