package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/update"
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
	case "backup":
		os.Exit(runBackup(os.Args[2:]))
	case "recover":
		os.Exit(runRecover(os.Args[2:]))
	case "update":
		os.Exit(runUpdate(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runBackup(args []string) int {
	if len(args) < 1 || args[0] != "run" {
		fmt.Fprintln(os.Stderr, "usage: wbr backup run [-socket PATH] [-config PATH]")
		return 2
	}
	fs := flag.NewFlagSet("backup run", flag.ExitOnError)
	socket := fs.String("socket", ipc.DefaultSocketPath, "helper Unix socket")
	configPath := fs.String("config", "/etc/webor/config.yaml", "config.yaml path")
	_ = fs.Parse(args[1:])
	raw, _ := json.Marshal(ipc.BackupRunArgs{ConfigPath: *configPath})
	resp, err := ipc.Call(*socket, ipc.Request{ID: "wbr", Operation: ipc.OpBackupRun, Args: raw})
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
	// Preview what root is approving (operation + bound args).
	prevRaw, _ := json.Marshal(ipc.GetApprovalArgs{ID: id})
	prev, err := ipc.Call(*socket, ipc.Request{ID: "wbr", Operation: ipc.OpGetApproval, Args: prevRaw})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !prev.OK {
		fmt.Fprintln(os.Stderr, prev.Error)
		return 1
	}
	fmt.Println("Approving request:")
	var preview map[string]any
	if err := json.Unmarshal([]byte(prev.Message), &preview); err != nil {
		fmt.Println(prev.Message)
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(preview)
	}
	fmt.Println()
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

func runRecover(args []string) int {
	if len(args) < 1 || args[0] != "totp" {
		fmt.Fprintln(os.Stderr, "usage: wbr recover totp [-state-dir PATH]")
		return 2
	}
	fs := flag.NewFlagSet("recover totp", flag.ExitOnError)
	stateDir := fs.String("state-dir", config.DefaultStateDir, "panel state directory (state.db)")
	_ = fs.Parse(args[1:])
	if err := requireRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	db, err := config.OpenDB(*stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer db.Close()
	acct := &auth.Account{DB: db}
	adm, err := acct.GetAdmin()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !adm.TOTPConfirmed && adm.TOTPSecret == "" {
		fmt.Println("TOTP is already disabled; password-only login is enough.")
		return 0
	}
	if err := acct.ResetTOTP(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("TOTP cleared for %s.\n", adm.Email)
	fmt.Println("Sign in with email and password, then re-enrol 2FA from Account if desired.")
	fmt.Println("All panel sessions were revoked.")
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
	fmt.Println("Lost authenticator (TOTP lockout):")
	fmt.Println("  sudo wbr recover totp")
	fmt.Println()
	fmt.Println("Helper socket:", ipc.DefaultSocketPath)
	if st, err := os.Stat(ipc.DefaultSocketPath); err != nil {
		fmt.Println("  missing — is webor-helper running?")
	} else {
		fmt.Printf("  present mode=%s\n", st.Mode())
	}
	fmt.Println()
	fmt.Println("systemd timers (per-site cron):")
	cmd := exec.Command("systemctl", "is-active", "timers.target")
	out, err := cmd.CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err != nil || state != "active" {
		fmt.Printf("  timers.target is %q — cron schedules need an active timers.target\n", state)
		fmt.Println("  Fix: systemctl start timers.target")
	} else {
		fmt.Println("  timers.target active — OK for per-site cron")
	}
	return 0
}

func runUpdate(args []string) int {
	sub := "apply"
	rest := args
	if len(args) > 0 {
		switch args[0] {
		case "check", "apply":
			sub = args[0]
			rest = args[1:]
		case "help", "-h", "--help":
			fmt.Fprintln(os.Stderr, `usage:
  wbr update check  [-channel beta] [-base-url URL] [-env PATH]
  wbr update apply  [-channel beta] [-base-url URL] [-env PATH] [-dry-run] [-force] [-save-channel] [-approval-id ID] [-socket PATH]
  wbr update        (same as apply)`)
			return 2
		}
	}

	fs := flag.NewFlagSet("update "+sub, flag.ExitOnError)
	channel := fs.String("channel", "", "stable or beta (default: WBR_CHANNEL from panel.env)")
	baseURL := fs.String("base-url", "", "release base URL (default from panel.env)")
	envPath := fs.String("env", update.DefaultPanelEnv, "panel.env path")
	dryRun := fs.Bool("dry-run", false, "show plan only (apply)")
	force := fs.Bool("force", false, "allow reinstall / non-newer version (apply)")
	saveChannel := fs.Bool("save-channel", false, "persist -channel/-base-url into panel.env")
	approvalID := fs.String("approval-id", "", "consume a pending ApplyUpdate approval via helper (apply)")
	socket := fs.String("socket", ipc.DefaultSocketPath, "helper Unix socket (with -approval-id)")
	_ = fs.Parse(rest)

	cfg := update.LoadChannelConfig(*envPath)
	ch := strings.TrimSpace(*channel)
	if ch == "" {
		ch = cfg.Channel
	}
	base := strings.TrimSpace(*baseURL)
	if base == "" {
		base = cfg.BaseURL
	}
	if err := update.ValidateChannel(ch); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if sub == "check" {
		st, err := update.Check(ctx, update.CheckOptions{Channel: ch, BaseURL: base})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(st)
		if st.Newer {
			fmt.Fprintf(os.Stderr, "update available: %s → %s (%s)\n", st.Current, st.Latest, st.Channel)
			fmt.Fprintln(os.Stderr, "apply with: sudo wbr update apply -channel "+st.Channel)
		} else {
			fmt.Fprintf(os.Stderr, "up to date on %s (%s)\n", st.Current, st.Channel)
		}
		return 0
	}

	// apply
	if err := requireRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if *approvalID != "" {
		st, err := update.Check(ctx, update.CheckOptions{Channel: ch, BaseURL: base})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		raw, _ := json.Marshal(ipc.ApplyUpdateArgs{
			Channel:     ch,
			BaseURL:     base,
			Version:     st.Latest,
			Force:       *force || *dryRun,
			SaveChannel: *saveChannel || strings.TrimSpace(*channel) != "",
			ApprovalID:  *approvalID,
		})
		if *dryRun {
			fmt.Printf("dry-run: would ApplyUpdate %s → %s (%s) approval=%s\n", st.Current, st.Latest, ch, *approvalID)
			return 0
		}
		resp, err := ipc.Call(*socket, ipc.Request{ID: "wbr", Operation: ipc.OpApplyUpdate, Args: raw})
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

	if *saveChannel || strings.TrimSpace(*channel) != "" {
		_ = update.SaveChannelConfig(*envPath, ch, base)
	}

	res, err := update.Apply(ctx, update.ApplyOptions{
		Channel:     ch,
		BaseURL:     base,
		DryRun:      *dryRun,
		Force:       *force,
		SaveChannel: *saveChannel || strings.TrimSpace(*channel) != "",
		PanelEnvPath: *envPath,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
	if res.DryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would update %s → %s (%s)\n", res.From, res.To, res.Channel)
	} else {
		fmt.Fprintf(os.Stderr, "updated %s → %s (%s); helper+panel restarted\n", res.From, res.To, res.Channel)
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
		return fmt.Errorf("must run as root (got uid %s)", u.Uid)
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
  wbr update check                 (any user; reads S3 channel VERSION)
  wbr update apply                 (root; download, verify, install, restart)
  wbr backup run                   (via helper; used by webor-backup.timer)
  wbr request-approval -domain example.com [-op DeleteSiteUser -delete-data]
  wbr approve list                 (root)
  wbr approve <approval-id>        (root; console step-up)
  wbr recover totp                 (root; clear 2FA after lockout; SSH to VPS)

`)
}
