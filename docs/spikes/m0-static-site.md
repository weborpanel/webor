# Milestone 0 exit — static site from config

Applies `lab-static.yaml`: create site user + `public/`, render Caddy JSON,
reload via admin API. Lab uses HTTP `:8080` with HTTPS disabled (no DNS/ACME).

## On the Debian VM

```bash
cd /media/sf_webor
export GOTOOLCHAIN=auto
go build -o /tmp/spike-m0-static ./cmd/spike-m0-static

sudo mkdir -p /etc/webor /var/lib/webor /srv/sites /run/webor-spike

# Start Caddy with any valid JSON first (admin must be up)
sudo cp internal/providers/caddy/testdata/good.json /etc/webor/caddy.json
sudo systemctl stop caddy 2>/dev/null || true
sudo systemd-run --unit=caddy-m0 --collect \
  /usr/bin/caddy run --config /etc/webor/caddy.json

sudo /tmp/spike-m0-static \
  -config internal/config/testdata/lab-static.yaml \
  -caddy-json /etc/webor/caddy.json \
  -disable-https

curl -sS -H 'Host: example.com' http://127.0.0.1:8080/
# expect: data-webor-seed="static-v2" and the site domain as brand

curl -sS -H 'Host: www.example.com' http://127.0.0.1:8080/
ls -la /srv/sites/example.com/public/
```

Cleanup: `sudo systemctl stop caddy-m0`.
