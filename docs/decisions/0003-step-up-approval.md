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
   - Console `wbz approve <id>` runs as **root** and calls helper `Approve`,
     which accepts only `SO_PEERCRED` uid 0.
   - The dangerous operation includes `approval_id`; the helper
     `Consume`s the row (must be approved, unexpired, unused, args match),
     then performs the work.
3. Spike stand-in op: `DangerousDemo` (noop after a successful consume).
4. The web UI must never be able to call `Approve`; only console/`wbz` as root.

## Consequences

- Milestone 3 wires real approval-class ops (install packages, delete site
  data, firewall) to the same consume path.
- Audit log should record request / approve / consume (follow-up).
- TOTP step-up can sit alongside console approve later; console remains the
  break-glass path if the panel is fully compromised.
- Runbook: [`docs/spikes/step-up-approval.md`](../spikes/step-up-approval.md).
