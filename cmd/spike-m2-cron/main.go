// Command spike-m2-cron installs site cron timers and exercises run-now + status.
// Intended to run as root after the app site exists (make lab-app).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/config"
	"github.com/webooza/webooza/internal/helper"
	"github.com/webooza/webooza/internal/providers/systemd"
	"github.com/webooza/webooza/internal/sites"
)

func main() {
	domain := flag.String("domain", "app.example.com", "site domain")
	job := flag.String("job", "lab-tick", "cron job name")
	flag.Parse()

	ops := helper.DefaultOps()
	if err := ops.CreateSiteUser(*domain); err != nil {
		fatal(err)
	}

	script := filepath.Join(sites.BinDir(*domain), "lab-tick")
	body := "#!/bin/sh\nset -e\ndate -Is >> tmp/cron-ran.txt\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		fatal(err)
	}
	u := sites.Username(*domain)
	_ = exec.Command("chown", u+":"+u, script).Run()
	_ = os.Remove(filepath.Join(sites.Tmp(*domain), "cron-ran.txt"))

	site := config.Site{
		Name: *domain,
		Type: config.SiteTypeApp,
		Cron: []config.CronJob{{
			Name:     *job,
			Schedule: "*/5 * * * *",
			Exec:     "./bin/lab-tick",
		}},
	}
	units, err := ops.InstallCronJob(site, site.Cron[0])
	if err != nil {
		fatal(err)
	}
	fmt.Printf("installed %s / %s calendars=%v\n", units.TimerName, units.ServiceName, units.OnCalendar)

	if err := ops.RunCronNow(*domain, *job); err != nil {
		fatal(err)
	}
	// oneshot exits quickly
	time.Sleep(300 * time.Millisecond)

	st, err := ops.CronStatus(*domain, *job)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("status: timer_active=%v result=%q failed=%v next=%s\n",
		st.TimerActive, st.LastResult, st.Failed, st.NextElapse)
	if !st.TimerActive {
		fatal(fmt.Errorf("timer not active"))
	}
	if st.Failed {
		fatal(fmt.Errorf("cron service failed"))
	}

	marker := filepath.Join(sites.Tmp(*domain), "cron-ran.txt")
	raw, err := os.ReadFile(marker)
	if err != nil {
		fatal(fmt.Errorf("marker missing: %w", err))
	}
	fmt.Printf("marker: %s", raw)
	if strings.TrimSpace(string(raw)) == "" {
		fatal(fmt.Errorf("empty marker"))
	}

	log, err := ops.JournalTail(systemd.CronServiceUnitName(*domain, *job), 20)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("--- journal ---\n%s", log)
	fmt.Println("ok: cron install + run-now verified")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "spike-m2-cron:", err)
	os.Exit(1)
}
