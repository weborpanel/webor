package logging

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileInfo is a listed log file for the UI.
type FileInfo struct {
	Name   string `json:"name"`
	Stream string `json:"stream"`
	Kind   string `json:"kind"` // current | rotated
	Date   string `json:"date,omitempty"`
	Index  int    `json:"index,omitempty"`
	Size   int64  `json:"size"`
	Mtime  string `json:"mtime"` // RFC3339 (UTC from disk; LocalizeMtimes may rewrite for panel TZ)
}

// LocalizeMtimes rewrites FileInfo.Mtime into tz (IANA). Unknown tz leaves values unchanged.
func LocalizeMtimes(files []FileInfo, tz string) {
	tz = strings.TrimSpace(tz)
	if tz == "" || len(files) == 0 {
		return
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return
	}
	for i := range files {
		files[i].Mtime = localizeRFC3339(files[i].Mtime, loc)
	}
}

// LocalizeFileMtime rewrites one FileInfo.Mtime into tz.
func LocalizeFileMtime(info FileInfo, tz string) FileInfo {
	files := []FileInfo{info}
	LocalizeMtimes(files, tz)
	return files[0]
}

func localizeRFC3339(s string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
		if err != nil {
			return s
		}
	}
	return t.In(loc).Format(time.RFC3339)
}

// ListDir lists allowlisted log files in dir, newest first.
func ListDir(dir string, hostOnly bool) ([]FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		stream, kind, date, index, err := ParseLogBasename(e.Name())
		if err != nil {
			continue
		}
		if hostOnly {
			if !ValidHostStreamName(stream) {
				continue
			}
		} else if stream == StreamBackup || !ValidStreamName(stream) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, FileInfo{
			Name:   e.Name(),
			Stream: stream,
			Kind:   kind,
			Date:   date,
			Index:  index,
			Size:   info.Size(),
			Mtime:  info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stream != out[j].Stream {
			return out[i].Stream < out[j].Stream
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == "current" // current first within stream
		}
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Index > out[j].Index
	})
	return out, nil
}

// ResolveSafePath joins dir with basename after allowlist checks.
func ResolveSafePath(dir, name string, hostOnly bool) (string, FileInfo, error) {
	stream, kind, date, index, err := ParseLogBasename(name)
	if err != nil {
		return "", FileInfo{}, err
	}
	if hostOnly {
		if !ValidHostStreamName(stream) {
			return "", FileInfo{}, fmt.Errorf("log file not allowed")
		}
	} else if stream == StreamBackup || !ValidStreamName(stream) {
		return "", FileInfo{}, fmt.Errorf("log file not allowed")
	}
	base := filepath.Base(strings.TrimSpace(name))
	if base != strings.TrimSpace(name) || base == "." || base == ".." {
		return "", FileInfo{}, fmt.Errorf("log file not allowed")
	}
	cleanDir := filepath.Clean(dir)
	full := filepath.Clean(filepath.Join(cleanDir, base))
	if filepath.Dir(full) != cleanDir {
		return "", FileInfo{}, fmt.Errorf("log file not allowed")
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", FileInfo{}, err
	}
	return full, FileInfo{
		Name:   base,
		Stream: stream,
		Kind:   kind,
		Date:   date,
		Index:  index,
		Size:   st.Size(),
		Mtime:  st.ModTime().UTC().Format(time.RFC3339),
	}, nil
}

// TailFile returns the last n lines of a file (n clamped 1..2000).
func TailFile(path string, n int) (string, error) {
	if n < 1 {
		n = 100
	}
	if n > 2000 {
		n = 2000
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := st.Size()
	if size == 0 {
		return "", nil
	}

	// Read from the end in chunks.
	const chunk = 32 * 1024
	var (
		pos   = size
		buf   []byte
		lines []string
	)
	for pos > 0 && len(lines) <= n {
		read := int64(chunk)
		if read > pos {
			read = pos
		}
		pos -= read
		tmp := make([]byte, read)
		if _, err := f.ReadAt(tmp, pos); err != nil && err != io.EOF {
			return "", err
		}
		buf = append(tmp, buf...)
		parts := strings.Split(string(buf), "\n")
		// Keep incomplete first line in buf if not at start.
		if pos > 0 {
			buf = []byte(parts[0])
			parts = parts[1:]
		} else {
			buf = nil
		}
		// Prepend collected complete lines.
		var rebuilt []string
		rebuilt = append(rebuilt, parts...)
		rebuilt = append(rebuilt, lines...)
		lines = rebuilt
		if len(lines) > n+1 {
			lines = lines[len(lines)-(n+1):]
		}
	}
	// Drop trailing empty from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// ReadFileHead reads up to maxBytes from the start (for small files / download preview).
func ReadFileHead(path string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := io.LimitReader(f, maxBytes)
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LineCount is a cheap approximate for tests; scans the file.
func LineCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}
