// Command spike-helper calls webooza-helper over its Unix socket (Milestone 0 ops).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/webooza/webooza/internal/ipc"
)

func main() {
	socket := flag.String("socket", ipc.DefaultSocketPath, "helper Unix socket")
	op := flag.String("op", "", "CreateSiteUser | WriteUnitFile | ReloadCaddy")
	domain := flag.String("domain", "", "site domain")
	unit := flag.String("unit", "", "unit basename for WriteUnitFile")
	contentPath := flag.String("content", "", "unit file path to read for WriteUnitFile")
	configPath := flag.String("config", "", "Caddy JSON path for ReloadCaddy")
	flag.Parse()

	if *op == "" {
		fmt.Fprintln(os.Stderr, "usage: spike-helper -op CreateSiteUser -domain example.com")
		os.Exit(2)
	}

	var args any
	switch *op {
	case ipc.OpCreateSiteUser:
		args = ipc.CreateSiteUserArgs{Domain: *domain}
	case ipc.OpWriteUnitFile:
		body, err := os.ReadFile(*contentPath)
		if err != nil {
			fatal(err)
		}
		args = ipc.WriteUnitFileArgs{Domain: *domain, Unit: *unit, Content: string(body)}
	case ipc.OpReloadCaddy:
		args = ipc.ReloadCaddyArgs{ConfigPath: *configPath}
	default:
		fatal(fmt.Errorf("unknown op %q", *op))
	}

	raw, err := json.Marshal(args)
	if err != nil {
		fatal(err)
	}
	resp, err := ipc.Call(*socket, ipc.Request{ID: "spike", Operation: *op, Args: raw})
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
	if !resp.OK {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "spike-helper:", err)
	os.Exit(1)
}
