# Repository quality boundaries

`quality-gate` adds a small, versioned boundary over exact Git snapshots. [The design and acceptance contract](designs/quality-boundaries.md) defines its exact scope. It checks five deterministic properties: conflict markers, forbidden paths, file byte limits, JSON syntax, and a required companion path. An optional decision model can add a review hold. It cannot clear a failed or missing required check, grant authority, or prove a claim true.

This is a portable check for a project's own policy, not a replacement for its tests, linters, security scanners, review, or protected branch. The kit's `task validate` remains its gate. [Workflow assurance](intent/assurance.md) distinguishes implementation, integration, adoption, and measured benefit; a generated hook proves none of those on its own.

## Adopt one repository

Build from a reviewed, trusted kit revision into a directory outside incoming project content (`go build -o /absolute/trusted/bin/quality-gate ./cmd/quality-gate`), put it on `PATH`, and retain `go version -m /absolute/trusted/bin/quality-gate` as its revision record. No package or global hook installer runs automatically.

Run the binary from that trusted installation. `init` creates only a new directory and refuses an existing one; inspect the generated policy, CUE shape, and Lefthook fragment before merging anything:

```sh
quality-gate init --dir .quality-gate
quality-gate validate --policy-file .quality-gate/policy.json
quality-gate site-digests --policy-file .quality-gate/policy.json
quality-gate check --repo . --boundary pre-commit --staged --observe --policy-file .quality-gate/policy.json
```

Start with `model.provider: "none"`. Set `rules[].boundaries` and selectors for the repository's real paths. Keep the policy in Git. Use `--index-file PATH` only when an explicit alternate index is the intended staged input. Snapshot limits count all changed before/after blobs, including paths outside a rule selector. New refs scan the entire proposed tree. Oversized snapshots return unknown; tune the bounded policy limits deliberately rather than truncating evidence.

A worktree scan is not equivalent to the staged tree: unstaged edits must not change the verdict.

The generated `lefthook.yml` is a **fragment**. Its commit gate lives in **`commit-msg`**, where it checks the index against the `pre-commit` policy boundary. Lefthook 1.13.6 skips its entire `pre-commit` runner on an empty staged set; the installed `commit-msg` path was tested with empty commits. Keep existing pre-commit and commit-msg jobs. Run index-changing formatters before this check; local hooks cannot fence later index mutations, so CI must verify the committed artifact. Merge its named command into the project's existing hook, retain every existing command, and inspect the effective result with `lefthook dump`. Lefthook `extends` can override commands with matching names, so check the final configuration rather than file presence. A pre-push command needs `use_stdin: true`; Lefthook forwards hook stdin to only one consumer when multiple commands request it. Give that one command the Git-provided `local_ref local_oid remote_ref remote_oid` rows, or explicitly fan them out yourself. Never guess a base from `HEAD^`. Git reports new refs with a zero remote object ID; scan their full tree. Report deletions; treat an unavailable base as unknown, not clean. [Git hook input](https://git-scm.com/docs/githooks) and [Lefthook stdin](https://lefthook.dev/configuration/use_stdin/) define these behaviors.

Validate both layers with `cue vet -d '#Policy' .quality-gate/schema.cue .quality-gate/policy.json` and the CLI validator. In the tested CUE version, `cue vet -c` can accept an omitted required collection; the CLI separately requires `rules`, `model.sites`, and present calibration `cases`. Passing CUE alone is insufficient. After examining an observe report, prove the actual installed hook rejects a planted violation in a disposable repository or temporary index. Test a missing policy, unavailable Git object, or malformed hook input as well. Restore the fixture and show the previous commands still run. Then remove `--observe` and pin the policy to committed Git content:

```sh
quality-gate check --repo . --boundary pre-commit --staged --policy-ref HEAD --policy-path .quality-gate/policy.json
```

`--policy-file` is for bootstrap and local policy validation. Enforcement reads `--policy-path` from `--policy-ref`; it cannot trust an unstaged policy rewrite. Exit `0` means pass or an explicit observe run, `1` a policy hold, and `2` an unknown or invalid invocation. A required check that did not run is never a pass. `--format json` emits typed reports for CI; the default is concise text. Reports include policy and snapshot identity, rule/site outcomes, and reasons, without raw source or secret values.

## CI is the authoritative boundary

