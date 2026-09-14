# GitHub Rulesets: making `fathom verify` and the compiled layer rules required

VB-15 (`docs/plans/verbs.md`; design `docs/vision/29-verbs-not-lookups.md` §3
wave C; ADR-035 decision 3): "governance moves from advice to enforcement" only
means something if a violated rule can actually block a merge. This page is
the stranger's path from a fresh clone to a branch protected by two required
checks:

- **`fathom / verify`** — the done-means-proven gate (`docs/usage.md`'s
  `verify` section): every `[x]` in a `docs/plans/*.md` checklist re-proven
  against a fresh index, no daemon required in CI.
- **`fathom / layer-rules`** — the compiled governance gate (VB-13/VB-14):
  `go test ./rules/...` over the file `fathom rules gen-test` writes from
  `.fathom/guard.cue`'s `layer_rules`, evaluated live against this checkout's
  real `go/packages` import graph.

Read this top to bottom once; after that, only the two file contents below
(the workflow and the ruleset JSON) need to exist in your repo.

## 1. One-time setup in the repo

```sh
# 1. author at least one layer rule (see docs/plans/verbs.md VB-13 for the
#    #LayerRule shape; schema/guard/schema.cue is the source of truth)
$EDITOR .fathom/guard.cue

# 2. generate the compiled test — commit the output like any other
#    generated file (protobuf, mockgen, ...); CI never needs to invoke
#    `fathom rules gen-test` itself, because the generated file re-reads
#    guard.cue LIVE at `go test` time (editing a rule's globs or sentence
#    later needs no regeneration — only moving guard.cue or the output
#    directory does)
fathom rules gen-test --root . --out rules
git add rules/layer_rules_test.go .fathom/guard.cue
git commit -m "chore: compile layer rules to a generated test (VB-14)"
```

`rules/layer_rules_test.go` is a normal Go test in its own package (`rules`
by default — `--out`'s basename), importable from any module because it goes
through the **public** `github.com/geoffmilleraz/fathom/pkg/layerrules`
facade, never an `internal/...` import path (which the Go compiler refuses
outside fathom's own module tree — this is exactly what makes it safe to
generate into a repo that merely *depends on* fathom rather than living
inside it).

## 2. The CI workflow: two jobs, two check names

GitHub names a required status check by `<workflow name> / <job name>`
(the `name:` at the top of the workflow file, then each `jobs.<id>.name` or
`<id>` itself when `name:` is omitted). The workflow below is named `fathom`
with jobs `verify` and `layer-rules`, so the two checks a PR reports — and
the two `context` strings a Ruleset can require — are **`fathom / verify`**
and **`fathom / layer-rules`**, exactly the two names used throughout this
page. Rename either and update the Ruleset JSON in step 3 to match — the
check name is the only thing that has to agree between the two files.

`.github/workflows/fathom.yml`:

```yaml
name: fathom

on:
  pull_request:
  push:
    branches: [main]

jobs:
  verify:
    name: verify
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: build fathom
        run: go build -o bin/fathom ./cmd/fathom
        env:
          CGO_ENABLED: 1
      - name: ingest into a throwaway store (no daemon needed in CI)
        run: |
          mkdir -p workspace/db
          bin/fathom ingest --service ${{ github.event.repository.name }} \
            --path . --store workspace/db/ci.sqlite
      - name: verify every plan
        run: |
          bin/fathom verify --service ${{ github.event.repository.name }} \
            --plan docs/plans/*.md --store workspace/db/ci.sqlite \
            --strict --quiet

  layer-rules:
    name: layer-rules
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: run the compiled layer rules
        run: go test ./rules/...
```

`verify`'s exit codes are 0 (every claimed task verifies), 1 (a claimed
`[x]` is unproven — the check must fail), and 2 (operational: empty index,
service never ingested). A workflow step that exits non-zero fails the job
by default, so no extra handling is needed for either 1 or 2 — but if your
repo has no `docs/plans/` yet, drop the `verify` job (or point `--plan` at
whatever plan files you do have) rather than shipping a job that always
exits 2.

## 3. The Ruleset: making both checks required

GitHub Rulesets (Settings → Rules → Rulesets → New ruleset, or the API/CLI
below) replace the older "branch protection rules" and are the recommended
surface going forward. Paste this JSON with **Import a ruleset** (or POST it
to `/repos/{owner}/{repo}/rulesets` — see step 4), or use it as a reference
for filling in the UI form by hand:

```json
{
  "name": "fathom governance",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/main"],
      "exclude": []
    }
  },
  "rules": [
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": true,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": true,
        "do_not_enforce_on_create": false,
        "required_status_checks": [
          { "context": "fathom / verify", "integration_id": null },
          { "context": "fathom / layer-rules", "integration_id": null }
        ]
      }
    }
  ],
  "bypass_actors": []
}
```

The `pull_request` rule is optional (drop it if you do not want a review
requirement bundled with this ruleset — it is included only as a realistic
example of a ruleset that carries more than one rule type). The rule that
matters for VB-15 is the `required_status_checks` block: its
`required_status_checks` array is what makes GitHub refuse to merge a PR
until both `fathom / verify` and `fathom / layer-rules` report success, and
`strict_required_status_checks_policy: true` additionally requires the
branch to be up to date with `main` before merging (so a check result can
never be stale relative to the base it is protecting).

Each entry's `context` must be **byte-identical** to the check name GitHub
actually reports on a PR (step 2's `<workflow name> / <job name>`) — a
mismatch is silent: GitHub simply never sees a matching check, so the
required check stays perpetually pending and the PR can never merge. If a
check is stuck pending, this is the first thing to verify — open a real PR,
look at the exact check names in its "Checks" tab, and diff them against
this JSON byte for byte.

## 4. Applying it (two ways)

**UI**: Settings → Rules → Rulesets → New branch ruleset → Import a ruleset
→ paste the JSON above → Create.

**CLI** (`gh`, once authenticated with `repo` scope):

```sh
gh api \
  --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/{owner}/{repo}/rulesets \
  --input ruleset.json
```

where `ruleset.json` is the file from step 3. `gh api /repos/{owner}/{repo}/rulesets`
(GET) lists existing rulesets — including their numeric `id` — needed to
`PATCH` or `DELETE` one later.

## 5. Verifying it actually bites (the stranger's proof, not a claim)

Do not take this page's word for it. Prove it on your own repo:

1. Edit `.fathom/guard.cue` to add a rule your own code currently violates
   (or seed one file that violates an existing rule).
2. Open a PR. The `fathom / layer-rules` check must go red, and the PR's
   merge button must be disabled ("Required status check `fathom / layer-rules`
   has not succeeded" or similar, depending on GitHub's UI wording that week).
3. Revert the violation. The check must go green and the merge button must
   re-enable, with no other action.
4. Repeat for `fathom / verify`: check off a plan-row box with no code
   behind it — the same red-then-green cycle must hold.

If any of the four steps does not hold on your repo, the ruleset JSON's
`context` values do not match your workflow's real check names (§3's last
paragraph) — fix that before trusting the gate.

## What this page does not claim

This is a template and a walkthrough, not a measurement. Whether a compiled
layer rule catches a real regression in *your* history, and whether the
gated arm out-performs an advisory-only arm, is VB-16's job
(`docs/plans/verbs.md`) — an operator-spend replay this page does not run.
Nothing here is evidence that governance-as-a-required-check changes an
outcome; it only proves the mechanism wires up the way this page describes.
