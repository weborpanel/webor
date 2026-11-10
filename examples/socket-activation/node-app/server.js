#!/usr/bin/env node
"use strict";

const http = require("http");
const fs = require("fs");

const server = http.createServer((req, res) => {
  res.writeHead(200, { "Content-Type": "text/plain" });
  res.end(`webooza-spike-b-node ok pid=${process.pid}\n`);
});

const listenFDs = parseInt(process.env.LISTEN_FDS || "0", 10);
const listenPID = parseInt(process.env.LISTEN_PID || "0", 10);

if (listenFDs > 0 && listenPID === process.pid) {
  // systemd passes the listening socket as FD 3 when Accept=no
  server.listen({ fd: 3 }, () => {
    console.error("listening on activated fd 3");
  });
} else {
  const path = process.env.WBZ_SOCK || "/tmp/webooza-spike-node.sock";
  try {
    fs.unlinkSync(path);
  } catch (_) {}
  server.listen(path, () => {
    console.error(`listening on ${path}`);
  });
}
