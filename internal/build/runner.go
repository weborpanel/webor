package build

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/sites"
)

// Job is one site build request.
type Job struct {
	Domain  string
	Command string
	WorkDir string // absolute; defaults to site root
	Limits  Limits
}

// Result is the outcome of a finished build unit.
type Result struct {
	Unit    string
	ExitCode int
	Stdout  string
	Stderr  string
	Duration time.Duration
}

// Runner executes builds. Tests replace Exec with a fake.
type Runner struct {
	Queue *Queue
	// Exec runs a prepared systemd-run argv. Defaults to exec.CommandContext.
	Exec func(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error)
}

// Run acquires the global queue and starts a transient user-scoped build unit.
func (r *Runner) Run(ctx context.Context, job Job) (*Result, error) {
	domain := strings.ToLower(strings.TrimSpace(job.Domain))
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if strings.TrimSpace(job.Command) == "" {
		return nil, fmt.Errorf("command is required")
	}
	q := r.Queue
	if q == nil {
		q = Global
	}
	if err := q.Acquire(ctx); err != nil {
		return nil, fmt.Errorf("build queue: %w", err)
	}
	defer q.Release()

	limits := job.Limits.withDefaults()
	workdir := job.WorkDir
	if workdir == "" {
		workdir = sites.Root(domain)
	}
	username := sites.Username(domain)
	unit := fmt.Sprintf("webor-build-%s-%d", strings.ReplaceAll(domain, ".", "-"), time.Now().UnixNano())
	// Cap unit name length for systemd.
	if len(unit) > 200 {
		unit = unit[:200]
	}

	args := []string{
		"--uid=" + username,
		"--gid=" + username,
		"--working-directory=" + workdir,
		"--unit=" + unit,
		"--collect",
		"--wait",
		"--quiet",
		"-p", "MemoryMax=" + limits.MemoryMax,
		"-p", "CPUQuota=" + limits.CPUQuota,
		"-p", fmt.Sprintf("TimeoutStartSec=%d", int(limits.Timeout.Seconds())),
		"-p", "Nice=" + fmt.Sprintf("%d", limits.Nice),
		"-p", "PrivateTmp=yes",
		"-p", "NoNewPrivileges=yes",
		"-p", "ProtectSystem=strict",
		"-p", "ReadWritePaths=" + sites.Root(domain),
		"/bin/bash", "-lc", job.Command,
	}

	execFn := r.Exec
	if execFn == nil {
		execFn = defaultExec
	}
	start := time.Now()
	stdout, stderr, code, err := execFn(ctx, "systemd-run", args...)
	res := &Result{
		Unit:     unit,
		ExitCode: code,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: time.Since(start),
	}
	if err != nil {
		return res, fmt.Errorf("systemd-run: %w", err)
	}
	if code != 0 {
		return res, fmt.Errorf("build exited %d", code)
	}
	return res, nil
}

func defaultExec(ctx context.Context, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
			err = nil
		}
	}
	return outBuf.String(), errBuf.String(), code, err
}
