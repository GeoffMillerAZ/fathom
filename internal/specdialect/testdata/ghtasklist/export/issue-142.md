# Retry the payment client (#142)

Exported from a GitHub issue body (design §3): a plain task list, no
dialect-owned id scheme.

## Tasks

- [x] Fix the retry bug in src/payments/client.go
- [x] Add exponential backoff {symbol: Retry, package: src/payments}
- [ ] Document the payment change in src/audit/log.go, see docs/payments.md
- [ ] Follow up with the team
