package helper_test

import (
	"strings"
	"testing"

	"github.com/weborpanel/webor/internal/helper"
)

func TestSiteUsername(t *testing.T) {
	got := helper.SiteUsername("Example.COM")
	if got != "wbr_example_com" {
		t.Fatalf("got %q", got)
	}
	long := helper.SiteUsername("this.is.a.very.long.subdomain.example.com")
	if len(long) > 32 {
		t.Fatalf("len=%d name=%q", len(long), long)
	}
	if !strings.HasPrefix(long, "wbr_") {
		t.Fatalf("prefix: %q", long)
	}
}

func TestSiteRoot(t *testing.T) {
	if helper.SiteRoot("Example.com") != "/srv/sites/example.com" {
		t.Fatal(helper.SiteRoot("Example.com"))
	}
}
