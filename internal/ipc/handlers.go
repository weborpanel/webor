package ipc

import (
	"encoding/json"
	"fmt"
)

// DefaultHandlers returns the Milestone 0 allowlist handlers.
// Each handler validates arguments and returns a not-implemented result
// until real apply logic is wired in later milestones.
func DefaultHandlers() map[string]HandlerFunc {
	return map[string]HandlerFunc{
		OpCreateSiteUser: handleCreateSiteUser,
		OpWriteUnitFile:  handleWriteUnitFile,
		OpReloadCaddy:    handleReloadCaddy,
	}
}

func handleCreateSiteUser(req Request, _ PeerCred) Response {
	var args CreateSiteUserArgs
	if err := json.Unmarshal(req.Args, &args); err != nil {
		return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
	}
	if err := ValidateCreateSiteUserArgs(args); err != nil {
		return Response{ID: req.ID, OK: false, Error: err.Error()}
	}
	return Response{
		ID:      req.ID,
		OK:      false,
		Error:   "not implemented",
		Message: fmt.Sprintf("would create site user for %s", args.Domain),
	}
}

func handleWriteUnitFile(req Request, _ PeerCred) Response {
	var args WriteUnitFileArgs
	if err := json.Unmarshal(req.Args, &args); err != nil {
		return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
	}
	if err := ValidateWriteUnitFileArgs(args); err != nil {
		return Response{ID: req.ID, OK: false, Error: err.Error()}
	}
	return Response{
		ID:      req.ID,
		OK:      false,
		Error:   "not implemented",
		Message: fmt.Sprintf("would write unit %s for %s", args.Unit, args.Domain),
	}
}

func handleReloadCaddy(req Request, _ PeerCred) Response {
	var args ReloadCaddyArgs
	if err := json.Unmarshal(req.Args, &args); err != nil {
		return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
	}
	if err := ValidateReloadCaddyArgs(args); err != nil {
		return Response{ID: req.ID, OK: false, Error: err.Error()}
	}
	return Response{
		ID:      req.ID,
		OK:      false,
		Error:   "not implemented",
		Message: fmt.Sprintf("would reload caddy from %s", args.ConfigPath),
	}
}
