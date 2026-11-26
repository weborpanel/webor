# Webor releases (legacy S3) — retired

**Status:** Historical. Distribution moved to [GitHub Releases](./release-github.md) after `v0.1.0-beta.10` (bridge) / `v0.1.0-beta.11` (GH-only).

Former public mirror:

| What | URL |
|------|-----|
| Bootstrap | `https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install` |
| Channel assets | `…/webor/downloads/<beta\|stable>/VERSION` (+ tarballs, `SHA256SUMS`) |

Bucket `webor-releases-360386317256` (ap-south-1) was used only for CI publish via `scripts/upload-public.sh`. After soak, empty/delete the bucket and remove repo secrets `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`.

Hosts still on the S3 `WBR_BASE_URL` should run:

```bash
sudo wbr update apply -channel beta -force
# or manually:
sudo wbr update apply -channel beta \
  -base-url https://github.com/weborpanel/webor/releases/download \
  -save-channel -force
```
