package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/weborpanel/webor/internal/ipc"
)

func (s *Server) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if name == "" {
		s.writeErr(w, 400, "site name required")
		return
	}
	sitesMu.Lock()
	cfg, err := s.loadOrEmptyConfig()
	sitesMu.Unlock()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	site, _, found := cfg.FindSite(name)
	if !found {
		s.writeErr(w, 404, "site not found")
		return
	}

	sel := strings.TrimSpace(r.URL.Query().Get("unit"))
	if sel == "" {
		sel = "app"
	}
	lines := 100
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 {
			s.writeErr(w, 400, "lines must be 1..500")
			return
		}
		lines = n
	}

	var apiIDs []string
	if sel == "apis" {
		for _, b := range site.EnabledBackends() {
			apiIDs = append(apiIDs, b.ID)
		}
		if len(apiIDs) == 0 {
			s.writeErr(w, 400, "no APIs configured")
			return
		}
	}

	unit, text, err := s.siteJournalTail(name, sel, lines, apiIDs)
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":    true,
		"unit":  unit,
		"lines": lines,
		"text":  text,
	})
}

// siteJournalTail reads journals via the privileged helper when available.
// The panel user cannot open system journals itself.
func (s *Server) siteJournalTail(domain, selector string, lines int, apiIDs []string) (unit, text string, err error) {
	if s.HelperSocket != "" {
		raw, _ := json.Marshal(ipc.JournalTailArgs{
			Domain: domain,
			Unit:   selector,
			Lines:  lines,
			APIIds: apiIDs,
		})
		resp, err := ipc.Call(s.HelperSocket, ipc.Request{
			ID: "panel", Operation: ipc.OpJournalTail, Args: raw,
		})
		if err != nil {
			return "", "", err
		}
		if !resp.OK {
			return "", "", fmt.Errorf("%s", resp.Error)
		}
		var out struct {
			Unit string `json:"unit"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(resp.Message), &out); err != nil {
			// Backward-compatible: older helpers returned "unit\ntext".
			msg := resp.Message
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				return msg[:i], msg[i+1:], nil
			}
			return "", "", fmt.Errorf("invalid journal response")
		}
		return out.Unit, out.Text, nil
	}
	if selector == "apis" {
		units, text, err := s.ops().SiteJournalTailAPIs(domain, apiIDs, lines)
		return strings.Join(units, ","), text, err
	}
	return s.ops().SiteJournalTail(domain, selector, lines)
}
