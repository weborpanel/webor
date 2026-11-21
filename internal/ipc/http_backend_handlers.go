package ipc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
)

func makeEnsureHTTPBackend(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args EnsureHTTPBackendArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnsureHTTPBackendArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		site := config.Site{
			Name: args.Domain,
			Type: config.SiteTypeStatic,
			Backend: &config.SiteBackend{
				Enabled:   true,
				Prefix:    args.Prefix,
				Paths:     args.Paths,
				WebSocket: args.WebSocket,
				Exec:      args.Exec,
				Port:      args.Port,
			},
		}
		config.NormalizeBackend(site.Backend)
		res, err := ops.EnsureHTTPBackend(site)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeRemoveHTTPBackend(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RemoveHTTPBackendArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRemoveHTTPBackendArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RemoveHTTPBackend(args.Domain); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "http backend removed"}
	}
}

func makeDeployHTTPBackend(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployHTTPBackendArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployHTTPBackendArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		res, err := ops.DeployHTTPBackend(ctx, args.Domain, args.BinaryPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeHTTPBackendStatus(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args HTTPBackendStatusArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateHTTPBackendStatusArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		active := ops.HTTPBackendActive(args.Domain)
		raw, _ := json.Marshal(map[string]any{"active": active})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
