package helper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/webooza/webooza/internal/providers/caddy"
)

const defaultLastGood = "/var/lib/webooza/caddy-last-good.json"

// ReloadCaddy reads a JSON config from path and applies it via the admin API.
func (o *Ops) ReloadCaddy(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	admin := o.CaddyAdminURL
	if admin == "" {
		admin = caddy.DefaultAdminURL
	}
	lastGood := o.CaddyLastGood
	if lastGood == "" {
		lastGood = defaultLastGood
	}
	if err := os.MkdirAll(filepath.Dir(lastGood), 0o755); err != nil {
		return fmt.Errorf("state dir: %w", err)
	}

	client := caddy.NewClient(admin, lastGood)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	}
	return client.Apply(ctx, data, caddy.ApplyOptions{})
}
