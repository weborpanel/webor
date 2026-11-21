// Tiny HTTP + WebSocket-capable sample for static-site backends.
// Bind: HOST (default 127.0.0.1) and PORT / WBR_PORT from the environment.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	host := envOr("HOST", "127.0.0.1")
	port := envOr("PORT", envOr("WBR_PORT", "8080"))
	addr := host + ":" + port

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":"webor-http-backend-sample"}`))
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "hello from backend path %s\n", r.URL.Path)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Echo upgrade request headers so operators can confirm Caddy proxies Upgrade.
		if r.Header.Get("Upgrade") == "websocket" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("websocket upgrade received (sample does not complete handshake)\n"))
			return
		}
		http.Error(w, "expected websocket upgrade", http.StatusUpgradeRequired)
	})

	log.Printf("listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
