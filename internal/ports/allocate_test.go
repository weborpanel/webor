package ports_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/config"
	"github.com/weborpanel/webor/internal/ports"
)

func TestAllocateSkipsUsed(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Sites: []config.Site{
			{
				Name: "a.example.com",
				Type: config.SiteTypeStatic,
				Backends: []config.SiteBackend{
					{
						ID:      "api",
						Enabled: true,
						Prefix:  "/api",
						Exec:    "./server",
						Port:    config.BackendPortMin,
					},
				},
			},
		},
	}
	p, err := ports.Allocate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p != config.BackendPortMin+1 {
		t.Fatalf("got port %d want %d", p, config.BackendPortMin+1)
	}
}

func TestAllocateMultipleBackendsSameSite(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Sites: []config.Site{
			{
				Name: "a.example.com",
				Type: config.SiteTypeStatic,
				Backends: []config.SiteBackend{
					{ID: "api", Enabled: true, Prefix: "/api", Exec: "./server", Port: config.BackendPortMin},
					{ID: "ssr", Enabled: true, Prefix: "/ssr", Exec: "./server", Port: config.BackendPortMin + 1},
				},
			},
		},
	}
	p, err := ports.Allocate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p != config.BackendPortMin+2 {
		t.Fatalf("got port %d want %d", p, config.BackendPortMin+2)
	}
}

func TestUsedPorts(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.Site{
			{
				Name: "x.com",
				Backends: []config.SiteBackend{
					{ID: "api", Enabled: true, Port: 20005},
					{ID: "ssr", Enabled: false, Port: 20006},
				},
			},
		},
	}
	u := ports.UsedPorts(cfg)
	if u[20005] != "x.com/api" {
		t.Fatalf("%v", u)
	}
	if _, ok := u[20006]; ok {
		t.Fatal("disabled backend should not claim port")
	}
}
