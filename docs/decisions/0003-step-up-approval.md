# Spike C: Step-up approval

**Status:** accepted

## Context

Dangerous helper operations (package installs, firewall changes, site data
deletion) must require step-up approval so a compromised panel cannot approve
its own requests. Approvals should be single-use, short-lived, and bound to the
exact operation and arguments.

## Decision

1. Approvals live in the helper’s SQLite state DB (`approvals` table), not in
   the panel process memory.
2. Flow:
   - Panel/helper `RequestApproval` creates a pending row bound to
     `operation` + canonical JSON args (TTL default 5 minutes).
   - Console `wbr approve <id>` runs as **root** and calls helper `Approve`,
     which accepts only `SO_PEERCRED` uid 0.
   - The dangerous operation includes `approval_id`; the helper
     `Consume`s the row (must be approved, unexpired, unused, args match),
     then performs the work.
3. Spike stand-in op: `DangerousDemo` (noop after a successful consume).
4. The web UI must never be able to call `Approve`; only console/`wbr` as root.

## Consequences

- Milestone 3 wires `DeleteSiteUser` (with `delete_data`) to the same consume
  path; package installs / firewall remain future approval-class ops.
- Audit log records request / approve / consume (hash-chained `audit_events`).
- TOTP step-up can sit alongside console approve later; console remains the
  break-glass path if the panel is fully compromised.
- Runbook: [`docs/spikes/step-up-approval.md`](../spikes/step-up-approval.md),
  install notes: [`docs/spikes/m3-install.md`](../spikes/m3-install.md).
