# Deploy from GitHub Actions

Push CI-built artifacts straight to an **existing** Webor site. The Action never creates sites, DNS, or backends.

## One-time setup

1. Create the site in the panel (static or app). For Node apps, set runtime + entry under Deploy (or let the Action set them).
2. Open the site → **CI** → **Create token**. Copy `WEBOR_TOKEN` once into the GitHub repo secret.
3. Set repository variable `WEBOR_PANEL_URL` to the panel URL:
   - Domain: `https://panel.example.com`
   - First-boot IP/port: `https://203.0.113.10:8443` (use `insecure: true` if the cert is self-signed)

## Workflow

```yaml
name: deploy
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      # build your artifact…
      - uses: weborpanel/deploy-action@v1
        with:
          panel-url: ${{ vars.WEBOR_PANEL_URL }}
          token: ${{ secrets.WEBOR_TOKEN }}
          site: example.com
          kind: static
          path: dist
```

## Kinds

| `kind` | Site type | Artifact | Notes |
|--------|-----------|----------|-------|
| `static` | `static` | dir or zip | Extracted into `public/` |
| `node` | `app` | dir or zip | `POST …/deploy-zip`; optional `entry` + `node-version` |
| `binary` | `app` | single executable | Action `chmod +x`; panel stores `0755` |
| `api` | `static` + backend | zip or binary | Requires `backend-id` |

## Failures

- Site missing → `site not found` (create it in the panel first)
- Wrong `kind` for the site type → clear error
- Token for a different site / missing scope → 403

## Related

- [Node CI build tips](./node-ci-deploy.md)
- [Decision 0011](../decisions/0011-deploy-tokens-github-action.md)
- [Deploy from URL / S3 (separate path)](../decisions/0008-deploy-url-s3.md)
