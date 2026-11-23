package helper

import (
	"os"
	"path/filepath"
	"testing"

	wbrruntime "github.com/weborpanel/webor/internal/runtime"
)

func TestPrepareNodeRuntimeAccessChmodsStateTree(t *testing.T) {
	tmp := t.TempDir()
	prev := wbrruntime.NodeRoot
	wbrruntime.NodeRoot = filepath.Join(tmp, "runtimes", "node")
	t.Cleanup(func() { wbrruntime.NodeRoot = prev })

	full := "22.23.5"
	binDir := wbrruntime.BinDir(full)
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	bin := wbrruntime.NodeBinary(full)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Restrict state dir like the old installer (0750) — prepare must open it up.
	stateDir := filepath.Dir(filepath.Dir(wbrruntime.NodeRoot))
	if err := os.Chmod(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := prepareNodeRuntimeAccess(full); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{stateDir, wbrruntime.NodeRoot, wbrruntime.DirForVersion(full)} {
		st, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm()&0o005 == 0 {
			t.Fatalf("%s mode=%o want other-executable", dir, st.Mode().Perm())
		}
	}
	st, err := os.Stat(bin)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("node binary not executable: %o", st.Mode().Perm())
	}
}
