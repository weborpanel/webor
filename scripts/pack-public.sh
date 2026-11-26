#!/usr/bin/env bash
# Build a public release tarball for S3 hosting.
# Usage (from repo root):
#   make web build   # if bins/UI stale
#   ./scripts/pack-public.sh
#   WEBOR_ARCH=arm64 ./scripts/pack-public.sh   # after cross-build into bin/
#
# Multi-arch: pack each arch, then merge SHA256SUMS before upload:
#   ./scripts/pack-public.sh && WEBOR_ARCH=arm64 ./scripts/pack-public.sh
#   ./scripts/merge-public-sums.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHANNEL="${WEBOR_CHANNEL:-stable}"
ARCH="${WEBOR_ARCH:-amd64}"
BASE_URL="${WEBOR_BASE_URL:-https://github.com/weborpanel/webor/releases/download}"
OUT_DIR="${ROOT}/dist/${CHANNEL}"
STAGE="${OUT_DIR}/webor-linux-${ARCH}"
TAR_NAME="webor-linux-${ARCH}.tar.gz"

need_bin() {
  [[ -x "${ROOT}/bin/$1" ]] || {
    echo "missing bin/$1 — run: make web && make build (or cross-build for ARCH=${ARCH})" >&2
    exit 1
  }
}

need_bin webor
need_bin webor-helper
need_bin wbr
[[ -f "${ROOT}/install/install.sh" ]] || { echo "missing install/install.sh" >&2; exit 1; }

rm -rf "$STAGE"
mkdir -p "${STAGE}/bin" "${STAGE}/install"

install -m 0755 "${ROOT}/bin/webor" "${STAGE}/bin/webor"
install -m 0755 "${ROOT}/bin/webor-helper" "${STAGE}/bin/webor-helper"
install -m 0755 "${ROOT}/bin/wbr" "${STAGE}/bin/wbr"

install -m 0755 "${ROOT}/install/install.sh" "${STAGE}/install/install.sh"
install -m 0644 "${ROOT}/install/webor.service" "${STAGE}/install/webor.service"
install -m 0644 "${ROOT}/install/webor-helper.service" "${STAGE}/install/webor-helper.service"
install -m 0644 "${ROOT}/install/webor-caddy.service" "${STAGE}/install/webor-caddy.service"
if [[ -f "${ROOT}/install/install.yaml.example" ]]; then
  install -m 0644 "${ROOT}/install/install.yaml.example" "${STAGE}/install/install.yaml.example"
fi

# Bootstrap script at upload root: .../webor/install
mkdir -p "${ROOT}/dist/upload"
install -m 0755 "${ROOT}/install/public/install" "${ROOT}/dist/upload/install"

(
  cd "$OUT_DIR"
  tar -czf "$TAR_NAME" "webor-linux-${ARCH}"
  sha256sum "$TAR_NAME" > "SHA256SUMS.${ARCH}"
)

mkdir -p "${ROOT}/dist/upload/downloads/${CHANNEL}"
cp -f "${OUT_DIR}/${TAR_NAME}" "${ROOT}/dist/upload/downloads/${CHANNEL}/${TAR_NAME}"
cp -f "${OUT_DIR}/SHA256SUMS.${ARCH}" "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS.${ARCH}"

# Keep a single-arch SHA256SUMS for convenience; merge-public-sums.sh replaces it for multi-arch.
if [[ ! -f "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS" ]]; then
  cp -f "${OUT_DIR}/SHA256SUMS.${ARCH}" "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS"
else
  # Append this arch line if missing
  line="$(cat "${OUT_DIR}/SHA256SUMS.${ARCH}")"
  grep -qF "$TAR_NAME" "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS" \
    || echo "$line" >> "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS"
fi

# Plain-text version for wbr update / panel check (prefer stamped binary, else env, else git describe).
VER="${WEBOR_VERSION:-}"
if [[ -z "$VER" ]]; then
  VER="$("${ROOT}/bin/wbr" version 2>/dev/null || true)"
fi
if [[ -z "$VER" ]]; then
  VER="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)"
  VER="${VER#v}"
fi
printf '%s\n' "$VER" > "${ROOT}/dist/upload/downloads/${CHANNEL}/VERSION"

cat <<EOF
Packed OK (${ARCH} → ${CHANNEL}, version ${VER}).

Upload kit:
  ${ROOT}/dist/upload/

Public URLs (after GitHub publish):
  ${BASE_URL}/${CHANNEL}/${TAR_NAME}
  ${BASE_URL}/${CHANNEL}/SHA256SUMS
  ${BASE_URL}/${CHANNEL}/VERSION

Publish with:
  ./scripts/publish-github.sh

Install (bootstrap defaults to beta):
  curl -fsSL https://webor.dev/install | sudo bash -s -- --skip-ptr

Files:
EOF
ls -lh "${ROOT}/dist/upload/install" \
  "${ROOT}/dist/upload/downloads/${CHANNEL}/${TAR_NAME}" \
  "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS" \
  "${ROOT}/dist/upload/downloads/${CHANNEL}/VERSION"
