package logging_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/logging"
)

func TestRotateDirDateAndSize(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, "access.log")
	yesterday := time.Date(2026, 11, 22, 12, 0, 0, 0, time.Local)
	if err := os.WriteFile(cur, []byte("old day\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cur, yesterday, yesterday); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.Local)
	if err := logging.RotateDir(logging.RotateDirOptions{
		Dir:      dir,
		MaxBytes: 1024,
		Now:      now,
	}); err != nil {
		t.Fatal(err)
	}
	rotated := filepath.Join(dir, "access.log.2026-11-22")
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("expected date rotate: %v", err)
	}
	if st, err := os.Stat(cur); err != nil || st.Size() != 0 {
		t.Fatalf("expected empty current recreated, st=%v err=%v", st, err)
	}

	// Size split same day.
	big := make([]byte, 200)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(cur, big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cur, now, now); err != nil {
		t.Fatal(err)
	}
	if err := logging.RotateDir(logging.RotateDirOptions{
		Dir:      dir,
		MaxBytes: 100,
		Now:      now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "access.log.2026-11-23")); err != nil {
		t.Fatalf("expected size rotate: %v", err)
	}
}

func TestRotateDirPrune(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "app.log.2026-01-01")
	keep := filepath.Join(dir, "app.log.2026-11-20")
	_ = os.WriteFile(old, []byte("x"), 0o644)
	_ = os.WriteFile(keep, []byte("y"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "app.log"), []byte("z"), 0o644)
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.Local)
	if err := logging.RotateDir(logging.RotateDirOptions{
		Dir:           dir,
		MaxBytes:      1 << 30,
		RetentionDays: 7,
		Now:           now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expected old rotated file pruned")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("expected recent rotated file kept")
	}
}

func TestResolveSafePathRejects(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "access.log"), []byte("hi"), 0o644)
	if _, _, err := logging.ResolveSafePath(dir, "../access.log", false); err == nil {
		t.Fatal("expected reject")
	}
	if _, _, err := logging.ResolveSafePath(dir, "backup.log", false); err == nil {
		t.Fatal("backup not allowed in site dir listing")
	}
	path, info, err := logging.ResolveSafePath(dir, "access.log", false)
	if err != nil || info.Stream != "access" || path == "" {
		t.Fatalf("resolve: %v %+v", err, info)
	}
}

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	body := ""
	for i := 1; i <= 20; i++ {
		body += "line-" + filepath.Base(p) + "\n"
	}
	// distinct last lines
	body = "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := logging.TailFile(p, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := "h\ni\nj\n"
	if text != want {
		t.Fatalf("got %q want %q", text, want)
	}
}
