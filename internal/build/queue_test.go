package build_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/build"
)

func TestQueueSerializes(t *testing.T) {
	q := &build.Queue{}
	var concurrent int32
	var max int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := q.Acquire(ctx); err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			n := atomic.AddInt32(&concurrent, 1)
			for {
				cur := atomic.LoadInt32(&max)
				if n <= cur || atomic.CompareAndSwapInt32(&max, cur, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&concurrent, -1)
			q.Release()
		}()
	}
	wg.Wait()
	if max != 1 {
		t.Fatalf("max concurrent=%d want 1", max)
	}
}

func TestRunnerUsesExec(t *testing.T) {
	q := &build.Queue{}
	var sawArgs []string
	r := &build.Runner{
		Queue: q,
		Exec: func(ctx context.Context, name string, args ...string) (string, string, int, error) {
			if name != "systemd-run" {
				t.Fatalf("name=%s", name)
			}
			sawArgs = append([]string{}, args...)
			return "ok\n", "", 0, nil
		},
	}
	res, err := r.Run(context.Background(), build.Job{
		Domain:  "example.com",
		Command: "npm run build",
		Limits:  build.Limits{MemoryMax: "256M", Timeout: time.Minute},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.Stdout != "ok\n" {
		t.Fatalf("%+v", res)
	}
	joined := strings.Join(sawArgs, "\n")
	if !strings.Contains(joined, "MemoryMax=256M") || !strings.Contains(joined, "npm run build") {
		t.Fatalf("args=%v", sawArgs)
	}
}
