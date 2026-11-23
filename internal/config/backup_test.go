package config_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestResticRepositoryURL(t *testing.T) {
	b := config.Backup{
		Enabled: true,
		S3: config.BackupS3{
			Endpoint: "https://s3.amazonaws.com",
			Bucket:   "my-backups",
			Prefix:   "prod/vps-1",
			Region:   "ap-south-1",
		},
	}
	got := b.ResticRepositoryURL()
	want := "s3:https://s3.amazonaws.com/my-backups/prod/vps-1/webor-restic"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidateBackup(t *testing.T) {
	if err := config.ValidateBackup(&config.Backup{Enabled: true}); err == nil {
		t.Fatal("expected error for missing s3")
	}
	b := &config.Backup{
		Enabled: true,
		S3: config.BackupS3{
			Endpoint: "https://example.com",
			Bucket:   "b",
		},
	}
	if err := config.ValidateBackup(b); err != nil {
		t.Fatal(err)
	}
	if b.Retention.KeepDaily != 7 || b.Retention.KeepWeekly != 4 {
		t.Fatalf("defaults: %+v", b.Retention)
	}
}

func TestParseBackupBlock(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel:
  port: 0
backup:
  enabled: true
  schedule: "0 3 * * *"
  s3:
    endpoint: https://s3.amazonaws.com
    region: us-east-1
    bucket: webor-bak
    prefix: lab
  retention:
    keep_daily: 3
    keep_weekly: 2
sites: []
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backup == nil || !cfg.Backup.Enabled || cfg.Backup.S3.Bucket != "webor-bak" {
		t.Fatalf("%+v", cfg.Backup)
	}
}
