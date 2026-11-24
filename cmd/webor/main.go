package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/weborpanel/webor/internal/api"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/metrics"
	"github.com/weborpanel/webor/internal/ui"
	"github.com/weborpanel/webor/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	stateDir := flag.String("state-dir", config.DefaultStateDir, "runtime state directory")
	configPath := flag.String("config", api.DefaultConfigPath, "desired-state config.yaml path")
	expectedA := flag.String("expected-a", "", "comma-separated IPv4s sites should point at (DNS checklist)")
	caddyJSON := flag.String("caddy-json", "/etc/webor/caddy.json", "Caddy JSON config path")
	httpListen := flag.String("http-listen", ":8080", "shared site HTTP listen for rendered Caddy config")
	disableHTTPS := flag.Bool("disable-https", true, "disable automatic HTTPS in rendered Caddy config (lab default)")
	helperSocket := flag.String("helper-socket", ipc.DefaultSocketPath, "webor-helper Unix socket (empty = in-process Ops)")
	setupToken := flag.String("setup-token", "", "seed first-run setup token (installer); ignored if admin exists")
	tlsCert := flag.String("tls-cert", "", "optional TLS certificate for first-run HTTPS")
	tlsKey := flag.String("tls-key", "", "optional TLS private key for first-run HTTPS")
	repairState := flag.Bool("repair-state", false, "repair corrupted state.db (stop webor + webor-helper first), then exit")
	flag.Parse()

	if *repairState {
		msg, err := config.RepairStateDB(*stateDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "repair-state:", err)
			os.Exit(1)
		}
		fmt.Println(msg)
		return
	}

	db, err := config.OpenDB(*stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open state db:", err)
		os.Exit(1)
	}
	defer db.Close()

	acct := &auth.Account{DB: db}
	if *setupToken != "" {
		if _, err := acct.SeedSetupToken(*setupToken); err != nil {
			fmt.Fprintln(os.Stderr, "seed setup token:", err)
			os.Exit(1)
		}
	}
	if tok, created, err := acct.EnsureSetupToken(); err != nil {
		fmt.Fprintln(os.Stderr, "setup token:", err)
		os.Exit(1)
	} else if created {
		log.Printf("first-run setup token (save this): %s", tok)
	}

	ops := helper.DefaultOps()
	hostCollector := &metrics.HostCollector{}
	sampler := &metrics.Sampler{
		Store: &metrics.Store{DB: db},
		Host:  hostCollector,
		Sites: func() ([]config.Site, error) {
			cfg, err := config.Load(*configPath)
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return cfg.Sites, nil
		},
		Usage: func(site config.Site, includeDisk bool) (metrics.SiteUsage, error) {
			res, err := ops.SiteResourcesWith(site.Name, &site, includeDisk)
			if err != nil {
				return metrics.SiteUsage{}, err
			}
			return metrics.SiteUsage{
				MemBytes:     res.MemoryCurrentBytes,
				DiskBytes:    res.DiskUsedBytes,
				CPUUsageUSec: res.CPUUsageUSec,
				Active:       res.AppActive,
			}, nil
		},
	}
	go sampler.Run(context.Background())

	srv := &api.Server{
		Account:         acct,
		Issuer:          "Webor",
		ConfigPath:      *configPath,
		Ops:             ops,
		Sampler:         sampler,
		Host:            hostCollector,
		HelperSocket:    strings.TrimSpace(*helperSocket),
		ExpectedA:       splitCSV(*expectedA),
		CaddyConfigPath: *caddyJSON,
		HTTPListen:      *httpListen,
		DisableHTTPS:    *disableHTTPS,
		PanelListen:     *listen,
		StateDir:        *stateDir,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"service": "webor",
			"version": version.Version,
		})
	})
	srv.MountAuth(mux)
	srv.MountSites(mux)
	srv.MountPanel(mux)
	srv.MountApprovals(mux)
	srv.MountMetrics(mux)
	srv.MountBackup(mux)
	srv.MountUpdate(mux)
	mux.Handle("/", ui.Handler())

	log.Printf("webor %s listening on %s (state=%s config=%s helper=%s)", version.Version, *listen, *stateDir, *configPath, srv.HelperSocket)
	if *tlsCert != "" && *tlsKey != "" {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}
		server := &http.Server{Addr: *listen, Handler: mux, TLSConfig: cfg}
		if err := server.ListenAndServeTLS(*tlsCert, *tlsKey); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := http.ListenAndServe(*listen, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
