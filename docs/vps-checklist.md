# VPS rehearsal checklist (native, pre-Podman)

Use on a fresh 1 GB+ VPS with a public IP and DNS you control.

Lab VM (Debian VirtualBox) can cover items marked **lab** without public DNS/HTTPS.
Real VPS is still required for Let's Encrypt and public DNS end-to-end.

1. [x] **lab** `install.sh` completes; setup token saved; admin account created
2. [x] **lab** Panel reachable; `wbr doctor` OK
3. [x] **lab** Add static site; upload / Files tab; HTTP works via IP/`Host` (`:8080`)
4. [ ] DNS A → VPS IP; checklist green; **Enable HTTPS**; cert issues *(needs public VPS + domain)*
5. [ ] Add app site; deploy sample; socket wake works *(lab: site create OK; full deploy smoke still via `make lab-deploy`)*
6. [x] **lab** Attach sqlite; postgres engine present (approve Ensure when adding); `/db/` available
7. [x] **lab** Cache: Redis or Valkey running; Attach in panel (install as root if missing — no approval install)
8. [x] **lab** Add subdomain; inherits DB/cache UI
9. [ ] Delete parent with confirm + approve; children + DBs + files gone *(destructive — run deliberately)*
10. [x] **lab** `systemctl restart webor webor-helper`; sites still serve via Caddy
11. [ ] Optional: build with tiny `memory_max` and confirm OOM error text *(needs real cgroup pressure / 1 GB VPS)*

Also verified on lab (2026-11-21): panel UI dashboard + metrics sampler, per-site Files manager (zip/tar extract into `public/` only; SQLite stays in `data/`).

Podman/containers are **out of scope** until this native path is solid.
