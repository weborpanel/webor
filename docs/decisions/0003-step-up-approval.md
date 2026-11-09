# Spike C: Step-up approval

**Status:** pending

## Context

Dangerous helper operations (package installs, firewall changes, site data
deletion) must require step-up approval so a compromised panel cannot approve
its own requests. Approvals should be single-use, short-lived, and bound to the
exact operation and arguments.

## Decision

_Pending spike._ Prototype console approval through `wbz approve`, with the
helper verifying the approval (not the web process).

## Consequences

_To be filled after the spike._ Shapes the approval table, IPC protocol, and
`wbz` CLI in Milestone 3.
