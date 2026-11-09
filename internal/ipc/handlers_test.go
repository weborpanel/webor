package ipc_test

import (
	"encoding/json"
	"testing"

	"github.com/webooza/webooza/internal/ipc"
)

func TestCreateSiteUserValidates(t *testing.T) {
	h := ipc.DefaultHandlers()[ipc.OpCreateSiteUser]
	args, _ := json.Marshal(ipc.CreateSiteUserArgs{Domain: "bad domain"})
	resp := h(ipc.Request{ID: "1", Operation: ipc.OpCreateSiteUser, Args: args}, ipc.PeerCred{})
	if resp.OK {
		t.Fatal("expected validation failure")
	}
	if resp.Error == "not implemented" {
		t.Fatalf("expected validation error, got %q", resp.Error)
	}

	args, _ = json.Marshal(ipc.CreateSiteUserArgs{Domain: "example.com"})
	resp = h(ipc.Request{ID: "2", Operation: ipc.OpCreateSiteUser, Args: args}, ipc.PeerCred{})
	if resp.Error != "not implemented" {
		t.Fatalf("want not implemented, got %q", resp.Error)
	}
}
