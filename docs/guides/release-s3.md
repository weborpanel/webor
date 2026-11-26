# Webor releases (legacy S3) — retired

**Status:** Historical. Distribution is [GitHub Releases](./release-github.md). There is no S3→GH bridge hop — re-run idempotent install/update against GitHub.

Former public mirror (do not rely on these staying online):

| What | URL |
|------|-----|
| Bootstrap | `https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install` |
| Channel assets | `…/webor/downloads/<beta\|stable>/VERSION` (+ tarballs, `SHA256SUMS`) |

Empty/delete bucket `webor-releases-360386317256` when ready, and remove repo secrets `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`.

```bash
curl -fsSL https://webor.dev/install | sudo bash -s -- --channel beta
# or:
sudo wbr update apply -channel beta \
  -base-url https://github.com/weborpanel/webor/releases/download \
  -save-channel -force
```
