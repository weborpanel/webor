# Milestone 0 helper ops — VM smoke test

Prereqs: Debian lab from [docs/dev/virtualbox-debian13.md](../dev/virtualbox-debian13.md),
shared folder at `/media/sf_webor`, Go installed. Spike A Caddy path still useful.

## Build and run the helper (as root)

```bash
cd /media/sf_webor
sudo mkdir -p /run/webor /etc/webor /var/lib/webor /srv/sites
export GOTOOLCHAIN=auto
go build -o /tmp/webor-helper ./cmd/webor-helper
go build -o /tmp/spike-helper ./cmd/spike-helper

# Prefer systemd-run so the process survives the SSH session
sudo systemd-run --unit=webor-helper-spike --collect \
  /tmp/webor-helper -socket /run/webor/helper.sock
sudo chmod 666 /run/webor/helper.sock   # lab only; production uses peercred + tighter perms
```

Stop later with `sudo systemctl stop webor-helper-spike`.

## CreateSiteUser

```bash
cd /media/sf_webor
go run ./cmd/spike-helper -socket /run/webor/helper.sock \
  -op CreateSiteUser -domain example.com
id wbr_example_com
ls -la /srv/sites/example.com
```

## WriteUnitFile

```bash
go run ./cmd/spike-helper -socket /run/webor/helper.sock \
  -op WriteUnitFile -domain example.com \
  -unit webor-example-com.service \
  -content internal/helper/testdata/site.service
systemctl cat webor-example-com.service
```

## ReloadCaddy

Start Caddy with a known-good config (Spike A), then install that JSON where the
helper is allowed to read it:

```bash
sudo cp internal/providers/caddy/testdata/good.json /etc/webor/caddy.json
# ensure caddy is running with admin on 127.0.0.1:2019
go run ./cmd/spike-helper -socket /run/webor/helper.sock \
  -op ReloadCaddy -config /etc/webor/caddy.json
curl -sS http://127.0.0.1:8080/
```
