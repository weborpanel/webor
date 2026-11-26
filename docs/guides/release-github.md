# Webor releases (GitHub Releases)

Tag builds publish linux **amd64** and **arm64** tarballs to GitHub Releases. Rolling **channel** releases (`beta`, `stable`) hold the mutable `VERSION` + tarballs that `wbr update` and the bootstrap installer poll.

## Public URLs

| What | URL |
|------|-----|
| Bootstrap (canonical) | `https://webor.dev/install` (defaults to **beta** until stable `v0.1.0`) |
| Bootstrap (repo) | `https://raw.githubusercontent.com/weborpanel/webor/main/install/public/install` |
| Bootstrap (channel asset) | `https://github.com/weborpanel/webor/releases/download/beta/install` |
| Beta amd64 | `https://github.com/weborpanel/webor/releases/download/beta/webor-linux-amd64.tar.gz` |
| Stable amd64 | `https://github.com/weborpanel/webor/releases/download/stable/webor-linux-amd64.tar.gz` |
| Checksums | `…/releases/download/<channel>/SHA256SUMS` |
| Version | `…/releases/download/<channel>/VERSION` |

Keep `landing/public/install` and `install/public/install` in sync (default channel + base URL).

## Install / refresh a VPS

Idempotent — re-run to pick up the latest channel build (no S3 hop required):

```bash
curl -fsSL https://webor.dev/install | sudo bash
# lab / no PTR:
curl -fsSL https://webor.dev/install | sudo bash -s -- --skip-ptr
# or force channel + persist base:
curl -fsSL https://webor.dev/install | sudo bash -s -- --channel beta
```

Already installed with an old S3 `WBR_BASE_URL`:

```bash
sudo wbr update apply -channel beta \
  -base-url https://github.com/weborpanel/webor/releases/download \
  -save-channel -force
```

(Or just re-run the bootstrap above; it persists the GitHub base.)

## Publish a release

```bash
# Public beta → rolling channel "beta" + immutable v0.1.0-beta.N
git tag v0.1.0-beta.12
git push origin v0.1.0-beta.12

# Stable → rolling channel "stable"
git tag v0.1.0
git push origin v0.1.0
```

Workflow [`.github/workflows/release.yml`](../../.github/workflows/release.yml) builds UI + both arches, packs, and runs `scripts/publish-github.sh`. Tag names containing `beta` publish to channel **beta** (prerelease); other `v*` tags → **stable**.

`GITHUB_TOKEN` with `contents: write` is enough (public repo). No PAT needed for downloads. AWS secrets are not used.

### Legacy S3

Retired. Historical notes: [release-s3.md](./release-s3.md).
