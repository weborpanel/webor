# Per-site Node runtime (managed or custom path)

**Status:** accepted

## Context

App sites often need Node. System `apt nodejs` is one version for the whole VPS and is awkward for CI-built Linux artifacts. Operators want either a pinned Node major or their own binary path.

## Decision

1. **CI builds artifacts; VPS runs them.** Recommended: GitHub Actions (or similar) produces a Linux amd64/arm64 bundle; Webor deploys the zip/files and starts `node <entry>`. On-box `npm ci` stays optional via `build_command`, not required.
2. **`runtime` on app sites:**
   - `kind: node` + `version: "22"` (major or full semver) → Webor downloads the official Node linux tarball into `/var/lib/webor/runtimes/node/<ver>/` (shared cache).
   - `kind: node` + `command: /abs/path/to/node` → bring-your-own; no download.
   - No `runtime` → current behavior: `exec` is the full ExecStart (Go binary, etc.).
3. **ExecStart:** with Node runtime, `exec` is the script/entry (e.g. `server.js` or `./dist/index.js`). Unit becomes `ExecStart=<node> <abs-entry>` plus `PATH=<node-bin>:…` so optional builds see matching `npm`.
4. **Curated majors in UI:** 20 / 22 / 24 LTS-oriented; exact patch resolved at ensure time and recorded under the runtimes dir.

## Consequences

- Helper: `EnsureNodeRuntime`, site unit re-render uses resolved node path.
- Panel Deploy: runtime picker + short GH Actions note.
- Native addons in uploaded `node_modules` must match Linux + that Node major (document failure mode).
