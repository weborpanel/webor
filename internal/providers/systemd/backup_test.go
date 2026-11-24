package systemd_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/providers/systemd"
)

func TestRenderBackupUnits(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Backup: &config.Backup{
			Enabled:  true,
			Schedule: "0 3 * * *",
			S3:       config.BackupS3{Endpoint: "https://s3.amazonaws.com", Bucket: "b"},
		},
	}
	unit, timer, bodyU, bodyT, err := systemd.RenderBackupUnits(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if unit != "webor-backup.service" || timer != "webor-backup.timer" {
		t.Fatalf("%s %s", unit, timer)
	}
	if !strings.Contains(bodyU, "wbr backup run") {
		t.Fatalf("%s", bodyU)
	}
	if !strings.Contains(bodyU, "StandardOutput=append:/var/log/webor/backup.log") {
		t.Fatalf("missing backup log append: %s", bodyU)
	}
	if !strings.Contains(bodyT, "OnCalendar=") {
		t.Fatalf("%s", bodyT)
	}

	_, _, emptyU, emptyT, err := systemd.RenderBackupUnits(&config.Config{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if emptyU != "" || emptyT != "" {
		t.Fatal("expected empty bodies when disabled")
	}
}
