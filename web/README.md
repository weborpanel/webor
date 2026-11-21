# Webor web UI

SvelteKit static frontend for the panel. Development:

```bash
npm install
npm run dev
npm run check
npm run build
```

Production embedding: `make web` builds into `web/build` and copies into
`internal/ui/dist` for `go:embed`. `make build` / `make lab-start` run this
automatically. Dev server still proxies `/api` to `http://127.0.0.1:8787`.
