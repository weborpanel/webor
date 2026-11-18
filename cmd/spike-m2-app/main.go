// Command spike-m2-app applies type:app sites with socket activation and reloads Caddy.
// Intended to run as root on the lab VM.
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
	"github.com/webooza/webooza/internal/providers/systemd"
	"github.com/webooza/webooza/internal/reconcile"
)

func main() {
	cfgPath := flag.String("config", "", "path to config.yaml")
	caddyJSON := flag.String("caddy-json", "/etc/webooza/caddy.json", "rendered Caddy JSON path")
	admin := flag.String("admin", caddy.DefaultAdminURL, "Caddy admin API URL")
	listen := flag.String("listen", ":8080", "HTTP listen address in rendered config")
	sample := flag.String("sample-binary", "", "install this binary as each app site's Exec target")
	httpsOff := flag.Bool("disable-https", true, "disable automatic HTTPS (lab)")
	flag.Parse()

	if *cfgPath == "" {
		fmt.Fprintln(os.Stderr, "usage: spike-m2-app -config path/to/config.yaml [-sample-binary path]")
		os.Exit(2)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}

	ops := helper.DefaultOps()
	ops.CaddyAdminURL = *admin
	ops.CaddyLastGood = "/var/lib/webooza/caddy-last-good.json"

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err = reconcile.ApplyAppSites(ctx, cfg, ops, reconcile.AppApplyOptions{
		CaddyConfigPath: *caddyJSON,
		CaddyAdminURL:   *admin,
		HTTPListen:      *listen,
		DisableHTTPS:    *httpsOff,
		SampleBinary:    *sample,
	})
	if err != nil {
		fatal(err)
	}

	fmt.Printf("ok: applied app sites %v → %s\n", caddy.AppSiteNames(cfg), *caddyJSON)
	for _, name := range caddy.AppSiteNames(cfg) {
		svc := systemd.ServiceUnitName(name)
		sock := systemd.SocketUnitName(name)
		fmt.Printf("  %s socket=%s service=%s\n", name, sock, svc)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "spike-m2-app:", err)
	os.Exit(1)
}
