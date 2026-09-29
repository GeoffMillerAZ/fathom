# Incident maps: committed declarations that pre-shape the first five minutes, reaching outside systems by link and parameter, never by fathom

Status: accepted as a direction 2026-09-29 (ADR-058); the build is deferred behind the stages in section 13. Plan:
`docs/plans/incident-maps.md` (IM-). Futures F55 (incident maps) and F56 (probes and next queries). Amends section
3c of `docs/design/exploration-maps.md` on two points, the kind and the gate, and moves its incident rows XM-26 to
XM-30, XM-M6 and XM-M7 here. Vision: `docs/vision/40-judgment-and-exploration-maps.md` section 13.

Owner direction of 2026-09-29, in substance: incident maps are loved, above all pre-shaping knowledge before an
incident, possibly as a type separate from live maps; they should be able to connect to other data sources and to
CLI commands; analyze now, defer the build, and keep the findings durable. The owner endorsed pre-shaping and the
separate kind. The dispositions on outward connection in section 6 are the analysis's recommendation, recorded with
reopen triggers so they can be revised, not silently lost.

How to read the file paths: all paths are repository-relative, and line numbers are as of commit `0e16a8a1`, the base
this analysis read. The draft was reviewed by an invariants critic and by an SRE and security critic. I read files and
grepped code; I ran no tests and no builds. Section 15 lists what was verified and what was not.

## 0. Decision

**What the owner asked, and the answer.**

| The owner's wish | Decision |
|---|---|
| Pre-shape incident knowledge before the incident | **Yes.** A team commits `.fathom/incidents.cue` beside `.fathom/guard.cue`: symptoms, footprint, window, runbook, links, blind spots. Reviewed in a PR, readable when fathom is down. |
| A type separate from live maps | **Yes, separate, and not a member of the map family either.** The user-facing name "incident map" stays. The artifact is a declaration served through `localize`, the prompt hook and one CLI verb. It may later name a live map as its `scope` (the pattern section 3c already uses for concern packs). |
| Connect to other data sources | **v1: by link and by typed parameter.** Deep links scoped to the incident window (a human clicks, fathom never fetches). A `--deployed <sha>` parameter the agent or human fills from their own deploy tool, which turns the suspect list into the deployed range. A `since_tag` window for release trains. |
| Connect to CLI commands | **Deferred.** Command templates (probes) and MCP next queries are a command runner in effect in the default setup, have no meter, and duplicate runbooks. They wait for an instrument that can see a probe run, a consumer, and the safety contract in section 6.4. |
| Anything that fetches, polls or stores telemetry | **Refused.** Fathom-side connectors (A) and attach (C). Attach has a narrow reopen trigger. |

**The smallest thing that proves value is the suspect list with no declaration at all.** Stage 0 is a zero-build
replay bench with two arms (seed files only versus a blind or mechanical footprint). Stage 1 is a bounded git read
that fills `localize`'s empty recency hop plus one prompt-hook line. Stage 2, the declaration, comes only if stage 1
is used and a named production consumer exists.

**The flow stays read-only and model-free.** A trace, error or alert goes into `localize`, the prompt hook, or the
CLI. Fathom returns owners, what changed (labeled "git-visible changes to this repo only, not cause"), prior
incidents and findings, knobs, reach, the runbook and links. Every facet reads a value, "checked, none", or "not
computed: reason".

**The section 3c gate changes.** XM-M2 measures sub-agent handoff tokens and was added to wave 4 only to work around
plan-grammar defect G2-6. The incident gate becomes IM-M0 (the replay), IM-M2 (the pointer is used) and a named
consumer (P20). RT-M1 applies only to durable state, which the accepted slices do not add.

## 1. How this was decided

The first draft proposed a distinct map kind with probes (B), MCP next queries (D) and attach (C) all accepted. Two
reviews changed it:

- **Back doors.** Attach kept fathom owning the ops-event schema, storage, TTL, dedup and producer reconciliation;
  only the HTTP call moved out. Its optional drop-file import was polling by proxy, and two of its event kinds
  (`alert_firing`, `error_count`) fed no facet. Probes and next queries were a command runner in effect:
  `command_scopes` guards only advise unless `enforce: true` (`docs/design/guarded-edits.md`, the command-scope
  paragraph near line 104), and bypass-permissions mode shows no prompt. Drills, drill receipts, a deploy and flag
  timeline, per-map readiness reports and an incident-tool export together rebuild an incident command center.
- **Kind.** The static-footprint slice uses none of the map model's machinery (no content-hashed entries with why
  lines, no `includes` DAG, no composite ids, no handoff). Calling it a map while escaping F50's gate has it both
  ways, and it would inherit the `map` verb's fate (XM-10).
- **Pull-shaped features die here.** `localize` had 40 calls in about 98k MCP calls; discoveries 13; anchored memory
  0 memories; a `history` commits tool was unregistered at 0 lifetime calls (SR-12, `internal/mcp/extra_tools.go`,
  the closing comment of the extended-tool registration near line 120). A probe sits two steps past the forced
  channel (expand the pointer, pick a probe, run it), and nothing could measure its use, so it could never be
  killed.
- **Operations correctness.** The suspect list was truncated at a 500-commit horizon and ordered by author date;
  clone lag and deploy lag were unmodeled; the CODEOWNERS matcher misroutes owners; the answer carried
  contributor-controlled text; the "floor" when the daemon is down was raw CUE.

The owner's intent survives intact: pre-shaping is the declaration; separateness is stronger than first drafted;
external connection starts with the forms that are safe, portable and meterable.

## 2. What already exists (do not re-decide)

- Section 3c of `docs/design/exploration-maps.md` decides: findings are not maps; a concern's scope is a live map
  (XM-23 to XM-25, F52); `localize` maps frames locally; a typed model may only order the suspect list after
  calibration (XM-D9); there is no command center (XM-X4) and fathom never mitigates (XM-X3). This decision keeps
  all of that and replaces only the incident rows XM-26 to XM-30, XM-M6 and XM-M7 (section 17).
- The 2026-09-29 note on F4 (`docs/intent/futures.md`) had made incident maps the form F4 takes. This decision
  splits them into F55 and returns F4 to precedent over a non-git corpus.
