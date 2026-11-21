// Package ports allocates loopback TCP ports for site HTTP backends.
package ports

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/weborpanel/webor/internal/config"
)

// UsedPorts returns the set of backend ports currently claimed in cfg.
func UsedPorts(cfg *config.Config) map[int]string {
	out := map[int]string{}
	if cfg == nil {
		return out
	}
	for _, s := range cfg.Sites {
		if s.Backend == nil || !s.Backend.Enabled || s.Backend.Port == 0 {
			continue
		}
		out[s.Backend.Port] = s.Name
	}
	return out
}

// IsFreeOnLoopback reports whether 127.0.0.1:port can be bound.
func IsFreeOnLoopback(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// Allocate picks the next free port in [BackendPortMin, BackendPortMax].
// excludeDomain's existing port is ignored so re-attach can keep or replace cleanly.
func Allocate(cfg *config.Config, excludeDomain string) (int, error) {
	used := UsedPorts(cfg)
	if excludeDomain != "" {
		for p, name := range used {
			if strings.EqualFold(name, excludeDomain) {
				delete(used, p)
			}
		}
	}
	for p := config.BackendPortMin; p <= config.BackendPortMax; p++ {
		if _, taken := used[p]; taken {
			continue
		}
		if !IsFreeOnLoopback(p) {
			continue
		}
		return p, nil
	}
	return 0, fmt.Errorf("no free backend port in %d-%d", config.BackendPortMin, config.BackendPortMax)
}
