# Security Policy

Webor ships a root helper (`webor-helper`) that performs privileged operations
on behalf of the unprivileged panel. Treat security reports as high priority.

## Supported versions

| Version | Supported |
| ------- | --------- |
| main (pre-release) | Yes |
| Public beta and later | Yes, for the latest stable and previous stable |

## Reporting a vulnerability

Please report vulnerabilities privately. Do **not** open a public GitHub issue
for security-sensitive findings.

Email: **security@webor.dev**

(This address is provisional until the project domain is finalized.)

Include:

- A description of the issue and its impact
- Steps to reproduce, or a proof of concept
- Affected version, commit, or binary when known
- Whether you are available for follow-up questions

We aim to acknowledge reports within a few business days and to keep you
informed while we investigate and fix the issue.

## Scope notes

Reports involving the helper allowlist, IPC argument validation, site isolation,
approval bypass, or privilege escalation are especially welcome.
