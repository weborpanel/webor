# Milestone 0 exit — static site from config

Applies `lab-static.yaml`: create site user + `public/`, render Caddy JSON,
reload via admin API. Lab uses HTTP `:8080` with HTTPS disabled (no DNS/ACME).

## On the Debian VM

```bash
cd /media/sf_webooza
export GOTOOLCHAIN=auto
go build -o /tmp/spike-m0-static ./cmd/spike-m0-static

sudo mkdir -p /etc/webooza /var/lib/webooza /srv/sites /run/webooza-spike

# Start Caddy with any valid JSON first (admin must be up)
sudo cp internal/providers/caddy/testdata/good.json /etc/webooza/caddy.json
sudo systemctl stop caddy 2>/dev/null || true
sudo systemd-run --unit=caddy-m0 --collect \
  /usr/bin/caddy run --config /etc/webooza/caddy.json

sudo /tmp/spike-m0-static \
  -config internal/config/testdata/lab-static.yaml \
  -caddy-json /etc/webooza/caddy.json \
  -disable-https

curl -sS -H 'Host: example.com' http://127.0.0.1:8080/
# expect: webooza-m0-static-ok

curl -sS -H 'Host: www.example.com' http://127.0.0.1:8080/
ls -la /srv/sites/example.com/public/
```

Cleanup: `sudo systemctl stop caddy-m0`.
