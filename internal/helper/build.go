package helper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/build"
	"github.com/weborpanel/webor/internal/sites"
)

// RunBuild runs a site build command under a transient systemd unit with limits.
func (o *Ops) RunBuild(ctx context.Context, domain, command string, memoryMax string, timeoutSec int) (*build.Result, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command is required")
	}
	limits := build.Limits{MemoryMax: memoryMax}
	if timeoutSec > 0 {
		limits.Timeout = time.Duration(timeoutSec) * time.Second
	}
	r := &build.Runner{Queue: build.Global}
	return r.Run(ctx, build.Job{
		Domain:  domain,
		Command: command,
		WorkDir: sites.Root(domain),
		Limits:  limits,
	})
}
