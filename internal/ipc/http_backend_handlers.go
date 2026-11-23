package ipc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
)

func backendIDOrDefault(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return "api"
	}
	return id
}

func makeEnsureHTTPBackend(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args EnsureHTTPBackendArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnsureHTTPBackendArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		backend := config.SiteBackend{
			ID:        backendIDOrDefault(args.ID),
			Enabled:   true,
			Prefix:    args.Prefix,
			Paths:     args.Paths,
			WebSocket: args.WebSocket,
			Exec:      args.Exec,
			Port:      args.Port,
			Runtime:   args.Runtime,
		}
		config.NormalizeBackend(&backend)
		site := config.Site{
			Name:     args.Domain,
			Type:     config.SiteTypeStatic,
			Backends: []config.SiteBackend{backend},
		}
		res, err := ops.EnsureHTTPBackend(site, backend)
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
		if err := ops.RemoveHTTPBackend(args.Domain, backendIDOrDefault(args.ID)); err != nil {
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
		res, err := ops.DeployHTTPBackend(ctx, args.Domain, backendIDOrDefault(args.ID), args.BinaryPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeDeployHTTPBackendZip(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeployHTTPBackendZipArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeployHTTPBackendZipArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		res, err := ops.DeployHTTPBackendZip(ctx, args.Domain, backendIDOrDefault(args.ID), args.ZipPath)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeRestartHTTPBackend(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RestartHTTPBackendArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateHTTPBackendStatusArgs(HTTPBackendStatusArgs{Domain: args.Domain, ID: args.ID}); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RestartHTTPBackend(args.Domain, backendIDOrDefault(args.ID)); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "http backend restarted"}
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
		active := ops.HTTPBackendActive(args.Domain, backendIDOrDefault(args.ID))
		raw, _ := json.Marshal(map[string]any{"active": active})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeListAPIDir(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ListAPIDirArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateListAPIDirArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		entries, err := ops.ListAPIDir(args.Domain, args.ID, args.Path)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(entries)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
