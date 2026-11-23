package distro

import "testing"

func TestInstallHintsNonEmpty(t *testing.T) {
	for _, fam := range []Family{FamilyDebian, FamilyRHEL, FamilyUnknown} {
		info := Info{Family: fam}
		if info.InstallHintPostgres() == "" || info.InstallHintMariaDB() == "" {
			t.Fatalf("empty hint for %s", fam)
		}
		if info.InstallHintValkey() == "" || info.InstallHintRestic() == "" {
			t.Fatalf("empty cache/restic hint for %s", fam)
		}
	}
	rhel := Info{Family: FamilyRHEL}
	if !contains(rhel.InstallHintPostgres(), "dnf") {
		t.Fatalf("rhel postgres hint should mention dnf: %s", rhel.InstallHintPostgres())
	}
	deb := Info{Family: FamilyDebian}
	if !contains(deb.InstallHintPostgres(), "apt-get") {
		t.Fatalf("debian postgres hint should mention apt-get: %s", deb.InstallHintPostgres())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
