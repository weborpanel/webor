package ipc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/weborpanel/webor/internal/helper"
)

func makeEnsureNodeRuntime(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args EnsureNodeRuntimeArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateEnsureNodeRuntimeArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		res, err := ops.EnsureNodeRuntime(ctx, args.Version)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(res)
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
