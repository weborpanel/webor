# Webooza web UI

SvelteKit static frontend for the panel. Development:

```bash
npm install
npm run dev
npm run check
npm run build
```

Production embedding into the `webooza` binary comes later via `go:embed` of the
`web/build` output. Until then, serve the static build separately or through the
panel placeholder HTTP server.
