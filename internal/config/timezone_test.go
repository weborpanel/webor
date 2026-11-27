package config_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/config"
)

func TestValidateTimezone(t *testing.T) {
	if err := config.ValidateTimezone("UTC"); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateTimezone("Asia/Kolkata"); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateTimezone("Not/AZone"); err == nil {
		t.Fatal("expected error")
	}
	if err := config.ValidateTimezone(""); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestEffectiveTimezone(t *testing.T) {
	cfg := &config.Config{Version: 1, Panel: config.Panel{Timezone: "America/New_York"}}
	if got := config.EffectiveTimezone(cfg); got != "America/New_York" {
		t.Fatalf("got %q", got)
	}
	empty := &config.Config{Version: 1}
	got := config.EffectiveTimezone(empty)
	if got == "" {
		t.Fatal("expected host or UTC fallback")
	}
}
