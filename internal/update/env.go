package update

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultBaseURL   = "https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor"
	DefaultPanelEnv  = "/etc/webor/panel.env"
	DefaultBinDir    = "/usr/local/bin"
	envChannelKey    = "WBR_CHANNEL"
	envBaseURLKey    = "WBR_BASE_URL"
)

// ChannelConfig is the persisted release channel preference.
type ChannelConfig struct {
	Channel string
	BaseURL string
	EnvPath string
}

// LoadChannelConfig reads WBR_CHANNEL / WBR_BASE_URL from panel.env (if present).
func LoadChannelConfig(envPath string) ChannelConfig {
	if envPath == "" {
		envPath = DefaultPanelEnv
	}
	cfg := ChannelConfig{
		Channel: "stable",
		BaseURL: DefaultBaseURL,
		EnvPath: envPath,
	}
	f, err := os.Open(envPath)
	if err != nil {
		return cfg
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case envChannelKey:
			if val != "" {
				cfg.Channel = strings.ToLower(val)
			}
		case envBaseURLKey:
			if val != "" {
				cfg.BaseURL = strings.TrimRight(val, "/")
			}
		}
	}
	return cfg
}

// SaveChannelConfig upserts WBR_CHANNEL (and optional WBR_BASE_URL) in panel.env.
func SaveChannelConfig(envPath, channel, baseURL string) error {
	if envPath == "" {
		envPath = DefaultPanelEnv
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if err := ValidateChannel(channel); err != nil {
		return err
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		baseURL = strings.TrimRight(baseURL, "/")
	}

	var lines []string
	if raw, err := os.ReadFile(envPath); err == nil {
		lines = strings.Split(string(raw), "\n")
		// Drop trailing empty line from Split so we can rejoin cleanly.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	set := map[string]string{envChannelKey: channel}
	if baseURL != "" {
		set[envBaseURLKey] = baseURL
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(lines)+2)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			out = append(out, line)
			continue
		}
		key, _, ok := strings.Cut(trim, "=")
		if !ok {
			out = append(out, line)
			continue
		}
		key = strings.TrimSpace(key)
		if val, hit := set[key]; hit {
			out = append(out, key+"="+val)
			seen[key] = true
			continue
		}
		out = append(out, line)
	}
	for k, v := range set {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	body := strings.Join(out, "\n") + "\n"
	tmp := envPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o640); err != nil {
		return fmt.Errorf("write panel.env: %w", err)
	}
	if err := os.Rename(tmp, envPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
