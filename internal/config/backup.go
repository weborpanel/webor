package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Backup holds whole-system restic → S3-compatible settings.
// Secrets live in /etc/webor/backup.env, not in this file.
type Backup struct {
	Enabled    bool             `yaml:"enabled,omitempty"`
	Schedule   string           `yaml:"schedule,omitempty"` // cron; empty = no timer
	S3         BackupS3         `yaml:"s3,omitempty"`
	Retention  BackupRetention  `yaml:"retention,omitempty"`
	Repository string           `yaml:"repository,omitempty"` // optional override (tests / local restic path)
}

// BackupS3 is the BYO bucket settings (no credentials).
type BackupS3 struct {
	Endpoint string `yaml:"endpoint,omitempty"` // e.g. https://s3.amazonaws.com or https://xxx.r2.cloudflarestorage.com
	Region   string `yaml:"region,omitempty"`
	Bucket   string `yaml:"bucket,omitempty"`
	Prefix   string `yaml:"prefix,omitempty"` // optional key prefix inside the bucket
}

// BackupRetention maps to restic forget keep_* flags.
type BackupRetention struct {
	KeepDaily  int `yaml:"keep_daily,omitempty"`
	KeepWeekly int `yaml:"keep_weekly,omitempty"`
}

// NormalizeBackup fills retention defaults when backup is enabled.
func NormalizeBackup(b *Backup) {
	if b == nil {
		return
	}
	if b.Retention.KeepDaily <= 0 {
		b.Retention.KeepDaily = 7
	}
	if b.Retention.KeepWeekly <= 0 {
		b.Retention.KeepWeekly = 4
	}
}

// ValidateBackup checks backup block when present/enabled.
func ValidateBackup(b *Backup) error {
	if b == nil || !b.Enabled {
		return nil
	}
	NormalizeBackup(b)
	if strings.TrimSpace(b.Repository) != "" {
		return nil // explicit repo (lab / local)
	}
	if strings.TrimSpace(b.S3.Bucket) == "" {
		return fmt.Errorf("backup.s3.bucket is required")
	}
	if strings.TrimSpace(b.S3.Endpoint) == "" {
		return fmt.Errorf("backup.s3.endpoint is required")
	}
	ep := strings.TrimSpace(b.S3.Endpoint)
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		return fmt.Errorf("backup.s3.endpoint must be an http(s) URL")
	}
	if _, err := url.Parse(ep); err != nil {
		return fmt.Errorf("backup.s3.endpoint: %w", err)
	}
	if b.Schedule != "" {
		parts := strings.Fields(b.Schedule)
		if len(parts) != 5 {
			return fmt.Errorf("backup.schedule: want 5-field cron, got %q", b.Schedule)
		}
	}
	return nil
}

// ResticRepositoryURL builds the restic -r argument for S3-compatible storage.
// Format: s3:<endpoint>/<bucket>/<prefix>/webor-restic
func (b Backup) ResticRepositoryURL() string {
	if r := strings.TrimSpace(b.Repository); r != "" {
		return r
	}
	ep := strings.TrimRight(strings.TrimSpace(b.S3.Endpoint), "/")
	bucket := strings.Trim(strings.TrimSpace(b.S3.Bucket), "/")
	prefix := strings.Trim(strings.TrimSpace(b.S3.Prefix), "/")
	path := bucket
	if prefix != "" {
		path = bucket + "/" + prefix
	}
	path = path + "/webor-restic"
	return "s3:" + ep + "/" + path
}
