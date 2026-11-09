# Spike A: Caddy admin API

**Status:** pending

## Context

Webooza must drive TLS and routing through Caddy without dropping connections on
reload. The panel needs validate-then-reload behavior and a last-good config
rollback if a bad config is applied.

## Decision

_Pending spike._ Evaluate driving Caddy through its JSON admin API (separate
managed process) with validate-before-reload and automatic rollback to the
previous working config.

## Consequences

_To be filled after the spike._ Exit criterion for Milestone 0: written decision
and a static site served by Caddy with a staging certificate in the VM.