Install a pinned, trusted binary outside incoming change content. Git is part of that trusted runtime: `--git-bin /absolute/trusted/git` pins its location. The gate rejects a Git executable located inside the candidate checkout, including symlinks, and clears Git redirection variables; the CI workflow, runtime and OS still need a trusted host. Check the proposed revision against the trusted base's policy, then require that CI status on the protected branch:

```sh
quality-gate check --repo . --boundary ci --base "$BASE_SHA" --revision "$HEAD_SHA" --policy-ref "$BASE_SHA" --policy-path .quality-gate/policy.json --format json
```

## Diagnose configuration before a run

```sh
quality-gate doctor --repo . --boundary pre-commit --policy-ref HEAD --format json
# During bootstrap, before the policy is committed:
quality-gate doctor --repo . --policy-file .quality-gate/policy.json
```

`doctor` reports check IDs, reasons and next actions for trusted Git, strict policy,
required boundary coverage and required model prerequisites. Local-file policy is
labeled untrusted bootstrap input. It makes no provider calls, writes no cache or
configuration, and never prints credential/environment values. Shadow-only model
readiness stays advisory. Requested model aliases may differ from expected returned
model IDs. Calibration checks whether any allowed clear confidence band is eligible
at the recorded time; runtime still checks the actual answer and confidence.
Exit 0 means required **configuration** checks passed;
exit 2 means invalid, missing or unknown configuration. Neither proves a proposed
snapshot passed, a provider works, hooks fired or protected CI is configured.
CI/publish diagnosis requires an explicit trusted `--policy-ref` just like enforcement.

CI and pre-publish enforcement require a committed `--revision` and explicit `--policy-ref`; staged input or omission exits 2. Explicitly choosing `HEAD` is still the caller's responsibility and is unsafe when it is proposed content.

If a change edits the policy, the trusted base policy still judges that change. Review and adopt the new policy in a later trusted base. A local Git hook can be bypassed and is not atomic with a commit; immutable Git object reads detect work in flight, but do not make local hooks an authority boundary. Do not compile the trusted gate from the proposed change. For untrusted forks, run without secrets or remote model access; do not use `pull_request_target` to execute incoming code. Require the unique CI status through [protected branch rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches). Verify a negative CI fixture fails before declaring the status required.

## Optional judgment sites

