# Milestone 2 slice 2 — app binary deploy + rollback

Installs a new release under `/srv/sites/<domain>/releases/<id>/server`, flips
`current` and `bin/server` symlinks, stops the app service so the next request
activates the new binary, probes the site Unix socket, and rolls back to the
previous release if the probe fails.

Helper ops: `DeployAppBinary`, `DeployAppGit` (clone/fetch + limited build + binary deploy).  
IPC: `DeployAppBinary`, `DeployAppGit`.  
Panel API (auth session):

- `POST /api/sites/{name}/deploy` — multipart field `file` (binary → `/var/lib/webor/uploads/{name}/server`)
- `POST /api/sites/{name}/deploy/git` — JSON `{git_url, git_ref?, build_command?, artifact?, memory_max?, timeout_sec?}`

UI: open an **app** site dashboard (`/sites/{name}`) → **Deploy** (Binary upload or Git deploy).
Add app sites from **Sites → Add site → Type: App**.

## On the Debian VM

Prereq: Caddy + panel up (`make lab-start`), then socket-activated sample site.

```bash
cd ~/webor
git pull
export GOTOOLCHAIN=auto
make lab-deploy
```

`lab-deploy` runs `lab-app` first, then `spike-m2-deploy`:

1. Copies `spike-socket-go` into a new release and activates it
2. Attempts a broken binary deploy and expects automatic rollback
3. Confirms `curl` still returns `webor-spike-b-go ok`

Expect:

```text
deployed release …
bad deploy failed as expected: …
curl: webor-spike-b-go ok pid=…
ok: deploy + rollback verified
```

Manual checks:

```bash
ls -l /srv/sites/app.example.com/current /srv/sites/app.example.com/bin/server
ls /srv/sites/app.example.com/releases
curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/
```
