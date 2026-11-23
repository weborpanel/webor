# Webor releases (public S3)

Tag builds publish linux **amd64** and **arm64** tarballs + the bootstrap installer to a public S3 bucket. Installers no longer use `2bts.in`.

## Public URLs

| What | URL |
|------|-----|
| Bootstrap | `https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install` |
| Stable amd64 | `…/webor/downloads/stable/webor-linux-amd64.tar.gz` |
| Stable arm64 | `…/webor/downloads/stable/webor-linux-arm64.tar.gz` |
| Checksums | `…/webor/downloads/stable/SHA256SUMS` |

Bucket: `webor-releases-360386317256` (ap-south-1). Objects under `webor/*` are world-readable via bucket policy.

## Install on a VPS

```bash
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash
# lab / no PTR:
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash -s -- --skip-ptr
```

Override mirror or channel:

```bash
WEBOR_BASE_URL=… WEBOR_CHANNEL=beta curl -fsSL "$WEBOR_BASE_URL/install" | sudo bash
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
git tag v0.1.0
git push origin v0.1.0
```

Workflow [`.github/workflows/release.yml`](../../.github/workflows/release.yml) builds UI + both arches, packs, and syncs to S3 under channel **stable**.

Manual / beta: Actions → **Release** → Run workflow → channel `beta`.

## Local pack + upload

```bash
make web
# native arch:
make build && WEBOR_CHANNEL=beta ./scripts/pack-public.sh
# or cross:
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/webor ./cmd/webor
# … helper + wbr, then:
WEBOR_ARCH=arm64 WEBOR_CHANNEL=beta ./scripts/pack-public.sh
./scripts/upload-public.sh
```

## OIDC (later)

Replace the access-key step in `release.yml` with `role-to-assume` via `aws-actions/configure-aws-credentials` and a GitHub OIDC provider trust on the IAM role. Secrets `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` can then be removed.
