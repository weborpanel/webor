package ipc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/helper"
)

func makeSetPanelListenHost(ops *helper.Ops, log *audit.Logger) HandlerFunc {
	return func(req Request, cred PeerCred) Response {
		var args SetPanelListenHostArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateSetPanelListenHostArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		host := strings.TrimSpace(args.Host)
		envPath := strings.TrimSpace(args.EnvPath)
		actor := fmt.Sprintf("uid=%d", cred.UID)
		bound := map[string]any{"host": host}
		if envPath != "" {
			bound["env_path"] = envPath
		}

		var (
			listenAddr string
			err        error
		)
		// Root CLI can restart synchronously; panel path needs the IPC reply first.
		if cred.UID == 0 {
			listenAddr, err = ops.SetPanelListenHostSync(host, envPath)
		} else {
			listenAddr, err = ops.SetPanelListenHost(host, envPath)
		}
		if err != nil {
			if log != nil {
				_ = log.Append(actor, OpSetPanelListenHost, bound, err.Error())
			}
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if log != nil {
			_ = log.Append(actor, OpSetPanelListenHost, bound, "ok "+listenAddr)
		}
		raw, _ := json.Marshal(map[string]any{
			"listen":        listenAddr,
			"host":          host,
			"public_listen": host == "0.0.0.0",
		})
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}
