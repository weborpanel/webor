// Package caddy drives a separately managed Caddy process through its admin API.
package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultAdminURL is Caddy's default admin endpoint (loopback only).
const DefaultAdminURL = "http://127.0.0.1:2019"

// Client talks to Caddy's JSON admin API.
type Client struct {
	AdminURL   string
	HTTPClient *http.Client
	// LastGoodPath is where the last successfully applied config is stored.
	// Used for Webooza-owned rollback after a successful load that fails a health probe.
	LastGoodPath string
}

// NewClient returns a client pointed at the default admin URL.
func NewClient(adminURL, lastGoodPath string) *Client {
	if adminURL == "" {
		adminURL = DefaultAdminURL
	}
	return &Client{
		AdminURL:     adminURL,
		LastGoodPath: lastGoodPath,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoadError is returned when Caddy rejects a config.
type LoadError struct {
	Status int
	Body   string
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("caddy load failed (HTTP %d): %s", e.Status, e.Body)
}

// ValidateJSON ensures cfg is non-empty object JSON (cheap preflight before /load).
func ValidateJSON(cfg []byte) error {
	cfg = bytes.TrimSpace(cfg)
	if len(cfg) == 0 {
		return fmt.Errorf("config is empty")
	}
	if cfg[0] != '{' {
		return fmt.Errorf("config must be a JSON object")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(cfg, &raw); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// Load posts a full config to POST /load. On failure Caddy keeps the previous
// active config (built-in zero-downtime rollback).
func (c *Client) Load(ctx context.Context, cfg []byte) error {
	if err := ValidateJSON(cfg); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.adminURL("/load"), bytes.NewReader(cfg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http().Do(req)
	if err != nil {
		return fmt.Errorf("admin /load: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 300 {
		return &LoadError{Status: resp.StatusCode, Body: string(bytes.TrimSpace(body))}
	}
	return nil
}

// CurrentConfig fetches GET /config/.
func (c *Client) CurrentConfig(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.adminURL("/config/"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("admin /config/: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, &LoadError{Status: resp.StatusCode, Body: string(bytes.TrimSpace(body))}
	}
	return body, nil
}

// ProbeOptions configures a post-load HTTP health check.
type ProbeOptions struct {
	URL            string
	ExpectStatus   int // 0 means any 2xx/3xx
	Timeout        time.Duration
	FollowRedirect bool
}

// Probe hits a URL to confirm the new config is serving as expected.
func (c *Client) Probe(ctx context.Context, opt ProbeOptions) error {
	if opt.URL == "" {
		return fmt.Errorf("probe URL is required")
	}
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !opt.FollowRedirect {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opt.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	if opt.ExpectStatus != 0 {
		if resp.StatusCode != opt.ExpectStatus {
			return fmt.Errorf("probe: status %d, want %d", resp.StatusCode, opt.ExpectStatus)
		}
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("probe: status %d", resp.StatusCode)
	}
	return nil
}

// ApplyOptions controls validate → load → optional probe → last-good persistence.
type ApplyOptions struct {
	Probe *ProbeOptions
}

// Apply validates, loads via the admin API, optionally probes, and updates last-good.
// If the probe fails after a successful load, it reloads the previous last-good config.
func (c *Client) Apply(ctx context.Context, cfg []byte, opt ApplyOptions) error {
	if err := ValidateJSON(cfg); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	prev, _ := c.readLastGood()

	if err := c.Load(ctx, cfg); err != nil {
		// Caddy already kept the previous live config.
		return err
	}

	if opt.Probe != nil {
		if err := c.Probe(ctx, *opt.Probe); err != nil {
			if len(prev) > 0 {
				if rbErr := c.Load(ctx, prev); rbErr != nil {
					return fmt.Errorf("probe failed (%v) and rollback failed (%w)", err, rbErr)
				}
				return fmt.Errorf("probe failed, rolled back to last-good: %w", err)
			}
			return fmt.Errorf("probe failed and no last-good available: %w", err)
		}
	}

	if err := c.writeLastGood(cfg); err != nil {
		return fmt.Errorf("config loaded but failed to persist last-good: %w", err)
	}
	return nil
}

func (c *Client) adminURL(path string) string {
	base := c.AdminURL
	if base == "" {
		base = DefaultAdminURL
	}
	return base + path
}

func (c *Client) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) readLastGood() ([]byte, error) {
	if c.LastGoodPath == "" {
		return nil, fmt.Errorf("no last-good path")
	}
	return os.ReadFile(c.LastGoodPath)
}

func (c *Client) writeLastGood(cfg []byte) error {
	if c.LastGoodPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.LastGoodPath), 0o755); err != nil {
		return err
	}
	tmp := c.LastGoodPath + ".tmp"
	if err := os.WriteFile(tmp, cfg, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.LastGoodPath)
}
