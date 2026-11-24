package ipc_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/ipc"
)

func TestValidateApplyUpdateArgs(t *testing.T) {
	if err := ipc.ValidateApplyUpdateArgs(ipc.ApplyUpdateArgs{Channel: "beta", Version: "0.1.0-beta.2"}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateApplyUpdateArgs(ipc.ApplyUpdateArgs{Channel: "nightly", Version: "1"}); err == nil {
		t.Fatal("expected channel error")
	}
	if err := ipc.ValidateApplyUpdateArgs(ipc.ApplyUpdateArgs{Channel: "stable"}); err == nil {
		t.Fatal("expected version error")
	}
}
