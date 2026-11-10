package dnscheck

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// System uses the process default resolver (and optional preferred servers later).
type System struct {
	R *net.Resolver
}

func (s *System) resolver() *net.Resolver {
	if s != nil && s.R != nil {
		return s.R
	}
	return net.DefaultResolver
}

func (s *System) LookupA(ctx context.Context, host string) ([]net.IP, error) {
	ips, err := s.resolver().LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

func (s *System) LookupAAAA(ctx context.Context, host string) ([]net.IP, error) {
	ips, err := s.resolver().LookupIP(ctx, "ip6", host)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

func (s *System) LookupCNAME(ctx context.Context, host string) (string, error) {
	cname, err := s.resolver().LookupCNAME(ctx, host)
	if err != nil {
		return "", err
	}
	return cname, nil
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
	// net.Resolver has no CAA helper; use LookupTXT-style raw via LookupIP is wrong.
	// Prefer miekg/dns later; for M1 use a best-effort Resolver.Lookup with net package
	// unavailable — return empty without error so checklist can warn when RequireCAA.
	_ = ctx
	_ = host
	return nil, fmt.Errorf("CAA lookup via system resolver not implemented yet; use Fake in tests")
}
