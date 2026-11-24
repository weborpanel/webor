package logging_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/logging"
	"github.com/weborpanel/webor/internal/sites"
)

func TestSelectorToStream(t *testing.T) {
	cases := map[string]string{
		"":           logging.StreamApp,
		"app":        logging.StreamApp,
		"backend":    logging.StreamBackend,
		"access":     logging.StreamAccess,
		"api:foo":    "api-foo",
		"cron:Nightly": "cron-nightly",
		"job:worker": "job-worker",
		"apis":       "",
		"unknown":    "",
	}
	for in, want := range cases {
		if got := logging.SelectorToStream(in); got != want {
			t.Fatalf("SelectorToStream(%q)=%q want %q", in, got, want)
		}
	}
}

func TestParseLogBasename(t *testing.T) {
	stream, kind, date, idx, err := logging.ParseLogBasename("access.log")
	if err != nil || stream != "access" || kind != "current" || date != "" || idx != 0 {
		t.Fatalf("current: %v %s %s %s %d", err, stream, kind, date, idx)
	}
	stream, kind, date, idx, err = logging.ParseLogBasename("access.log.2026-11-23")
	if err != nil || stream != "access" || kind != "rotated" || date != "2026-11-23" || idx != 0 {
		t.Fatalf("rotated: %v %s %s %s %d", err, stream, kind, date, idx)
	}
	stream, kind, date, idx, err = logging.ParseLogBasename("cron-nightly.log.2026-11-23.2")
	if err != nil || stream != "cron-nightly" || kind != "rotated" || date != "2026-11-23" || idx != 2 {
		t.Fatalf("split: %v %s %s %s %d", err, stream, kind, date, idx)
	}
	if _, _, _, _, err := logging.ParseLogBasename("../etc/passwd"); err == nil {
		t.Fatal("expected reject traversal")
	}
	if _, _, _, _, err := logging.ParseLogBasename("evil.log.bak"); err == nil {
		t.Fatal("expected reject unknown pattern")
	}
}

func TestSiteCurrentPath(t *testing.T) {
	got := logging.SiteCurrentPath("example.com", logging.StreamAccess)
	want := sites.Logs("example.com") + "/access.log"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidHostStreamName(t *testing.T) {
	if !logging.ValidHostStreamName("backup") {
		t.Fatal("backup should be host stream")
	}
	if logging.ValidHostStreamName("access") {
		t.Fatal("access is not host stream")
	}
}
