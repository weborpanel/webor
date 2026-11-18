// Command spike-m2-deploy exercises app binary deploy + probe rollback.
// Intended to run as root after make lab-app (socket-activated sample site).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/helper"
	"github.com/webooza/webooza/internal/sites"
)

func main() {
	domain := flag.String("domain", "app.example.com", "app site domain")
	binary := flag.String("binary", "", "absolute path to a healthy app binary")
	flag.Parse()

	if *binary == "" {
		fmt.Fprintln(os.Stderr, "usage: spike-m2-deploy -binary /path/to/spike-socket-go [-domain app.example.com]")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*binary)
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(abs); err != nil {
		fatal(err)
	}

	ops := helper.DefaultOps()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := ops.CreateSiteUser(*domain); err != nil {
		fatal(err)
	}

	before := sites.CurrentReleaseID(*domain)
	fmt.Printf("current before: %q\n", before)

	staged := filepath.Join(sites.Tmp(*domain), sites.DefaultAppBinary)
	if err := copyFile(abs, staged); err != nil {
		fatal(err)
	}
	// Ensure a distinct release id (second-resolution timestamps).
	time.Sleep(1100 * time.Millisecond)

	res, err := ops.DeployAppBinary(ctx, *domain, staged)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("deployed release %s → %s\n", res.ReleaseID, res.Binary)
	if res.ReleaseID == before {
		fatal(fmt.Errorf("expected new release id, still %q", before))
	}
	if got := sites.CurrentReleaseID(*domain); got != res.ReleaseID {
		fatal(fmt.Errorf("current=%q want %q", got, res.ReleaseID))
	}

	good := res.ReleaseID
	bad := filepath.Join(sites.Tmp(*domain), "bad-server")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\necho bad-deploy\nexit 1\n"), 0o755); err != nil {
		fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)

	_, err = ops.DeployAppBinary(ctx, *domain, bad)
	if err == nil {
		fatal(fmt.Errorf("expected bad deploy to fail and roll back"))
	}
	if !strings.Contains(err.Error(), "rolled back") && !strings.Contains(err.Error(), "probe failed") {
		fatal(fmt.Errorf("unexpected bad-deploy error: %w", err))
	}
	fmt.Printf("bad deploy failed as expected: %v\n", err)

	if got := sites.CurrentReleaseID(*domain); got != good {
		fatal(fmt.Errorf("after rollback current=%q want %q", got, good))
	}

	out, err := exec.Command("curl", "-sf", "-H", "Host: "+*domain, "http://127.0.0.1:8080/").CombinedOutput()
	if err != nil {
		fatal(fmt.Errorf("curl after rollback: %w (%s)", err, strings.TrimSpace(string(out))))
	}
	body := strings.TrimSpace(string(out))
	fmt.Printf("curl: %s\n", body)
	if !strings.Contains(body, "webooza-spike-b-go ok") {
		fatal(fmt.Errorf("unexpected response %q", body))
	}

	fmt.Println("ok: deploy + rollback verified")
}

func copyFile(src, dest string) error {
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
	fmt.Fprintln(os.Stderr, "spike-m2-deploy:", err)
	os.Exit(1)
}
