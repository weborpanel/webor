package main

import (
	"fmt"
	"os"

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
		fmt.Println("wbz approve: not implemented (see docs/decisions/0003-step-up-approval.md)")
		os.Exit(1)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `wbz — Webooza admin CLI

Usage:
  wbz version
  wbz approve   (placeholder; console step-up approvals)

`)
}
