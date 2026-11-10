package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/helper"
)

// DefaultHandlers returns handlers without an approval store (tests / limited).
func DefaultHandlers() map[string]HandlerFunc {
	return Handlers(helper.DefaultOps(), nil)
}

// Handlers wires validated IPC operations to helper.Ops and optional approvals.
func Handlers(ops *helper.Ops, store *auth.Store) map[string]HandlerFunc {
	if ops == nil {
		ops = helper.DefaultOps()
	}
	h := map[string]HandlerFunc{
		OpCreateSiteUser: makeCreateSiteUser(ops),
		OpWriteUnitFile:  makeWriteUnitFile(ops),
		OpReloadCaddy:    makeReloadCaddy(ops),
		OpRequestApproval: makeRequestApproval(store),
		OpApprove:         makeApprove(store),
		OpDangerousDemo:   makeDangerousDemo(store),
	}
	return h
}

func makeCreateSiteUser(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args CreateSiteUserArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateCreateSiteUserArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.CreateSiteUser(args.Domain); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("created site user %s under %s", helper.SiteUsername(args.Domain), helper.SiteRoot(args.Domain)),
		}
	}
}

func makeWriteUnitFile(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args WriteUnitFileArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateWriteUnitFileArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.WriteUnitFile(args.Domain, args.Unit, args.Content); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("wrote unit %s for %s", args.Unit, args.Domain),
		}
	}
}

func makeReloadCaddy(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ReloadCaddyArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateReloadCaddyArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ops.ReloadCaddy(ctx, args.ConfigPath); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("reloaded caddy from %s", args.ConfigPath),
		}
	}
}
