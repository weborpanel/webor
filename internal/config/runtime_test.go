package config_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestValidateSiteRuntime(t *testing.T) {
	if err := config.ValidateSiteRuntime(config.SiteTypeApp, &config.SiteRuntime{Kind: "node", Version: "22"}); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateSiteRuntime(config.SiteTypeApp, &config.SiteRuntime{Kind: "node", Command: "/usr/bin/node"}); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateSiteRuntime(config.SiteTypeStatic, &config.SiteRuntime{Kind: "node", Version: "22"}); err == nil {
		t.Fatal("expected error for static")
	}
	if err := config.ValidateSiteRuntime(config.SiteTypeApp, &config.SiteRuntime{Kind: "node", Command: "relative/node"}); err == nil {
		t.Fatal("expected abs path")
	}
}

func TestParseRuntime(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
panel: {port: 0}
sites:
  - name: app.example.com
    type: app
    exec: server.js
    runtime:
      kind: node
      version: "22"
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sites[0].Runtime == nil || cfg.Sites[0].Runtime.Version != "22" {
		t.Fatalf("%+v", cfg.Sites[0].Runtime)
	}
}