- `docs/intent/incidents.md` carries the planned extension (an incident yields the affected path's map). It stays
  planned and re-points to IM-16.

## 3. Constraints carried in

| Constraint | Consequence here |
|---|---|
| Non-goals (`docs/intent/scope.md`): no domain connectors; no hosted backend; no chat or incident command center; no dispatch loop (revisit never); no productivity score; no net MCP tool growth | A and C refused. No polling, subscription, drop-file import, timeline, drill state or incident status. No new MCP tool (`localize` gains a section and two arguments). No person ranked or keyed. |
| I14 (loopback only; off-host egress only via LLM compile and the decision port) | No accepted part adds an egress path to fathom. The answer still reaches the model provider through the agent, so "no egress" is never claimed unqualified. |
| I56 (never downloads on its own authority) | Links are never fetched, not even to check they resolve. |
| I61, Q5 (loopback data is readable by any local process) | Declarations hold identifiers, globs and links, never secrets. |
| I9, I53, I35 (hooks fail open, stay inside their timeout, inject at most 320 tokens and 6 rules) | The pointer is one line, silent on failure, and the stanza says silence is not evidence. |
| I38, I44 (no conclusion the inputs could not earn; a missing input is a named lower bound) | Every facet reads value, "checked, none", or "not computed: reason". The change facet always names its horizon and its blind spots. |
| I48, I49, I50 (bounded reads; admission by cost class; a read never ingests inline) | The suspect list comes from a bounded git read with a timeout. The git subprocess's cost class is decided at build. The answer names the index commit and never ingests. |
| I57, I58, I59, I60 | Tiers shown, heuristics never gate; no human ranked (no committers as owners, no author field); a typed model advisory and opt-in. |
| P8, P13, P15 (law 15), P16, P17, P18, P20, P23 | No usage kill clock on a declaration; sharing is a human verb; fathom never edits the declaration; delivery by configuration; authored text in CUE, state computed; fail open; a named consumer; a guest on the operator's machine. |
| ADR-056 decision 13 ("output rewriting and command execution by fathom" refused), R1, R3, R5, R6, R7; the no-grep-blocking non-goal (hooks inject, gates block, P16) | Fathom renders nothing executable in v1. No blocking. No durable table before RT-M1. |
| Precision on "never runs commands" | Fathom already execs read-only `git` through `internal/gitro` on request paths (`internal/mcp/meta.go`, `internal/console/join.go`). The rule refuses operator-supplied or arbitrary commands. The deleted `shell_out` was a code analyzer with 0 lifetime calls (SR-19), not a command runner, so it is not the precedent. |

## 4. What practice says (only what carries weight)

- **What changed is the first question.** The Google SRE book reports that roughly 70 percent of outages trace to
  changes in a live system ([introduction](https://sre.google/sre-book/introduction/)). Incident-management products
  lead with change correlation too (product documentation, not re-checked here).
- **Correlation is not causation.** The SRE book's troubleshooting chapter warns that correlated events can share a
  third cause ([effective troubleshooting](https://sre.google/sre-book/effective-troubleshooting/)). The change list
  is a lead, and it is labeled as one.
- **Playbooks rot.** Vendor material says so and so does experience. Rot is the main failure of prepared incident
  knowledge, so readiness must measure the staleness it can compute.

Weak support, kept only as context: executable runbooks (notebooks that run commands) are what fathom refuses, so
they do not support probes. Read-only modes in observability MCP servers exist, but a read-only flag is enforced by
the server at call time and cannot be verified from a committed template; that supports only D, which is deferred.

## 5. Decision 1: a committed declaration, not a map kind

### 5.1 Why separate from live maps

| Property | Live map (F50, XM-D1) | Incident declaration |
|---|---|---|
| Author | Agents and planners, as a working set | A team, ahead of time, reviewed in a PR |
| Home | Thread fields and the memo registry on one machine | `.fathom/incidents.cue`, committed; survives loss of the store |
| Addressed by | Id, purpose, footprint overlap | Symptom (alert name, error literal) and footprint |
| Liveness | Named and used; F50 drops the feature if no map is mounted in 3 runs in 180 days | Rare use is the success state (P8); liveness is readiness (section 9); the feature-level kill counts incidents, not days |
| Latency | No contract | Must answer under pressure (section 8.3) |
| Delivery | Handoff, edit footprint, session brief | A pasted trace or alert; `localize`; the CLI |

### 5.2 Why not a member of the map family

- The first slice uses none of the map model: no content-hashed entries with why lines, no `includes` DAG, no
  composite ids, no handoff. Its globs and contract keys are guard-style scopes.
- Membership would inherit F50's gate (no live map before XM-M2 and RT-M1 read PASS) and XM-10's `map` verb, which
  exists only if maps are promoted. Escaping that gate while claiming the family is having it both ways.
- The bridge stays open for free: an optional `scope: "<live map id>"` field once XM-D1 ships (IM-D4), the same
  shape F52 gives concern packs.

### 5.3 Fields (v1; illustrative CUE; hosts are placeholders per I19)

```cue
incidents: [{
	id:      "checkout-errors"
	purpose: "Checkout requests fail or slow down: who owns it, what changed, where to look" // required
	symptoms: {
		alerts: ["CheckoutHighErrorRate"]           // exact token; served as doc-claimed (rules live outside the repo)
		errors: ["payment authorization timed out"] // literal substring, at least 12 characters
	}
	footprint: {
		symbols: ["services/checkout.Service.Submit"]
		paths:   ["services/checkout/**", "deploy/checkout/**", "charts/shared/**"]
		keys:    ["route:POST /v1/checkout"] // contract keys the index holds
	}
	window:  {last: "72h"} // or {since_tag: "checkout-v*"} for a release train
	runbook: {path: "docs/runbooks/checkout.md"} // in-repo: its age against footprint churn is computable
	links: [
		{kind: "dashboard", url: "https://dash.example.invalid/d/checkout?from={{window_start}}&to={{now}}"},
		{kind: "escalation", url: "https://oncall.example.invalid/services/checkout"},
		{kind: "runbook", url: "https://wiki.example.invalid/checkout"}, // readiness: unverifiable
	]
	blind_spots: ["flag changes live in the flag service", "prod values live in the gitops repo"]
}]
```

Kept from the first draft: id, purpose, symptoms, footprint, window, links. Added: `runbook.path` (in-repo, works
offline, rot is measurable), `blind_spots` (authored, always printed with the change facet), `since_tag` windows.
Cut: `probes` (deferred, IM-D1), `bind` (the k8s extractor already indexes `Kind/namespace/name`,
`internal/extractor/k8sextract.go` near line 127; a copy would rot), `includes` (until F50).

Computed, not authored: the owning module's manifests and lockfiles (`go.mod`, `go.sum`, `package.json` and its
lockfile, and the like) join the footprint as labeled "dependency inputs", because a dependency bump is a classic
cause that a service-path glob misses.

### 5.4 Load-time refusals (valid and invalid testdata pairs per `schema/CLAUDE.md`)

- A missing `purpose`.
- Secret-shaped strings, using the detector behind I14's research-bundle tests, plus structural denials the
  detector's catalog lacks: URL userinfo, token-named query keys (`token`, `access_token`, `api_key`, `key`, `sig`,
  `signature`, `X-Amz-Signature` and similar), and the key prefixes of dashboard, on-call and payment services (the
  gap is item 14 in section 15).
- Error literals under 12 characters (proposed), so "timeout" cannot outrank a real match.
- Any link that is not `https`, carries userinfo, or has a placeholder in its scheme or authority.
- A placeholder other than those in section 6.2, or one outside its host rule.
- Remote declarations (by URL) and values bound from the environment, the keychain or files outside the repo.

## 6. Decision 2: connecting outward

### 6.1 Options and dispositions

| Option | What it is | Disposition | Reason |
|---|---|---|---|
| **E. Local facts** | Joins over the index and a bounded git read: commits, owners, prior incidents, findings, env reads, reach | **Accepted, the foundation** | Zero egress; the facts exist; most of the first five minutes live here |
| **L. Links** | URL templates scoped to the window, rendered, never fetched | **Accepted (v1 outward)** | What a human actually clicks; no credentials, no execution, no fetch |
| **P. Typed parameters and ref windows** | `--deployed <sha> [--previous <sha>]`; `window.since_tag` | **Accepted (v1 inward)** | Fixes most deploy lag with no egress and no durable state; the agent reads the SHA with its own tools |
| **B. Probes** | Command templates rendered as argv | **Deferred** (IM-D1) | Command runner in effect by default; no meter; duplicates runbooks |
| **D. MCP next queries** | Tool calls addressed to the agent's MCP servers | **Deferred with B** (IM-D2) | Operator-local server aliases and vendor schemas in a committed file: not portable, breaks on upgrades, readiness cannot check it |
| **C. Attach** | External tools push deploy, flag and alert events into fathom | **Refused, reopen trigger** (IM-X2) | A connector with the HTTP call moved out: fathom still owns schema, storage, TTL, dedup, reconciliation |
| **A. Fathom-side connectors** | Fathom calls ops APIs | **Refused** (IM-X1) | Domain-connectors and dispatch non-goals, I14, I56, credentials, liability |

### 6.2 Accepted: links

- Placeholders, v1:
  - `{{window_start}}` and `{{now}}`: RFC 3339 in UTC with a `Z` suffix (no `+`), percent-encoded per URL
    component. Allowed on any `https` host.
  - `{{commit}}` (40 hex) and `{{key:<kind>}}` (the name part of a footprint contract key, rendered only when it
    matches `[A-Za-z0-9._-]{1,63}`): only on hosts listed in the `[incident] link_hosts` configuration. Otherwise a
    pull request could send hashes or service names to any host a human clicks.
  - No `{{file}}` or `{{symbol}}` in v1.
- Values never come from commit subjects, alert text, error text or pasted input.
- A placeholder with no valid value leaves the link unrendered with a named reason. The template is still shown.
- Every rendered link shows its host, so a reviewer and an on-call engineer see where it goes.

### 6.3 Accepted: typed parameters and ref windows

- `--deployed <sha>` and `--previous <sha>` on the CLI, and matching optional `deployed` and `previous` arguments on
  `localize`. Each is 7 to 40 hex characters and must resolve locally (`rev-parse --verify` through gitro, no
  fetch). The suspect list becomes `previous..deployed`, or `deployed`'s ancestry inside the window when `previous`
  is absent.
- A SHA not in the clone reads "not computed: sha not in clone; fetch, then retry". A `deployed` that is an ancestor
  of `previous` is a rollback: the answer lists the commits the rollback removed, labeled as removed.
- `window.since_tag` takes the newest tag matching the glob as the window start, which covers release trains with
  zero egress.
- Without a deployed SHA, the answer says so once: "time window used; pass the deployed SHA for the deployed range".
  That is the whole external connection for deploy state: the agent or human reads the SHA from their own deploy
  tool, under their own permissions, and hands fathom a typed value.

### 6.4 Deferred: probes (B) and next queries (D)

Why deferred rather than refused: the owner wants maps to reach CLI commands, and a safe, meterable form may exist.
None exists today.

**Promotion needs all of:**

1. **An instrument.** An inject-only PostToolUse(Bash) sensor, after Q32 answers how exit codes arrive, that matches
   a run argv to a rendered probe id. The reading counts probes rendered, probes run, and whether a run changed the
   next action. Without it, a probe feature can never be killed (the discoveries failure without a kill path).
2. **A consumer** whose runbooks do not already carry the commands, asking for templates.
3. **For D, a portable server identity** that readiness can check, not an operator-local alias such as
   `server: "logs"`.

**The contract a promoted probe must meet** (recorded now so the analysis is not lost):

- **Rendering is a gate, because bypass-permissions mode exists.** The harness prompt and `command_scopes` guards
  are not a mitigation in the default setup, and "fathom said run this" carries authority. Safety must be intrinsic
  to what fathom renders.
- **argv only, one argv element per placeholder, never re-split.** Fathom also emits a POSIX-quoted display string
  so the agent never quotes.
- **A built-in verb-and-flag allowlist** for known binaries: `kubectl get|describe|logs|top|events`,
  `helm status|history|get`, `git log|show|diff`, cloud-CLI `describe|list|get`, `curl` GET only. An unlisted binary
  renders only under an "unverified" banner. The per-repo `probe_binaries` list becomes a render gate.
- **Always refused:** shells, interpreters and launchers (`sh -c`, `bash`, `env`, `xargs`, `find -exec`, `awk`,
  `python`, `node`, `git -c`, `--exec-path`); endpoint, proxy and credential redirection (`--server`,
  `--endpoint-url`, `--proxy`, `--kubeconfig`, `curl -H|-K|-o|-T`); secret-reading verbs and full-object dumps
  (`get secret`, `-o yaml|json` on workloads and configmaps, `ssm get-parameter --with-decryption`,
  `secretsmanager get-secret-value`) unless the probe is marked aggregate.
- **Bound values:** no leading `-`; characters limited to `[A-Za-z0-9._:/=@,+%-]`; otherwise not rendered. Literal
  template text may not contain `$`, a backtick, `;`, `|`, `&`, `<`, `>`, `(`, `)`, `~` or a newline (a `$TOKEN`
  passes the secret detector and expands in a shell).
- **Required fields:** `sensitivity`; pinned targets (`--context`, `--namespace`, `--profile`, `--region`); per-tool
  timeout hints (`--request-timeout`); an optional `requires: [<gate probe>]` reachability check first.
- **`empty_means` applies only on exit 0 with a declared `expect`.** Any other exit is "not computed". "No resources
  found" from the wrong cluster must not read as "no pods".
- **At most 6 probes, cheapest first.** The output says probe results reach the model provider.
- **`effect` has one value, `read`, served as `doc-claimed`.** Fathom cannot verify it and says so.

### 6.5 Refused: attach (C)

- **Why.** It keeps fathom owning an ops-event schema, storage, a TTL, dedup and reconciliation between producers.
  The drop-file import at SessionStart is polling by proxy. Two event kinds stored telemetry that no facet used.
  Deploy lag is mostly covered by 6.3 at zero state.
- **Reopen trigger.** IM-M1 records that git-invisible changes (flag flips, out-of-band config) caused at least 20
  percent of change-caused incidents at the consumer, and `since_tag` plus `--deployed` do not cover them. Then a new
  ADR, not a flag flip.
- **The contract if it is ever reopened** (from the security review): a guard at least as strict as RT-59's loopback
  Host and same-origin Origin plus a required `Content-Type: application/json`; a per-daemon 0600 token; every event
  shown as "unauthenticated"; events in a separate block that never merges into the git timeline and never
  influences matching, ordering or rendering; events accepted only for keys some declaration's footprint names;
  `observed_at` later than now plus skew rejected; secret-shaped fields rejected, not redacted and stored; actors
  only in a `system:` or service-account pattern; expired rows physically pruned on the next write; no free text.

### 6.6 Refused: fathom-side connectors (A)

Egress, credentials and vendor liability; the domain-connectors, dispatch and no-hosted-backend non-goals; I14 and
I56. Revisit only on the domain-connectors trigger (`scope.md`: an external party ships a conforming module on the
published spec with zero connectors from us), then through an ADR and an I14 amendment, invoked and never polled.

## 7. The suspect list: the core facet, and what makes it wrong

### 7.1 Source: a bounded git read, no new table

Today's commit data cannot serve it (verified at `0e16a8a1`):

- Ingest stores the newest 500 commits of HEAD only (`git log -n 500 --name-only`, author date `%aI`, no `-M`;
  `internal/fusion/pipeline.go` near line 367). A monorepo with more than 500 commits in 72 hours loses the start
  of its window.
- `ListCommits` (`internal/store/sqlite.go` near line 1351) reads every stored row. That is bounded by the 500-row
  table, not by the query. The real defect is the horizon and the date semantics, not an unbounded read.
- A failed or empty `git log` keeps the old rows (`pipeline.go` near line 1455), so stale rows can sit under a fresh
  watermark.
- A larger stored horizon needs a normalized commit-files table, which collides with "no table before RT-M1".

**Decision: compute the list at request time from a bounded `git log` through gitro, and store nothing.** This
resolves the RT-M1 collision explicitly. Arguments:

- `--since` or the `previous..deployed` range; `-n` capped (proposed 300 scanned, 20 shown, remainder counted);
  footprint globs as `:(glob)` pathspecs; `CommandContext` timeout.
- Committer date (`%cI`), so rebased and cherry-picked commits land where they landed; first-parent landing order on
  the default branch; `--diff-merges=first-parent`, so merge commits carry their files.
- `-M`, so pre-move paths still match; old paths recorded.
- Environment `GIT_NO_LAZY_FETCH=1` and `GIT_TERMINAL_PROMPT=0` (gitro sets only `GIT_OPTIONAL_LOCKS=0` today,
  `internal/gitro/gitro.go` near line 36), so a partial clone never does hidden network I/O on a degraded network.
- The cost class of this subprocess is decided at build (proposed: `index`, with a 2 s timeout). It is not free.
- The IM-M0 and IM-M1 replays use exactly these flags, so the bench measures what production serves.

### 7.2 Git mechanics

- **Reverts.** `This reverts commit <hash>` pairs are marked deterministically. An in-window revert is shown as a
  mitigation signal.
- **Bulk and bot commits.** Commits touching more than a threshold of files (proposed 200) are marked `bulk` and
  collapsed.
- **Shallow clones** are detected (`.git/shallow`) and the list is labeled a lower bound.
- **Sparse checkouts.** Index-dependent facets for paths outside the sparse set read "not computed: outside sparse
  checkout".
- **Rows carry no author field** (I58). The hash leads to the author in one step.

### 7.3 What git cannot see

- **Clone lag.** The freshness line shows local HEAD against the remote-tracking default branch (a local read) and
  the last fetch time (`.git/FETCH_HEAD` mtime). Fathom never fetches. It names `git fetch` as a human step.
- **Deploy lag.** A time window is not the deployed range. Section 6.3's `--deployed` fixes most of it.
- **Config and the outside world.** SaaS flags, out-of-band infrastructure changes, values in a separate gitops
  repo, certificate expiry, quotas, upstreams, traffic and data shape. The facet is always titled "git-visible
  changes to this repo only", and the declaration's `blind_spots` print with it. "Checked, none" must never read as
  "nothing changed".
- **Shared dependencies.** Section 5.3's computed dependency inputs cover manifests and lockfiles. Base images, CI
  config, generated code and shared charts go in `footprint.paths` by hand. Companion services and a 1-hop
  dependency closure are deferred (IM-D8).

## 8. The answer

### 8.1 Flow and facets

1. **Input.** A stack trace, error text, failing test or alert name, through `localize`, the prompt hook, or
   `fathom incident --stack-file|--alert`.
2. **Seeds.** Frames give files and symbols through the existing localize parsers. Frames that resolve to indexed
   repo files are preferred over library frames. When the deployed SHA differs from HEAD, frames match by symbol
   name, not line.
3. **Match.** A symptom literal, or a seed inside a footprint. Deterministic order: symptom match, then seeds inside
   the footprint, then id. At most 3 declarations, each with its match reason.
4. **No match: the unprepared view, which is the floor.** The same facets over the seed files, labeled "no incident
   map covers these frames". It is stage 1 of the build and works with no pre-shaping.
5. **Facets, in order.**
   1. **Freshness.** Index commit and age; commits behind the remote-tracking branch; last fetch age; shallow or
      sparse notes. Never ingests (I50).
   2. **Owners.** CODEOWNERS entries for the footprint with the matching rule and its line, then the escalation
      link. Unsupported syntax (GitLab sections, `!`, `[...]`) reads "not computed: unsupported CODEOWNERS syntax at
      line N". No committers fallback.
   3. **What changed.** Section 7, labeled "changed in window, not cause; git-visible changes to this repo only",
      with `blind_spots`.
   4. **Prior incidents and findings.** INC records naming footprint files, incidents whose Outcome names this
      declaration, findings near the footprint (ADR-040, quoted, never judged).
   5. **Knobs.** Env reads in the footprint (Go, JS, Python), with tier. Flags and config keys after SG-20. Never
      toggled.
   6. **Reach.** Covering tests with certificate state and recent failures; `route:` and `rpc:` neighbours at the
      typed tier; cross-repo only on request.
   7. **Runbook and links.** `runbook.path` with its age against churn; rendered links with their hosts; never
      fetched.

### 8.2 Untrusted text in the answer

Commit subjects, finding quotes, INC text, CODEOWNERS handles and pasted error text are contributor-controlled, and
the answer enters an agent that holds private data and tools.

- Strip control, ANSI and bidi characters. Cap subjects at 80 characters.
- Put them in a separate field labeled untrusted.
- Never put a subject in the hook line.
- Any markdown output escapes markup, links and mentions (`@here`, `@channel`).
- No free text ever binds into a link template.

### 8.3 Latency and degradation (proposed invariant text, not in force)

- The answer uses only `point` and `index` work. `tree`, `fleet` and `egress` never run on this path.
- Each facet is admitted separately (I49). A refused or late facet is named and the rest are served. A facet that
  misses the deadline reads "not computed: deadline".
- Single-flight per declaration id, so the hook, `localize` and the console share one computation.
- The in-memory memo keyed by (id, watermark) is cold after every daemon restart and every ingest, and ingest runs
  after each commit, including fix commits during the incident. **So the target is cold, not warm.** Readings, not
  claims (IM-M5): cold p95 under 3 s and warm p95 under 1 s for a 500-anchor declaration right after an ingest.
- Hooks: 100 ms queue wait at most, then silence (I9, I53).
- Footprint resolution is capped (proposed 500 anchors), so a declaration pays for its answer, never for the
  repository (P19).
- Under the refuse rung and in safe mode, point and index work is still admitted (`internal/tenancy/gate.go`), so
  this answer is among the last to go dark on a stressed machine (P23).

### 8.4 Delivery (P16)

- **Prompt hook, stage 1.** When a pasted trace resolves to repo files: "N commits touched these frames' files in the
  last <window>; `localize` expands". The hook takes the first 5 candidates today (`maxPromptCandidates`,
  `cmd/fathom/prompthook.go`); it must prefer frames that resolve to indexed repo files. Once per session, within
  I35, silent on failure.
- **Prompt hook, stage 2.** Adds the matched declaration's id and purpose.
- **The stanza** (CLAUDE.md and AGENTS.md text) says a missing pointer is not evidence of no map, since the hook may
  have timed out, and names the explicit CLI path.
- **`localize`** gains an incident section and the `deployed` and `previous` arguments. No new MCP tool (R6,
  ADR-053).
- **One CLI verb**, `fathom incident` (name open): `--stack-file`, `--alert`, `--deployed`, `--previous`,
  `--offline`, `--readiness`, `--json`. CLI only. It exists for the human path and the daemon-down path (INC-23
  showed the daemon can be unusable exactly when it is needed, and it must not be restarted from an agent's shell).
- **`--offline`** runs in-process: it reads the CUE, CODEOWNERS and the bounded `git log` through gitro under a
  timeout. Index-dependent facets read "not computed: index unavailable".
- **A doctor row** summarizes readiness (section 9).
- **Deferred:** a committed rendered snapshot and a hook literal-match fallback when the daemon is down (IM-D9); a
  read-only console view (IM-D6).

### 8.5 Offline matrix

| Situation | Works | Fails or degrades |
|---|---|---|
| Daemon down | `fathom incident --offline`: owners, the suspect list, runbook path, links; the CUE file read directly | Reach, knobs and prior findings read "not computed: index unavailable"; the hook is silent |
| Network, VPN or SSO down | Every local facet, up to the last fetch; the freshness line shows fetch age | Links (a wiki runbook shares the outage, hence `runbook.path`); the agent's own model calls; any tool the agent would use to find the deployed SHA |
| Stale clone | Behind-count and fetch age are named | The newest suspects are missing until a human fetches |
| Shallow, partial or sparse clone | Lower bound named; no lazy fetch | History past the shallow boundary; files outside the sparse set |

### 8.6 Pre-shaping versus pre-resolution

- **Pre-shaping** is authoring the declaration before the incident. It costs nothing at runtime and survives
  everything, because it is a committed file. This is the part the owner values.
- **Pre-resolution** stores footprint resolution at ingest so serving is a point read. That is a durable memo,
  deferred to RT-M1 (R5) and a cold p95 over target (IM-D3). Until then, resolution is computed on read and held in
  the existing in-memory memo.

## 9. Readiness: facts, never grades

A doctor row and `fathom incident --readiness`, not a verb of its own. Review Kit owns the standard of "ready"
(XM-30's split stands). Facts per declaration:

- **Anchors resolve.** Every symbol key exists, every glob matches at least one file, every contract key exists.
- **An owner is present** through a non-wildcard CODEOWNERS rule, and `.fathom/incidents.cue` itself has a
  non-wildcard owner (tamper review).
- **Runbook age against churn.** "`docs/runbooks/checkout.md` last touched 400 days ago; the footprint took 212
  commits since." This is the computable proxy for rot, and the best idea in the analysis.
- **URL-only runbooks read "unverifiable".** A wiki edited yesterday would look 400 days old by its link line, and a
  fresh line can point at a dead page.
- **Symptoms read "unverifiable: no alerts-as-code in this repo"** until IM-D5.
- **Symptom collisions.** One literal matching several declarations is listed.
- **Knobs present, live certificates, change volume in the window.** Zero live certificates reads "none", not a gap,
  until a consumer declares them.

A degrading declaration is flagged, never deleted, and fathom never edits the file (law 15). Drills, drill receipts,
"last drill" and probe bindability are cut (IM-X4). The retrospective replay is a bench (IM-M0, IM-M1), not a
product verb.

## 10. Postmortems

- XM-29 moves as IM-16: an incident's Outcome names the declaration id; the intent audit notes a closed incident
  that names none, never as a failure.
- The proposed declaration diff (cause files outside the footprint become proposed anchors) is deferred (IM-D7). It
  needs a machine-readable cited cause kept in the repo. A consumer's postmortems live in incident tools, and
  importing them means a connector or free text, both refused.
- Fathom's own INC-1 to INC-24 are development and process incidents. They cannot be the value reading (P20) and
  cannot even feed the replay (section 15 item 11).

## 11. Security and privacy (corrected)

- **No credentials anywhere.** Declarations hold identifiers, globs and links. Section 5.4 refuses secret-shaped
  strings at load, with structural denials for what the DLP catalog misses.
- **Egress.** Fathom adds no egress path. The answer, and anything the agent does with it, reaches the model
  provider. The unqualified claim "no egress" is refused (IM-X10).
- **The trifecta, honestly.** Private data: yes (source and index). Untrusted content: yes, mitigated by section
  8.2, not absent. Outbound action: fathom writes nothing outward and, with B and D deferred, hands the agent no
  commands, so fathom adds no outbound leg of its own. The first draft's "two legs absent by construction" is
  withdrawn.
- **Links** follow section 6.2: `https` only, no userinfo, no token-named keys, no placeholder in scheme or
  authority, commit and key placeholders only on allowlisted hosts, per-component encoding, host always shown.
- **Tampering.** A declaration is repo content reviewed in a PR. With no probes, a malicious declaration can at
  worst mislead (a wrong owner or a wrong link), which review, the shown host and the CODEOWNERS readiness check
  address.
- **Loopback.** This design adds no endpoint. On the base read here, RT-59 already requires a loopback Host and a
  same-origin Origin on every route, `/rpc` included (`cmd/fathom/admission_http.go`, `newDaemonServer`). The main
  branch lacked that guard when this was written, and neither checks `Content-Type`. That gap predates this design
  and is decided on its own.

## 12. What a typed model may do

- **Deferred (XM-D9 stands, trigger re-pointed to IM-M3): ordering the suspect list.** Opt-in (I60), only after a
  calibration whose Wilson 95 percent lower bound is at least 0.90 for reordering a default list (R3, I59), never
  membership, every row still shown. One consumer's incident count (tens a year) may never reach the n needed, so
  the deterministic order is the product.
- **Dropped from the register: drafting a declaration from a postmortem.** Its input, a machine-readable cause in
  the repo, does not exist. It may be proposed again if IM-D7's trigger fires.
- **Refused (IM-X9):** naming a root cause or "likely cause" (I59); choosing the match (R1); judging runtime data;
  severity; mitigation; anything touching a security property (R7, X6).
- Provider `none` works everywhere in this design.

## 13. Measurement and gates

### 13.1 Why the section 3c gate changes

- XM-26 needed `XM-D1, XM-M2, RT-M1`. XM-M2 is the handoff A/B on sub-agent tokens. It was added to waves 3 and 4
  only because a deferred row satisfies `needs` by disposition alone (G2-6, `docs/plans/exploration-maps.md` header
  and `docs/plans/sdd-reconciler-v0.md`, where G2-6 is open).
- The declaration needs no query evaluator (XM-D1) and adds no durable state (RT-M1).
- The old gate could kill incident maps if handoff maps failed, which is a category error.
- The new root rows name (op) readings, never a deferred row, so G2-6 does not bite.

### 13.2 IM-M0: the zero-build replay (bias fixed)

- **Two arms at equal list length.**
  - Arm (a), seed files only: the unprepared view, equivalent to `git log --since -- <seed files>`.
  - Arm (b), a footprint built **mechanically** from the seeds (the seeds' service directory plus that directory's
    CODEOWNERS patterns and its dependency inputs) or written **blind** by someone who has not seen the cause.
  - The value of pre-shaping for the suspect list is recall(b) minus recall(a) at equal k.
- **Corpus without a consumer: fast reverts in large public repositories.** A `Revert "..."` commit within 14 days
  gives the cause (the reverted commit) and the detection time (about the revert time).
  - **Seed simulation.** Seeds equal to the cause's own files would hand arm (a) the answer. So seeds are one to
    three files that share a directory with a cause file but were not touched by it (a failure often surfaces near
    the change, not at it; the reading states this assumption). Reverts with no such file are excluded and counted.
  - **Known bias.** Reverts lean toward breakage caught in CI, not production. The proxy can kill; only the consumer
    replay (IM-M1) can keep.
  - n: at least 50 reverts across at least 3 repositories.
- **Same source as production.** The replay runs the section 7.1 flags (committer date, first parent, merges, `-M`,
  the `-n` cap), not full history.
- **Bars, fixed before the run (proposed).**
  - BUILD the suspect list (stage 1) if either arm reaches recall@20 of at least 0.5 with a median list of at most
    25 rows.
  - The declaration's suspect-list claim holds if recall(b) minus recall(a) is at least 0.15 at equal k. A smaller
    gain does not kill the declaration, whose owners, runbook, links and symptoms are valued separately. It removes
    the claim that pre-shaping improves the suspect list.
  - KILL the suspect facet if both arms fall under 0.5 or the median list exceeds 25.
- **Fathom's own records cannot feed it** (section 15 item 11).

**IM-M1, the consumer replay (the value reading).** The same two arms over at least 10 change-caused incidents
(ideally 20) at a named consumer, with blind footprints recorded before anyone views the cause commit, ref-range
windows, and two extra records: commit-to-deploy lag, and the share of causes invisible to git (the attach reopen
evidence).

### 13.3 Stages

| Stage | Contents | Starts when | Meter and kill |
|---|---|---|---|
| 0 | IM-M0 (and IM-M1 once a consumer exists) | Any time; it builds nothing in the product | IM-M0 KILL ends stage 1 before it starts |
| 1 | Bounded git read (IM-1), `localize` recency hop (IM-3), untrusted-text handling (IM-4), hook line (IM-5) | The owner ends the deferral and IM-M0 reads BUILD | IM-M2: pointer expansion under 10 percent cuts the pointer (the XM-M3 rule) |
| 2 | The declaration, links, parameters, owners, answer, CLI, readiness (IM-6 to IM-16) | IM-M2 reads KEEP and a named consumer is recorded (IM-V1) | IM-M3 (cause in the list), IM-M4 (readiness gaps mattered), IM-M5 (cold p95) |
| Later | Deferred rows | Each row's own trigger | Each row's own reading |

Independent of the deferral: two defects in shipped code (IM-F1 CODEOWNERS, IM-F2 gitro environment) are bugs today
and can be fixed whenever the owner chooses.

## 14. Decision register

| # | Part | Disposition | Reason | Trigger or kill |
|---|---|---|---|---|
| 1 | Committed declaration `.fathom/incidents.cue`, named "incident map" for users; not a map kind | Accepted direction, deferred | Different author, home, addressing, liveness and latency from live maps; uses none of the map machinery (5.1, 5.2) | Stage 2 |
| 2 | Suspect list from a bounded request-time git read; no new table | Accepted direction, deferred | The stored 500-commit, author-date horizon is wrong; a table collides with RT-M1 (7.1) | IM-M0 BUILD |
| 3 | `localize` recency hop filled from row 2 | Accepted direction, deferred | The hop is a zero-yield stub today (VB-26) | With row 2 |
| 4 | Prompt-hook line, stage 1 then stage 2 | Accepted direction, deferred | The forced channel (P16) | IM-M2 under 10 percent cuts it |
| 5 | Unprepared view as the floor | Accepted direction, deferred | Works with no pre-shaping | With row 2 |
| 6 | Owners from CODEOWNERS with the rule shown, plus the escalation link | Accepted direction, deferred | Owners first is practice; the rule makes a wrong match visible | Needs IM-F1 |
| 7 | Links as URL templates (v1 outward) | Accepted direction, deferred | What a human clicks; nothing fetched or executed (6.2) | Stage 2 |
| 8 | `--deployed`/`--previous` and `since_tag` (v1 inward) | Accepted direction, deferred | Deploy lag and release trains at zero egress and zero state (6.3) | Stage 2 |
| 9 | Freshness line with fetch age and behind-count; fathom never fetches | Accepted direction, deferred | Clone lag hides the newest suspects (7.3) | With row 2 |
| 10 | Untrusted-text handling | Accepted direction, deferred | Contributor-controlled text reaches a tool-holding agent (8.2) | With row 2 |
| 11 | Admission and degradation contract: point and index only, partial answers, single-flight, cold target | Accepted direction, deferred (proposed invariant) | Must answer under pressure; a missing facet is named (I44) | IM-M5 |
| 12 | One CLI verb with `--offline` | Accepted direction, deferred | The human and daemon-down paths (INC-23) | Stage 2 |
| 13 | Readiness facts as a doctor row and a flag | Accepted direction, deferred | Rot is the main failure; facts, never grades | IM-M4 |
| 14 | No `os/exec` reachable from incident code except gitro; nothing fetched | Accepted direction, deferred | The testable form of "renders, never runs" | Stage 2 |
| 15 | Gate: IM-M0, IM-M2 and a consumer replace XM-M2 and XM-D1; F4 split into F55 | Accepted direction | XM-M2 measures an unrelated benefit (13.1) | ADR-058 |
| 16 | Suspect rows with no author field | Accepted direction | I58, blameless practice | none |
| 17 | IM-M0 revert-corpus two-arm replay | Experiment | The cheapest way to test the core facet and the pre-shaping gain (13.2) | Its own bars |
| 18 | IM-M1 consumer replay | Experiment | The only reading that can keep the feature | Needs IM-V1 |
| 19 | Runbook age against churn and the symptom-collision report | Experiment (inside row 13) | Computable proxies for rot and mis-ranking | IM-M4 DROP removes them |
| 20 | Outcome names the declaration id (XM-29 moved) | Experiment | Postmortem feedback without importing postmortems | IM-M3 DROP removes it |
| 21 | Computed dependency inputs in the footprint | Experiment | Dependency bumps are classic causes a service glob misses | IM-M1 shows whether they held causes |
| 22 | Probes (B) | Deferred | Command runner in effect by default; no meter; duplicates runbooks (6.4) | Instrument plus consumer, then the 6.4 contract |
| 23 | MCP next queries (D) | Deferred | Non-portable operator aliases and vendor schemas in a committed file | With row 22, plus a portable server identity |
| 24 | Pre-resolution at ingest | Deferred | A durable memo (R5) | RT-M1 PASS and IM-M5 over target |
| 25 | `scope` naming a live map | Deferred | Needs live maps | XM-D1 |
| 26 | Symptoms checked against alerts-as-code | Deferred | Alert-rule extraction unverified | A consumer keeps alert rules in the repo and extraction is verified |
| 27 | Read-only console view | Deferred | Must earn readers; editing and timeline forms refused | A consumer asks after stage 2 |
| 28 | Postmortem proposes a declaration diff | Deferred | Needs a machine-readable cause in the repo | That input exists |
| 29 | Companion services (at most 3) and a 1-hop dependency closure | Deferred | Cost and noise before evidence | IM-M1 or IM-M3 shows causes there |
| 30 | Committed rendered snapshot with a CI check; hook literal match when the daemon is down | Deferred | Useful only if daemon-down incidents happen at the consumer | A consumer reading shows it |
| 31 | A typed model orders the suspect list (XM-D9) | Deferred | R3, I60; n may never be reached | IM-M3 KEEP and a calibrated site |
| 32 | A: fathom-side connectors | Refused | Non-goals, I14, I56, credentials, liability | The domain-connectors trigger, then an ADR and an I14 amendment |
| 33 | C: attach | Refused, reopen trigger | A connector with the call moved out (6.5) | IM-M1 git-invisible share at least 20 percent and row 8 fails to cover it, then an ADR |
| 34 | Fathom executes probes, fetches links or calls other MCP servers | Refused | "Command execution by fathom" (ADR-056 decision 13), I56 | never |
| 35 | Command-center accretion: drills and drill receipts as product state, "last drill", a deploy/flag/alert timeline, an incident-tool export | Refused | Together they rebuild an incident-management product (XM-X4) | The command-center non-goal's trigger only |
| 36 | An incident map as a kind in the map family, and the `map` verb for it | Refused | Has F50's gate both ways (5.2) | XM-D1 ships and a declaration needs map machinery |
| 37 | Frequent committers as owners | Refused | Selects humans by commit count (I58) for little value | never |
| 38 | Remote or fetched declarations; values bound from env, keychain or outside files; a static `bind` copying indexed facts | Refused | I56, Q5; copies rot | never |
| 39 | Hook auto-run of anything; the change facet presented as complete; no pointer presented as no map | Refused | I9, I44 | never |
| 40 | Credentials or secret-bearing strings in declarations or config | Refused | Q5, I61 | never |
| 41 | A typed model names a cause, picks the match, judges runtime data, sets severity or proposes mitigation | Refused | I59, R1, R7 | never |
| 42 | The unqualified claim "no egress" | Refused | Answers reach the model provider | never |
| 43 | Mutating anything and an incident command center | Refused (XM-X3, XM-X4 stand) | Fathom never mitigates; the non-goal | As recorded there |

## 15. Findings on the current records and code

Verified at `0e16a8a1` unless marked otherwise.

1. **The base moved during the analysis.** `0e16a8a1` already carried section 3c and wave 4 of the exploration-maps
   plan. This decision amends them.
2. **The gate is mismatched.** XM-26 needed XM-M2 only as a G2-6 workaround; G2-6 is open in
   `docs/plans/sdd-reconciler-v0.md`.
3. **The F4 note gave one future two promote conditions** (the resolved-finding count and live-map readings).
4. **F50's drop rule would kill incident maps by design** if they were live maps.
5. **The `localize` recency hop is a zero-yield stub** (`internal/mcp/localize_tool.go`, the recency hop near line
   775). Its comment says no churn analysis is wired, yet `ListCommits` is on `QueryInterface`
   (`internal/mcp/server.go`) and churn is stored at ingest (`internal/store/filechurn.go`).
6. **The commit horizon, not the query, is the defect.** Ingest keeps 500 commits of HEAD with author dates, no `-M`
   and no merge files (`internal/fusion/pipeline.go`); `ListCommits` reads all stored rows
   (`internal/store/sqlite.go`); a failed `git log` keeps old rows. The first draft's "loads the whole history" was
   wrong.
7. **The CODEOWNERS matcher misroutes owners** (`internal/ownership/ownership.go`, `matchPattern`). A leading-slash
   pattern (`/services/checkout/`) never matches, because paths carry no leading slash, and `docs/` matches only at
   the root where GitHub matches it at any depth. The SRE review reproduced both on main with a throwaway test. It
   also feeds `internal/review/reviewers.go` today. Fix: IM-F1.
8. **gitro sets only `GIT_OPTIONAL_LOCKS=0`** (`internal/gitro/gitro.go`, `Command`). Fix: IM-F2.
9. **Command guards advise by default.** `command_scopes` deny only with `enforce: true`
   (`docs/design/guarded-edits.md`), and bypass-permissions mode shows no prompt.
10. **Pull-shaped features die here.** The `history` tool was unregistered at 0 lifetime calls
    (`internal/mcp/extra_tools.go`); `localize` 40 of about 98k calls; discoveries 13; anchored memory 0.
11. **Fathom's INC corpus cannot feed a replay.** 24 records carry 7 distinct hashes, mostly evidence or push
    references, and no cause-and-detection pair; most are process incidents. The base's 2,108 commits hold no
    `Revert` commit (the invariants review counted one by a looser match).
12. **The k8s extractor already indexes `Kind/namespace/name`** (`internal/extractor/k8sextract.go`), so a static
    `bind` would copy an indexed fact.
13. **The prompt hook takes the first 5 candidates** (`cmd/fathom/prompthook.go`, `maxPromptCandidates`), and the top
    frames of a trace are often library code.
14. **The secret detector's documented catalog** covers PEM keys, cloud key ids, source-host, chat and Google tokens
    and JWTs, but not dashboard-service, on-call-service or payment-service keys, cloud signed-URL parameters or URL
    userinfo (SRE review, on main; not re-checked at `0e16a8a1`).
15. **Daemon loopback differs by branch.** The base's RT-59 guards every route (`cmd/fathom/admission_http.go`); main
    had no `admission_http.go` when this was written. Neither checks `Content-Type`.
16. **Knobs are thin.** Env reads only (Go, JS, Python). Flags and config keys wait on SG-20, deploy and image keys
    on SG-21.
17. **Certificates.** 0 live declarations on the analysis machine.
18. **The first draft's `alert_firing` and `error_count` kinds fed no facet**, and its XM-M9 (probes changed the
    investigation) had no instrument. Both are gone.
19. **The deleted `shell_out` was a code analyzer with 0 lifetime calls (SR-19)**, not a command runner. The refusal
    that applies is "command execution by fathom".

## 16. Open facts to verify before building

- **Q32**: the PostToolUse(Bash) exit code and PostToolUse versus PostToolUseFailure (row 22's instrument).
- **Frame parsing** for Java, JS, .NET and Ruby traces in `localize` is unverified. A per-runtime fixture bench comes
  before any claim of "maps a stack trace".
- **Whether k8s extraction covers alert-rule and other custom resources** (row 26).
- **RT-M1** is unread; **SG-20 and SG-21** are open.
- **Which of the decisions and rules cited here are on main.** Only the line of history this analysis started from (commit `0e16a8a1`) was read in depth.
- **Whether a named production consumer exists** (IM-V1), and **its commit-to-deploy lag**.
- **Whether fast reverts are a valid stand-in** for change-caused production incidents, and whether the "surfaces
  near the change" seed simulation holds.
- **The git subprocess's cost class** and its cold latency on a large monorepo.

## 17. Where this landed

The decision is durable in these records; nothing here is built by them.

- **ADR-058** in `ADR.md` and its row in `docs/intent/decisions/README.md`.
- **Plan** `docs/plans/incident-maps.md` (IM-): readings first (IM-V1, IM-M0, IM-M1), two independent defect fixes
  (IM-F1, IM-F2), stage 1 (IM-1 to IM-5, IM-M2), stage 2 (IM-6 to IM-16, IM-M3 to IM-M5), and the deferred and
  refused rows (IM-D1 to IM-D9, IM-X1 to IM-X10).
- **Futures** F55 (incident maps) and F56 (probes and next queries) in `docs/intent/futures.md`; F4 carries a dated
  note returning it to precedent over a non-git corpus. The numbers F53 and F54 were left free on purpose because
  another design in flight names F53.
- **Exploration maps**: `docs/plans/exploration-maps.md` rows XM-26 to XM-30, XM-M6 and XM-M7 are `[-]` with ids
  reserved and a pointer here; XM-D9's trigger is re-pointed to IM-M3; XM-X3 and XM-X4 stand; concern scopes
  (XM-23 to XM-25) are untouched. Section 3c of the exploration-maps design carries a pointer to this document.
- **Vision 40** section 13 links the two kinds in one paragraph.
- **Non-goals** in `docs/intent/scope.md` (no domain connectors; no chat or incident command center) carry dated
  notes that incident maps respect them. The non-goals themselves are unchanged.
- **Incidents**: `docs/intent/incidents.md` points its planned extension at IM-16.
- **Observations to file** (`fathom observe`), not deferred by this decision: the CODEOWNERS matcher (IM-F1), the
  gitro environment (IM-F2), the stale recency-hop comment, and the loopback and `Content-Type` gap on main.
- **Once stage 2 is built**, propose an invariant: "No incident path executes a command other than read-only git,
  fetches a link, or calls another MCP server; the answer uses only point and index work and names every facet it
  could not compute, and the change facet always names its horizon and blind spots."

## 18. Guidance for agentic-kit (project-agnostic; to return to the kit)

**Incident readiness with pre-shaped maps.** Prepare incident knowledge as a small declaration committed beside the
code and reviewed like code: the symptoms that identify the failure (alert names, error literals), the code
footprint it lives in, a change window (a duration or "since the last release tag"), an in-repository runbook, deep
links scoped to the window, and the blind spots nobody's git can see. When a trace or alert arrives, answer in the
order responders need: freshness first, then owners with the rule that matched, then what changed, labeled "changes
in this window, not the cause, and only those git can see", then prior incidents, configuration knobs, test reach,
the runbook and the links. Every facet reads a value, "checked, none", or "not computed: why", because a silent
empty looks like good news. Build the change list from a bounded read of version control using landing dates, merge
files and renames, and let the operator pass the deployed revision to turn a time window into the deployed range.
Link out and never fetch; render nothing executable until an instrument can see whether rendered commands are run,
and when you do, make the rendering itself the safety gate, because permission prompts vanish in bypass modes. Treat
commit subjects and pasted errors as untrusted input to an agent that holds tools. Judge readiness by computable
facts (anchors resolve, an owner is present, the runbook's age against the code's churn), never by grades, and never
delete a stale declaration automatically. Before building any of it, replay past incidents with two arms at equal
list length, seed files only against a footprint written blind, and build only what beats the seed-only baseline.
