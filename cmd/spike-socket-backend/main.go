// Backend that does NOT speak socket activation — listens on a fixed Unix path.
// Used with systemd-socket-proxyd as the Spike B fallback.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	path := os.Getenv("WBZ_SOCK")
	if path == "" {
		path = "/run/webooza-spike/backend.sock"
	}
	_ = os.Remove(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "webooza-spike-b-proxy-backend ok pid=%d\n", os.Getpid())
	})
	log.Printf("backend listening on %s (no activation)", path)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.Serve(ln))
}
