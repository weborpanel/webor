package postgres

import "strings"

// PanelAuthHBAMarker marks the pg_hba.conf lines that let the panel open
// site databases over localhost with a password. Without them, distros that
// use ident/peer for 127.0.0.1 reject DBAdminer and Browse returns not found.
const PanelAuthHBAMarker = "# webor: localhost password auth for the panel database explorer"

// MergePanelAuthHBA prepends scram password rules for loopback if missing.
// Existing peer rules for local sockets stay below, so sudo -u postgres psql
// keeps working.
func MergePanelAuthHBA(content string) (string, bool) {
	if strings.Contains(content, PanelAuthHBAMarker) {
		return content, false
	}
	block := PanelAuthHBAMarker + "\n" +
		"host all all 127.0.0.1/32 scram-sha-256\n" +
		"host all all ::1/128 scram-sha-256\n"
	if content != "" && !strings.HasPrefix(content, "\n") {
		block += "\n"
	}
	return block + content, true
}
