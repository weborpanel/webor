// Command spike-m0-static applies static sites from a config file and reloads Caddy.
// Intended to run as root on the lab VM (creates users under /srv/sites).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/helper"
	"github.com/webooza/webooza/internal/providers/caddy"
	"github.com/webooza/webooza/internal/reconcile"
)

func main() {
	cfgPath := flag.String("config", "", "path to config.yaml")
	caddyJSON := flag.String("caddy-json", "/etc/webooza/caddy.json", "rendered Caddy JSON path")
	admin := flag.String("admin", caddy.DefaultAdminURL, "Caddy admin API URL")
	listen := flag.String("listen", ":8080", "HTTP listen address in rendered config")
	seed := flag.Bool("seed-index", true, "write demo index.html if missing")
	httpsOff := flag.Bool("disable-https", true, "disable automatic HTTPS (lab)")
	flag.Parse()

	if *cfgPath == "" {
		fmt.Fprintln(os.Stderr, "usage: spike-m0-static -config path/to/config.yaml")
		os.Exit(2)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}

	ops := helper.DefaultOps()
	ops.CaddyAdminURL = *admin
	ops.CaddyLastGood = "/var/lib/webooza/caddy-last-good.json"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = reconcile.ApplyStaticSites(ctx, cfg, ops, reconcile.StaticApplyOptions{
		CaddyConfigPath: *caddyJSON,
		CaddyAdminURL:   *admin,
		HTTPListen:      *listen,
		DisableHTTPS:    *httpsOff,
		SeedIndex:       *seed,
	})
	if err != nil {
		fatal(err)
	}
	fmt.Printf("ok: applied static sites %v → %s\n", caddy.StaticSiteNames(cfg), *caddyJSON)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "spike-m0-static:", err)
	os.Exit(1)
}
