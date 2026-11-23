package ipc_test

import (
	"testing"

	"github.com/weborpanel/webor/internal/ipc"
)

func TestValidateBackupRestoreArgs(t *testing.T) {
	err := ipc.ValidateBackupRestoreArgs(ipc.BackupRestoreArgs{
		SnapshotID: "abc",
		Sites:      []string{"example.com"},
	})
	if err == nil {
		t.Fatal("expected approval_id required")
	}
	err = ipc.ValidateBackupRestoreArgs(ipc.BackupRestoreArgs{
		ApprovalID: "a1",
		SnapshotID: "abc",
		Sites:      []string{"example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ipc.ValidateBackupRestoreArgs(ipc.BackupRestoreArgs{
		ApprovalID: "a1",
		SnapshotID: "abc",
	})
	if err == nil {
		t.Fatal("expected sites or panel")
	}
}
