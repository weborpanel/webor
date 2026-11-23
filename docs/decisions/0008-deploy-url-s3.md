# Deploy from public HTTPS URL (CI → S3)

**Status:** proposed

## Context

Node runtime (decision 0007) assumes CI builds Linux artifacts, but operators still hand-copy Actions artifacts or scp onto the VPS. A public S3-compatible object with a stable HTTPS URL is easier than the Actions UI. Webor should pull that URL into a release without SSH.

This is separate from whole-system restic backups (decision 0006): **artifact bucket ≠ backup bucket**.

## Decision

1. **Operator-owned public (or CDN) object storage.** GitHub Actions (or similar) uploads a zip or single binary to BYO S3 / R2 / MinIO. Prefer public `GetObject` on a prefix (or CloudFront); listing stays private. No Webor-hosted artifact CDN.
2. **Panel: Deploy from URL.** App (and backend binary) deploy gains `POST …/deploy/url` with `{ "url": "https://…" }`. Helper downloads over HTTPS, size-capped like multipart upload, then reuses the existing release/rollback path (zip unpack or binary stage).
3. **URL rules:** `https` only; no redirects to non-HTTPS; reject private/link-local destinations (SSRF). Optional later: allowlist host suffixes in config.
4. **CI contract (documented, not enforced):** publish `…-latest.zip` plus immutable `…-$SHA.zip`. Example workflow lives in `docs/guides/deploy-url-s3.md`.
5. **Secrets stay out of the object.** Public artifacts must not contain `.env`, keys, or customer data. Private buckets use presigned URLs (same deploy endpoint; URL already carries auth).

## Consequences

- Deploy UI: URL field alongside binary upload / git.
- Helper: download + existing `Deploy*` release plumbing; SSRF tests required.
- Guide + sample workflow; Node guide (0007) only points here — runtime plan unchanged.
- Out of scope for this decision: Webor calling GitHub Releases API, or writing to the operator’s S3 from the panel.