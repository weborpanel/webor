# Milestone 2 — long-running jobs

Config:

```yaml
jobs:
  - name: worker
    exec: ./bin/worker
```

Renders `webooza-job-<domain>-<job>.service` (`Type=simple`, `Restart=on-failure`).
Installed on app reconcile via `InstallSiteJobs`.

```bash
make lab-job
```
