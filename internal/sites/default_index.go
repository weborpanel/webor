package sites

import (
	"fmt"
	"html"
	"strings"
)

// DefaultIndexHTML is the placeholder page written when a new static site's
// public/ directory has no index.html yet.
func DefaultIndexHTML(domain string) string {
	host := strings.ToLower(strings.TrimSpace(domain))
	if host == "" {
		host = "your site"
	}
	safe := html.EscapeString(host)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-webor-seed="static-v2">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%[1]s</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9..144,500;9..144,700&family=Sora:wght@400;500;600&display=swap" rel="stylesheet" />
  <style>
    :root {
      --ink: #0b1c24;
      --ink-2: #123040;
      --mist: #d7ece6;
      --foam: #f3faf7;
      --accent: #e8a87c;
      --accent-soft: color-mix(in srgb, var(--accent) 28%%, transparent);
      --line: color-mix(in srgb, var(--mist) 18%%, transparent);
    }

    * { box-sizing: border-box; }
    html, body { height: 100%%; }
    body {
      margin: 0;
      color: var(--foam);
      font-family: "Sora", sans-serif;
      background:
        radial-gradient(1200px 700px at 12%% -10%%, #1f5a5a 0%%, transparent 55%%),
        radial-gradient(900px 600px at 100%% 0%%, #3a2f1f 0%%, transparent 50%%),
        radial-gradient(800px 500px at 70%% 110%%, #16485a 0%%, transparent 55%%),
        linear-gradient(160deg, var(--ink) 0%%, var(--ink-2) 100%%);
      overflow-x: hidden;
    }

    body::before {
      content: "";
      position: fixed;
      inset: 0;
      pointer-events: none;
      opacity: 0.18;
      background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 0 0.55 0'/></filter><rect width='100%%25' height='100%%25' filter='url(%%23n)'/></svg>");
      mix-blend-mode: soft-light;
    }

    .orb {
      position: fixed;
      width: 28rem;
      height: 28rem;
      border-radius: 50%%;
      background: radial-gradient(circle, var(--accent-soft), transparent 70%%);
      filter: blur(8px);
      top: 18%%;
      right: -8%%;
      animation: drift 14s ease-in-out infinite alternate;
      pointer-events: none;
    }

    main {
      min-height: 100%%;
      display: grid;
      place-items: center;
      padding: clamp(1.5rem, 5vw, 3rem);
      position: relative;
      z-index: 1;
    }

    .stage {
      width: min(42rem, 100%%);
      text-align: left;
      animation: rise 0.9s cubic-bezier(0.22, 1, 0.36, 1) both;
    }

    .brand {
      margin: 0 0 1.25rem;
      font-family: "Fraunces", Georgia, serif;
      font-weight: 700;
      font-size: clamp(2.4rem, 7vw, 4.4rem);
      line-height: 0.95;
      letter-spacing: -0.03em;
      color: var(--foam);
      text-wrap: balance;
    }

    .rule {
      width: 3.5rem;
      height: 2px;
      margin: 0 0 1.35rem;
      border: 0;
      background: linear-gradient(90deg, var(--accent), transparent);
      animation: draw 1.1s 0.2s cubic-bezier(0.22, 1, 0.36, 1) both;
    }

    h1 {
      margin: 0 0 0.7rem;
      font-family: "Sora", sans-serif;
      font-weight: 600;
      font-size: clamp(1.05rem, 2.4vw, 1.25rem);
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: var(--mist);
    }

    p {
      margin: 0;
      max-width: 34rem;
      font-size: clamp(1rem, 2.2vw, 1.15rem);
      line-height: 1.65;
      color: color-mix(in srgb, var(--mist) 82%%, white);
      animation: rise 1s 0.15s cubic-bezier(0.22, 1, 0.36, 1) both;
    }

    footer {
      margin-top: 2.25rem;
      padding-top: 1.1rem;
      border-top: 1px solid var(--line);
      font-size: 0.82rem;
      color: color-mix(in srgb, var(--mist) 55%%, transparent);
      animation: rise 1s 0.28s cubic-bezier(0.22, 1, 0.36, 1) both;
    }

    @keyframes rise {
      from { opacity: 0; transform: translateY(1.1rem); }
      to { opacity: 1; transform: translateY(0); }
    }
    @keyframes draw {
      from { width: 0; opacity: 0; }
      to { width: 3.5rem; opacity: 1; }
    }
    @keyframes drift {
      from { transform: translate3d(0, 0, 0) scale(1); }
      to { transform: translate3d(-2rem, 1.5rem, 0) scale(1.08); }
    }

    @media (prefers-reduced-motion: reduce) {
      .orb, .stage, h1, p, footer, .rule { animation: none !important; }
    }
  </style>
</head>
<body>
  <div class="orb" aria-hidden="true"></div>
  <main>
    <div class="stage">
      <p class="brand">%[1]s</p>
      <hr class="rule" />
      <h1>You're live</h1>
      <p>This is the starter page for your site. Replace it anytime from <strong>Files</strong> in the Webor panel.</p>
      <footer>Served by Webor</footer>
    </div>
  </main>
</body>
</html>
`, safe)
}
