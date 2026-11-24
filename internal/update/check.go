package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/weborpanel/webor/internal/version"
)

// Status is the result of a channel version check.
type Status struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Channel string `json:"channel"`
	BaseURL string `json:"base_url"`
	Newer   bool   `json:"newer"`
	Same    bool   `json:"same"`
	TarURL  string `json:"tar_url"`
	SumsURL string `json:"sums_url"`
	VerURL  string `json:"version_url"`
	Arch    string `json:"arch"`
}

// CheckOptions controls Check.
type CheckOptions struct {
	BaseURL string
	Channel string
	Current string // empty → version.Version
	Arch    string // empty → runtime GOARCH mapped
	Client  *http.Client
}

// Check fetches downloads/<channel>/VERSION and compares to the running binary.
func Check(ctx context.Context, opt CheckOptions) (*Status, error) {
	base := strings.TrimRight(strings.TrimSpace(opt.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	ch := strings.ToLower(strings.TrimSpace(opt.Channel))
	if ch == "" {
		ch = "stable"
	}
	if err := ValidateChannel(ch); err != nil {
		return nil, err
	}
	cur := Normalize(opt.Current)
	if cur == "" {
		cur = Normalize(version.Version)
	}
	arch := opt.Arch
	if arch == "" {
		var err error
		arch, err = releaseArch()
		if err != nil {
			return nil, err
		}
	}
	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	verURL := fmt.Sprintf("%s/downloads/%s/VERSION", base, ch)
	latest, err := fetchText(ctx, client, verURL)
	if err != nil {
		return nil, fmt.Errorf("fetch VERSION: %w (channel %q — missing object often shows as HTTP 403; publish VERSION or use -channel beta)", err, ch)
	}
	latest = Normalize(latest)
	if latest == "" {
		return nil, fmt.Errorf("empty VERSION at %s", verURL)
	}

	tarName := fmt.Sprintf("webor-linux-%s.tar.gz", arch)
	st := &Status{
		Current: cur,
		Latest:  latest,
		Channel: ch,
		BaseURL: base,
		Newer:   Compare(latest, cur) > 0,
		Same:    Compare(latest, cur) == 0,
		TarURL:  fmt.Sprintf("%s/downloads/%s/%s", base, ch, tarName),
		SumsURL: fmt.Sprintf("%s/downloads/%s/SHA256SUMS", base, ch),
		VerURL:  verURL,
		Arch:    arch,
	}
	return st, nil
}

func releaseArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
}

func fetchText(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d (%s)", res.StatusCode, url)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
