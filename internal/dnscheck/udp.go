package dnscheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Client talks DNS over UDP/TCP to a specific server (authoritative or public).
type Client struct {
	// Server is host:port, e.g. "8.8.8.8:53". Empty uses the system default via net.Resolver for A/AAAA/CNAME/NS,
	// and 1.1.1.1:53 for CAA when needed.
	Server  string
	Timeout time.Duration
	Net     string // "udp" (default) or "tcp"
}

func (c *Client) server() string {
	if c != nil && c.Server != "" {
		return c.Server
	}
	return "1.1.1.1:53"
}

func (c *Client) network() string {
	if c != nil && c.Net != "" {
		return c.Net
	}
	return "udp"
}

func (c *Client) timeout() time.Duration {
	if c != nil && c.Timeout > 0 {
		return c.Timeout
	}
	return 5 * time.Second
}

func (c *Client) exchange(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	client := &dns.Client{Net: c.network(), Timeout: c.timeout()}
	// dns.Client doesn't take context directly; honor deadline roughly via Timeout.
	_ = ctx
	resp, _, err := client.Exchange(msg, c.server())
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("empty DNS response")
	}
	if resp.Rcode != dns.RcodeSuccess && resp.Rcode != dns.RcodeNameError {
		return nil, fmt.Errorf("DNS rcode %s", dns.RcodeToString[resp.Rcode])
	}
	return resp, nil
}

func (c *Client) query(ctx context.Context, name string, qtype uint16) (*dns.Msg, error) {
	name = dns.Fqdn(strings.TrimSpace(name))
	m := new(dns.Msg)
	m.SetQuestion(name, qtype)
	m.RecursionDesired = true
	return c.exchange(ctx, m)
}

func (c *Client) LookupA(ctx context.Context, host string) ([]net.IP, error) {
	resp, err := c.query(ctx, host, dns.TypeA)
	if err != nil {
		return nil, err
	}
	var out []net.IP
	for _, rr := range resp.Answer {
		if a, ok := rr.(*dns.A); ok {
			out = append(out, a.A)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no A records for %s", host)
	}
	return out, nil
}

func (c *Client) LookupAAAA(ctx context.Context, host string) ([]net.IP, error) {
	resp, err := c.query(ctx, host, dns.TypeAAAA)
	if err != nil {
		return nil, err
	}
	var out []net.IP
	for _, rr := range resp.Answer {
		if a, ok := rr.(*dns.AAAA); ok {
			out = append(out, a.AAAA)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no AAAA records for %s", host)
	}
	return out, nil
}

func (c *Client) LookupCNAME(ctx context.Context, host string) (string, error) {
	resp, err := c.query(ctx, host, dns.TypeCNAME)
	if err != nil {
		return "", err
	}
	for _, rr := range resp.Answer {
		if cname, ok := rr.(*dns.CNAME); ok {
			return strings.TrimSuffix(cname.Target, "."), nil
		}
	}
	return "", fmt.Errorf("no CNAME for %s", host)
}

func (c *Client) LookupNS(ctx context.Context, host string) ([]string, error) {
	resp, err := c.query(ctx, host, dns.TypeNS)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, rr := range resp.Answer {
		if ns, ok := rr.(*dns.NS); ok {
			out = append(out, strings.TrimSuffix(ns.Ns, "."))
		}
	}
	// Also check authority section (common for referrals / apex queries).
	for _, rr := range resp.Ns {
		if ns, ok := rr.(*dns.NS); ok {
			out = append(out, strings.TrimSuffix(ns.Ns, "."))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no NS for %s", host)
	}
	return uniqueStrings(out), nil
}

func (c *Client) LookupCAA(ctx context.Context, host string) ([]CAA, error) {
	resp, err := c.query(ctx, host, dns.TypeCAA)
	if err != nil {
		return nil, err
	}
	var out []CAA
	for _, rr := range resp.Answer {
		if caa, ok := rr.(*dns.CAA); ok {
			out = append(out, CAA{Tag: caa.Tag, Value: caa.Value})
		}
	}
	return out, nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		s = strings.ToLower(s)
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
