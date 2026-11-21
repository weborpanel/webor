package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/version"
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
	case "doctor":
		os.Exit(runDoctor(os.Args[2:]))
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
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: wbr approve list | wbr approve <approval-id>")
		return 2
	}
	if err := requireRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if fs.Arg(0) == "list" {
		resp, err := ipc.Call(*socket, ipc.Request{ID: "wbr", Operation: ipc.OpListApprovals, Args: json.RawMessage(`{}`)})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !resp.OK {
			fmt.Fprintln(os.Stderr, resp.Error)
			return 1
		}
		var rows []map[string]any
		if err := json.Unmarshal([]byte(resp.Message), &rows); err != nil {
			fmt.Println(resp.Message)
			return 0
		}
		if len(rows) == 0 {
			fmt.Println("no pending approvals")
			return 0
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rows)
		return 0
	}
	id := fs.Arg(0)
	raw, _ := json.Marshal(ipc.ApproveArgs{ID: id, ApprovedBy: "wbr"})
	resp, err := ipc.Call(*socket, ipc.Request{ID: "wbr", Operation: ipc.OpApprove, Args: raw})
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
	domain := fs.String("domain", "", "domain arg for DangerousDemo / DeleteSiteUser")
	deleteData := fs.Bool("delete-data", false, "bind delete_data=true (DeleteSiteUser)")
	argsJSON := fs.String("args", "", "raw JSON args object to bind (overrides -domain)")
	_ = fs.Parse(args)

	var nested json.RawMessage
	if strings.TrimSpace(*argsJSON) != "" {
		nested = json.RawMessage(*argsJSON)
	} else if *domain != "" {
		m := map[string]any{"domain": *domain}
		if *deleteData || *op == ipc.OpDeleteSiteUser {
			m["delete_data"] = true
			if *op == ipc.OpDangerousDemo {
				*op = ipc.OpDeleteSiteUser
			}
		}
		nested, _ = json.Marshal(m)
	} else {
		fmt.Fprintln(os.Stderr, "usage: wbr request-approval -domain example.com [-op DeleteSiteUser -delete-data]")
		return 2
	}
	raw, _ := json.Marshal(ipc.RequestApprovalArgs{Operation: *op, Args: nested})
	resp, err := ipc.Call(*socket, ipc.Request{ID: "wbr", Operation: ipc.OpRequestApproval, Args: raw})
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

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	_ = fs.Parse(args)
	fmt.Println("wbr doctor — host checks (guidance only; no changes made)")
	fmt.Println()
	printPort("80")
	printPort("443")
	fmt.Println()
	fmt.Println("Firewall: installer detects ufw/firewalld/nft/iptables.")
	fmt.Println("  Re-run: sudo ./install/install.sh --yes --skip-ptr   # reprints hints")
	fmt.Println("  Or open TCP for the panel first-run port plus 80/443 when behind Caddy.")
	fmt.Println()
	fmt.Println("Approvals:")
	fmt.Println("  sudo wbr approve list")
	fmt.Println("  sudo wbr approve <id>")
	fmt.Println()
	fmt.Println("Helper socket:", ipc.DefaultSocketPath)
	if st, err := os.Stat(ipc.DefaultSocketPath); err != nil {
		fmt.Println("  missing — is webor-helper running?")
	} else {
		fmt.Printf("  present mode=%s\n", st.Mode())
	}
	return 0
}

func printPort(port string) {
	fmt.Printf("Port %s: check with `ss -ltn 'sport = :%s'` on the host\n", port, port)
}

func requireRoot() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	if u.Uid != "0" {
		return fmt.Errorf("wbr approve must run as root (got uid %s)", u.Uid)
	}
	return nil
}

func printResp(resp ipc.Response) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func usage() {
	fmt.Fprintf(os.Stderr, `wbr — Webor admin CLI

Usage:
  wbr version
  wbr doctor
  wbr request-approval -domain example.com [-op DeleteSiteUser -delete-data]
  wbr approve list                 (root)
  wbr approve <approval-id>        (root; console step-up)

`)
}
