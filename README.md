# Webor

Lightweight, non-Docker hosting panel for a VPS owner who wants to run several
sites and apps: static sites (with build commands), native Node/Go/Rust apps,
and optional rootless containers — with automatic SSL, per-site isolation, and
cron.

The panel being down must never mean the sites are down. Caddy and site units
run independently of the panel process.

**Status:** Public beta. Install via `https://webor.dev/install` (defaults to the
beta channel). Milestone 0–5 lab-complete; Milestone 6 release gate done.
Podman containers deferred. Not a stable 1.0.

## Binaries

| Binary | Role |
| ------ | ---- |
| `webor` | Panel daemon (API + embedded UI) |
| `webor-helper` | Root helper (typed IPC allowlist) |
| `wbr` | Admin CLI (approvals, recovery, diagnostics) |

## Paths (production)

| Item | Path |
| ---- | ---- |
| Config | `/etc/webor/config.yaml` |
| Runtime state | `/var/lib/webor/` |
| Site data | `/srv/sites/<domain>/` |

## Repository layout

```
cmd/           webor, webor-helper, wbr
internal/      config, reconcile, providers, build, dnscheck, auth, audit, ipc, ui
web/           SvelteKit frontend (static export; embedded via make web)
install/       install.sh, install.yaml.example, systemd unit templates
brand/         logo SVGs and color tokens
docs/          development plan and spike decisions
test/          isolation and e2e tests (later)
```

## Development

Requires Go matching `go.mod` and Node 20+.

```bash
make test
make web && make build   # embed UI, then Go binaries
# Lab (Debian VM):
make lab-start           # helper + caddy + panel on :8787 / :8080
# Or production-shaped install:
sudo ./install/install.sh --yes --bin-dir ./bin
```

Operator docs: [docs/install.md](docs/install.md), [docs/dns-https.md](docs/dns-https.md),
[docs/config-reference.md](docs/config-reference.md), [docs/troubleshooting.md](docs/troubleshooting.md),
[docs/threat-model.md](docs/threat-model.md). Index: [docs/README.md](docs/README.md).

Dev: [docs/plan.md](docs/plan.md), [docs/dev/virtualbox-debian13.md](docs/dev/virtualbox-debian13.md),
[docs/dev/lab-ssh-testing.md](docs/dev/lab-ssh-testing.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).

## Security

See [SECURITY.md](SECURITY.md) for private vulnerability reporting (`security@webor.dev`).
