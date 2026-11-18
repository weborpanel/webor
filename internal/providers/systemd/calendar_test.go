package systemd_test

import (
	"strings"
	"testing"

	"github.com/webooza/webooza/internal/providers/systemd"
)

func TestCronToOnCalendarDaily(t *testing.T) {
	got, err := systemd.CronToOnCalendar("0 3 * * *")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "*-*-* 03:00:00" {
		t.Fatalf("got %#v", got)
	}
}

func TestCronToOnCalendarEveryFiveMinutes(t *testing.T) {
	got, err := systemd.CronToOnCalendar("*/5 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "*-*-* *:00/5:00" {
		t.Fatalf("got %#v", got)
	}
}

func TestCronToOnCalendarWeekdays(t *testing.T) {
	got, err := systemd.CronToOnCalendar("0 9 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "Mon..Fri *-*-* 09:00:00" {
		t.Fatalf("got %#v", got)
	}
}

func TestCronToOnCalendarDOMAndDOW(t *testing.T) {
	got, err := systemd.CronToOnCalendar("0 0 1 * 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 calendars, got %#v", got)
	}
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "*-*-01 00:00:00") || !strings.Contains(joined, "Mon *-*-* 00:00:00") {
		t.Fatalf("got %#v", got)
	}
}

func TestCronToOnCalendarRejectsBad(t *testing.T) {
	if _, err := systemd.CronToOnCalendar("0 3 * *"); err == nil {
		t.Fatal("expected error")
	}
}
