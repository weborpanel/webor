# Milestone 2 slice 4 — per-site cron timers

Each `cron:` entry becomes a `Type=oneshot` `.service` plus a matching `.timer`
with `OnCalendar=` translated from a 5-field cron schedule.

## Config

```yaml
cron:
  - name: nightly-cleanup
    schedule: 0 3 * * *   # → OnCalendar=*-*-* 03:00:00
    exec: ./bin/cleanup
```

Unit names: `webooza-cron-<domain>-<job>.{service,timer}`.

Helper: `InstallCronJob`, `RunCronNow`, `CronStatus`, `JournalTail`.  
IPC: `RunCronNow`, `CronStatus`.

## On the Debian VM

Prereq: site user exists (e.g. after `make lab-app`).

```bash
cd ~/webooza
git pull
export GOTOOLCHAIN=auto
make lab-cron
```

Expect:

```text
installed webooza-cron-…-lab-tick.timer / …service
status: timer_active=true …
marker: 2026-…
ok: cron install + run-now verified
```

Manual:

```bash
systemctl list-timers 'webooza-cron-*'
sudo systemctl start webooza-cron-app-example-com-lab-tick.service
cat /srv/sites/app.example.com/tmp/cron-ran.txt
```
