package ipc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
)

func backupCfgPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/etc/webor/config.yaml"
	}
	return p
}

func makeBackupConfigure(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, peer PeerCred) Response {
		var args BackupConfigureArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateBackupConfigureArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		bound := map[string]any{
			"endpoint": args.Endpoint,
			"bucket":   args.Bucket,
			"prefix":   args.Prefix,
			"region":   args.Region,
		}
		actor := "panel"
		if store != nil {
			if err := store.Consume(args.ApprovalID, OpBackupConfigure, bound); err != nil {
				_ = log.Append(actor, OpBackupConfigure, bound, "approval: "+err.Error())
				return Response{ID: req.ID, OK: false, Error: err.Error()}
			}
		}
		b := config.Backup{
			Enabled:  true,
			Schedule: args.Schedule,
			S3: config.BackupS3{
				Endpoint: args.Endpoint,
				Region:   args.Region,
				Bucket:   args.Bucket,
				Prefix:   args.Prefix,
			},
			Retention: config.BackupRetention{
				KeepDaily:  args.KeepDaily,
				KeepWeekly: args.KeepWeekly,
			},
			Repository: args.Repository,
		}
		res, err := ops.BackupConfigure(backupCfgPath(args.ConfigPath), b, args.AccessKey, args.SecretKey, args.ResticPassword, args.Region)
		if err != nil {
			_ = log.Append(actor, OpBackupConfigure, bound, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpBackupConfigure, bound, "ok")
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeBackupRun(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args BackupRunArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateBackupRunArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		res, err := ops.BackupRun(ctx, backupCfgPath(args.ConfigPath))
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeBackupList(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args BackupListArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateBackupListArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		res, err := ops.BackupList(ctx, backupCfgPath(args.ConfigPath))
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeBackupForget(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args BackupForgetArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateBackupForgetArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		bound := map[string]any{}
		actor := "panel"
		if store != nil {
			if err := store.Consume(args.ApprovalID, OpBackupForget, bound); err != nil {
				_ = log.Append(actor, OpBackupForget, bound, "approval: "+err.Error())
				return Response{ID: req.ID, OK: false, Error: err.Error()}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := ops.BackupForget(ctx, backupCfgPath(args.ConfigPath)); err != nil {
			_ = log.Append(actor, OpBackupForget, bound, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpBackupForget, bound, "ok")
		return Response{ID: req.ID, OK: true, Message: "forget ok"}
	}
}

func makeBackupRestore(ops *helper.Ops, store *auth.Store, log *audit.Logger) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args BackupRestoreArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateBackupRestoreArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		bound := map[string]any{
			"snapshot_id":   args.SnapshotID,
			"sites":         args.Sites,
			"include_panel": args.IncludePanel,
		}
		actor := "panel"
		if store != nil {
			if err := store.Consume(args.ApprovalID, OpBackupRestore, bound); err != nil {
				_ = log.Append(actor, OpBackupRestore, bound, "approval: "+err.Error())
				return Response{ID: req.ID, OK: false, Error: err.Error()}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		err := ops.BackupRestore(ctx, helper.BackupRestoreArgs{
			SnapshotID:   args.SnapshotID,
			Sites:        args.Sites,
			IncludePanel: args.IncludePanel,
			ConfigPath:   backupCfgPath(args.ConfigPath),
			StateDir:     args.StateDir,
		})
		if err != nil {
			_ = log.Append(actor, OpBackupRestore, bound, err.Error())
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		_ = log.Append(actor, OpBackupRestore, bound, "ok")
		return Response{ID: req.ID, OK: true, Message: "restore ok"}
	}
}

func makeBackupStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args BackupStatusArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateBackupStatusArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		res, err := ops.BackupStatus(backupCfgPath(args.ConfigPath))
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeEnsureBackupSchedule(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args EnsureBackupScheduleArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnsureBackupScheduleArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		cfg, err := config.Load(backupCfgPath(args.ConfigPath))
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.EnsureBackupSchedule(cfg); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "schedule ok"}
	}
}
