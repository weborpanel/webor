#!/usr/bin/env node
"use strict";

/**
 * Socket-activated Node HTTP sample (Spike B / Milestone 2).
 * Honors LISTEN_FDS (fd 3) and WBR_IDLE_TIMEOUT (Go-style duration) so the
 * service can scale to zero while the .socket unit keeps listening.
 */

const http = require("http");
const fs = require("fs");

function parseIdleTimeout(raw) {
  const s = String(raw || "").trim();
  if (!s) return 0;
  const m = s.match(/^(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)$/);
  if (!m) {
    throw new Error(`WBR_IDLE_TIMEOUT: invalid duration ${JSON.stringify(raw)}`);
  }
  const n = Number(m[1]);
  const unit = m[2];
  const msPer = {
    ns: 1e-6,
    us: 0.001,
    "µs": 0.001,
    ms: 1,
    s: 1000,
    m: 60_000,
    h: 3_600_000,
  };
  const ms = n * msPer[unit];
  if (!(ms >= 1000)) {
    throw new Error("WBR_IDLE_TIMEOUT must be at least 1s");
  }
  return ms;
}

let idleMs;
try {
  idleMs = parseIdleTimeout(process.env.WBR_IDLE_TIMEOUT);
} catch (err) {
  console.error(String(err.message || err));
  process.exit(1);
}

let lastTouch = Date.now();
const touch = () => {
  lastTouch = Date.now();
};

const server = http.createServer((req, res) => {
  touch();
  res.writeHead(200, { "Content-Type": "text/plain" });
  res.end(`webor-spike-b-node ok pid=${process.pid}\n`);
});

function startIdleWatcher() {
  if (!idleMs) return;
  const timer = setInterval(() => {
    const since = Date.now() - lastTouch;
    if (since < idleMs) return;
    clearInterval(timer);
    console.error(
      `idle for ${since}ms (limit ${idleMs}ms); shutting down`,
    );
    server.close(() => process.exit(0));
    // Force exit if close hangs on inherited systemd socket.
    setTimeout(() => process.exit(0), 3000).unref();
  }, 200);
  timer.unref?.();
}

const listenFDs = parseInt(process.env.LISTEN_FDS || "0", 10);
const listenPID = parseInt(process.env.LISTEN_PID || "0", 10);

if (listenFDs > 0 && listenPID === process.pid) {
  // systemd passes the listening socket as FD 3 when Accept=no
  server.listen({ fd: 3 }, () => {
    console.error(
      `listening on activated fd 3 (idle=${idleMs ? idleMs + "ms" : "off"})`,
    );
    startIdleWatcher();
  });
} else {
  const path = process.env.WBR_SOCK || "/tmp/webor-spike-node.sock";
  try {
    fs.unlinkSync(path);
  } catch (_) {
    /* ignore missing */
  }
  server.listen(path, () => {
    console.error(
      `listening on ${path} (idle=${idleMs ? idleMs + "ms" : "off"})`,
    );
    startIdleWatcher();
  });
}
