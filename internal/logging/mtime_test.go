package logging_test

import (
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/logging"
)

func TestLocalizeMtimes(t *testing.T) {
	utc := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	files := []logging.FileInfo{{Name: "app.log", Mtime: utc}}
	logging.LocalizeMtimes(files, "Asia/Kolkata")
	got := files[0].Mtime
	if got != "2024-06-15T17:30:00+05:30" {
		t.Fatalf("got %q", got)
	}
	one := logging.LocalizeFileMtime(logging.FileInfo{Mtime: utc}, "America/New_York")
	if one.Mtime != "2024-06-15T08:00:00-04:00" {
		t.Fatalf("got %q", one.Mtime)
	}
}
