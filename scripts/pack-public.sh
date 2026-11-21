#!/usr/bin/env bash
# Build a beta public tarball for temporary hosting (e.g. 2bts.in).
# Usage (from repo root):
#   make web build   # if bins/UI stale
#   ./scripts/pack-public.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHANNEL="${WEBOR_CHANNEL:-beta}"
ARCH="${WEBOR_ARCH:-amd64}"
OUT_DIR="${ROOT}/dist/${CHANNEL}"
STAGE="${OUT_DIR}/webor-linux-${ARCH}"
TAR_NAME="webor-linux-${ARCH}.tar.gz"

need_bin() {
  [[ -x "${ROOT}/bin/$1" ]] || {
    echo "missing bin/$1 — run: make web && make build" >&2
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

# Bootstrap script lives at channel root for easy URL: .../webor/install
mkdir -p "${OUT_DIR}/../upload"
install -m 0755 "${ROOT}/install/public/install" "${ROOT}/dist/upload/install"

(
  cd "$OUT_DIR"
  tar -czf "$TAR_NAME" "webor-linux-${ARCH}"
  sha256sum "$TAR_NAME" > SHA256SUMS
)

# Flat upload kit: copy tarball + sums next to install
cp -f "${OUT_DIR}/${TAR_NAME}" "${ROOT}/dist/upload/${TAR_NAME}"
cp -f "${OUT_DIR}/SHA256SUMS" "${ROOT}/dist/upload/SHA256SUMS"

# Also mirror downloads/beta layout for the bootstrap URLs
mkdir -p "${ROOT}/dist/upload/downloads/${CHANNEL}"
cp -f "${OUT_DIR}/${TAR_NAME}" "${ROOT}/dist/upload/downloads/${CHANNEL}/${TAR_NAME}"
cp -f "${OUT_DIR}/SHA256SUMS" "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS"

cat <<EOF
Packed OK.

Upload the contents of:
  ${ROOT}/dist/upload/

To your host so these URLs work:
  https://2bts.in/webor/install
  https://2bts.in/webor/downloads/${CHANNEL}/${TAR_NAME}
  https://2bts.in/webor/downloads/${CHANNEL}/SHA256SUMS

Suggested remote layout under the webor/ web root:
  install
  downloads/${CHANNEL}/${TAR_NAME}
  downloads/${CHANNEL}/SHA256SUMS

Then on a clean VPS:
  curl -fsSL https://2bts.in/webor/install | sudo bash -s -- --skip-ptr

Files:
EOF
ls -lh "${ROOT}/dist/upload/install" \
  "${ROOT}/dist/upload/downloads/${CHANNEL}/${TAR_NAME}" \
  "${ROOT}/dist/upload/downloads/${CHANNEL}/SHA256SUMS"
