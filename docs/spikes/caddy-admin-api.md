# Spike A — Caddy admin API

## Docker smoke test (host)

Useful while the VirtualBox guest is being set up. Covers validate → load →
probe → last-good rollback only — not systemd users, cgroups, or socket
activation.

```bash
# Docker Desktop / daemon must be running and reachable from WSL
./scripts/spike-caddy-docker.sh
```

Or manually:

```bash
cd docker/spike-caddy && docker compose up -d
curl -sS http://127.0.0.1:8080/   # webor-spike-a-ok
go run ./cmd/spike-caddy -admin http://127.0.0.1:2019 \
  -config docker/spike-caddy/good.json -probe http://127.0.0.1:8080/
go run ./cmd/spike-caddy -admin http://127.0.0.1:2019 \
  -config docker/spike-caddy/bad.json -probe http://127.0.0.1:8080/
docker compose -f docker/spike-caddy/compose.yml down
```

Admin listens on `0.0.0.0:2019` **inside the container only** so the published
port works; production stays on loopback.

---

## Debian 13 VM (full spike)

Assumes the VirtualBox guest has a clean snapshot and network/SSH access.
Run these from a checkout of this repo on the guest (shared folder, `git clone`,
or `scp`).

## 1. Install Caddy and Go

```bash
sudo apt update
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl

# Official Caddy package (or use distro caddy if preferred)
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy golang-go
```

Stop the packaged service if it holds ports:

```bash
sudo systemctl disable --now caddy 2>/dev/null || true
```

## 2. Start Caddy with the good spike config

From the repo root:

```bash
caddy run --config internal/providers/caddy/testdata/good.json
```

Leave that terminal open. In another shell:

```bash
curl -sS http://127.0.0.1:8080/
# expect: webor-spike-a-ok
```

## 3. Apply good config via the harness

```bash
go run ./cmd/spike-caddy \
  -config internal/providers/caddy/testdata/good.json \
  -probe http://127.0.0.1:8080/ \
  -last-good /tmp/webor-caddy-last-good.json
```

Expect: `ok: applied ...`

## 4. Prove bad config does not stick

```bash
go run ./cmd/spike-caddy \
  -config internal/providers/caddy/testdata/bad.json \
  -probe http://127.0.0.1:8080/ \
  -last-good /tmp/webor-caddy-last-good.json
```

Expect: non-zero exit (Caddy load error and/or probe + rollback). Then:

```bash
curl -sS http://127.0.0.1:8080/
# still: webor-spike-a-ok
```

## 5. Record the result

If both steps behave as above, Spike A is verified on the VM. The written
decision is already in `docs/decisions/0001-caddy-admin-api.md`.
