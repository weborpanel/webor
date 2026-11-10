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
cd /media/sf_webooza
export GOTOOLCHAIN=auto
sudo mkdir -p /usr/local/lib/webooza-spike /run/webooza-spike
go build -o /tmp/spike-socket-go ./cmd/spike-socket-go
go build -o /tmp/spike-socket-backend ./cmd/spike-socket-backend
sudo install -m 0755 /tmp/spike-socket-go /usr/local/lib/webooza-spike/spike-socket-go
sudo install -m 0755 /tmp/spike-socket-backend /usr/local/lib/webooza-spike/spike-socket-backend
sudo mkdir -p /usr/local/lib/webooza-spike/node-app
sudo cp examples/socket-activation/node-app/server.js /usr/local/lib/webooza-spike/node-app/
sudo cp examples/socket-activation/units/*.service examples/socket-activation/units/*.socket /etc/systemd/system/
# Fix proxyd path if needed:
PROXYD=$(command -v systemd-socket-proxyd || ls /lib/systemd/systemd-socket-proxyd /usr/lib/systemd/systemd-socket-proxyd 2>/dev/null | head -1)
sudo sed -i "s|/lib/systemd/systemd-socket-proxyd|$PROXYD|" /etc/systemd/system/webooza-spike-proxy.service
sudo systemctl daemon-reload
```

## A) Native Go activation

```bash
sudo systemctl stop webooza-spike-go.service 2>/dev/null || true
sudo systemctl start webooza-spike-go.socket
systemctl is-active webooza-spike-go.service   # expect: inactive
curl --unix-socket /run/webooza-spike/go.sock http://localhost/
# expect: webooza-spike-b-go ok ...
systemctl is-active webooza-spike-go.service   # expect: active
```

## B) Native Node activation

```bash
sudo systemctl stop webooza-spike-node.service 2>/dev/null || true
sudo systemctl start webooza-spike-node.socket
systemctl is-active webooza-spike-node.service
curl --unix-socket /run/webooza-spike/node.sock http://localhost/
systemctl is-active webooza-spike-node.service
```

## C) systemd-socket-proxyd fallback

```bash
sudo systemctl enable --now webooza-spike-backend.service
sudo systemctl stop webooza-spike-proxy.service 2>/dev/null || true
sudo systemctl start webooza-spike-proxy.socket
curl --unix-socket /run/webooza-spike/proxy.sock http://localhost/
# expect: webooza-spike-b-proxy-backend ok ...
```

## Connection warmup check

With the Go socket active and service stopped:

```bash
sudo systemctl stop webooza-spike-go.service
START=$(date +%s%N)
curl -sf --unix-socket /run/webooza-spike/go.sock http://localhost/
END=$(date +%s%N)
echo "warmup_ns=$((END-START))"
# First request must succeed (systemd starts the service).
```

## Cleanup

```bash
sudo systemctl stop \
  webooza-spike-go.socket webooza-spike-go.service \
  webooza-spike-node.socket webooza-spike-node.service \
  webooza-spike-proxy.socket webooza-spike-proxy.service \
  webooza-spike-backend.service
```
