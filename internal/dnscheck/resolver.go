package dnscheck

import (
	"context"
	"net"
)

// CAA is one Certification Authority Authorization record.
type CAA struct {
	Tag   string // issue, issuewild, iodef
	Value string
}

// Resolver looks up DNS records. Tests inject a fake implementation.
type Resolver interface {
	LookupA(ctx context.Context, host string) ([]net.IP, error)
	LookupAAAA(ctx context.Context, host string) ([]net.IP, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupNS(ctx context.Context, host string) ([]string, error)
	LookupCAA(ctx context.Context, host string) ([]CAA, error)
}
