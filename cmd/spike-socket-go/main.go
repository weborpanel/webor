// Socket-activated HTTP sample for Spike B (LISTEN_FDS / Unix socket).
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "webooza-spike-b-go ok pid=%d\n", os.Getpid())
	})

	ln, err := listen()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s (activated=%v)", ln.Addr(), os.Getenv("LISTEN_FDS") != "")
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.Serve(ln))
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
	path := os.Getenv("WBZ_SOCK")
	if path == "" {
		path = "/tmp/webooza-spike-go.sock"
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
