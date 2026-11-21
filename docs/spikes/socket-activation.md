# Spike B — socket activation (Debian VM)

Samples live under `examples/socket-activation/` (units + Node) and
`cmd/spike-socket-go`, `cmd/spike-socket-backend`.

## Install tools

```bash
sudo apt-get install -y nodejs
# Go already installed from earlier spikes
which node systemd-socket-proxyd
# Debian paths:
#   /lib/systemd/systemd-socket-proxyd
#   or /usr/lib/systemd/systemd-socket-proxyd
```

## Build and install binaries

```bash
cd /media/sf_webor
export GOTOOLCHAIN=auto
sudo mkdir -p /usr/local/lib/webor-spike /run/webor-spike
go build -o /tmp/spike-socket-go ./cmd/spike-socket-go
go build -o /tmp/spike-socket-backend ./cmd/spike-socket-backend
sudo install -m 0755 /tmp/spike-socket-go /usr/local/lib/webor-spike/spike-socket-go
sudo install -m 0755 /tmp/spike-socket-backend /usr/local/lib/webor-spike/spike-socket-backend
sudo mkdir -p /usr/local/lib/webor-spike/node-app
sudo cp examples/socket-activation/node-app/server.js /usr/local/lib/webor-spike/node-app/
sudo cp examples/socket-activation/units/*.service examples/socket-activation/units/*.socket /etc/systemd/system/
# Fix proxyd path if needed:
PROXYD=$(command -v systemd-socket-proxyd || ls /lib/systemd/systemd-socket-proxyd /usr/lib/systemd/systemd-socket-proxyd 2>/dev/null | head -1)
sudo sed -i "s|/lib/systemd/systemd-socket-proxyd|$PROXYD|" /etc/systemd/system/webor-spike-proxy.service
sudo systemctl daemon-reload
```

## A) Native Go activation

```bash
sudo systemctl stop webor-spike-go.service 2>/dev/null || true
sudo systemctl start webor-spike-go.socket
systemctl is-active webor-spike-go.service   # expect: inactive
curl --unix-socket /run/webor-spike/go.sock http://localhost/
# expect: webor-spike-b-go ok ...
systemctl is-active webor-spike-go.service   # expect: active
```

## B) Native Node activation

```bash
sudo systemctl stop webor-spike-node.service 2>/dev/null || true
sudo systemctl start webor-spike-node.socket
systemctl is-active webor-spike-node.service
curl --unix-socket /run/webor-spike/node.sock http://localhost/
systemctl is-active webor-spike-node.service
```

For the Milestone 2 path (site user, Caddy, idle-stop, deploy/rollback) use
`make lab-node` — see [`m2-app-node.md`](m2-app-node.md). The sample also
honors `WBR_IDLE_TIMEOUT` like the Go binary.

## C) systemd-socket-proxyd fallback

```bash
sudo systemctl enable --now webor-spike-backend.service
sudo systemctl stop webor-spike-proxy.service 2>/dev/null || true
sudo systemctl start webor-spike-proxy.socket
curl --unix-socket /run/webor-spike/proxy.sock http://localhost/
# expect: webor-spike-b-proxy-backend ok ...
```

## Connection warmup check

With the Go socket active and service stopped:

```bash
sudo systemctl stop webor-spike-go.service
START=$(date +%s%N)
curl -sf --unix-socket /run/webor-spike/go.sock http://localhost/
END=$(date +%s%N)
echo "warmup_ns=$((END-START))"
# First request must succeed (systemd starts the service).
```

## Cleanup

```bash
sudo systemctl stop \
  webor-spike-go.socket webor-spike-go.service \
  webor-spike-node.socket webor-spike-node.service \
  webor-spike-proxy.socket webor-spike-proxy.service \
  webor-spike-backend.service
```
