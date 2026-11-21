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
				Backend: &config.SiteBackend{
					Enabled: true,
					Prefix:  "/api",
					Exec:    "./bin/server",
					Port:    config.BackendPortMin,
				},
			},
		},
	}
	p, err := ports.Allocate(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if p != config.BackendPortMin+1 {
		t.Fatalf("got port %d want %d", p, config.BackendPortMin+1)
	}
}

func TestAllocateReusesExcludeDomain(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Sites: []config.Site{
			{
				Name: "a.example.com",
				Type: config.SiteTypeStatic,
				Backend: &config.SiteBackend{
					Enabled: true,
					Prefix:  "/api",
					Exec:    "./bin/server",
					Port:    config.BackendPortMin,
				},
			},
		},
	}
	p, err := ports.Allocate(cfg, "a.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p != config.BackendPortMin {
		t.Fatalf("got port %d want %d (own port reusable)", p, config.BackendPortMin)
	}
}

func TestUsedPorts(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.Site{
			{Name: "x.com", Backend: &config.SiteBackend{Enabled: true, Port: 20005}},
			{Name: "y.com", Backend: &config.SiteBackend{Enabled: false, Port: 20006}},
		},
	}
	u := ports.UsedPorts(cfg)
	if u[20005] != "x.com" {
		t.Fatalf("%v", u)
	}
	if _, ok := u[20006]; ok {
		t.Fatal("disabled backend should not claim port")
	}
}
