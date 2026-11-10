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
	if resp.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestWriteUnitFileValidates(t *testing.T) {
	h := ipc.DefaultHandlers()[ipc.OpWriteUnitFile]
	args, _ := json.Marshal(ipc.WriteUnitFileArgs{
		Domain:  "example.com",
		Unit:    "../evil.service",
		Content: "[Unit]\nDescription=x\n",
	})
	resp := h(ipc.Request{ID: "1", Operation: ipc.OpWriteUnitFile, Args: args}, ipc.PeerCred{})
	if resp.OK {
		t.Fatal("expected validation failure")
	}
}

func TestReloadCaddyValidates(t *testing.T) {
	h := ipc.DefaultHandlers()[ipc.OpReloadCaddy]
	args, _ := json.Marshal(ipc.ReloadCaddyArgs{ConfigPath: "/tmp/evil.json"})
	resp := h(ipc.Request{ID: "1", Operation: ipc.OpReloadCaddy, Args: args}, ipc.PeerCred{})
	if resp.OK {
		t.Fatal("expected validation failure")
	}
}
