package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/helper"
	"github.com/webooza/webooza/internal/ipc"
	"github.com/webooza/webooza/internal/version"
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
	srv := ipc.NewServer(*socket, ipc.Handlers(helper.DefaultOps(), store))
	fmt.Fprintf(os.Stderr, "webooza-helper %s (state=%s)\n", version.Version, *stateDir)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
