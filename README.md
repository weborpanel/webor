# Webooza

Lightweight, non-Docker hosting panel for a VPS owner who wants to run several
sites and apps: static sites (with build commands), native Node/Go/Rust apps,
and optional rootless containers — with automatic SSL, per-site isolation, and
cron.

The panel being down must never mean the sites are down. Caddy and site units
run independently of the panel process.

**Status:** Milestone 0 foundations (scaffold, config schema, helper IPC stub,
SvelteKit shell). Not production-ready.

## Binaries

| Binary | Role |
| ------ | ---- |
| `webooza` | Panel daemon (API + embedded UI) |
| `webooza-helper` | Root helper (typed IPC allowlist) |
| `wbz` | Admin CLI (approvals, recovery, diagnostics) |

## Paths (production)

| Item | Path |
| ---- | ---- |
| Config | `/etc/webooza/config.yaml` |
| Runtime state | `/var/lib/webooza/` |
| Site data | `/srv/sites/<domain>/` |

## Repository layout

```
cmd/           webooza, webooza-helper, wbz
internal/      config, reconcile, providers, build, dnscheck, auth, audit, ipc
web/           SvelteKit frontend (static export; embedded later)
install/       install script stub and systemd unit templates
brand/         logo SVGs and color tokens
docs/          development plan and spike decisions
test/          isolation and e2e tests (later)
```

## Development

Requires Go 1.23+ and Node 20+.

```bash
go test ./...
cd web && npm install && npm run check && npm run build
```

See [docs/plan.md](docs/plan.md) for the full roadmap.

## License

Apache License 2.0. See [LICENSE](LICENSE).

## Security

See [SECURITY.md](SECURITY.md) for private vulnerability reporting.
