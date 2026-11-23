# Deploy Node apps (CI-built artifacts)

Webor runs `node <entry>` with a managed or custom Node binary (decision 0007). **Build on GitHub Actions** (Linux) so native addons match the VPS; deploy the bundle and start the process.

For **pushing builds to a public S3 bucket** and (planned) **Deploy from URL**, see [deploy-url-s3.md](./deploy-url-s3.md) and [decision 0008](../decisions/0008-deploy-url-s3.md).

Hybrid **static + API** sites: use the site **API** tab (binary or Node) — [decision 0009](../decisions/0009-backend-node-runtime.md).

## Panel setup

1. Create an **app** site.
2. Deploy → **Runtime**: pick Node 20/22/24 (downloaded once into `/var/lib/webor/runtimes/node/`) or set a custom absolute `node` path.
3. Set **Entry** to your script, e.g. `server.js` or `dist/index.js` (not `npm start` unless you intentionally want that).
4. Upload a zip/files of the built app (via Files or your release pipeline), or use git deploy with an optional `build_command`.

## Example GitHub Actions workflow

Build on Linux, then deploy with the official Action ([full guide](./github-actions-deploy.md)). Mint a site token under Site → **CI**.

```yaml
name: build-and-deploy
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
      - run: npm ci
      - run: npm run build
      - run: npm prune --omit=dev
      - uses: weborpanel/deploy-action@v1
        with:
          panel-url: ${{ vars.WEBOR_PANEL_URL }}
          token: ${{ secrets.WEBOR_TOKEN }}
          site: app.example.com
          kind: node
          path: ./
          entry: server.js
          node-version: "22"
```

The Action probes the site first (fails if missing), uploads a zip via `POST …/deploy-zip`, and can set Node runtime + entry. Do not auto-create the site from CI.

## What fails

- `node_modules` built on macOS/Windows with native addons → rebuild on `ubuntu-latest`.
- Node major on the VPS ≠ the major used in CI.