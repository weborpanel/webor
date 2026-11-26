#!/usr/bin/env bash
# Publish packed release assets to GitHub Releases:
#   1) immutable tag release (vX.Y.Z or vX.Y.Z-beta.N)
#   2) rolling channel release (beta|stable) with clobbered VERSION/tarballs
#
# Expects dist/upload from pack-public.sh (downloads/<channel>/…).
# Usage:
#   WEBOR_CHANNEL=beta WEBOR_VERSION=0.1.0-beta.10 ./scripts/publish-github.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHANNEL="${WEBOR_CHANNEL:?WEBOR_CHANNEL required}"
VER="${WEBOR_VERSION:?WEBOR_VERSION required}"
VER="${VER#v}"
TAG="v${VER}"
SRC="${ROOT}/dist/upload/downloads/${CHANNEL}"
INSTALL_SRC="${ROOT}/dist/upload/install"
REPO="${WEBOR_GH_REPO:-weborpanel/webor}"

case "$CHANNEL" in
  beta|stable) ;;
  *)
    echo "unsupported channel: $CHANNEL" >&2
    exit 1
    ;;
esac

[[ -d "$SRC" ]] || { echo "missing ${SRC} — run pack-public.sh first" >&2; exit 1; }
[[ -f "${SRC}/VERSION" ]] || { echo "missing ${SRC}/VERSION" >&2; exit 1; }
[[ -f "${SRC}/SHA256SUMS" ]] || { echo "missing ${SRC}/SHA256SUMS" >&2; exit 1; }
[[ -f "${SRC}/webor-linux-amd64.tar.gz" ]] || { echo "missing amd64 tarball" >&2; exit 1; }
[[ -f "${SRC}/webor-linux-arm64.tar.gz" ]] || { echo "missing arm64 tarball" >&2; exit 1; }
command -v gh >/dev/null 2>&1 || { echo "gh CLI required" >&2; exit 1; }

notes() {
  cat <<EOF
Webor **${VER}** (${CHANNEL}).

Install / update:
\`\`\`bash
curl -fsSL https://webor.dev/install | sudo bash -s -- --channel ${CHANNEL}
wbr update check
sudo wbr update apply -channel ${CHANNEL}
\`\`\`

Channel assets (rolling):
- https://github.com/${REPO}/releases/download/${CHANNEL}/VERSION
- https://github.com/${REPO}/releases/download/${CHANNEL}/webor-linux-amd64.tar.gz
EOF
}

echo "Publishing ${TAG} + channel ${CHANNEL} to ${REPO}"

# Immutable version release
if ! gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  PRE=()
  if [[ "$CHANNEL" == "beta" || "$VER" == *beta* ]]; then
    PRE=(--prerelease)
  fi
  gh release create "$TAG" \
    --repo "$REPO" \
    --title "Webor ${VER}" \
    --notes "$(notes)" \
    "${PRE[@]}"
fi
gh release upload "$TAG" \
  --repo "$REPO" \
  --clobber \
  "${SRC}/webor-linux-amd64.tar.gz" \
  "${SRC}/webor-linux-arm64.tar.gz" \
  "${SRC}/SHA256SUMS" \
  "${SRC}/VERSION"

# Rolling channel release (fixed download URLs)
if gh release view "$CHANNEL" --repo "$REPO" >/dev/null 2>&1; then
  echo "channel release ${CHANNEL} exists"
else
  CH_PRE=()
  [[ "$CHANNEL" == "beta" ]] && CH_PRE=(--prerelease)
  gh release create "$CHANNEL" \
    --repo "$REPO" \
    --title "Webor ${CHANNEL} channel (rolling)" \
    --notes "Rolling pointer for the **${CHANNEL}** channel. Prefer versioned v* releases for changelogs; clients poll this tag's assets." \
    "${CH_PRE[@]}" \
    --target "$(git -C "$ROOT" rev-parse HEAD)"
fi

UPLOAD=(
  "${SRC}/VERSION"
  "${SRC}/SHA256SUMS"
  "${SRC}/webor-linux-amd64.tar.gz"
  "${SRC}/webor-linux-arm64.tar.gz"
)
[[ -f "$INSTALL_SRC" ]] && UPLOAD+=("$INSTALL_SRC")

gh release upload "$CHANNEL" \
  --repo "$REPO" \
  --clobber \
  "${UPLOAD[@]}"

echo "Done."
echo "  VERSION: https://github.com/${REPO}/releases/download/${CHANNEL}/VERSION"
echo "  amd64:   https://github.com/${REPO}/releases/download/${CHANNEL}/webor-linux-amd64.tar.gz"
