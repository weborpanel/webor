package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/webooza/webooza/internal/ipc"
	"github.com/webooza/webooza/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version.Version)
	case "approve":
		os.Exit(runApprove(os.Args[2:]))
	case "request-approval":
		os.Exit(runRequestApproval(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runApprove(args []string) int {
	fs := flag.NewFlagSet("approve", flag.ExitOnError)
	socket := fs.String("socket", ipc.DefaultSocketPath, "helper Unix socket")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: wbz approve [-socket path] <approval-id>")
		return 2
	}
	if err := requireRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	id := fs.Arg(0)
	raw, _ := json.Marshal(ipc.ApproveArgs{ID: id, ApprovedBy: "wbz"})
	resp, err := ipc.Call(*socket, ipc.Request{ID: "wbz", Operation: ipc.OpApprove, Args: raw})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printResp(resp)
	if !resp.OK {
		return 1
	}
	return 0
}

func runRequestApproval(args []string) int {
	fs := flag.NewFlagSet("request-approval", flag.ExitOnError)
	socket := fs.String("socket", ipc.DefaultSocketPath, "helper Unix socket")
	op := fs.String("op", ipc.OpDangerousDemo, "operation to bind")
	domain := fs.String("domain", "", "domain arg for DangerousDemo")
	argsJSON := fs.String("args", "", "raw JSON args object to bind (overrides -domain)")
	_ = fs.Parse(args)

	var nested json.RawMessage
	if strings.TrimSpace(*argsJSON) != "" {
		nested = json.RawMessage(*argsJSON)
	} else if *domain != "" {
		nested, _ = json.Marshal(map[string]string{"domain": *domain})
	} else {
		fmt.Fprintln(os.Stderr, "usage: wbz request-approval -domain example.com")
		return 2
	}
	raw, _ := json.Marshal(ipc.RequestApprovalArgs{Operation: *op, Args: nested})
	resp, err := ipc.Call(*socket, ipc.Request{ID: "wbz", Operation: ipc.OpRequestApproval, Args: raw})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printResp(resp)
	if !resp.OK {
		return 1
	}
	return 0
}

func requireRoot() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	if u.Uid != "0" {
		return fmt.Errorf("wbz approve must run as root (got uid %s)", u.Uid)
	}
	return nil
}

func printResp(resp ipc.Response) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func usage() {
	fmt.Fprintf(os.Stderr, `wbz — Webooza admin CLI

Usage:
  wbz version
  wbz request-approval -domain example.com
  wbz approve <approval-id>          (root; console step-up)

`)
}
