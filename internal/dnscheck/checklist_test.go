package dnscheck_test

import (
	"context"
	"net"
	"testing"

	"github.com/weborpanel/webor/internal/dnscheck"
)

func TestCheckAPass(t *testing.T) {
	r := &dnscheck.Fake{
		A: map[string][]net.IP{
			"example.com": {net.ParseIP("203.0.113.10")},
		},
		CAA: map[string][]dnscheck.CAA{
			"example.com": {{Tag: "issue", Value: "letsencrypt.org"}},
		},
	}
	rep, err := dnscheck.Check(context.Background(), r, dnscheck.Expect{
		Host:       "example.com",
		A:          []string{"203.0.113.10"},
		RequireCAA: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		t.Fatalf("expected OK: %+v", rep.Items)
	}
	if rep.Items[0].Status != dnscheck.StatusPass || rep.Items[0].Record != dnscheck.RecordA {
		t.Fatalf("%+v", rep.Items[0])
	}
}

func TestCheckAFail(t *testing.T) {
	r := &dnscheck.Fake{
		A: map[string][]net.IP{
			"example.com": {net.ParseIP("198.51.100.1")},
		},
	}
	rep, err := dnscheck.Check(context.Background(), r, dnscheck.Expect{
		Host: "example.com",
		A:    []string{"203.0.113.10"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK {
		t.Fatal("expected failure")
	}
}

func TestCheckCNAME(t *testing.T) {
	r := &dnscheck.Fake{
		CNAME: map[string]string{
			"www.example.com": "example.com.",
		},
	}
	rep, err := dnscheck.Check(context.Background(), r, dnscheck.Expect{
		Host:  "www.example.com",
		CNAME: "example.com",
	})
	if err != nil || !rep.OK {
		t.Fatalf("err=%v ok=%v items=%+v", err, rep != nil && rep.OK, rep)
	}
}

func TestCheckCAAWarn(t *testing.T) {
	r := &dnscheck.Fake{
		A: map[string][]net.IP{
			"example.com": {net.ParseIP("203.0.113.10")},
		},
		CAA: map[string][]dnscheck.CAA{},
	}
	rep, err := dnscheck.Check(context.Background(), r, dnscheck.Expect{
		Host:       "example.com",
		A:          []string{"203.0.113.10"},
		RequireCAA: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		t.Fatal("CAA warn must not fail the report")
	}
	var caa dnscheck.Item
	for _, it := range rep.Items {
		if it.Record == dnscheck.RecordCAA {
			caa = it
		}
	}
	if caa.Status != dnscheck.StatusWarn {
		t.Fatalf("caa=%+v", caa)
	}
}
