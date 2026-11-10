package dnscheck_test

import (
	"context"
	"net"
	"testing"

	"github.com/webooza/webooza/internal/dnscheck"
)

func TestCanIssueCertificate(t *testing.T) {
	if dnscheck.CanIssueCertificate(nil) {
		t.Fatal("nil")
	}
	if !dnscheck.CanIssueCertificate(&dnscheck.Report{OK: true}) {
		t.Fatal("want ok")
	}
	if dnscheck.CanIssueCertificate(&dnscheck.Report{OK: false}) {
		t.Fatal("want blocked")
	}
}

func TestCheckAuthoritativeAddsAuthRow(t *testing.T) {
	r := &dnscheck.Fake{
		A: map[string][]net.IP{
			"example.com": {net.ParseIP("203.0.113.10")},
		},
		NS: map[string][]string{
			"example.com": {"ns1.example.com"},
		},
		CAA: map[string][]dnscheck.CAA{},
	}
	// Auth path will try to resolve ns1.example.com via real DNS — may warn.
	rep, err := dnscheck.CheckAuthoritative(context.Background(), r, dnscheck.Expect{
		Host: "example.com",
		A:    []string{"203.0.113.10"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		// Public A passed; auth may warn without failing if NS unreachable in test env.
		for _, it := range rep.Items {
			if it.Record == "A" && it.Status == dnscheck.StatusFail {
				t.Fatalf("%+v", rep.Items)
			}
		}
	}
	found := false
	for _, it := range rep.Items {
		if it.Record == "A@auth" || it.Record == "NS" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected auth/NS item: %+v", rep.Items)
	}
}