One site asks one bounded typed `clear | concern | insufficient` choice about selected changed bytes. Useful advisory questions include possible policy contradictions, missing test coverage as a review hint, unsupported prose claims, and ambiguous agent instructions. A model cannot establish exhaustive conflict detection, test adequacy, source truth, or instruction authority. [Decision 0026](intent/decisions/0026-reject-model-only-authority-and-completeness.md) keeps those floors independent. OpenAI [Decisions](https://developers.openai.com/api/docs/guides/decisions) and TypeSafe [Jev](https://docs.typesafe.ai/introduction) document structured choices; their native probabilities do not certify local accuracy.

Keep sites in `shadow` while collecting independent human labels. A `hold` site may clear only with unexpired calibration bound to its exact site, question digest, provider, and resolved model, including adequate per-class labels and confidence evidence for prior predictions of the same answer class. Lots of correct concern predictions cannot hide false clears. `site-digests` provides the digest after a question edit. Unknown, refusal, parse failure, changed model, expiry, or insufficient evidence holds a required site. Send holds to the repository reviewer to fix the input or review the trusted policy. This CLI grants no automatic waiver or model-authorized override. Do not copy the model's own answers into the label set or call one replay many independent observations. Measure false clears, false holds, unknowns, latency, actual cost, and review time before promoting a site. A model concern may add a hold; no model answer removes a deterministic failure.

All configured model sites in one policy share their selected context. Calibration now binds the full site cohort, fixed instructions/options and state/confidence recipe. Adding another site or changing its scope invalidates existing hashes. Use separate policies/invocations for separate disclosure scopes. Re-evaluate actual labeled cases under the new recipe; changing only `question_sha256` does not supply new evidence.

Remote providers require a narrowly selected, trusted policy scope, environment-held credentials/model pins, and explicit `--allow-remote` on that invocation. There is no automatic paid call, retry, escalation, or fallback. Budget at most one batch per snapshot with file, byte, and time limits. `model.max_batches` defaults to **one across the entire invocation**, including all pushed refs; explicitly choose 1..32 in trusted policy when needed. Failed provider attempts consume a batch; exact shadow-cache hits do not. A required site with no remaining batch holds as unknown. The complete push receives hard-check and required-model-input preflight before any provider dispatch, so a later bad ref cannot waste an earlier paid call. All HTTP response bodies, including errors, are capped at 1 MiB. Prefer remote shadow screening in CI/push, keeping the commit path fast and offline. `--cache-dir PATH` is optional: exact shadow observations may be reused, while hold decisions never use cache to enforce. Keep code and secrets out of model scope unless that disclosure is explicitly authorized. Provider `none` keeps offline checking available.

## Configure project rules

Add these objects to `rules`, with unique IDs and the boundaries you need. Selectors are literal prefixes/suffixes, never globs. A companion proves a changed path exists, not that its tests or docs are adequate. Deleting a forbidden path is allowed. Deleting a selected contract still requires its changed surviving companion.

```json
[
  {"id":"runtime-artifacts","kind":"forbidden-path","boundaries":["pre-commit","pre-push","ci"],"select":{"prefixes":["workspace/","tmp/"]}},
  {"id":"source-size","kind":"max-bytes","boundaries":["pre-commit","ci"],"select":{"prefixes":["src/"],"suffixes":[".go"]},"max_bytes":131072},
  {"id":"schema-tests","kind":"require-companion","boundaries":["pre-commit","ci"],"select":{"prefixes":["schemas/"],"suffixes":[".cue"]},"companion":{"prefixes":["schemas/testdata/"]}}
]
```

For advisory judgments, select `openai`, `typesafe` (Jev direct) or `openrouter` (Jev transport). The generated env names hold the requested model, exact expected response model and API key. Set them in your secret manager/session, never in policy or hook arguments. Keep `mode: "shadow"` and add `--allow-remote` only to the chosen trusted invocation. Missing credentials/pins produce an unknown observation; a hold site blocks. `site-digests` prints question bindings, not calibration evidence.

## Project recipes and limits

| Project | First policy to try | Keep separate |
|---|---|---|
| agentic-kit | Conflict markers, JSON syntax, forbidden generated/workspace paths, schema companion paths where required | `task validate`, CUE vet, race tests, downstream checks |
| app-kit | Conflict markers, generated artifact limits, JSON config validity; shadow site for ambiguous UI instructions | Visual, keyboard, narrow-screen, and accessibility checks |
| platform/review repository | Required evidence or policy companion files; shadow site for unsupported claim hints | Domain authority, source freshness, review acceptance |
| repository without Fathom | The same Git snapshot checks; no index dependency | Its ordinary test and review gates |

Selectors and required companion rules belong to the adopting project's policy. This command does not run arbitrary repository commands, manage approvals or waivers, maintain a lost-pin ledger, or implement [future F18](intent/futures.md). Record what the installed hook and required CI status actually blocked, at which revision; do not equate a policy file or model score with enforced adoption.

Reproduce the offline installed-hook proof with `scripts/quality-gate-smoke.sh`. [Evidence and review](evidence/quality-boundaries/2026-10-22/lefthook.md) identify tested versions and unverified layers.

To use the skill for another project, ask your coding agent to use `<trusted-kit-checkout>/.claude/skills/quality-gate-adopt/SKILL.md` with that project as the target. The skill resolves its guide from this checkout and scopes mutations to the named target. It is discoverable in this kit through both `.claude/skills` and `.agents/skills`; no global skill installation is implied.

## Read the scope of a result

The JSON/text report identifies configured required rules/sites, shadow sites and changed paths outside selectors. Text lists at most 20 quoted unselected paths; JSON retains the full bounded list. Exclusions have reasons. A boundary with no required rules or hold sites returns unknown; use `--observe` for a shadow-only policy. An omitted or unbound required model receipt cannot turn into success. A pass covers the configured properties and selected changed inputs, not every standard or all repository history.

Calibration eligibility means the **declared site-specific bars** passed. Small samples and permissive bars can be statistically uninformative even when they pass. Choose held-out class coverage and error costs before the study; inspect false clears/holds, Wilson lower bounds and sample counts. No universal accuracy threshold fits every domain. Rounded provider probability sums within 0.01 are accepted conservatively: overfull sums reduce confidence, underfull sums never increase it. This confidence rule is part of the bound recipe.

For other workflows, use [the deterministic working-method designs](designs/deterministic-workflows.md): independent expected coverage, typed bounded investigations, exact action proposals, dependency invalidation, replay and reviewed learning. Their individual integration/adoption remains in the uplift work order.
