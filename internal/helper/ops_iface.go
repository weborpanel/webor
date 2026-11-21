package helper

import "context"

// SiteFS is the privileged filesystem surface used by reconcile and the panel.
// *Ops (in-process, lab/root) and ipc.SiteClient both implement it.
type SiteFS interface {
	CreateSiteUser(domain string) error
	WritePublicFile(domain, relPath string, data []byte) error
	ReloadCaddy(ctx context.Context, configPath string) error
}

// Ensure *Ops implements SiteFS.
var _ SiteFS = (*Ops)(nil)
