# ADR-005: Local Agent Architecture

## Status
Accepted

## Context

Operations that require access to local files, SSH keys, or private networks
cannot safely execute in the cloud. The Local Agent enables these operations
while maintaining security.

## Decision

- The **Local Agent** is a separate Go binary installed on the user's computer.
- It authenticates to OmniConnector using **short-lived device credentials**.
- It advertises its **supported capabilities** at registration.
- It receives only **authorized tasks** based on explicit capability grants.
- It **rejects** expired, replayed, or unauthorized requests.
- The **Web Dashboard** cannot gain unrestricted access to the user's computer
  simply because a Local Agent is installed.

## Consequences

- Local secrets remain on the user's device by default.
- The Agent must be explicitly granted capabilities; it is not a general-purpose
  remote administration backdoor.
- Device revocation immediately disables all Agent sessions.

## Alternatives Considered

1. **No Local Agent** — Rejected because local-only resources (SSH keys, private
   networks) cannot be accessed from the cloud.
2. **Always-on cloud execution** — Rejected for security and privacy reasons.
3. **Browser-based local access** — Rejected because browsers cannot securely
   access local SSH keys or private networks.