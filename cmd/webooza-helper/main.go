package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/webooza/webooza/internal/ipc"
	"github.com/webooza/webooza/internal/version"
)

func main() {
	socket := flag.String("socket", ipc.DefaultSocketPath, "Unix socket path for helper IPC")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	srv := ipc.NewServer(*socket, ipc.DefaultHandlers())
	fmt.Fprintf(os.Stderr, "webooza-helper %s\n", version.Version)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
