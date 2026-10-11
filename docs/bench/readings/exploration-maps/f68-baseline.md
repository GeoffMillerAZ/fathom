# XM-M10: how often one upstream repository is researched (F68 baseline)

XM-M10 verdict: NOT MET

Plain reading: in the last 30 days of local transcripts no upstream repository was researched in 3 or more sessions across 2 or more repositories. 228 upstreams were seen, each in exactly one session and one repository, so F68's Promote-when has not fired and a `reference` map kind (XM-D10) stays unbuilt. The window is also thin evidence for the other direction: 1 of 4662 sessions touched an upstream at all, because almost every agent runs inside one long-lived orchestrator session in one repository. Re-read when a second repository carries real agent traffic; the trigger needs repositories, which this window cannot show.

Reading of the row in docs/plans/exploration-maps.md (XM-M10) against the Promote-when in docs/intent/futures/f68-f73.md (F68). Scan date 2026-10-11, window 30 days, script `docs/bench/readings/exploration-maps/f68-scan.py` run read-only over the local Claude Code transcript root. $0, no network, no model. Aggregate counts only: no upstream name, URL, path, session id or prompt is committed (ADR-011; a private or client repository can appear in a transcript).

## Method

- Count: Bash `git clone` and `gh repo clone` commands, and WebFetch calls to github.com repository URLs. A clone from a local path is counted apart and is not an upstream. A github.com path that is not a repository (orgs, settings, search and the like) is dropped.
- Group: by normalized upstream host/owner/repo, per session and per repository.
- Session: a top-level transcript with its sub-agent and workflow transcripts folded in. Its repository is its working repository, folded to its origin: a clone or worktree counts as the repository it was cloned from (XM-37's fold), so dozens of worker clones are one repository. A working directory that no longer exists folds through the mangled project directory name to the repository whose path mangles to it, else it is `unresolved`.
- A clone or fetch of the session's own repository is not research and is not counted. A second view also drops any upstream that is some session's own repository.
- Trigger (F68 Promote-when): one upstream in at least 3 sessions within 30 days, across at least 2 repositories.
- A variant treats every transcript file (each sub-agent) as its own unit, to see whether the single-session structure hides repeats.

## Counts

| Quantity | Value |
|---|---|
| Sessions scanned (30 days) | 4662 |
| Transcript files scanned | 6625 |
| Repositories seen (folded) | 9 |
| Sessions whose repository is unresolved (working directory gone, no mangled match) | 606 |
| Clone commands | 1384 (of which from a local path: 676) |
| WebFetch calls to a github.com repository | 89 |

| By session | Value |
|---|---|
| Sessions with at least one upstream hit | 1 |
| Upstreams seen | 228 |
| Sessions per upstream | 1: 228 |
| Repositories per upstream | 1: 228 |
| Upstreams meeting the trigger | 0 |
| Same, dropping any upstream that is a session's own repository: upstreams seen / meeting | 223 / 0 |

| By transcript file (each agent a unit) | Value |
|---|---|
| Units with at least one upstream hit | 168 of 6625 |
| Upstreams seen | 228 |
| Agents per upstream | 1: 175, 2: 28, 3: 7, 11+: 3, 4-5: 10, 6-10: 5 |
| Repositories per upstream | 1: 228 |
| Upstreams meeting the trigger | 0 |

## What this does and does not say

- NOT MET holds in both views. The by-file view shows repeats (some upstreams read by 3 or more agents) but every one stays inside a single repository, so the repository leg of the trigger is what fails.
- Repeats inside one orchestrator session are what a map handoff (XM-1) already serves; F68 asks for the cross-repository case, which this window has almost no data for.
- The window can be re-run with the same script: `python3 -I docs/bench/readings/exploration-maps/f68-scan.py <transcript root> --days 30`. Upstream names are available only through its `--names-out` file, which stays out of the tree.
- Drop if (F68): the baseline is not met by 2027-01-31.
