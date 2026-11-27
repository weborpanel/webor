package postgres

import (
	"strings"
	"testing"
)

func TestMergePanelAuthHBA(t *testing.T) {
	in := "local all all peer\nhost all all 127.0.0.1/32 ident\n"
	out, changed := MergePanelAuthHBA(in)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.HasPrefix(out, PanelAuthHBAMarker) {
		t.Fatalf("marker missing: %q", out)
	}
	if !strings.Contains(out, "host all all 127.0.0.1/32 scram-sha-256") {
		t.Fatalf("ipv4 rule missing: %q", out)
	}
	if !strings.Contains(out, "local all all peer") {
		t.Fatal("existing peer rule was dropped")
	}
	again, changed := MergePanelAuthHBA(out)
	if changed || again != out {
		t.Fatal("second merge should be a no-op")
	}
}
