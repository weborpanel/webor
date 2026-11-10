package dnscheck

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
)

// Status values for a single checklist row.
const (
	StatusPass = "pass"
	StatusFail = "fail"
	StatusWarn = "warn"
	StatusSkip = "skip"
)

// Record kinds shown in the domain checklist UI.
const (
	RecordA     = "A"
	RecordAAAA  = "AAAA"
	RecordCNAME = "CNAME"
	RecordCAA   = "CAA"
)

// Expect describes what the panel wants DNS to show for a domain.
type Expect struct {
	// Host is the FQDN under check (apex or alias).
	Host string
	// A is the set of expected IPv4 addresses (panel / VPS public IP).
	A []string
	// AAAA is the set of expected IPv6 addresses (optional; empty skips AAAA fail).
	AAAA []string
	// CNAME, when set, means Host should be a CNAME to this target (aliases).
	CNAME string
	// RequireCAA warns when no issue/issuewild CAA is present (never hard-fails).
	RequireCAA bool
}

// Item is one row of the DNS checklist.
type Item struct {
	Record  string `json:"record"`
	Host    string `json:"host"`
	Want    string `json:"want,omitempty"`
	Got     string `json:"got,omitempty"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

// Report is the full checklist result for one host.
type Report struct {
	Host    string `json:"host"`
	OK      bool   `json:"ok"` // true when no StatusFail items
	Items   []Item `json:"items"`
}

// Check runs the record checklist against r.
func Check(ctx context.Context, r Resolver, exp Expect) (*Report, error) {
	if r == nil {
		return nil, fmt.Errorf("resolver is nil")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(exp.Host)), ".")
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}
	rep := &Report{Host: host, OK: true}

	if exp.CNAME != "" {
		rep.Items = append(rep.Items, checkCNAME(ctx, r, host, exp.CNAME))
	} else {
		rep.Items = append(rep.Items, checkA(ctx, r, host, exp.A))
		rep.Items = append(rep.Items, checkAAAA(ctx, r, host, exp.AAAA))
	}
	rep.Items = append(rep.Items, checkCAA(ctx, r, host, exp.RequireCAA))

	for _, it := range rep.Items {
		if it.Status == StatusFail {
			rep.OK = false
			break
		}
	}
	return rep, nil
}

func checkA(ctx context.Context, r Resolver, host string, want []string) Item {
	item := Item{Record: RecordA, Host: host, Want: strings.Join(want, ",")}
	if len(want) == 0 {
		item.Status = StatusSkip
		item.Detail = "no expected A addresses configured"
		return item
	}
	ips, err := r.LookupA(ctx, host)
	if err != nil {
		item.Status = StatusFail
		item.Detail = err.Error()
		return item
	}
	got := ipStrings(ips)
	item.Got = strings.Join(got, ",")
	if sameSet(got, want) {
		item.Status = StatusPass
		return item
	}
	item.Status = StatusFail
	item.Detail = "A records do not match expected addresses"
	return item
}

func checkAAAA(ctx context.Context, r Resolver, host string, want []string) Item {
	item := Item{Record: RecordAAAA, Host: host, Want: strings.Join(want, ",")}
	if len(want) == 0 {
		item.Status = StatusSkip
		item.Detail = "IPv6 not required"
		return item
	}
	ips, err := r.LookupAAAA(ctx, host)
	if err != nil {
		item.Status = StatusFail
		item.Detail = err.Error()
		return item
	}
	got := ipStrings(ips)
	item.Got = strings.Join(got, ",")
	if sameSet(got, want) {
		item.Status = StatusPass
		return item
	}
	item.Status = StatusFail
	item.Detail = "AAAA records do not match expected addresses"
	return item
}

func checkCNAME(ctx context.Context, r Resolver, host, want string) Item {
	want = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(want)), ".")
	item := Item{Record: RecordCNAME, Host: host, Want: want}
	got, err := r.LookupCNAME(ctx, host)
	if err != nil {
		item.Status = StatusFail
		item.Detail = err.Error()
		return item
	}
	got = strings.TrimSuffix(strings.ToLower(got), ".")
	item.Got = got
	if got == want {
		item.Status = StatusPass
		return item
	}
	item.Status = StatusFail
	item.Detail = "CNAME target mismatch"
	return item
}

func checkCAA(ctx context.Context, r Resolver, host string, require bool) Item {
	item := Item{Record: RecordCAA, Host: host}
	recs, err := r.LookupCAA(ctx, host)
	if err != nil {
		if require {
			item.Status = StatusWarn
			item.Detail = err.Error()
			return item
		}
		item.Status = StatusSkip
		item.Detail = err.Error()
		return item
	}
	var parts []string
	hasIssue := false
	for _, c := range recs {
		parts = append(parts, c.Tag+" "+c.Value)
		if c.Tag == "issue" || c.Tag == "issuewild" {
			hasIssue = true
		}
	}
	item.Got = strings.Join(parts, "; ")
	if !require {
		item.Status = StatusSkip
		if len(recs) > 0 {
			item.Detail = "CAA present (informational)"
		} else {
			item.Detail = "CAA not required"
		}
		return item
	}
	if hasIssue {
		item.Status = StatusPass
		return item
	}
	item.Status = StatusWarn
	item.Detail = "no issue/issuewild CAA; Let's Encrypt may still work, but CAA is recommended"
	return item
}

func ipStrings(ips []net.IP) []string {
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		out = append(out, ip.String())
	}
	slices.Sort(out)
	return out
}

func sameSet(got, want []string) bool {
	g := normalizeIPs(got)
	w := normalizeIPs(want)
	if len(g) != len(w) {
		return false
	}
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}

func normalizeIPs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			out = append(out, ip.String())
			continue
		}
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}
