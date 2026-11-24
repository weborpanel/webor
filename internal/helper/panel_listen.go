package helper

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/panelenv"
)

// SetPanelListenHost rewrites panel.env -listen host and schedules a webor restart
// so the new bind address takes effect. Restart is deferred briefly so IPC callers
// can receive the success response before the panel process exits.
func (o *Ops) SetPanelListenHost(host, envPath string) (listenAddr string, err error) {
	listenAddr, err = panelenv.SetListenHost(envPath, host)
	if err != nil {
		return "", err
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		if err := o.SystemctlRestart("webor"); err != nil {
			// Fallback: try start if restart fails on a stopped unit.
			log.Printf("webor-helper: restart webor after listen change: %v", err)
			cmd := exec.Command("systemctl", "restart", "webor")
			if out, err2 := cmd.CombinedOutput(); err2 != nil {
				log.Printf("webor-helper: restart webor retry: %v (%s)", err2, strings.TrimSpace(string(out)))
			}
		}
	}()
	return listenAddr, nil
}

// SetPanelListenHostSync rewrites panel.env and restarts webor immediately (CLI path).
func (o *Ops) SetPanelListenHostSync(host, envPath string) (listenAddr string, err error) {
	listenAddr, err = panelenv.SetListenHost(envPath, host)
	if err != nil {
		return "", err
	}
	if err := o.SystemctlRestart("webor"); err != nil {
		return listenAddr, fmt.Errorf("panel.env updated to %s but restart failed: %w", listenAddr, err)
	}
	return listenAddr, nil
}
