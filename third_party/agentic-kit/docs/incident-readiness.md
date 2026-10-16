# Incident readiness: pre-shaped maps, what changed first, links out, nothing executed

Practice for a project whose responders, or whose coding agents, must answer the first
questions of an incident quickly: who owns this, what changed, what did we already know,
where do I look next. It covers what to prepare before the incident, what to answer when a
trace or alert arrives, how to reach other systems without becoming a connector, and what a
tool must never do. Written 2026-09-29 from one consuming project's design; the ledger entry
is [practices §27](practices.md#27-cold-start-incident-response).

**What this rests on, stated plainly.** One consumer's analysis, reviewed by an invariants
critic and an operations and security critic. Nothing is built and nothing is measured. The
consumer accepted the direction and deferred the build behind a zero-build replay (section
9), so every rule below is a design, not a result. The kit ships no package for any of it.
Section 11 was added on 2026-10-11 from the same consumer's second design, an evidence plane
over index snapshots (one index per service per commit); it is also a design, and the engine
side of it is not built.

**Evidence tags.** [source] backed by primary public material, linked where used. [design]
designed and reviewed, with a measurement plan and no result yet; do not cite it as proven.
[harness] depends on host-tool behavior; the doc says which.

| # | Practice | Status |
|---|---|---|
| 1 | Pre-shape incident knowledge as a committed declaration | [design] |
| 2 | Answer in responder order, with "what changed" first and labeled | [source][design] |
| 3 | Every facet reads a value, "checked, none", or "not computed: why" | [design] |
| 4 | Build the change list from a bounded read of version control | [design] |
| 5 | Reach other systems by link and typed parameter, never by connector | [design] |
| 6 | Probes and next queries for other tools are data, deferred, with a contract | [design][harness] |
| 7 | Attach (external tools push events in) is refused, with a reopen trigger | [design] |
| 8 | Never execute, never mitigate, treat text as untrusted | [design][harness] |
| 9 | Judge readiness by computable facts; rehearse by replay | [design] |
| 10 | Measure before building, with the bars fixed first | [design] |
| 11 | Evidence before claim: read the answer basis, then the fact sheet, and cite receipts | [design][source] |

The closest shipped pieces are the shapes named in the ledger entry: a refusal that lives in
the value ([practices §17](practices.md#17-instruments-that-degrade-toward-healthy) and
`foundations/quality/structural-refusal`), the structural effect gate for anything that
runs ([§18](practices.md#18-irreversible-actions-without-a-structural-gate)), and the
untrusted-content fence ([§5](practices.md#5-trusting-model-and-tool-output)).

## 1. Pre-shape incident knowledge as a committed declaration [design]

**The problem.** During an incident, knowledge is either generated fresh under pressure by
a tool that has none prepared, or it lives in a wiki that rots or in one person's head.

**The practice.** Write a small declaration, committed beside the code and reviewed like
code, so it survives the loss of any tooling and is readable when the tooling is down.
Fields:

- **Id and purpose.** The purpose is required: one line naming the question the
  declaration answers. A declaration without one is refused.
- **Symptoms.** Alert names as exact tokens; error literals as substrings with a minimum
  length (12 characters was proposed), so a common word cannot match everything. Alert
  rules usually live outside the repository, so a symptom match is a claim the tool cannot
  verify, and it is served that way.
- **Footprint.** The code it lives in: symbols, path globs, contract keys the index holds.
  Shared inputs such as base images, CI configuration, generated code and shared deployment
  charts go in by hand, because no manifest read finds them.
- **Window.** A duration, or "since the newest release tag matching a pattern" for release
  trains.
- **Runbook.** An in-repository path when there is one. Its age against the footprint's
  churn is then computable (section 9).
- **Links.** URL templates scoped to the incident window (section 5).
- **Blind spots.** Authored, and always printed beside the change list: "flag changes live
  in the flag service", "production values live in a separate deployment repository".

A declaration holds identifiers, globs and links. It never holds secrets, credentials or
commands, because anything a local service serves is readable by any local process.

**Why not simply a shared exploration map.** [Orchestrating agents §1](orchestrating-agents.md#1-shared-exploration-maps-design)
describes live maps. Incident declarations look similar and are deliberately a different
artifact:

| Property | Live map | Incident declaration |
|---|---|---|
| Author | Agents and planners, as a working set | A team, ahead of time, reviewed in a pull request |
| Home | Working state on one machine | A committed file; survives loss of the store |
| Addressed by | Id, purpose, footprint overlap | Symptom (alert name, error literal) and footprint |
| Liveness | Dropped if unused for a period | Rare use is the success state, so no idle kill clock; liveness is readiness (section 9) |
| Latency | No contract | Must answer under pressure (section 8) |
| Delivery | Handoff, edit footprint, session brief | A pasted trace or alert; a query; one command-line verb |

The declaration uses none of the live map's machinery (content-hashed entries with why
lines, an includes graph, composite ids, handoff), and calling it a map would inherit the
live map's build gate by name while escaping it in practice. People may still say "incident
map". The bridge stays open for free: an optional field naming a live map as the
declaration's scope, once live maps exist.

## 2. Answer in responder order, with "what changed" first [source][design]

**Input.** A stack trace, error text, failing test or alert name.

**Seeds and match.** Frames give files and symbols; prefer frames that resolve to files in
the repository over library frames. When the deployed revision differs from the checkout,
match frames by symbol name, not line number. A declaration matches on a symptom literal,
or on a seed inside its footprint, in a fixed order (symptom match, then footprint match,
then id), at most 3 declarations, each showing why it matched.

**No match: the unprepared view is the floor.** Run the same facets over the seed files
and label the answer "no incident map covers these frames". This works with no
pre-shaping at all, which is why it is the first thing to build (section 10).

**Facets, in this order.**

1. **Freshness.** The index or checkout's own commit and age, commits behind the
   remote-tracking default branch, the time of the last fetch, and shallow or sparse
   notes. Never refresh inline.
2. **Owners.** The ownership-file entries for the footprint with the matching rule and its
   line, then the escalation link. An unsupported ownership-file syntax reads "not
   computed: unsupported syntax at line N". Never fall back to frequent committers: that
   selects people by commit count.
3. **What changed.** Section 4, labeled "changes in this window, not the cause, and only
   those version control can see", with the declaration's blind spots.
4. **Prior incidents and findings.** Recorded incidents that name footprint files, and
   review findings near the footprint, quoted and never judged.
5. **Knobs.** Environment reads, flags and configuration keys in the footprint, with how
   confidently each was found. Read, never toggled.
6. **Reach.** Covering tests with their certificate state and recent failures, and
   neighbouring routes and calls.
7. **Runbook and links.** The runbook path with its age against churn, and the rendered
   links with their hosts.

**Why change first.** Site reliability practice reports that roughly 70% of outages are due
to changes in a live system ([Google SRE book, introduction](https://sre.google/sre-book/introduction/)),
and commercial incident platforms also lead with recent-change correlation. Read the figure
as a strong prior, not a law. The same book is explicit that correlation is not causation
([effective troubleshooting](https://sre.google/sre-book/effective-troubleshooting/)):
events correlate by coincidence or through a shared cause. So the change list is a lead,
labeled as one, and a tool that names a cause has claimed more than its inputs can earn.

## 3. Every facet reads a value, "checked, none", or "not computed: why" [design]

A silent empty looks like good news. Under pressure, a responder reads "no recent changes"
as "nothing changed", and a missing pointer as "no map covers this". So every facet says
which of three things happened:

- a value;
- "checked, none", meaning the read ran and found nothing;
- "not computed: reason" (sha not in clone, index unavailable, unsupported syntax,
  outside sparse checkout, deadline), meaning the read did not run or did not finish.

Put the refusal in the value, never in a warning beside it: a warning field next to a value
is read as the value ([practices §17](practices.md#17-instruments-that-degrade-toward-healthy);
`foundations/quality/structural-refusal` is the shape). The change facet always names its
horizon and its blind spots, so "checked, none" can never read as "nothing changed".
Anything that injects a one-line pointer into a prompt fails silent, and the instructions
that mention it say silence is not evidence: the hook may have timed out.

## 4. Build the change list from a bounded read of version control [design]

**Source.** Compute the list at request time from a bounded history read and store nothing.
A stored recent-commit table has a horizon (a monorepo can exceed it inside one incident
window), wrong date semantics, and stale rows that outlive a failed refresh. If a design
forbids new durable tables, this also honors that.

**Mechanics that decide whether the list is right.**

- Use the committer date, not the author date, so rebased and cherry-picked commits land
  where they landed, and walk first-parent landing order on the default branch so merge
  commits carry their files.
- Follow renames, so a path that moved still matches, and record the old path.
- Cap what is scanned (300 was proposed) and what is shown (20), count the remainder, and
  set a timeout. State the cost class of the subprocess; it is not free.
- Disable hidden network reads in partial clones and disable interactive prompts, so a
  degraded network never turns a local read into a hang.
- Pair revert commits deterministically and show an in-window revert as a possible
  mitigation signal. Collapse bulk and bot commits past a file-count threshold (200 was
  proposed).
- Detect a shallow clone and label the list a lower bound. Name a sparse checkout, and read
  "not computed: outside sparse checkout" for paths outside it.
- Carry no author column. A hash leads to its author in one step; ranking people by
  commits is not the tool's job.

**What version control cannot see, printed every time.**

- **Clone lag.** Show local HEAD against the remote-tracking default branch and the last
  fetch time. The tool never fetches; it names the fetch as a human step.
- **Deploy lag.** A time window is not the deployed range. Accept a deployed revision as a
  typed parameter (section 5).
- **Configuration and the outside world.** Flag services, out-of-band infrastructure
  changes, values in a separate deployment repository, certificate expiry, quotas,
  upstreams, traffic and data shape. The facet is titled "git-visible changes to this
  repo only" and the declaration's blind spots print with it.
- **Shared dependencies.** Manifests and lockfiles can be computed; the rest goes in the
  footprint by hand. A capped one-hop dependency closure and companion services were
  deferred until a replay shows causes there.

**Untrusted text.** Commit subjects, finding quotes, prior-incident text, owner handles and
pasted error text are contributor-controlled, and the answer enters an agent that holds
private data and tools. Strip control, ANSI and bidirectional characters; cap subjects (80
characters); put them in a field labeled untrusted; never put one in a one-line hook
message; escape markup, links and mentions in rendered output; never bind free text into a
link. `foundations/security/untrusted-content-wrapper` fences such text and
`foundations/security/injection-detector` screens it. Both are defense in depth, not a
boundary.

## 5. Reach other systems by link and typed parameter, never by connector [design]

The question raised was whether a declaration should connect to other data sources and
command-line tools. The accepted answer is: yes for what a human clicks and what an agent
can hand over as a typed value, not yet for anything the tool would fetch or run.

| Option | What it is | Disposition |
|---|---|---|
| Local facts | Joins over the index and the bounded history read | **Accepted**, the foundation |
| Links | URL templates scoped to the window, rendered, never fetched | **Accepted** (outward) |
| Typed parameters | A deployed revision and an optional previous one; a release-tag window | **Accepted** (inward) |
| Probes | Command templates rendered as argument lists | **Deferred** (section 6) |
| Next queries | Tool calls addressed to the agent's own tool servers | **Deferred** with probes (section 6) |
| Attach | External tools push deploy, flag and alert events in | **Refused**, with a reopen trigger (section 7) |
| Tool-side connectors | The tool calls operations APIs, polls or subscribes | **Refused** |

**Links.**

- `{{window_start}}` and `{{now}}` render as RFC 3339 in UTC with a `Z` suffix, percent-encoded
  per URL component, and are allowed on any `https` host.
- A commit hash or a contract-key name renders only on hosts the operator lists in
  configuration, and a key name only when it matches a narrow pattern
  (`[A-Za-z0-9._-]{1,63}`). Otherwise a pull request could send hashes or service names to
  any host a human later clicks.
- Values never come from commit subjects, alert text, error text or pasted input, and no
  file or symbol placeholder exists.
- `https` only, no userinfo, no token-named query keys, no placeholder in scheme or
  authority. A placeholder with no valid value leaves the link unrendered with a named
  reason, and the template is still shown.
- Every rendered link shows its host, so a reviewer and an on-call engineer both see where
  it goes. Links are never fetched, not even to check they resolve.

**Typed parameters.** A deployed revision (`deployed`) and an optional previous one
(`previous`), each 7 to 40 hex characters and each resolved locally with no fetch. The
suspect list becomes `previous..deployed`, or the deployed revision's ancestry inside the
window when `previous` is absent. A revision not in the clone reads "not computed: sha not
in clone; fetch, then retry". A deployed revision that is an ancestor of the previous one
is a rollback: list the commits the rollback removed and label them removed. A tag-pattern
window covers release trains with no network at all. With no deployed revision given, say
so once: "time window used; pass the deployed revision for the deployed range". That is the
whole external connection for deploy state. The agent or human reads the revision from
their own deploy tool, under their own permissions, and hands the tool a typed value.

## 6. Probes and next queries for other tools are data, deferred, with a contract [design][harness]

A **probe** is a read-only command template rendered for a responder (for example, a
cluster CLI's describe or logs verb with pinned targets). A **next query** is a tool call
addressed to one of the agent's own tool servers (logs, metrics, tickets). The wish to
reach command-line tools and other data sources this way is reasonable, and it is deferred
rather than refused because a safe, meterable form may exist. None exists today.

**Why deferred.**

- In a default agent setup they are a command runner in effect. Advisory guards do not
  enforce unless configured to, and permission prompts do not appear in bypass modes, so
  "the tool said run this" carries authority with no human gate. [harness]
- There is no meter. A probe sits two steps past the channel an agent cannot skip (expand
  the pointer, pick a probe, run it), and nothing sees whether it ran, so the feature could
  never be killed. Pull-shaped features fail this way: in one consumer's logs, a
  localization tool had about 40 calls in roughly 98,000 tool calls
  ([orchestrating agents §1](orchestrating-agents.md#1-shared-exploration-maps-design)).
- Probes duplicate runbooks, and a next query needs a server alias that is local to one
  operator's machine and a vendor schema that changes on upgrade, in a committed file. Not
  portable, and readiness cannot check it.
- A server-side read-only mode is only as strong as its enforcement at call time. Treat a
  declared effect as a claim ([practices §18](practices.md#18-irreversible-actions-without-a-structural-gate)).

**Promote when all of these hold.**

1. **An instrument.** A post-tool-use sensor that matches a run command to a rendered probe
   id (after the host's way of reporting exit codes is known). It counts probes rendered,
   probes run, and whether a run changed the next action.
2. **A consumer** whose runbooks do not already carry the commands and who asks for
   templates.
3. **For next queries, a portable server identity** that readiness can check, not an
   operator-local alias.

**The contract a promoted probe must meet** (recorded now so the analysis is not lost).

- **Rendering is the gate.** Because bypass modes exist, safety must be intrinsic to what
  is rendered, not delegated to a prompt.
- **Argument list only,** one element per placeholder, never re-split. Also emit a quoted
  display string, so the agent never quotes anything itself.
- **A built-in verb-and-flag allowlist** for known binaries: read verbs of a cluster CLI
  (get, describe, logs, top, events), package-release status and history, version-control
  log, show and diff, cloud CLI describe, list and get, and HTTP GET only. An unlisted
  binary renders only under an "unverified" banner, and a per-repository binaries list
  becomes a render gate.
- **Always refused.** Shells, interpreters and launchers (`sh -c`, `bash`, `env`, `xargs`,
  `find -exec`, `awk`, `python`, `node`, version-control `-c` and exec-path options).
  Endpoint, proxy and credential redirection (server, endpoint, proxy and config-file
  flags; HTTP header, config-file and output-file options). Secret-reading verbs and
  full-object dumps of workloads and configuration objects, unless the probe is marked
  aggregate.
- **Bound values.** No leading `-`; characters limited to `[A-Za-z0-9._:/=@,+%-]`;
  otherwise not rendered. Literal template text may not contain `$`, a backtick, `;`, `|`,
  `&`, `<`, `>`, `(`, `)`, `~` or a newline. A `$TOKEN` passes a secret detector and
  expands in a shell.
- **Required fields.** A sensitivity class; pinned targets (context, namespace, profile,
  region); per-tool timeout hints; an optional reachability probe that must pass first.
- **Empty means "none" only on exit 0 with a declared expectation.** Any other exit is
  "not computed". "No resources found" from the wrong cluster must not read as "no pods".
- **At most 6 probes, cheapest first,** and the output says probe results reach the model
  provider.
- **One effect value, `read`,** served as unverified: the tool cannot check it and says so.

## 7. Attach (external tools push events in) is refused, with a reopen trigger [design]

Attach means deploy, flag and alert tools push events into the readiness tool so the change
list can include what version control cannot see.

**Why refused.** It keeps the tool owning an operations-event schema, storage, a TTL,
deduplication and reconciliation between producers: a connector with the HTTP call moved to
the other side. A drop-file import at session start is polling by proxy. Two of the
proposed event kinds (alert firing, error count) fed no facet at all. And deploy lag is
mostly covered by the typed deployed-revision parameter at zero state.

**Reopen trigger.** A replay at a named consumer (section 10) records that changes
invisible to version control (flag flips, out-of-band configuration) caused at least 20
percent of the change-caused incidents, and that tag windows plus the deployed-revision
parameter do not cover them. Then write a new decision record. It is not a flag flip.

**The contract if it is ever reopened.**

- A guard on the receiving endpoint at least as strict as a loopback host check and a
  same-origin check, plus a required JSON content type.
- A per-instance token with owner-only file permissions.
- Every event shown as "unauthenticated", in a separate block that never merges into the
  version-control timeline and never influences matching, ordering or rendering.
- Events accepted only for keys some declaration's footprint names.
- A timestamp later than now plus a small skew is rejected.
- Secret-shaped fields are rejected, not redacted and stored.
- Actors only in a service-account pattern, never a person.
- Expired rows are physically pruned on the next write, and there is no free text.

**Fetching connectors stay refused** for the same reasons plus credentials and vendor
liability. Revisit only when a project's own non-goal on domain connectors is revisited,
through a decision record, and invoked rather than polled.

## 8. Never execute, never mitigate, treat text as untrusted [design][harness]

- **Nothing executable in the first version.** The tool does not run a probe, fetch a link
  or call another tool server. Precision matters here: a bounded read-only history read on
  request is fine. The refusal is of operator-supplied or arbitrary commands.
- **Never mitigate.** No rollback, restart, flag flip, page, or status change. The tool
  does not become an incident command center, so it keeps no drill state, no incident
  status, no timeline of deploys, flags and alerts, and no export to an incident tool.
- **A judgment model may only assist ordering.** Use the bars in
  [fast-judgment](fast-judgment.md): opt-in, calibrated per site, and only after a Wilson
  95% lower bound of at least 0.90 for reordering a default list, never for membership,
  with every row still shown. A consumer with tens of incidents a year may never reach the
  sample size needed, in which case the deterministic order is the product. A model never
  names a root cause or "likely cause", never picks which declaration matched, never judges
  runtime data, sets severity or proposes mitigation, and never touches anything that
  affects a security property. The provider `none` works everywhere.
- **Honest accounting of the trifecta** ([practices §13](practices.md#13-the-lethal-trifecta)).
  Private data: yes (source and index). Untrusted content: yes, mitigated by section 4, not
  absent. Outbound action: the tool writes nothing outward and, with probes deferred,
  hands the agent no commands. And the answer, and anything the agent does with it, reaches
  the model provider, so an unqualified "no egress" claim is refused.
- **Degrade under pressure, do not wait.** Use only cheap point and index reads on this
  path. Admit each facet separately, so a late facet reads "not computed: deadline" and the
  rest are served. Compute once per declaration and share it between the hook, the query
  and any view. Plan for cold, not warm: an in-memory cache is empty after every restart
  and after every index refresh, and refreshes run after each commit, including fix
  commits made during the incident. A hook waits a short bounded time, then goes silent.
  Cap footprint resolution (500 anchors was proposed), so a declaration pays for its own
  answer and never for the repository.
- **A human path that works when the service is down.** One command-line verb that runs
  in-process from committed files and the bounded history read, with index-dependent
  facets reading "not computed: index unavailable". Offline behavior:

| Situation | Still works | Degrades |
|---|---|---|
| Service down | The offline verb: owners, the change list, runbook path, links | Reach, knobs and prior findings; the hook is silent |
| Network, VPN or SSO down | Every local facet, up to the last fetch | Links (a wiki runbook shares the outage, hence the in-repository path); the agent's own model calls |
| Stale clone | Behind-count and fetch age are named | The newest changes are missing until a human fetches |
| Shallow, partial or sparse clone | A named lower bound; no lazy fetch | History past the shallow boundary; files outside the sparse set |

- **Delivery by configuration, not by a tool the agent must choose to call.** When a pasted
  trace resolves to repository files, inject one line (how many commits touched those files
  in the window, how to expand it), once per session, within the injected-context budget,
  failing silent. Match a declaration's id and purpose in a second stage. No new default
  tool: the surface is an extended existing query and a command-line verb.

## 9. Judge readiness by computable facts; rehearse by replay [design]

Runbooks and playbooks rot; incident-tool vendors say so and it matches experience, but
treat it as a plausible failure, not a measured rate. So readiness should measure the
staleness it can compute, and report facts rather than a grade. One doctor row and one
command-line flag, not a verb of its own:

- **Anchors resolve.** Every symbol key exists, every glob matches at least one file, every
  contract key exists.
- **An owner is present** through a non-wildcard ownership rule, and the declaration file
  itself has a non-wildcard owner, for tamper review.
- **Runbook age against churn.** "`docs/runbooks/checkout.md` last touched 400 days ago;
  the footprint took 212 commits since." This is the computable proxy for rot and the best
  single idea in the analysis.
- **URL-only runbooks read "unverifiable".** A wiki edited yesterday would look 400 days
  old by its link line, and a fresh link line can point at a dead page.
- **Symptoms read "unverifiable"** until the repository holds alert rules as code that the
  index can extract.
- **Symptom collisions.** One literal matching several declarations is listed.
- **Volume in the window, knobs present, live certificates.** Zero live certificates reads
  "none", not "gap", until a consumer declares them.

A degrading declaration is flagged, never deleted, and the tool never edits the file: the
declaration is the team's. A postmortem may name the declaration it exercised in its
outcome, and an audit may note a closed incident that names none, never as a failure. A
tool that proposes a declaration diff from a postmortem needs a machine-readable cited
cause kept in the repository, which most consumers do not have, so that is deferred until
one does.

**Drills.** Do not build drills, drill receipts, a last-drill time or a deploy, flag and
alert timeline as product state. Together they rebuild an incident command center.
Rehearsal is a retrospective replay (section 10), a bench and not a product verb. Human
game days are fine; their records belong to the team's own incident tooling.

## 10. Measure before building, with the bars fixed first [design]

Pull-shaped features die (section 6), so replay past incidents before building anything.

**The zero-build replay.** Two arms at equal list length. Arm (a), the unprepared view:
seed files only, equivalent to a history query limited to those files. Arm (b), a footprint
built **mechanically** from the seeds (their service directory plus that directory's
ownership patterns and dependency inputs) or written **blind** by someone who has not seen
the cause. The value of pre-shaping for the suspect list is recall(b) minus recall(a) at
equal list length. Run the replay with exactly the history-read flags production would use,
not full history.

**A corpus without a consumer.** Fast reverts in large public repositories: a revert commit
within 14 days gives the cause (the reverted commit) and a detection time (about the revert
time). Seeds equal to the cause's own files would hand arm (a) the answer, so take one to
three files that share a directory with a cause file but were not touched by it (a failure
often surfaces near the change, not at it; the reading states that assumption). Reverts with
no such file are excluded and counted. Use at least 50 reverts across at least 3
repositories. Known bias: reverts lean toward breakage caught in continuous integration,
not production, so the proxy can kill a facet but only the consumer replay can keep it.

**Bars, fixed before the run (proposed).**

- BUILD the suspect list if either arm reaches recall@20 of at least 0.5 with a median list
  of at most 25 rows.
- The declaration's suspect-list claim holds if recall(b) minus recall(a) is at least 0.15
  at equal list length. A smaller gain does not kill the declaration, whose owners,
  runbook, links and symptoms are valued separately. It removes only the claim that
  pre-shaping improves the suspect list.
- KILL the suspect facet if both arms fall under 0.5 or the median list exceeds 25 rows.

**The consumer replay is the value reading.** The same two arms over at least 10
change-caused incidents (ideally 20) at a named consumer, with blind footprints recorded
before anyone views the cause commit, ref-range windows, and two extra records:
commit-to-deploy lag, and the share of causes invisible to version control (the reopen
evidence in section 7). A tool's own development incidents cannot feed either reading:
they are process incidents from one source, and self-derived ground truth
([practices §21](practices.md#21-self-derived-ground-truth-n1-evidence-and-precision-less-set-grading))
is not evidence of value.

**Stages.**

| Stage | Contents | Starts when | Meter and kill |
|---|---|---|---|
| 0 | The replay (and the consumer replay once one exists) | Any time; it builds nothing | A KILL reading ends stage 1 before it starts |
| 1 | Bounded history read, recency hop in the localization query, untrusted-text handling, one hook line | The replay reads BUILD | Pointer expansion under 10 percent cuts the pointer |
| 2 | The declaration, links, parameters, owners, the answer, the command-line verb, readiness | The pointer is used and a named consumer exists | Cause present in the list; readiness gaps mattered; cold p95 latency reading |
| Later | Deferred rows (probes, next queries, a read-only console view, stored pre-resolution) | Each row's own trigger | Each row's own reading |

**Deferred, each with its trigger.** Probes and next queries (section 6). Footprint
pre-resolution stored at ingest, a durable memo (when a durable-state gate passes and cold
latency is over target). A declaration naming a live map (when live maps exist). Symptoms
checked against alert rules in the index (when a consumer keeps alert rules in the repo and
their extraction is verified). A read-only console view (when a consumer asks after stage
2). A committed rendered snapshot with a hook literal match when the service is down (when a
consumer records a service-down incident). **Refused:** fetching connectors, attach,
executing anything, an incident command center, frequent committers as owners, remote or
fetched declarations, values bound from the environment or keychain, a hook that runs
anything or presents the change list as complete, a model that names a cause.

## 11. Evidence before claim: read the answer basis, then the fact sheet, and cite receipts [design][source]

Added 2026-10-11 from the consumer's design for an evidence plane: CI exports a code index per
service at each commit, an environment server holds the indexes for the commits production
runs and serves them read-only, and every answer says which commit it describes. Nothing here
is built or measured on the engine side. This section stays engine-neutral; the mapping to
one engine's tools, flags and row ids is in [docs/plans/fathom.md](plans/fathom.md).

**The problem.** An agent investigating an incident asks an index about code. The index may
describe another commit than production runs, or a developer's uncommitted bytes, and a
plain answer does not say which. Published benchmarks put incident agents' root-cause accuracy
near 4 to 12 percent ([OpenRCA, ICLR 2025](https://proceedings.iclr.cc/paper_files/paper/2025/file/d29b8d53678015079e1d245c023e49d2-Paper-Conference.pdf);
[a 2026 replication](https://arxiv.org/pdf/2602.09937v1)) [source]. A root-cause write-up
that states code facts without a pointer cannot be checked, so a wrong fact survives review.

**1. Name the commit first.** Name the deployed commit as a typed value (section 5) before the
first code question. Ask an index that serves that commit read-only and never refreshes itself
during the incident; a live index can re-ingest mid-incident, and an index snapshot cannot
change.

**2. Weigh every answer by its basis.** The answer basis is a value in the answer, never a
warning beside it (section 3). It names the commit each answer read and one state.

| Basis | Means | The responder |
|---|---|---|
| `exact` | the caller named a commit, and the index's commit equals it | may state it as a fact about that commit, with its receipt |
| `differs` | the caller named a commit, and the index answered at another one, which the basis names | treats it as a lead, names the index's commit, asks again where the deployed commit is served |
| `unchecked` | the caller named no commit; the basis still names the commit the index read | treats it as a lead until that commit is confirmed as the deployed one; asks again with the deployed commit |
| `uncommitted` | a local index answered over bytes not committed | never states it as a fact about production; labels it "local working tree" |
| `behind N` | the caller's own git counts N commits from the basis commit to the deployed one | treats it as `differs` for deploy facts and lists the files those N commits touched; never upgrades it |
| none served | an engine that reports no basis yet | treats every answer as `unchecked` and reads text at the commit itself (`git show <commit>:<path>`) |

A gate that decides a deploy accepts only `exact`. Anything else takes the fail-open path in
point 6.

**3. Read the fact sheet first.** A fact sheet is the engine's facts for one service at one
commit, stored before the incident, plus joins the engine computes when asked: a liveness probe
whose handler reaches a database, retries times timeout along each hop, where an alert routes,
what a role can reach. Each fact carries anchors and a coverage statement, and no grade. One
read answers part of facets 4 to 6 (what we already knew, knobs, reach). Read order is not
answer order: the sheet is read first because it is one cheap read at a named commit, and the
answer still follows responder order (section 2), with what changed labeled as a lead.

**4. Cite a receipt for every code fact in the write-up.** A receipt names the anchor (path,
line range, commit, git blob id, excerpt digest, anchor state) and who can re-derive it: git
alone, a named toolchain, or the engine at a named version. A fact with no receipt is written
as a hypothesis and labeled. An absence ("no retry budget", "no disruption budget selects this
workload") states its scope, its snapshot and its coverage, and incomplete coverage reads as a
lower bound (decision D10). A withheld anchor (a file that may hold secrets) stays withheld:
no excerpt, no digest, no reconstruction. Excerpts and commit subjects are untrusted text
(section 4). The cause is the responders' judgment and is labeled as judgment; the engine
never names one (section 8).

**5. The engine's silence is not evidence.** One consumer measured hook-path calls abandoned
under load: a localization call 200 times out of 223 at a two-second deadline, each one simply
absent (the consumer's own call log, 2026-10-09 to 2026-10-11, not yet committed as a
reading) [harness]. Ask directly with a deadline, and read a missing answer as "not computed:
deadline".

**6. Fail open, by name.** If the engine is down or stale, or its basis is not `exact`, the
response goes on: mitigation never waits on evidence. Record `evidence: unavailable` and run the
evaluation afterwards ([practices §20](practices.md#20-fail-open-gates-mistaken-for-gates)). A
rollback to a commit that already has an evaluation, or a break-glass deploy, is admitted the
same way.

**7. Close the loop into prevention.** The write-up names the declaration it exercised
(section 9), a test that certifies the fix, and, when the cause was invisible to the engine, the
fact kind that would have shown it. A review tool decides whether the fix holds; the evidence
engine does not grade.

**Kit shapes that carry it.** `foundations/quality/structural-refusal` keeps `unknown` in the
value; `foundations/telemetry/read-manifest` makes an incomplete read a named lower bound;
`foundations/evidence/evidence-span` and `foundations/evidence/evidence-producer` carry a
fact's locator and digest and cap its tier by its producer, refusing a circular origin;
`foundations/quality/honesty-gate` hedges or blocks an overclaim;
`foundations/quality/integrity-evaluator` refuses an unresolvable reference;
`foundations/security/untrusted-content-wrapper` fences excerpts. Known gaps: a span has no
commit, git blob id or anchor state, and its author is human or ai only, so a deterministic
tool's fact cannot pass `ValidateAsEvidence` honestly today ([futures F20](intent/futures.md)).

**Where the procedure lives.** This section is the doctrine. A skill that binds it to one
engine's tools belongs to that engine (proposed
[decision 0019](intent/decisions/0019-incident-evidence-doctrine-in-kit-engine-binding-in-engine.md);
[futures F19](intent/futures.md)). An agent built on the kit that registers the engine's tools
declares each tool's effect (decision 0002): queries `read`, anything that ingests or writes
otherwise. A read-only server is still a claim until a hostile sweep shows it writes nothing
([practices §18](practices.md#18-irreversible-actions-without-a-structural-gate)).

## Checklist

**Before building anything:** run the replay with the bars written down first; name a
consumer; confirm the delivery channel is one the agent cannot skip.

**When adopting the practice:** the declaration is committed and owned; every facet has its
three-state reading; the change facet prints its horizon and blind spots; links show hosts
and are never fetched; the deployed revision is a typed parameter; nothing renders that
could run; commit subjects and pasted text are fenced as untrusted; readiness reports
facts, and a stale declaration is flagged, not deleted.

**At the incident:** freshness first, owners with the matching rule, what changed labeled
as a lead, then prior incidents, knobs, reach, the runbook and links. Say which facets were
not computed and why.

**When an evidence engine answers:** name the deployed commit, read the basis on every answer,
read the fact sheet first, cite a receipt for every code fact in the write-up, write an absence
with its scope and coverage, and record `evidence: unavailable` when the engine cannot answer.
