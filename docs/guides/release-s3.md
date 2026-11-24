# Webor releases (public S3)

Tag builds publish linux **amd64** and **arm64** tarballs + the bootstrap installer to a public S3 bucket. Installers no longer use `2bts.in`.

## Public URLs

| What | URL |
|------|-----|
| Bootstrap | `https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install` |
| Stable amd64 | `…/webor/downloads/stable/webor-linux-amd64.tar.gz` |
| Stable arm64 | `…/webor/downloads/stable/webor-linux-arm64.tar.gz` |
| Checksums | `…/webor/downloads/stable/SHA256SUMS` |
| Version | `…/webor/downloads/stable/VERSION` (plain text; used by `wbr update`) |

Bucket: `webor-releases-360386317256` (ap-south-1). Objects under `webor/*` are world-readable via bucket policy.

## Install on a VPS

```bash
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash
# lab / no PTR:
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash -s -- --skip-ptr
# beta channel (flag must be after bash -s — WEBOR_CHANNEL=… before curl does not reach sudo):
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash -s -- --channel beta
```

Override mirror (env must be on the sudo side):

```bash
curl -fsSL "$WEBOR_BASE_URL/install" | sudo env WEBOR_BASE_URL=… WEBOR_CHANNEL=beta bash
# or:
curl -fsSL …/install | sudo bash -s -- --channel beta --base-url …
```

## Publish a release

### AWS (already set up)

| Piece | Value |
|-------|--------|
| Bucket | `webor-releases-360386317256` (ap-south-1) |
| Public read | bucket policy on `webor/*` |
| CI IAM user | `webor-releases-ci` |
| CI policy | inline `webor-releases-s3-publish` — ListBucket on the bucket; Get/Put/Delete on `webor/*` |

### GitHub Actions secrets (until OIDC)

Repo secrets (used by [`.github/workflows/release.yml`](../../.github/workflows/release.yml)):

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`

Access key belongs to `webor-releases-ci` only (not account root). Rotate by creating a new key on that user and updating the secrets.

### Tag and push

```bash
# Public beta → S3 channel "beta"
git tag v0.1.0-beta.1
git push origin v0.1.0-beta.1

# Stable → S3 channel "stable"
git tag v0.1.0
git push origin v0.1.0
```

Workflow [`.github/workflows/release.yml`](../../.github/workflows/release.yml) builds UI + both arches, packs, and syncs to S3. Tag pushes whose name contains `beta` publish to channel **beta**; other `v*` tags publish to **stable**. The tag (leading `v` stripped) is stamped into binaries via `-ldflags -X …/internal/version.Version=…`, which the panel sidebar shows as `Webor <version>`.

Manual override: Actions → **Release** → Run workflow → choose channel.
## Local pack + upload

```bash
make web
# native arch (embeds VERSION from git describe, or VERSION=0.1.0 make build):
make build && WEBOR_CHANNEL=beta ./scripts/pack-public.sh
# or cross with the same ldflags Makefile uses:
VERSION=0.1.0-test.7 make build
WEBOR_ARCH=arm64 WEBOR_CHANNEL=beta ./scripts/pack-public.sh
./scripts/upload-public.sh
```

## OIDC (later)

Replace the access-key step in `release.yml` with `role-to-assume` via `aws-actions/configure-aws-credentials` and a GitHub OIDC provider trust on the IAM role. Secrets `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` can then be removed.
