# Contributing

Thanks for interest in Webor. The product is in **public beta**; please keep
changes focused and testable.

## Development

Requires Go matching `go.mod` and Node 20+.

```bash
make test              # Go unit + isolation tests
make web && make build # embed UI, then binaries
```

Lab (Debian VM with systemd):

```bash
make lab-start   # panel :8787, sites :8080
make lab-stop
```

Production-shaped local install from a checkout:

```bash
sudo ./install/install.sh --yes --skip-ptr --bin-dir ./bin
```

## Pull requests

- Prefer small PRs with a clear problem statement.
- Run `make test` (and `make web` if you touch `web/`) before opening a PR.
- Match existing Go and Svelte patterns; avoid drive-by refactors.
- Do not commit secrets, credentials, or machine-specific lab paths.

## Security

Report vulnerabilities privately via [SECURITY.md](SECURITY.md) — do not open a
public issue for security-sensitive findings.

## Docs

Operator docs live under `docs/`. The public site at [webor.dev](https://webor.dev)
mirrors install and day-2 guides; keep in-repo docs authoritative for the panel.
