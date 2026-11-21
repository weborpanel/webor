package ipc

import (
	"context"
	"encoding/json"
	"fmt"
)

// SiteClient runs privileged site filesystem ops over the helper Unix socket.
type SiteClient struct {
	Socket string
}

func (c *SiteClient) socket() string {
	if c != nil && c.Socket != "" {
		return c.Socket
	}
	return DefaultSocketPath
}

func (c *SiteClient) call(op string, args any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	resp, err := Call(c.socket(), Request{ID: "panel", Operation: op, Args: raw})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// CreateSiteUser creates the per-site Linux user via the helper.
func (c *SiteClient) CreateSiteUser(domain string) error {
	return c.call(OpCreateSiteUser, CreateSiteUserArgs{Domain: domain})
}

// WritePublicFile writes under public/ via the helper.
func (c *SiteClient) WritePublicFile(domain, relPath string, data []byte) error {
	return c.call(OpWritePublicFile, WritePublicFileArgs{
		Domain:  domain,
		Path:    relPath,
		Content: string(data),
	})
}

// ReloadCaddy asks the helper to apply a Caddy JSON config.
func (c *SiteClient) ReloadCaddy(ctx context.Context, configPath string) error {
	_ = ctx
	return c.call(OpReloadCaddy, ReloadCaddyArgs{ConfigPath: configPath})
}
