# Security policy

## Supported versions

Fathom is beta. Only the newest release receives security fixes. Older tags do not.

## Report a vulnerability

Use GitHub private vulnerability reporting: the repository's **Security** tab, then
**Report a vulnerability**. Do not open a public issue or pull request for a vulnerability.

Include the version (`fathom version`), what you did, what you expected and what happened.
Expect an acknowledgement within a few days. There is no bounty.

## In scope

- The `fathom` binary and its daemon (`serve-http`, `serve-mcp`): admission checks, bind
  address, anything that lets a non-local peer or a hostile repository reach the daemon.
- Unintended outbound network calls, or data leaving the machine without an opt-in.
- A hostile repository that makes fathom write outside its own store, rewrite the repository,
  or run code on ingest.
- Release provenance: archives, `checksums.txt`, the release workflow.
- The Claude Code plugin and the hooks `fathom init` writes.

## Out of scope

- Another process running as the same user reading the store or calling the loopback daemon.
  Fathom is a local, single-user tool and has no authentication yet. See
  [docs/security.md](docs/security.md).
- The egress guard being bypassed by a script, encoded payload or `python -c`. It is a
  tripwire, not data-loss prevention.
- Vulnerabilities in a dependency, an agent harness or a model provider with no fathom-specific
  exposure. Report those upstream.

The threat model, every egress path and a reviewer checklist: [docs/security.md](docs/security.md).
