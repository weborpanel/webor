# Milestone 0 helper ops — VM smoke test

Prereqs: Debian lab from [docs/dev/virtualbox-debian13.md](../dev/virtualbox-debian13.md),
shared folder at `/media/sf_webooza`, Go installed. Spike A Caddy path still useful.

## Build and run the helper (as root)

```bash
cd /media/sf_webooza
sudo mkdir -p /run/webooza /etc/webooza /var/lib/webooza /srv/sites
export GOTOOLCHAIN=auto
go build -o /tmp/webooza-helper ./cmd/webooza-helper
go build -o /tmp/spike-helper ./cmd/spike-helper

# Prefer systemd-run so the process survives the SSH session
sudo systemd-run --unit=webooza-helper-spike --collect \
  /tmp/webooza-helper -socket /run/webooza/helper.sock
sudo chmod 666 /run/webooza/helper.sock   # lab only; production uses peercred + tighter perms
```

Stop later with `sudo systemctl stop webooza-helper-spike`.

## CreateSiteUser

```bash
cd /media/sf_webooza
go run ./cmd/spike-helper -socket /run/webooza/helper.sock \
  -op CreateSiteUser -domain example.com
id wbz_example_com
ls -la /srv/sites/example.com
```

## WriteUnitFile

```bash
go run ./cmd/spike-helper -socket /run/webooza/helper.sock \
  -op WriteUnitFile -domain example.com \
  -unit webooza-example-com.service \
  -content internal/helper/testdata/site.service
systemctl cat webooza-example-com.service
```

## ReloadCaddy

Start Caddy with a known-good config (Spike A), then install that JSON where the
helper is allowed to read it:

```bash
sudo cp internal/providers/caddy/testdata/good.json /etc/webooza/caddy.json
# ensure caddy is running with admin on 127.0.0.1:2019
go run ./cmd/spike-helper -socket /run/webooza/helper.sock \
  -op ReloadCaddy -config /etc/webooza/caddy.json
curl -sS http://127.0.0.1:8080/
```
