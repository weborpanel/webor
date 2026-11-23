package ipc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
)

func makeInstallCronJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args InstallCronJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateInstallCronJobArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		site := config.Site{Name: args.Domain, Limits: args.Limits}
		job := config.CronJob{Name: args.Job, Schedule: args.Schedule, Exec: args.Exec, Runtime: args.Runtime}
		units, err := ops.InstallCronJob(site, job)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{
			"service": units.ServiceName,
			"timer":   units.TimerName,
		})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeRemoveCronJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RemoveCronJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRemoveCronJobArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RemoveCronJob(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "cron job removed"}
	}
}

func makeEnableCronTimer(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args EnableCronTimerArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnableCronTimerArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		site := config.Site{Name: args.Domain, Limits: args.Limits}
		job := config.CronJob{Name: args.Job, Schedule: args.Schedule, Exec: args.Exec, Runtime: args.Runtime}
		if err := ops.EnableCronTimer(site, job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "cron timer enabled"}
	}
}

func makeDisableCronTimer(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DisableCronTimerArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDisableCronTimerArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.DisableCronTimer(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "cron timer disabled"}
	}
}

func makeDeployCronBinary(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployCronBinaryArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployCronBinaryArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		dest, err := ops.DeployCronBinary(ctx, args.Domain, args.Job, args.BinaryPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{"binary": dest})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeDeployCronZip(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployCronZipArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployCronZipArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		root, err := ops.DeployCronZip(ctx, args.Domain, args.Job, args.ZipPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{"root": root})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeListCronDir(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ListCronDirArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateListCronDirArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		entries, err := ops.ListCronDir(args.Domain, args.Job, args.Path)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(entries)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeInstallJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args InstallJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateInstallJobArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		site := config.Site{Name: args.Domain, Limits: args.Limits}
		job := config.Job{Name: args.Job, Exec: args.Exec, Runtime: args.Runtime}
		units, err := ops.InstallJobUnit(site, job, args.Start)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{"service": units.ServiceName})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeRemoveJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RemoveJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRemoveJobArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RemoveJob(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "job removed"}
	}
}

func makeJobStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args JobStatusArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateJobStatusArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		st, err := ops.JobStatus(args.Domain, args.Job)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(st)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeStartJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args StartJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateJobStatusArgs(JobStatusArgs{Domain: args.Domain, Job: args.Job}); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.StartJob(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "job started"}
	}
}

func makeStopJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args StopJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateJobStatusArgs(JobStatusArgs{Domain: args.Domain, Job: args.Job}); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.StopJob(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "job stopped"}
	}
}

func makeRestartJob(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RestartJobArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateJobStatusArgs(JobStatusArgs{Domain: args.Domain, Job: args.Job}); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RestartJob(args.Domain, args.Job); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "job restarted"}
	}
}

func makeDeployProcessBinary(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployProcessBinaryArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployProcessBinaryArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		dest, err := ops.DeployProcessBinary(ctx, args.Domain, args.Job, args.BinaryPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{"binary": dest})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeDeployProcessZip(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployProcessZipArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployProcessZipArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		root, err := ops.DeployProcessZip(ctx, args.Domain, args.Job, args.ZipPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(map[string]any{"root": root})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeListProcessDir(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ListProcessDirArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateListProcessDirArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		entries, err := ops.ListProcessDir(args.Domain, args.Job, args.Path)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(entries)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
