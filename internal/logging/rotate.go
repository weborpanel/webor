package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/config"
)

// RotateDirOptions controls rotation for one directory of log streams.
type RotateDirOptions struct {
	Dir           string
	MaxBytes      int64 // rotate current file when size >= MaxBytes (0 = use default 50M)
	RetentionDays int   // keep rotated files this many days (0 = DefaultRetentionDays)
	Now           time.Time
	// HostOnlyStreams when non-empty, only rotate these stream names (host dir).
	HostOnlyStreams []string
}

// RotateDir rotates and prunes log files under dir.
func RotateDir(opt RotateDirOptions) error {
	if opt.Dir == "" {
		return fmt.Errorf("log dir required")
	}
	if err := os.MkdirAll(opt.Dir, 0o755); err != nil {
		return err
	}
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	today := now.Format("2006-01-02")
	maxBytes := opt.MaxBytes
	if maxBytes <= 0 {
		maxBytes = config.ParseByteSize(DefaultSiteLogMax)
	}
	retain := opt.RetentionDays
	if retain <= 0 {
		retain = DefaultRetentionDays
	}
	cutoff := now.AddDate(0, 0, -retain).Format("2006-01-02")

	entries, err := os.ReadDir(opt.Dir)
	if err != nil {
		return err
	}

	streams := map[string]struct{}{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		stream, kind, _, _, err := ParseLogBasename(e.Name())
		if err != nil {
			continue
		}
		if len(opt.HostOnlyStreams) > 0 && !containsString(opt.HostOnlyStreams, stream) {
			continue
		}
		if kind == "current" {
			streams[stream] = struct{}{}
		}
	}
	// Also consider empty current files we may create for known host streams.
	for _, s := range opt.HostOnlyStreams {
		streams[s] = struct{}{}
	}

	for stream := range streams {
		cur := filepath.Join(opt.Dir, CurrentBasename(stream))
		st, err := os.Stat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		modDay := st.ModTime().In(now.Location()).Format("2006-01-02")
		needDate := modDay != today
		needSize := st.Size() >= maxBytes
		if !needDate && !needSize {
			continue
		}
		date := modDay
		if needDate {
			date = modDay
		} else {
			date = today
		}
		dest := nextRotatedPath(opt.Dir, stream, date)
		if err := os.Rename(cur, dest); err != nil {
			return fmt.Errorf("rotate %s: %w", cur, err)
		}
		// Recreate empty current so writers with open FDs still append to old inode;
		// systemd append reopens on restart; Caddy may need reload — acceptable.
		f, err := os.OpenFile(cur, os.O_CREATE|os.O_WRONLY, 0o640)
		if err == nil {
			_ = f.Close()
		}
	}

	return pruneDir(opt.Dir, cutoff, opt.HostOnlyStreams)
}

func nextRotatedPath(dir, stream, date string) string {
	base := filepath.Join(dir, RotatedBasename(stream, date, 0))
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}
	for i := 1; i < 1000; i++ {
		p := filepath.Join(dir, RotatedBasename(stream, date, i))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
	return filepath.Join(dir, RotatedBasename(stream, date, int(time.Now().Unix()%100000)))
}

func pruneDir(dir, cutoff string, hostOnly []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type item struct {
		name   string
		stream string
		date   string
		index  int
	}
	byStream := map[string][]item{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		stream, kind, date, index, err := ParseLogBasename(e.Name())
		if err != nil || kind != "rotated" {
			continue
		}
		if len(hostOnly) > 0 && !containsString(hostOnly, stream) {
			continue
		}
		byStream[stream] = append(byStream[stream], item{e.Name(), stream, date, index})
	}
	for _, list := range byStream {
		sort.Slice(list, func(i, j int) bool {
			if list[i].date != list[j].date {
				return list[i].date > list[j].date
			}
			return list[i].index > list[j].index
		})
		for _, it := range list {
			if it.date < cutoff {
				_ = os.Remove(filepath.Join(dir, it.name))
			}
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// MaxBytesFromLogMax parses limits.log_max or falls back to DefaultSiteLogMax.
func MaxBytesFromLogMax(logMax string) int64 {
	b := config.ParseByteSize(strings.TrimSpace(logMax))
	if b <= 0 {
		return config.ParseByteSize(DefaultSiteLogMax)
	}
	return b
}
