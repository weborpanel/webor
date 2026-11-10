# Spike C — step-up approval (Debian VM)

## Build

```bash
cd /media/sf_webooza
export GOTOOLCHAIN=auto
go build -o /tmp/webooza-helper ./cmd/webooza-helper
go build -o /tmp/wbz ./cmd/wbz
go build -o /tmp/spike-helper ./cmd/spike-helper
sudo mkdir -p /run/webooza /var/lib/webooza
```

## Start helper

```bash
sudo systemd-run --unit=webooza-helper-spike --collect \
  /tmp/webooza-helper -socket /run/webooza/helper.sock -state-dir /var/lib/webooza
sudo chmod 666 /run/webooza/helper.sock   # lab only
```

## Happy path

```bash
# as webooza — request approval bound to DangerousDemo + domain
/tmp/wbz request-approval -socket /run/webooza/helper.sock -domain example.com
# note approval_id=... from message

# without approve, dangerous op must fail
/tmp/spike-helper -socket /run/webooza/helper.sock -op DangerousDemo \
  -domain example.com -approval-id <id>
# expect error: approval not console-approved

# as root — console approve
sudo /tmp/wbz approve -socket /run/webooza/helper.sock <id>

# as webooza — succeed once
/tmp/spike-helper -socket /run/webooza/helper.sock -op DangerousDemo \
  -domain example.com -approval-id <id>

# second use must fail (single-use)
/tmp/spike-helper -socket /run/webooza/helper.sock -op DangerousDemo \
  -domain example.com -approval-id <id>
```

## Cleanup

```bash
sudo systemctl stop webooza-helper-spike
```
