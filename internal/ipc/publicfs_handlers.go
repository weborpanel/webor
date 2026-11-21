package ipc

import (
	"encoding/json"
	"fmt"

	"github.com/weborpanel/webor/internal/helper"
)

func makeListPublicDir(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ListPublicDirArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateListPublicDirArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		entries, err := ops.ListPublicDir(args.Domain, args.Path)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, err := json.Marshal(entries)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeMkdirPublic(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args MkdirPublicArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateMkdirPublicArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.MkdirPublic(args.Domain, args.Path); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: fmt.Sprintf("mkdir public/%s", args.Path)}
	}
}

func makeRenamePublic(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args RenamePublicArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateRenamePublicArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.RenamePublic(args.Domain, args.From, args.To); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: fmt.Sprintf("renamed %s -> %s", args.From, args.To)}
	}
}

func makeDeletePublicPath(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args DeletePublicPathArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateDeletePublicPathArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := ops.DeletePublicPath(args.Domain, args.Path, args.Recursive); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: fmt.Sprintf("deleted public/%s", args.Path)}
	}
}

func makeReadPublicFile(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ReadPublicFileArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateReadPublicFileArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		name, b64, size, err := ops.ReadPublicFileBase64(args.Domain, args.Path)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		raw, err := json.Marshal(map[string]any{"name": name, "size": size, "content_b64": b64})
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: string(raw)}
	}
}

func makeExtractPublicArchive(ops *helper.Ops) HandlerFunc {
	return func(req Request, _ PeerCred) Response {
		var args ExtractPublicArchiveArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return Response{ID: req.ID, OK: false, Error: "invalid args: " + err.Error()}
		}
		if err := ValidateExtractPublicArchiveArgs(args); err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		dest, err := ops.ExtractPublicArchive(args.Domain, args.ArchivePath, args.Dest, args.Format)
		if err != nil {
			return Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{ID: req.ID, OK: true, Message: fmt.Sprintf("extracted into %s", dest)}
	}
}
