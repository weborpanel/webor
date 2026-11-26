#!/usr/bin/env bash
# LEGACY — S3 publish retired. Prefer scripts/publish-github.sh.
# Sync dist/upload → public S3 prefix (webor/). Kept only for emergency rollback.
# Requires AWS credentials with s3:PutObject on the bucket.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUCKET="${WEBOR_RELEASE_BUCKET:-webor-releases-360386317256}"
PREFIX="${WEBOR_RELEASE_PREFIX:-webor}"
REGION="${WEBOR_RELEASE_REGION:-ap-south-1}"
SRC="${ROOT}/dist/upload"

[[ -d "$SRC" ]] || { echo "missing ${SRC} — run ./scripts/pack-public.sh first" >&2; exit 1; }
[[ -f "${SRC}/install" ]] || { echo "missing ${SRC}/install" >&2; exit 1; }
[[ -d "${SRC}/downloads" ]] || { echo "missing ${SRC}/downloads" >&2; exit 1; }

echo "Syncing ${SRC}/ → s3://${BUCKET}/${PREFIX}/"
aws s3 sync "${SRC}/" "s3://${BUCKET}/${PREFIX}/" \
  --region "$REGION" \
  --cache-control 'public,max-age=60'

aws s3 cp "${SRC}/install" "s3://${BUCKET}/${PREFIX}/install" \
  --region "$REGION" \
  --cache-control 'public,max-age=60' \
  --content-type 'text/x-shellscript'

BASE_URL="${WEBOR_BASE_URL:-https://${BUCKET}.s3.${REGION}.amazonaws.com/${PREFIX}}"
echo "Done. Bootstrap (defaults to beta; canonical URL is webor.dev):"
echo "  curl -fsSL https://webor.dev/install | sudo bash"
echo "  curl -fsSL ${BASE_URL}/install | sudo bash   # S3 mirror"
