# Spike C — step-up approval (Debian VM)

## Build

```bash
cd /media/sf_webor
export GOTOOLCHAIN=auto
go build -o /tmp/webor-helper ./cmd/webor-helper
go build -o /tmp/wbr ./cmd/wbr
go build -o /tmp/spike-helper ./cmd/spike-helper
sudo mkdir -p /run/webor /var/lib/webor
```

## Start helper

```bash
sudo systemd-run --unit=webor-helper-spike --collect \
  /tmp/webor-helper -socket /run/webor/helper.sock -state-dir /var/lib/webor
sudo chmod 666 /run/webor/helper.sock   # lab only
```

## Happy path

```bash
# as webor — request approval bound to DangerousDemo + domain
/tmp/wbr request-approval -socket /run/webor/helper.sock -domain example.com
# note approval_id=... from message

# without approve, dangerous op must fail
/tmp/spike-helper -socket /run/webor/helper.sock -op DangerousDemo \
  -domain example.com -approval-id <id>
# expect error: approval not console-approved

# as root — list + console approve
sudo /tmp/wbr approve -socket /run/webor/helper.sock list
sudo /tmp/wbr approve -socket /run/webor/helper.sock <id>

# as webor — succeed once
/tmp/spike-helper -socket /run/webor/helper.sock -op DangerousDemo \
  -domain example.com -approval-id <id>

# second use must fail (single-use)
/tmp/spike-helper -socket /run/webor/helper.sock -op DangerousDemo \
  -domain example.com -approval-id <id>
```

Production delete-with-data uses the same flow with `-op DeleteSiteUser -delete-data`
(see `docs/spikes/m3-install.md`).

## Cleanup

```bash
sudo systemctl stop webor-helper-spike
```
