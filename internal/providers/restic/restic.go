// Package restic wraps the restic CLI with argv arrays (no shell).
package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// EnvFile is the production path for backup credentials.
const EnvFile = "/etc/webor/backup.env"

// Client runs restic against a repository.
type Client struct {
	Bin        string            // default "restic"
	Repository string            // -r value
	Env        map[string]string // merged into process env (AWS_*, RESTIC_PASSWORD, …)
	Timeout    time.Duration
}

func (c *Client) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "restic"
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 30 * time.Minute
}

func (c *Client) baseEnv() []string {
	env := os.Environ()
	for k, v := range c.Env {
		env = append(env, k+"="+v)
	}
	if c.Repository != "" {
		env = append(env, "RESTIC_REPOSITORY="+c.Repository)
	}
	return env
}

// Available reports whether restic is on PATH.
func Available() bool {
	_, err := exec.LookPath("restic")
	return err == nil
}

// Run executes restic with args; stdout is returned.
func (c *Client) Run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.Env = c.baseEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("restic %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// Init creates a new repository if missing (idempotent-ish: ignores already-initialized).
func (c *Client) Init(ctx context.Context) error {
	_, err := c.Run(ctx, "init")
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "already initialized") || strings.Contains(err.Error(), "config file already") {
		return nil
	}
	return err
}

// Backup uploads path as a snapshot with tags.
func (c *Client) Backup(ctx context.Context, path string, tags []string) (string, error) {
	args := []string{"backup", "--json", path}
	for _, t := range tags {
		args = append(args, "--tag", t)
	}
	out, err := c.Run(ctx, args...)
	if err != nil {
		return "", err
	}
	// Last summary line often has snapshot_id
	id := ""
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		if m["message_type"] == "summary" {
			if s, ok := m["snapshot_id"].(string); ok {
				id = s
			}
		}
	}
	return id, nil
}

// Snapshot is a restic snapshots --json entry (subset).
type Snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	Tags     []string  `json:"tags"`
	Paths    []string  `json:"paths"`
}

// Snapshots lists repository snapshots.
func (c *Client) Snapshots(ctx context.Context) ([]Snapshot, error) {
	out, err := c.Run(ctx, "snapshots", "--json")
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 || string(out) == "null" {
		return nil, nil
	}
	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		return nil, fmt.Errorf("parse snapshots: %w", err)
	}
	return snaps, nil
}

// Restore extracts snapshot into targetDir. includes are restic --include paths (empty = all).
func (c *Client) Restore(ctx context.Context, snapshotID, targetDir string, includes []string) error {
	args := []string{"restore", snapshotID, "--target", targetDir}
	for _, inc := range includes {
		args = append(args, "--include", inc)
	}
	_, err := c.Run(ctx, args...)
	return err
}

// ForgetPrune applies keep_daily / keep_weekly then prune.
func (c *Client) ForgetPrune(ctx context.Context, keepDaily, keepWeekly int) error {
	args := []string{"forget", "--prune", "--tag", "webor"}
	if keepDaily > 0 {
		args = append(args, "--keep-daily", fmt.Sprintf("%d", keepDaily))
	}
	if keepWeekly > 0 {
		args = append(args, "--keep-weekly", fmt.Sprintf("%d", keepWeekly))
	}
	_, err := c.Run(ctx, args...)
	return err
}

// DumpFile restores a single file from a snapshot to stdout path via restore --include.
func (c *Client) DumpFile(ctx context.Context, snapshotID, includePath, destFile string) error {
	tmp, err := os.MkdirTemp("", "webor-restic-dump-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := c.Restore(ctx, snapshotID, tmp, []string{includePath}); err != nil {
		return err
	}
	// restic restores with full path under target
	cand := tmp + "/" + strings.TrimPrefix(includePath, "/")
	data, err := os.ReadFile(cand)
	if err != nil {
		// try walking
		var found string
		_ = walkFind(tmp, func(p string) bool {
			if strings.HasSuffix(p, includePath) || strings.HasSuffix(p, "/"+strings.TrimPrefix(includePath, "/")) {
				found = p
				return true
			}
			return false
		})
		if found == "" {
			return fmt.Errorf("restored file not found for %s", includePath)
		}
		data, err = os.ReadFile(found)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(destFile, data, 0o600)
}

func walkFind(root string, match func(string) bool) error {
	return walk(root, match)
}

func walk(dir string, match func(string) bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := dir + "/" + e.Name()
		if e.IsDir() {
			if err := walk(p, match); err != nil {
				return err
			}
			continue
		}
		if match(p) {
			return nil
		}
	}
	return nil
}

// LoadEnvFile parses KEY=VALUE lines into a map (ignores comments/blank).
func LoadEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

// WriteEnvFile writes backup credentials atomically.
func WriteEnvFile(path string, values map[string]string) error {
	var b strings.Builder
	b.WriteString("# Webor backup credentials — mode 0600\n")
	keys := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION", "RESTIC_PASSWORD", "AWS_ACCESS_KEY", "AWS_SECRET_KEY"}
	seen := map[string]struct{}{}
	for _, k := range keys {
		if v, ok := values[k]; ok && v != "" {
			fmt.Fprintf(&b, "%s=%s\n", k, v)
			seen[k] = struct{}{}
		}
	}
	for k, v := range values {
		if _, ok := seen[k]; ok {
			continue
		}
		if v == "" {
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
