package dnscheck

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// Fake is an in-memory Resolver for tests.
type Fake struct {
	A     map[string][]net.IP
	AAAA  map[string][]net.IP
	CNAME map[string]string
	NS    map[string][]string
	CAA   map[string][]CAA
	Errs  map[string]error // keyed by "A:host", "CNAME:host", etc.
}

func (f *Fake) err(kind, host string) error {
	if f == nil || f.Errs == nil {
		return nil
	}
	return f.Errs[kind+":"+strings.ToLower(host)]
}

func (f *Fake) LookupA(ctx context.Context, host string) ([]net.IP, error) {
	_ = ctx
	host = strings.ToLower(host)
	if err := f.err("A", host); err != nil {
		return nil, err
	}
	if f == nil || f.A == nil {
		return nil, fmt.Errorf("no A records for %s", host)
	}
	ips, ok := f.A[host]
	if !ok {
		return nil, fmt.Errorf("no A records for %s", host)
	}
	return append([]net.IP(nil), ips...), nil
}

func (f *Fake) LookupAAAA(ctx context.Context, host string) ([]net.IP, error) {
	_ = ctx
	host = strings.ToLower(host)
	if err := f.err("AAAA", host); err != nil {
		return nil, err
	}
	if f == nil || f.AAAA == nil {
		return nil, fmt.Errorf("no AAAA records for %s", host)
	}
	ips, ok := f.AAAA[host]
	if !ok {
		return nil, fmt.Errorf("no AAAA records for %s", host)
	}
	return append([]net.IP(nil), ips...), nil
}

func (f *Fake) LookupCNAME(ctx context.Context, host string) (string, error) {
	_ = ctx
	host = strings.ToLower(host)
	if err := f.err("CNAME", host); err != nil {
		return "", err
	}
	if f == nil || f.CNAME == nil {
		return "", fmt.Errorf("no CNAME for %s", host)
	}
	v, ok := f.CNAME[host]
	if !ok {
		return "", fmt.Errorf("no CNAME for %s", host)
	}
	return v, nil
}

func (f *Fake) LookupNS(ctx context.Context, host string) ([]string, error) {
	_ = ctx
	host = strings.ToLower(host)
	if err := f.err("NS", host); err != nil {
		return nil, err
	}
	if f == nil || f.NS == nil {
		return nil, fmt.Errorf("no NS for %s", host)
	}
	v, ok := f.NS[host]
	if !ok {
		return nil, fmt.Errorf("no NS for %s", host)
	}
	return append([]string(nil), v...), nil
}

func (f *Fake) LookupCAA(ctx context.Context, host string) ([]CAA, error) {
	_ = ctx
	host = strings.ToLower(host)
	if err := f.err("CAA", host); err != nil {
		return nil, err
	}
	if f == nil || f.CAA == nil {
		return nil, nil
	}
	v, ok := f.CAA[host]
	if !ok {
		return nil, nil
	}
	return append([]CAA(nil), v...), nil
}
