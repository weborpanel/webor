package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		OpCreateSiteUser:  makeCreateSiteUser(ops),
		OpDeleteSiteUser:  makeDeleteSiteUser(ops, store),
		OpWritePublicFile: makeWritePublicFile(ops),
		OpDeployStaticZip: makeDeployStaticZip(ops),
		OpDeployAppBinary: makeDeployAppBinary(ops),
		OpDeployAppGit:    makeDeployAppGit(ops),
		OpRunBuild:        makeRunBuild(ops),
		OpWriteUnitFile:   makeWriteUnitFile(ops),
		OpReloadCaddy:     makeReloadCaddy(ops),
		OpRunCronNow:      makeRunCronNow(ops),
		OpCronStatus:      makeCronStatus(ops),
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

func makeDeleteSiteUser(ops *helper.Ops, store *auth.Store) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeleteSiteUserArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeleteSiteUserArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if args.DeleteData {
			if store == nil {
				return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
			}
			bound := map[string]any{
				"domain":      strings.ToLower(strings.TrimSpace(args.Domain)),
				"delete_data": true,
			}
			if err := store.Consume(args.ApprovalID, OpDeleteSiteUser, bound); err != nil {
				return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
			}
		}
		if err := ops.DeleteSiteUser(args.Domain, args.DeleteData); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		msg := fmt.Sprintf("deleted site user %s", helper.SiteUsername(args.Domain))
		if args.DeleteData {
			msg += " and data under " + helper.SiteRoot(args.Domain)
		}
		return Response{ID: req.ID, OK: true, Message: msg}
	}
}

func makeRunBuild(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RunBuildArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRunBuildArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 65*time.Minute)
		defer cancel()
		res, err := ops.RunBuild(ctx, args.Domain, args.Command, args.MemoryMax, args.TimeoutSec)
		if err != nil {
			msg := err.Error()
			if res != nil && res.Stderr != "" {
				msg = msg + ": " + truncate(res.Stderr, 512)
			}
			return Response{ID: req.ID, OK: false, Error: msg}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("build ok unit=%s duration=%s", res.Unit, res.Duration),
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func makeWritePublicFile(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args WritePublicFileArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateWritePublicFileArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.WritePublicFile(args.Domain, args.Path, []byte(args.Content)); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("wrote public/%s for %s", args.Path, args.Domain),
		}
	}
}

func makeDeployStaticZip(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployStaticZipArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployStaticZipArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		pub, err := ops.DeployStaticZip(args.Domain, args.ZipPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("extracted zip into %s", pub),
		}
	}
}

func makeDeployAppBinary(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployAppBinaryArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployAppBinaryArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		res, err := ops.DeployAppBinary(ctx, args.Domain, args.BinaryPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("deployed release %s → %s", res.ReleaseID, res.Binary),
		}
	}
}

func makeDeployAppGit(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployAppGitArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployAppGitArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		timeout := 65 * time.Minute
		if args.TimeoutSec > 0 {
			timeout = time.Duration(args.TimeoutSec+30) * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		res, err := ops.DeployAppGit(ctx, args.Domain, args.GitURL, args.GitRef, args.BuildCommand, args.Artifact, args.MemoryMax, args.TimeoutSec)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("git deploy release %s → %s", res.ReleaseID, res.Binary),
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

func makeRunCronNow(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RunCronNowArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRunCronNowArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RunCronNow(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			ID:      req.ID,
			OK:      true,
			Message: fmt.Sprintf("started cron job %q for %s", args.Job, args.Domain),
		}
	}
}

func makeCronStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args CronStatusArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateCronStatusArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		st, err := ops.CronStatus(args.Domain, args.Job)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		msg := fmt.Sprintf("timer=%s active=%v result=%s failed=%v next=%s",
			st.Timer, st.TimerActive, st.LastResult, st.Failed, st.NextElapse)
		return Response{ID: req.ID, OK: true, Message: msg}
	}
}
