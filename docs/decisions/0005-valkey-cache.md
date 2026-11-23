# Shared Valkey cache with per-site ACL

**Status:** accepted

## Context

Native apps need a cache without Docker. Dedicated Valkey per site is heavy for a single VPS; a shared instance with ACL key prefixes matches the Postgres peer-isolation model.

## Decision

1. One shared Valkey (or Redis) instance on the host, installed by an operator as root (`apt install valkey`, or `dnf install redis` on Alma/RHEL). The panel does not package-install Valkey via approvals; Attach fails with install instructions until the service is present and active.
2. Parent sites attach with `cache: valkey`. Children inherit credentials; they cannot declare `cache`.
3. Per-site ACL user = `sites.Username(parent)` with key pattern `username:*`, password rotatable, shown once on create/rotate.
4. Secrets at `state/secrets/<domain>.valkey.json`.
5. Dedicated per-site Valkey instances and a Valkey UI browser are deferred.

## Consequences

- Isolation test: site B cannot read keys under site A’s prefix.
- Panel API: `/api/sites/{name}/cache` (+ rotate).
- Spike notes live in `docs/spikes/m4-databases.md`.
