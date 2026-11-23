# HTTP backend: binary or Node on static sites

**Status:** accepted

## Context

Hybrid static sites need an API process behind path prefixes (`/api` or whole-host on `api.…`). Operators want either a compiled binary or a Node entry (`server.js`) without converting the site to `type: app`. App-site Node runtime remains decision 0007; this decision covers **backend.runtime** only.

## Decision

1. **`backend.runtime`** on enabled static backends mirrors app `SiteRuntime`: managed Node version, BYO absolute `node` path, or omitted for raw binary `ExecStart`.
2. **`backend.exec`** is the binary path (default `./bin/server`) or the Node entry script (default `server.js` when Node is selected).
3. **Ship:** binary upload → existing release install; Node → zip extract into **site root** + unit restart (Files tab still works).
4. Still **static + path proxy** — not a site-type conversion to app.

## Consequences

- `ResolveBackendExec` / `RenderHTTPBackendUnit` emit `node <entry>` + PATH when needed.
- Panel API tab: interpreter picker, adaptive deploy, PORT snippets for binary and Node.
- Deploy-from-URL (0008) remains separate.