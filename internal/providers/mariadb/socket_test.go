package mariadb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnixSocketPathDefault(t *testing.T) {
	p := UnixSocketPath()
	if p == "" {
		t.Fatal("empty socket path")
	}
	if !strings.Contains(p, "mysql") && !strings.Contains(p, "mysqld") {
		t.Fatalf("unexpected socket %q", p)
	}
}

func TestUnixSocketPathPrefersExisting(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "mysql.sock")
	if err := os.WriteFile(sock, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	old := CommonUnixSockets
	CommonUnixSockets = []string{sock, "/run/mysqld/mysqld.sock"}
	t.Cleanup(func() { CommonUnixSockets = old })
	if got := UnixSocketPath(); got != sock {
		t.Fatalf("got %q want %q", got, sock)
	}
}

func TestPeerURLIncludesSocket(t *testing.T) {
	u := PeerURL("example.com", "default")
	if !strings.Contains(u, "socket=") {
		t.Fatalf("missing socket in %q", u)
	}
}
