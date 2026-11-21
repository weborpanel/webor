// Socket-activated HTTP sample for Spike B (LISTEN_FDS / Unix socket).
// Honors WBR_IDLE_TIMEOUT (Go duration) to exit when idle so systemd can
// scale the service to zero while the .socket unit keeps listening.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	idle, err := idleTimeout()
	if err != nil {
		log.Fatal(err)
	}

	var mu sync.Mutex
	last := time.Now()
	touch := func() {
		mu.Lock()
		last = time.Now()
		mu.Unlock()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		touch()
		fmt.Fprintf(w, "webor-spike-b-go ok pid=%d\n", os.Getpid())
	})

	ln, err := listen()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s (activated=%v idle=%v)", ln.Addr(), os.Getenv("LISTEN_FDS") != "", idle)

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if idle > 0 {
		go func() {
			t := time.NewTicker(200 * time.Millisecond)
			defer t.Stop()
			for range t.C {
				mu.Lock()
				since := time.Since(last)
				mu.Unlock()
				if since < idle {
					continue
				}
				log.Printf("idle for %s (limit %s); shutting down", since.Round(time.Millisecond), idle)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_ = srv.Shutdown(ctx)
				cancel()
				return
			}
		}()
	}

	err = srv.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func idleTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("WBR_IDLE_TIMEOUT"))
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("WBR_IDLE_TIMEOUT: %w", err)
	}
	if d < time.Second {
		return 0, fmt.Errorf("WBR_IDLE_TIMEOUT must be at least 1s")
	}
	return d, nil
}

func listen() (net.Listener, error) {
	if fds := listenFDs(); len(fds) > 0 {
		ln, err := net.FileListener(fds[0])
		if err != nil {
			return nil, err
		}
		_ = fds[0].Close()
		return ln, nil
	}
	path := os.Getenv("WBR_SOCK")
	if path == "" {
		path = "/tmp/webor-spike-go.sock"
	}
	_ = os.Remove(path)
	return net.Listen("unix", path)
}

func listenFDs() []*os.File {
	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return nil
	}
	n, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || n < 1 {
		return nil
	}
	files := make([]*os.File, 0, n)
	for i := 0; i < n; i++ {
		fd := 3 + i
		syscall.CloseOnExec(fd)
		files = append(files, os.NewFile(uintptr(fd), "LISTEN_FD_"+strconv.Itoa(fd)))
	}
	return files
}
