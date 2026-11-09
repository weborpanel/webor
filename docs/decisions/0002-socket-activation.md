# Spike B: Socket activation

**Status:** pending

## Context

Native apps should accept traffic over Unix sockets owned by the site user.
Socket activation with idle stop (scale to zero) is a target; some apps may not
support systemd socket activation natively.

## Decision

_Pending spike._ Prototype socket activation with a Go app and a Node app,
including `systemd-socket-proxyd` as a fallback, plus connection-warmup
validation for the first request while the app wakes.

## Consequences

_To be filled after the spike._ Informs app unit rendering and always-on mode
in Milestone 2.
