# Spike B: Socket activation

**Status:** accepted

## Context

Native apps should accept traffic over Unix sockets owned by the site user.
Socket activation with idle stop (scale to zero) is a target; some apps may not
support systemd socket activation natively (`LISTEN_FDS` / FD 3).

## Decision

1. **Default for first-party and cooperative apps:** systemd `.socket` + `.service`
   with `Accept=no`, `ListenStream=` on a per-site Unix path under the site
   runtime dir. The app inherits the listening FD (`LISTEN_FDS` / `LISTEN_PID`).
2. **Go:** use `LISTEN_FDS` → `net.FileListener` (no extra dependency required).
3. **Node:** `server.listen({ fd: 3 })` when `LISTEN_FDS` is set and
   `LISTEN_PID` matches.
4. **Fallback for apps that only bind a fixed socket/port:** run the app as an
   always-on (or separately managed) unit and put
   `systemd-socket-proxyd` on the public activated socket, forwarding to the
   backend Unix socket or `host:port`.
5. **Always-on mode:** skip the socket unit and start the service directly when
   the site config sets `socket_activation: false` (Milestone 2).
6. **Idle stop:** pursue `StopWhenUnneeded=` / idle timeout in Milestone 2 after
   warmup behavior is covered by tests; Spike B verifies wake-on-first-request.

Caddy continues to reverse-proxy to the **public** site Unix socket (activated
or proxied), never to a raw TCP port on the host by default.

## Consequences

- Unit renderer must emit matched `.socket` / `.service` pairs and choose
  native vs proxyd based on app capability.
- Proxyd does not forward SCM credentials; peer auth at the public socket stops
  at the proxy. Prefer native activation when possible.
- First-request latency includes service start time; health/warmup checks belong
  in deploy flows (Milestone 2).
- Samples and VM runbook:
  [`examples/socket-activation/`](../../examples/socket-activation/),
  [`docs/spikes/socket-activation.md`](../spikes/socket-activation.md).
