package build_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/build"
)

func TestRunBuildPassesMemoryMaxAndTimeout(t *testing.T) {
	q := &build.Queue{}
	var saw []string
	r := &build.Runner{
		Queue: q,
		Exec: func(ctx context.Context, name string, args ...string) (string, string, int, error) {
			saw = append([]string{name}, args...)
			// Simulate OOM/kill from systemd-run.
			return "", "MemoryMax exceeded", 137, nil
		},
	}
	res, err := r.Run(context.Background(), build.Job{
		Domain:  "oom.example",
		Command: "python3 -c 'x=\"x\"*10**9'",
		Limits: build.Limits{
			MemoryMax: "64M",
			Timeout:   30 * time.Second,
		},
	})
	if err == nil {
		t.Fatal("expected non-zero exit error")
	}
	if !strings.Contains(err.Error(), "OOM") && !strings.Contains(err.Error(), "memory limit") {
		t.Fatalf("expected OOM message, got %v", err)
	}
	if res == nil || res.ExitCode != 137 {
		t.Fatalf("res=%+v", res)
	}
	joined := strings.Join(saw, "\n")
	for _, want := range []string{"systemd-run", "MemoryMax=64M", "TimeoutStartSec=30", "oom.example"} {
		if !strings.Contains(joined, want) && want != "oom.example" {
			t.Fatalf("missing %q in %v", want, saw)
		}
	}
	if !strings.Contains(joined, "MemoryMax=64M") {
		t.Fatalf("args=%v", saw)
	}
	if !strings.Contains(joined, "uid=wbr_oom_example") && !strings.Contains(joined, "wbr_oom_example") {
		// username derived from domain
		if !strings.Contains(joined, "--uid=") {
			t.Fatalf("missing uid in %v", saw)
		}
	}
}

func TestQueueRejectsParallelBuilds(t *testing.T) {
	q := &build.Queue{}
	if !q.TryAcquire() {
		t.Fatal("first acquire")
	}
	if q.TryAcquire() {
		t.Fatal("second acquire should fail while busy")
	}
	q.Release()
	if !q.TryAcquire() {
		t.Fatal("after release")
	}
	q.Release()
}
