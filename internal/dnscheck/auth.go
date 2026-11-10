package dnscheck

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// CheckAuthoritative resolves NS for the host's zone, then verifies A records
// against those nameservers (in addition to the public checklist).
func CheckAuthoritative(ctx context.Context, public Resolver, exp Expect) (*Report, error) {
	rep, err := Check(ctx, public, exp)
	if err != nil {
		return nil, err
	}
	if exp.CNAME != "" || len(exp.A) == 0 {
		return rep, nil
	}

	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(exp.Host)), ".")
	nsList, err := public.LookupNS(ctx, parentZone(host))
	if err != nil {
		// Fall back to querying the host itself for NS.
		nsList, err = public.LookupNS(ctx, host)
	}
	if err != nil {
		rep.Items = append(rep.Items, Item{
			Record: "NS",
			Host:   host,
			Status: StatusWarn,
			Detail: "could not discover authoritative NS: " + err.Error(),
		})
		return rep, nil
	}

	var authGot []string
	var lastErr error
	for _, ns := range nsList {
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", ns)
		if err != nil || len(ips) == 0 {
			lastErr = err
			continue
		}
		cli := &Client{Server: net.JoinHostPort(ips[0].String(), "53")}
		a, err := cli.LookupA(ctx, host)
		if err != nil {
			lastErr = err
			continue
		}
		for _, ip := range a {
			authGot = append(authGot, ip.String())
		}
		break
	}
	item := Item{
		Record: "A@auth",
		Host:   host,
		Want:   strings.Join(exp.A, ","),
		Got:    strings.Join(normalizeIPs(authGot), ","),
	}
	if len(authGot) == 0 {
		item.Status = StatusWarn
		if lastErr != nil {
			item.Detail = lastErr.Error()
		} else {
			item.Detail = "no authoritative A answer"
		}
	} else if sameSet(authGot, exp.A) {
		item.Status = StatusPass
		item.Detail = fmt.Sprintf("via %s", strings.Join(nsList, ","))
	} else {
		item.Status = StatusFail
		item.Detail = "authoritative A does not match expected"
		rep.OK = false
	}
	rep.Items = append(rep.Items, item)
	return rep, nil
}

// CanIssueCertificate is true when the checklist has no hard failures.
// CAA warnings do not block issuance.
func CanIssueCertificate(rep *Report) bool {
	return rep != nil && rep.OK
}

func parentZone(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	// For foo.example.com check example.com NS; for example.com check itself.
	return strings.Join(parts[len(parts)-2:], ".")
}
