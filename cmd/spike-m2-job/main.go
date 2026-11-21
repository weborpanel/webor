// Command spike-m2-job installs a long-running site job and checks it is active.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/systemd"
	"github.com/weborpanel/webor/internal/sites"
)

func main() {
	domain := flag.String("domain", "app.example.com", "site domain")
	job := flag.String("job", "lab-keep", "job name")
	script := flag.String("script", "", "absolute path to job script/binary (installed as bin/<job>)")
	flag.Parse()

	if *script == "" {
		fmt.Fprintln(os.Stderr, "usage: spike-m2-job -script /path/to/keep-alive [-domain app.example.com] [-job lab-keep]")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*script)
	if err != nil {
		fatal(err)
	}

	ops := helper.DefaultOps()
	if err := ops.CreateSiteUser(*domain); err != nil {
		fatal(err)
	}

	dest := filepath.Join(sites.BinDir(*domain), filepath.Base(abs))
	if err := copyExec(abs, dest); err != nil {
		fatal(err)
	}
	u := sites.Username(*domain)
	_ = exec.Command("chown", u+":"+u, dest).Run()

	site := config.Site{
		Name: *domain,
		Type: config.SiteTypeApp,
		Jobs: []config.Job{{Name: *job, Exec: "./bin/" + filepath.Base(abs)}},
	}
	units, err := ops.InstallJob(site, site.Jobs[0])
	if err != nil {
		fatal(err)
	}
	fmt.Printf("installed %s\n", units.ServiceName)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ops.JobIsActive(*domain, *job) {
			fmt.Printf("active: %s\n", systemd.JobServiceUnitName(*domain, *job))
			fmt.Println("ok: long-running job verified")
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fatal(fmt.Errorf("job %q not active", *job))
}

func copyExec(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, in, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "spike-m2-job:", err)
	os.Exit(1)
}
