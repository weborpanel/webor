package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/webooza/webooza/internal/api"
	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/helper"
	"github.com/webooza/webooza/internal/ui"
	"github.com/webooza/webooza/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	stateDir := flag.String("state-dir", config.DefaultStateDir, "runtime state directory")
	configPath := flag.String("config", api.DefaultConfigPath, "desired-state config.yaml path")
	expectedA := flag.String("expected-a", "", "comma-separated IPv4s sites should point at (DNS checklist)")
	caddyJSON := flag.String("caddy-json", "/etc/webooza/caddy.json", "Caddy JSON config path")
	httpListen := flag.String("http-listen", ":8080", "shared site HTTP listen for rendered Caddy config")
	disableHTTPS := flag.Bool("disable-https", true, "disable automatic HTTPS in rendered Caddy config (lab default)")
	flag.Parse()

	db, err := config.OpenDB(*stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open state db:", err)
		os.Exit(1)
	}
	defer db.Close()

	acct := &auth.Account{DB: db}
	if tok, created, err := acct.EnsureSetupToken(); err != nil {
		fmt.Fprintln(os.Stderr, "setup token:", err)
		os.Exit(1)
	} else if created {
		log.Printf("first-run setup token (save this): %s", tok)
	}

	srv := &api.Server{
		Account:         acct,
		Issuer:          "Webooza",
		ConfigPath:      *configPath,
		Ops:             helper.DefaultOps(),
		ExpectedA:       splitCSV(*expectedA),
		CaddyConfigPath: *caddyJSON,
		HTTPListen:      *httpListen,
		DisableHTTPS:    *disableHTTPS,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"service": "webooza",
			"version": version.Version,
		})
	})
	srv.MountAuth(mux)
	srv.MountSites(mux)
	mux.Handle("/", ui.Handler())

	log.Printf("webooza %s listening on %s (state=%s config=%s)", version.Version, *listen, *stateDir, *configPath)
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
