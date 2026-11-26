package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/weborpanel/webor/internal/audit"
	"github.com/weborpanel/webor/internal/auth"
	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/version"
)

func main() {
	socket := flag.String("socket", ipc.DefaultSocketPath, "Unix socket path for helper IPC")
	stateDir := flag.String("state-dir", config.DefaultStateDir, "runtime state directory (SQLite)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	db, err := config.OpenDB(*stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open state db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	store := &auth.Store{DB: db}
	log := &audit.Logger{DB: db}
	ops := helper.DefaultOps()
	_ = ops.EnsureLogrotateSchedule()
	// Refresh host units every start; full site/Caddy migrate once per version.
	if err := ops.EnsureSystemUnits(); err != nil {
		fmt.Fprintf(os.Stderr, "ensure system units: %v\n", err)
	}
	if err := ops.PostUpdateMigrateIfNeeded("", *stateDir); err != nil {
		fmt.Fprintf(os.Stderr, "post-update migrate: %v\n", err)
	}
	srv := ipc.NewServer(*socket, ipc.Handlers(ops, store, log))
	fmt.Fprintf(os.Stderr, "webor-helper %s (state=%s)\n", version.Version, *stateDir)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
