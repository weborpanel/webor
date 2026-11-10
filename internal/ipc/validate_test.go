package ipc_test

import (
	"strings"
	"testing"

	"github.com/webooza/webooza/internal/ipc"
)

func TestValidateDomain(t *testing.T) {
	if err := ipc.ValidateDomain("example.com"); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateDomain("../etc"); err == nil {
		t.Fatal("expected rejection")
	}
	if err := ipc.ValidateDomain(""); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestValidateSitePath(t *testing.T) {
	if err := ipc.ValidateSitePath("example.com", "/srv/sites/example.com/public"); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateSitePath("example.com", "/srv/sites/other.com/x"); err == nil {
		t.Fatal("expected escape rejection")
	}
	if err := ipc.ValidateSitePath("example.com", "/etc/passwd"); err == nil {
		t.Fatal("expected escape rejection")
	}
}

func TestValidateUnitName(t *testing.T) {
	if err := ipc.ValidateUnitName("site.service"); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateUnitName("../evil.service"); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestValidateWriteUnitFileArgs(t *testing.T) {
	err := ipc.ValidateWriteUnitFileArgs(ipc.WriteUnitFileArgs{
		Domain:  "example.com",
		Unit:    "app.service",
		Content: "[Unit]\nDescription=test\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ipc.ValidateWriteUnitFileArgs(ipc.WriteUnitFileArgs{
		Domain:  "example.com",
		Unit:    "app.service",
		Content: strings.Repeat("x", 300*1024),
	})
	if err == nil {
		t.Fatal("expected size rejection")
	}
}

func TestValidateReloadCaddyArgs(t *testing.T) {
	if err := ipc.ValidateReloadCaddyArgs(ipc.ReloadCaddyArgs{ConfigPath: "/etc/webooza/caddy.json"}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateReloadCaddyArgs(ipc.ReloadCaddyArgs{ConfigPath: "/tmp/evil.json"}); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestValidateDeployStaticZipArgs(t *testing.T) {
	if err := ipc.ValidateDeployStaticZipArgs(ipc.DeployStaticZipArgs{
		Domain:  "example.com",
		ZipPath: "/srv/sites/example.com/tmp/site.zip",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateDeployStaticZipArgs(ipc.DeployStaticZipArgs{
		Domain:  "example.com",
		ZipPath: "/tmp/evil.zip",
	}); err == nil {
		t.Fatal("expected path rejection")
	}
}

func TestValidateDeleteSiteUserArgs(t *testing.T) {
	if err := ipc.ValidateDeleteSiteUserArgs(ipc.DeleteSiteUserArgs{Domain: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := ipc.ValidateDeleteSiteUserArgs(ipc.DeleteSiteUserArgs{Domain: "example.com", DeleteData: true}); err == nil {
		t.Fatal("expected approval_id requirement")
	}
	if err := ipc.ValidateDeleteSiteUserArgs(ipc.DeleteSiteUserArgs{
		Domain: "example.com", DeleteData: true, ApprovalID: "abc",
	}); err != nil {
		t.Fatal(err)
	}
}
