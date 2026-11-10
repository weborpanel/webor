package dnscheck

import (
	"context"
	"net"
	"strings"
)

// System uses the OS default resolver for A/AAAA/CNAME/NS and a public DNS
// client (1.1.1.1) for CAA via miekg/dns.
type System struct {
	R   *net.Resolver
	CAA *Client // optional override for CAA / raw queries
}

func (s *System) resolver() *net.Resolver {
	if s != nil && s.R != nil {
		return s.R
	}
	return net.DefaultResolver
}

func (s *System) caaClient() *Client {
	if s != nil && s.CAA != nil {
		return s.CAA
	}
	return &Client{Server: "1.1.1.1:53"}
}

func (s *System) LookupA(ctx context.Context, host string) ([]net.IP, error) {
	return s.resolver().LookupIP(ctx, "ip4", host)
}

func (s *System) LookupAAAA(ctx context.Context, host string) ([]net.IP, error) {
	return s.resolver().LookupIP(ctx, "ip6", host)
}

func (s *System) LookupCNAME(ctx context.Context, host string) (string, error) {
	cname, err := s.resolver().LookupCNAME(ctx, host)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(cname, "."), nil
}

func (s *System) LookupNS(ctx context.Context, host string) ([]string, error) {
	ns, err := s.resolver().LookupNS(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, strings.TrimSuffix(n.Host, "."))
	}
	return out, nil
}

func (s *System) LookupCAA(ctx context.Context, host string) ([]CAA, error) {
	return s.caaClient().LookupCAA(ctx, host)
}
