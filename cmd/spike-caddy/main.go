// Command spike-caddy exercises Caddy admin API validate → load → probe → rollback.
//
// Typical VM flow (Debian guest with Caddy installed):
//
//	caddy run --config internal/providers/caddy/testdata/good.json
//	go run ./cmd/spike-caddy -config internal/providers/caddy/testdata/good.json -probe http://127.0.0.1:8080/
//	go run ./cmd/spike-caddy -config internal/providers/caddy/testdata/bad.json -probe http://127.0.0.1:8080/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/webooza/webooza/internal/providers/caddy"
)

func main() {
	admin := flag.String("admin", caddy.DefaultAdminURL, "Caddy admin API base URL")
	configPath := flag.String("config", "", "path to Caddy JSON config to apply")
	lastGood := flag.String("last-good", "/tmp/webooza-caddy-last-good.json", "path to persist last-good config")
	probeURL := flag.String("probe", "", "optional HTTP URL to probe after load (e.g. http://127.0.0.1:8080/)")
	expect := flag.Int("expect", 200, "expected probe HTTP status")
	timeout := flag.Duration("timeout", 10*time.Second, "overall timeout")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: spike-caddy -config path/to/caddy.json [-probe URL]")
		os.Exit(2)
	}
	cfg, err := os.ReadFile(*configPath)
	if err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := caddy.NewClient(*admin, *lastGood)
	opt := caddy.ApplyOptions{}
	if *probeURL != "" {
		opt.Probe = &caddy.ProbeOptions{
			URL:          *probeURL,
			ExpectStatus: *expect,
			Timeout:      5 * time.Second,
		}
	}

	if err := client.Apply(ctx, cfg, opt); err != nil {
		fatal(err)
	}
	fmt.Printf("ok: applied %s (last-good=%s)\n", *configPath, *lastGood)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "spike-caddy:", err)
	os.Exit(1)
}
