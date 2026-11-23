package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/config"
	wbrruntime "github.com/weborpanel/webor/internal/runtime"
	"github.com/weborpanel/webor/internal/sites"
)

// AppExec describes the resolved ExecStart and optional PATH for an app/backend unit.
type AppExec struct {
	ExecStart string // full ExecStart= value (may include args)
	PathEnv   string // optional directory to prepend to PATH
}

// ResolveAppExec builds ExecStart for a type:app site. nodeBinOverride is used in tests;
// production passes "" and resolves from runtime on disk.
func ResolveAppExec(site config.Site, nodeBinOverride string) (AppExec, error) {
	execRaw := strings.TrimSpace(site.Exec)
	if execRaw == "" {
		return AppExec{}, fmt.Errorf("site %s: exec is required", site.Name)
	}
	if !site.HasNodeRuntime() {
		return resolveBinaryExec(sites.Root(site.Name), execRaw)
	}
	return resolveNodeExec(site.Name, sites.Root(site.Name), execRaw, site.Runtime, nodeBinOverride)
}

// ResolveBackendExec builds ExecStart for one static-site HTTP backend under apis/<id>/.
func ResolveBackendExec(site config.Site, backend config.SiteBackend, nodeBinOverride string) (AppExec, error) {
	if !backend.Enabled {
		return AppExec{}, fmt.Errorf("site %s: backend not enabled", site.Name)
	}
	config.NormalizeBackend(&backend)
	execRaw := strings.TrimSpace(backend.Exec)
	if execRaw == "" {
		return AppExec{}, fmt.Errorf("site %s backend %s: exec is required", site.Name, backend.ID)
	}
	apiRoot := sites.APIs(site.Name, backend.ID)
	if !backend.HasNodeRuntime() {
		return resolveBinaryExec(apiRoot, execRaw)
	}
	return resolveNodeExec(site.Name, apiRoot, execRaw, backend.Runtime, nodeBinOverride)
}

func resolveBinaryExec(workDir, execRaw string) (AppExec, error) {
	fields := strings.Fields(strings.TrimSpace(execRaw))
	if len(fields) == 0 {
		return AppExec{}, fmt.Errorf("empty exec")
	}
	path := fields[0]
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}
	if len(fields) == 1 {
		return AppExec{ExecStart: path}, nil
	}
	return AppExec{ExecStart: path + " " + strings.Join(fields[1:], " ")}, nil
}

func resolveNodeExec(domain, workDir, execRaw string, r *config.SiteRuntime, nodeBinOverride string) (AppExec, error) {
	if r == nil {
		return AppExec{}, fmt.Errorf("site %s: runtime is nil", domain)
	}
	config.NormalizeRuntime(r)

	nodeBin := strings.TrimSpace(nodeBinOverride)
	var binDir string
	if nodeBin == "" {
		if r.Command != "" {
			nodeBin = r.Command
			binDir = filepath.Dir(nodeBin)
		} else {
			full := r.Version
			if !strings.Contains(full, ".") {
				if aliased, err := wbrruntime.ReadMajorAlias(full); err == nil {
					full = aliased
				}
			}
			var err error
			nodeBin, err = wbrruntime.ResolveNodeBinary(r, full)
			if err != nil {
				return AppExec{}, err
			}
			binDir = wbrruntime.BinDir(full)
		}
	} else {
		binDir = filepath.Dir(nodeBin)
	}

	entry := execRaw
	fields := strings.Fields(entry)
	if len(fields) >= 2 && (fields[0] == "node" || strings.HasSuffix(fields[0], "/node")) {
		entry = strings.Join(fields[1:], " ")
	}
	if !filepath.IsAbs(entry) {
		ef := strings.Fields(entry)
		if len(ef) == 0 {
			return AppExec{}, fmt.Errorf("site %s: empty exec", domain)
		}
		ef[0] = filepath.Join(workDir, ef[0])
		entry = strings.Join(ef, " ")
	}
	if st, err := os.Stat(strings.Fields(entry)[0]); err == nil && st.IsDir() {
		return AppExec{}, fmt.Errorf("site %s: exec points at a directory", domain)
	}

	return AppExec{
		ExecStart: nodeBin + " " + entry,
		PathEnv:   binDir,
	}, nil
}
