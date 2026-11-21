package ipc_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/ipc"
)

func TestValidatePostgresArgs(t *testing.T) {
	if err := ipc.ValidateInstallPostgresArgs(ipc.InstallPostgresArgs{}); err == nil {
		t.Fatal("expected approval_id required")
	}
	if err := ipc.ValidateInstallPostgresArgs(ipc.InstallPostgresArgs{ApprovalID: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateEnsureSiteDatabaseArgs(ipc.EnsureSiteDatabaseArgs{Domain: "bad", Engine: "postgres"}); err == nil {
		t.Fatal("expected invalid domain")
	}
	if err := ipc.ValidateEnsureSiteDatabaseArgs(ipc.EnsureSiteDatabaseArgs{
		Domain:   "app.example.com",
		Engine:   "sqlite",
		StateDir: "/var/lib/webor",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateEnsureSiteDatabaseArgs(ipc.EnsureSiteDatabaseArgs{
		Domain: "app.example.com",
		Engine: "oracle",
	}); err == nil {
		t.Fatal("expected engine rejected")
	}
}
