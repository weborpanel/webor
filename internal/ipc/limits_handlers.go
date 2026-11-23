package ipc

import (
	"encoding/json"

	"github.com/weborpanel/webor/internal/helper"
)

func makeApplySiteLimits(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ApplySiteLimitsArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateApplySiteLimitsArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.ApplySiteLimits(args.Site); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: "site limits applied"}
	}
}
