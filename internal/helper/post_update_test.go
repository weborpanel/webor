package helper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPanelEnvCaddyHints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel.env")
	body := "WBR_PANEL_ARGS=-listen 127.0.0.1:21215 -disable-https=true -state-dir /tmp\nWBR_PANEL_PORT=21215\n"
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	off, listen, port := readPanelEnvCaddyHints(path)
	if !off {
		t.Fatal("expected disable-https")
	}
	if listen != "127.0.0.1:21215" {
		t.Fatalf("listen=%q", listen)
	}
	if port != 21215 {
		t.Fatalf("port=%d", port)
	}
}
