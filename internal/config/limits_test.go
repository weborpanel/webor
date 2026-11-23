package config_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestCPUQuotaFromCores(t *testing.T) {
	if got := config.CPUQuotaFromCores(1); got != "100%" {
		t.Fatalf("got %q", got)
	}
	if got := config.CoresFromCPUQuota("200%"); got != 2 {
		t.Fatalf("got %d", got)
	}
}

func TestValidatePanelLimitsDiskCeiling(t *testing.T) {
	err := config.ValidatePanelLimits(config.Limits{DiskQuota: "20G"}, 0, 0)
	if err == nil {
		t.Fatal("expected error for 20G")
	}
	if err := config.ValidatePanelLimits(config.Limits{DiskQuota: "10G"}, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidatePanelLimits(config.Limits{DiskQuota: "10M"}, 0, 0); err != nil {
		t.Fatal(err)
	}
	if config.DefaultDiskQuota != "10M" {
		t.Fatalf("default=%q", config.DefaultDiskQuota)
	}
}

func TestMemoryOptionsForHost(t *testing.T) {
	opts := config.MemoryOptionsForHost(600 << 20) // 600 MiB
	for _, o := range opts {
		if o.Value == "1G" {
			t.Fatalf("1G should be filtered on 600M host: %+v", opts)
		}
	}
	found := false
	for _, o := range opts {
		if o.Value == "512M" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 512M in %+v", opts)
	}
}

func TestCPUOptionsForHost(t *testing.T) {
	opts := config.CPUOptionsForHost(2)
	for _, o := range opts {
		if o.Value == "400%" {
			t.Fatalf("4c should be filtered on 2-CPU host: %+v", opts)
		}
	}
}
