package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/webooza/webooza/internal/api"
	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	stateDir := flag.String("state-dir", config.DefaultStateDir, "runtime state directory")
	configPath := flag.String("config", api.DefaultConfigPath, "desired-state config.yaml path")
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

	srv := &api.Server{Account: acct, Issuer: "Webooza", ConfigPath: *configPath}
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "webooza %s — panel UI not embedded yet\n", version.Version)
	})

	log.Printf("webooza %s listening on %s (state=%s config=%s)", version.Version, *listen, *stateDir, *configPath)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
