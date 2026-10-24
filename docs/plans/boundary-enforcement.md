# Boundary enforcement

Grammar: v2. Prefix: BE-.
Scope: the 2026-10-22 owner decisions' boundary controls alongside ADR-080/ADR-081. Existing I9 fail-open and I53 deadlines stand. No table, column, cache, background job, tenancy/governor change, network fetch, budget increase or model spend. Source pointers are checked at HEAD before implementation; historical audit counts are not assumed current.

- [x] Conflict-marker and backup-file check in pre-commit and validate; prevent recurrence of the ADR.md conflict marker removed in the docs correction. {id: BE-1, oracle: lexical, path: scripts/check-merge-debris.sh, pattern: "(?s)git diff --cached --name-only --diff-filter=ACMR -z.*backup or reject file.*LC_ALL=C (git )?grep "}
- [x] Commit-msg hook requires a conventional subject and Verified: line, forbids Co-Authored-By and Claude-Session trailers, exempts merges/reverts, and checks the same pushed range in CI. {id: BE-2, oracle: lexical, path: scripts/commit-msg-check.sh, pattern: "(?s)subject must use a conventional commit type.*forbidden provenance trailer.*body requires a nonempty Verified: line"}
- [ ] Agent clones install hooks through a documented setup task. {id: BE-3}
- [x] Fail-open lines print their first error line and the conserve operational skip is root-caused: a branch whose configured upstream has no tracking ref made `git rev-parse ... @{u}` print the literal `@{u}`, so conserve ran with `--since @{u}`, failed and fell open in every scratch-clone push gate; the fallback now follows git's exit status. {id: BE-4, oracle: lexical, path: scripts/boundary-hooks_test.sh, pattern: "conserve-literal-upstream"}
- [x] No silent skips: missing bin/fathom builds it under the gate lease (a failed build fails verify with its first real error line; advisory steps name the skip), and missing cue fails when CI=true. {id: BE-5, oracle: lexical, path: scripts/boundary-hooks_test.sh, pattern: "cue-missing-ci"}
- [ ] CI runs verify --strict --since over the pushed range. {id: BE-6}
- [x] CI cancels in-progress runs only off main so every main commit can receive its Linux verdict. {id: BE-7, oracle: lexical, path: .github/workflows/ci.yml, pattern: "(?s)group: ci-.*github.ref == 'refs/heads/main' && github.sha.*cancel-in-progress:.*github.ref != 'refs/heads/main'"}
- [x] Command guards match per shell segment in the edit hook and always try whole-string repository/user globs; advisory matches never hide enforced guards. The embedded git-safety pack denies git push/commit --no-verify (commit -n too), push/commit with LEFTHOOK disabled (0/false/no/off, any case) or -c core.hooksPath overrides, forced updates or deletions of remote main/master, and force or mirror pushes without an explicit target branch, each with a remedy. Unknown targets, unreadable safety input, alias configuration and --config-env warn, never deny on a guess; pack notes follow repository/user notes. The pack follows enforceEnabled/FATHOM_ENFORCE at the nearest project guards.enforce setting and supports [guards] git_safety = false. Git push -n stays dry-run and allowed; local git operations stay allowed. {id: BE-8, symbol: MatchCommandGuard, package: internal/guardspec, reachable_from: evaluateEnforcedCommand}
- [ ] Add repo-local .fathom/guard.cue guards for launchctl kickstart/bootout, ingest of a shared store and git add -A in an owner checkout, and deny agent writes to the receipt directory (LE-2). These repository-specific guards are separate from the shipped git-safety pack. {id: BE-8b}
- [ ] Turn gate_preflight on in this repository after observe support and honest-unknown behavior. {id: BE-9}
- [ ] Tasks t, fmt:changed, verify:plan, ci:watch, box and release wrap the repeated operator intents with the existing environment, gate and safety contracts. {id: BE-10}
- [-] (op) Await the owner's decision on requiring Linux CI on main with a fast local pre-push tier. {id: BE-11, disposition: deferred, reason: "gate-tier policy awaits the owner decision", trigger: "owner decides"}
- [-] (op) Await the owner's decision on GitHub secret scanning, push protection and a main ruleset. {id: BE-12, disposition: deferred, reason: "hosting protection policy awaits the owner decision", trigger: "owner decides"}

Dependencies: BE-9 follows `advice-delivery.md` AD-16 and AD-17 and participates in AD-19's pre-registered trial. BE-4 preserves fail-open semantics while making operational failure visible; it must investigate, not assume, the conserve root cause. BE-11 and BE-12 record pending owner decisions, not permission to change gate tiers, hosting settings or branch policy. BE-10's tasks retain existing budget and gate requirements. Add product symbol contracts when implementing and verifying rows; implementation boxes stay open for the controller; BE-11 and BE-12 carry native deferred dispositions.

Verify before claiming any row: `fathom verify --service fathom --plan docs/plans/boundary-enforcement.md`.

## Decision models

None applies: marker/id/message/command checks and gate wiring are deterministic with provider none. BE-11 and BE-12 require an owner decision, not a model judgment. No model spend is authorized.
