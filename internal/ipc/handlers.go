package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
)

// DefaultHandlers returns handlers without an approval store (tests / limited).
func DefaultHandlers() map[string]HandlerFunc {
	return Handlers(helper.DefaultOps(), nil, nil)
}

// Handlers wires validated IPC operations to helper.Ops and optional approvals.
func Handlers(ops *helper.Ops, store *auth.Store, log *audit.Logger) map[string]HandlerFunc {
	if ops == nil {
		ops = helper.DefaultOps()
	}
	h := map[string]HandlerFunc{
		OpCreateSiteUser:         makeCreateSiteUser(ops),
		OpDeleteSiteUser:         makeDeleteSiteUser(ops, store, log),
		OpWritePublicFile:        makeWritePublicFile(ops),
		OpListPublicDir:          makeListPublicDir(ops),
		OpMkdirPublic:            makeMkdirPublic(ops),
		OpRenamePublic:           makeRenamePublic(ops),
		OpDeletePublicPath:       makeDeletePublicPath(ops),
		OpReadPublicFile:         makeReadPublicFile(ops),
		OpExtractPublicArchive:   makeExtractPublicArchive(ops),
		OpDeployStaticZip:        makeDeployStaticZip(ops),
		OpDeployAppBinary:        makeDeployAppBinary(ops),
		OpDeployAppZip:           makeDeployAppZip(ops),
		OpDeployAppGit:           makeDeployAppGit(ops),
		OpRunBuild:               makeRunBuild(ops),
		OpWriteUnitFile:          makeWriteUnitFile(ops),
		OpReloadCaddy:            makeReloadCaddy(ops),
		OpRunCronNow:             makeRunCronNow(ops),
		OpCronStatus:             makeCronStatus(ops),
		OpInstallCronJob:         makeInstallCronJob(ops),
		OpRemoveCronJob:          makeRemoveCronJob(ops),
		OpEnableCronTimer:        makeEnableCronTimer(ops),
		OpDisableCronTimer:       makeDisableCronTimer(ops),
		OpDeployCronBinary:       makeDeployCronBinary(ops),
		OpDeployCronZip:          makeDeployCronZip(ops),
		OpListCronDir:            makeListCronDir(ops),
		OpInstallJob:             makeInstallJob(ops),
		OpRemoveJob:              makeRemoveJob(ops),
		OpJobStatus:              makeJobStatus(ops),
		OpStartJob:               makeStartJob(ops),
		OpStopJob:                makeStopJob(ops),
		OpRestartJob:             makeRestartJob(ops),
		OpDeployProcessBinary:    makeDeployProcessBinary(ops),
		OpDeployProcessZip:       makeDeployProcessZip(ops),
		OpListProcessDir:         makeListProcessDir(ops),
		OpJournalTail:            makeJournalTail(ops),
		OpListSiteLogFiles:       makeListSiteLogFiles(ops),
		OpReadSiteLogFile:        makeReadSiteLogFile(ops),
		OpListHostLogFiles:       makeListHostLogFiles(ops),
		OpReadHostLogFile:        makeReadHostLogFile(ops),
		OpRotateLogs:             makeRotateLogs(ops),
		OpEnsureLogrotate:        makeEnsureLogrotate(ops),
		OpRequestApproval:        makeRequestApproval(store, log),
		OpApprove:                makeApprove(store, log),
		OpListApprovals:          makeListApprovals(store),
		OpGetApproval:            makeGetApproval(store),
		OpDangerousDemo:          makeDangerousDemo(store, log),
		OpInstallPostgres:        makeInstallPostgres(ops, store, log),
		OpInstallMariaDB:         makeInstallMariaDB(ops, store, log),
		OpInstallValkey:          makeInstallValkey(ops, store, log),
		OpEnsureSiteDatabase:     makeEnsureSiteDatabase(ops, store, log),
		OpDropSiteDatabase:       makeDropSiteDatabase(ops, store, log),
		OpRotateDatabasePassword: makeRotateDatabasePassword(ops, log),
		OpEnsureSiteCache:        makeEnsureSiteCache(ops, log),
		OpDropSiteCache:          makeDropSiteCache(ops, log),
		OpRotateCachePassword:    makeRotateCachePassword(ops, log),
		OpPostgresStatus:         makePostgresStatus(ops),
		OpEnsurePostgresAuth:     makeEnsurePostgresAuth(ops),
		OpEnsureMariaDBGrants:    makeEnsureMariaDBGrants(ops),
		OpMariaDBStatus:          makeMariaDBStatus(ops),
		OpValkeyStatus:           makeValkeyStatus(ops),
		OpEnsureHTTPBackend:      makeEnsureHTTPBackend(ops),
		OpRemoveHTTPBackend:      makeRemoveHTTPBackend(ops),
		OpDeployHTTPBackend:      makeDeployHTTPBackend(ops),
		OpDeployHTTPBackendZip:   makeDeployHTTPBackendZip(ops),
		OpHTTPBackendStatus:      makeHTTPBackendStatus(ops),
		OpRestartHTTPBackend:     makeRestartHTTPBackend(ops),
		OpListAPIDir:             makeListAPIDir(ops),
		OpBackupConfigure:        makeBackupConfigure(ops, store, log),
		OpBackupRun:              makeBackupRun(ops),
		OpBackupList:             makeBackupList(ops),
		OpBackupForget:           makeBackupForget(ops, store, log),
		OpBackupRestore:          makeBackupRestore(ops, store, log),
		OpBackupStatus:           makeBackupStatus(ops),
		OpEnsureBackupSchedule:   makeEnsureBackupSchedule(ops),
		OpEnsureNodeRuntime:      makeEnsureNodeRuntime(ops),
		OpApplySiteLimits:        makeApplySiteLimits(ops),
		OpCheckUpdate:            makeCheckUpdate(ops),
		OpApplyUpdate:            makeApplyUpdate(ops, store, log),
		OpSetPanelListenHost:     makeSetPanelListenHost(ops, log),
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

func makeDeleteSiteUser(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args DeleteSiteUserArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeleteSiteUserArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		domain := strings.ToLower(strings.TrimSpace(args.Domain))
		actor := fmt.Sprintf("uid=%d", cred.UID)
		if args.DeleteData {
			if store == nil {
				return Response{ID: req.ID, OK: false, Error: "approvals not configured"}
			}
			bound := map[string]any{
				"domain":      domain,
				"delete_data": true,
			}
			if err := store.Consume(args.ApprovalID, OpDeleteSiteUser, bound); err != nil {
				_ = log.Append(actor, OpDeleteSiteUser, bound, "approval: "+err.Error())
				return Response{ID: req.ID, OK: false, Error: "approval: " + err.Error()}
			}
		}
		if err := ops.DeleteSiteUser(domain, args.DeleteData); err != nil {
			_ = log.Append(actor, OpDeleteSiteUser, args, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if args.DeleteData {
			for _, child := range args.CascadeDomains {
				child = strings.ToLower(strings.TrimSpace(child))
				if child == "" || child == domain {
					continue
				}
				if err := ops.DeleteSiteUser(child, true); err != nil {
					_ = log.Append(actor, OpDeleteSiteUser, map[string]any{"domain": child, "cascade": true}, err.Error())
					return Response{ID: req.ID, OK: false, Error: "cascade " + child + ": " + err.Error()}
				}
			}
			stateDir := strings.TrimSpace(args.StateDir)
			for _, db := range args.DropDatabases {
				eng := strings.ToLower(strings.TrimSpace(db.Engine))
				id := strings.TrimSpace(db.ID)
				if eng == "" {
					continue
				}
				if err := ops.DropSiteDatabase(domain, eng, id, stateDir); err != nil {
					_ = log.Append(actor, OpDropSiteDatabase, map[string]any{"domain": domain, "engine": eng, "id": id}, err.Error())
					return Response{ID: req.ID, OK: false, Error: "drop database " + id + ": " + err.Error()}
				}
			}
		}
		msg := fmt.Sprintf("deleted site user %s", helper.SiteUsername(domain))
		if args.DeleteData {
			msg += " and data under " + helper.SiteRoot(domain)
			if len(args.CascadeDomains) > 0 {
				msg += fmt.Sprintf(" (+%d subdomains)", len(args.CascadeDomains))
			}
			if len(args.DropDatabases) > 0 {
				msg += fmt.Sprintf(" (+%d databases)", len(args.DropDatabases))
			}
		}
		_ = log.Append(actor, OpDeleteSiteUser, args, "ok")
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
		res, err := ops.RunBuild(ctx, args.Domain, args.Command, args.MemoryMax, args.CPUQuota, args.TimeoutSec)
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

func makeDeployAppZip(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployAppZipArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployAppZipArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		res, err := ops.DeployAppZip(ctx, args.Domain, args.ZipPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
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
		res, err := ops.DeployAppGit(ctx, args.Domain, args.GitURL, args.GitRef, args.BuildCommand, args.Artifact, args.MemoryMax, args.CPUQuota, args.TimeoutSec)
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
		raw, _ := json.Marshal(st)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeJournalTail(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args JournalTailArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateJournalTailArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		sel := strings.TrimSpace(args.Unit)
		if sel == "" {
			sel = "app"
		}
		unit, text, err := ops.SiteLogTail(args.Domain, sel, args.Lines, args.APIIds)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{"unit": unit, "text": text})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeListSiteLogFiles(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ListSiteLogFilesArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateListSiteLogFilesArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		files, err := ops.ListSiteLogFiles(args.Domain)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: helper.LogFilesJSON(files)}
	}
}

func makeReadSiteLogFile(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ReadSiteLogFileArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateReadSiteLogFileArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		info, text, err := ops.ReadSiteLogFile(args.Domain, args.File, args.Lines)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: helper.LogFileReadJSON(info, text)}
	}
}

func makeListHostLogFiles(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		files, err := ops.ListHostLogFiles()
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: helper.LogFilesJSON(files)}
	}
}

func makeReadHostLogFile(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ReadHostLogFileArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateReadHostLogFileArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		info, text, err := ops.ReadHostLogFile(args.File, args.Lines)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: helper.LogFileReadJSON(info, text)}
	}
}

func makeRotateLogs(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RotateLogsArgs
		if len(req.Args) > 0 && string(req.Args) != "null" {
			if err := json.Unmarshal(req.Args, &args); err != nil {
				return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
			}
		}
		cfgPath := strings.TrimSpace(args.ConfigPath)
		if cfgPath == "" {
			cfgPath = "/etc/webor/config.yaml"
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			// Still rotate host logs even if config missing.
			cfg = &config.Config{Version: 1}
		}
		if err := ops.RotateAllLogs(cfg); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "ok"}
	}
}

func makeEnsureLogrotate(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		if err := ops.EnsureLogrotateSchedule(); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "ok"}
	}
}
