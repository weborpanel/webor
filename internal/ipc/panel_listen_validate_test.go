package ipc_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/ipc"
	"github.com/weborpanel/webor/internal/panelenv"
)

func TestValidateSetPanelListenHostArgs(t *testing.T) {
	if err := ipc.ValidateSetPanelListenHostArgs(ipc.SetPanelListenHostArgs{Host: panelenv.HostLocal}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateSetPanelListenHostArgs(ipc.SetPanelListenHostArgs{Host: panelenv.HostPublic}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateSetPanelListenHostArgs(ipc.SetPanelListenHostArgs{Host: "10.0.0.1"}); err == nil {
		t.Fatal("expected reject")
	}
	if err := ipc.ValidateSetPanelListenHostArgs(ipc.SetPanelListenHostArgs{
		Host:    panelenv.HostLocal,
		EnvPath: "/etc/passwd",
	}); err == nil {
		t.Fatal("expected env_path reject")
	}
	if err := ipc.ValidateSetPanelListenHostArgs(ipc.SetPanelListenHostArgs{
		Host:    panelenv.HostLocal,
		EnvPath: panelenv.DefaultEnvPath,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateSetPanelListenHostArgs(ipc.SetPanelListenHostArgs{
		Host:    panelenv.HostLocal,
		EnvPath: "/tmp/panel.env",
	}); err != nil {
		t.Fatal(err)
	}
}
