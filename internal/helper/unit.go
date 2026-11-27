package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const unitDir = "/etc/systemd/system"

// WriteUnitFile writes a systemd unit basename under /etc/systemd/system and runs daemon-reload.
func (o *Ops) WriteUnitFile(domain, unit, content string) error {
	if err := o.WriteUnitFileNoReload(domain, unit, content); err != nil {
		return err
	}
	return o.SystemctlDaemonReload()
}

// WriteUnitFileNoReload writes the unit file without daemon-reload.
// Call SystemctlDaemonReload once after a batch of writes.
func (o *Ops) WriteUnitFileNoReload(domain, unit, content string) error {
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
	return nil
}

// writeUnitFiles writes several units then runs a single daemon-reload.
func (o *Ops) writeUnitFiles(domain string, files map[string]string) error {
	if len(files) == 0 {
		return nil
	}
	for unit, content := range files {
		if err := o.WriteUnitFileNoReload(domain, unit, content); err != nil {
			return err
		}
	}
	return o.SystemctlDaemonReload()
}
