package helper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const unitDir = "/etc/systemd/system"

// WriteUnitFile writes a systemd unit basename under /etc/systemd/system and runs daemon-reload.
func (o *Ops) WriteUnitFile(domain, unit, content string) error {
	_ = domain // reserved for future per-site unit path policy
	name := filepath.Base(strings.TrimSpace(unit))
	dest := filepath.Join(unitDir, name)
	if filepath.Clean(dest) != dest || !strings.HasPrefix(dest, unitDir+string(os.PathSeparator)) {
		return fmt.Errorf("refusing unit path %q", dest)
	}

	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install unit: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
