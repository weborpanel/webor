# VPS rehearsal checklist (native, pre-Podman)

Use on a fresh 1 GB+ VPS with a public IP and DNS you control.

Lab VM (Debian VirtualBox) can cover items marked **lab** without public DNS/HTTPS.
Live HTTPS and public DNS have been verified on real VPS/EC2 hosts (Debian + Alma; two providers + EC2).

1. [x] **lab** `install.sh` completes; setup token saved; admin account created
2. [x] **lab** Panel reachable; `wbr doctor` OK
3. [x] **lab** Add static site; upload / Files tab; HTTP works via IP/`Host` (`:8080`)
4. [x] DNS A → VPS IP; checklist green; **Enable HTTPS**; cert issues (Debian + Alma; two VPS providers + EC2)
5. [ ] Add app site; deploy sample; socket wake works *(lab: site create OK; full deploy smoke still via `make lab-deploy`)*
6. [x] **lab** Attach sqlite + postgres (approve Ensure); `/db/` lists both
7. [x] **lab** Cache: Redis or Valkey running; Attach in panel (install as root if missing — no approval install)
8. [x] **lab** Add subdomain; inherits DB/cache UI
9. [x] Delete parent with confirm + approve; children + DBs + files gone
10. [x] **lab** `systemctl restart webor webor-helper`; sites still serve via Caddy
11. [x] Build with tiny `MemoryMax` under cgroup pressure on 1 GB VPS; OOM kill leaves other sites up *(2026-11-23 Debian t3.micro `i-0617e7d3f82277f73`)*

Also verified on lab (2026-11-21): panel UI dashboard + metrics sampler, per-site Files manager (zip/tar extract into `public/` only; SQLite stays in `data/`).

**1 GB footprint (2026-11-23):** Debian 12 `t3.micro` (`webor-1gb-test`, `i-0617e7d3f82277f73`, stopped after run). Idle RSS after install + setup: webor 17 MB + webor-helper 13 MB + caddy 35 MB ≈ **65 MB** (under ~150 MB target). Panel restart under load on Alma `i-024894af6b68fe0af`: 40/40 HTTPS requests to `fakerequest.com` succeeded while `webor`/`webor-helper` restarted.

Podman/containers are **out of scope** until this native path is solid.
