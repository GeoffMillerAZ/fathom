# GitHub Action: `fathom review --format github` as a PR comment and a required check

VB-23 (`docs/plans/verbs.md`; design `docs/vision/29-verbs-not-lookups.md` §3
wave F; ADR-035 decision 4/R11): `review_diff`/`fathom review` are
"deterministic and cited and reach no human" (R11) — this page closes that
gap. It builds a scratch index for the checkout, runs
`fathom review --format github` (VB-22) against the PR's diff, and upserts
ONE comment on the PR — found by VB-22's fixed marker line
(`<!-- fathom-review -->`), so a second push updates the same comment
instead of piling up a new one — while also reporting a `fathom / review`
status check that a Ruleset (`docs/adoption/github-rulesets.md`) can require.

Read this top to bottom once; after that, only the workflow file below needs
to exist in your repo.

## 1. What the workflow does, in order

1. Check out the PR's head commit.
2. Build `fathom` (a pinned release binary, or `go install` — §2 below).
3. Ingest the checkout into a **throwaway** SQLite store under `workspace/db/`
   (no daemon needed in CI — the same pattern `github-rulesets.md`'s
   `verify` job uses).
4. Run `fathom review --format github --since origin/${{ github.base_ref }}`
   against that store, capturing the Markdown to a file.
5. Upsert the PR comment: list existing comments via the GitHub API, find one
   whose body contains the marker line, `PATCH` it if found, `POST` a new one
   otherwise.
6. Exit non-zero (failing the `fathom / review` check) only when the review
   itself could not run (ingest failed, the tool errored) — a clean verdict
   with active findings still exits 0: this check's job is "the receipt
   posted", not "the diff is clean". Gate on content instead via
   `fathom / verify` and `fathom / layer-rules` (`github-rulesets.md`), which
   already fail for the right reasons.

## 2. Installing fathom in the runner

Two options — pick one:

**A. A pinned release** (fastest; replace `vX.Y.Z` and the asset name with a
real published release of your fathom fork/build):

```yaml
      - name: install fathom
        run: |
          curl -sSL -o fathom.tar.gz \
            https://github.com/${{ github.repository_owner }}/fathom/releases/download/vX.Y.Z/fathom_linux_amd64.tar.gz
          tar -xzf fathom.tar.gz
          chmod +x fathom
          echo "$PWD" >> "$GITHUB_PATH"
```

**B. `go install`** (no release needed; builds from source each run):

```yaml
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: build fathom
        run: go build -o bin/fathom ./cmd/fathom
        env:
          CGO_ENABLED: 1
      - run: echo "$PWD/bin" >> "$GITHUB_PATH"
```

The rest of this page assumes `fathom` is on `PATH` either way.

## 3. The exact permissions block

The comment-upsert step calls the GitHub REST API with the workflow's own
`GITHUB_TOKEN` — no PAT, no secret to provision. It needs exactly:

```yaml
permissions:
  contents: read        # actions/checkout
  pull-requests: write  # list + create/update the PR comment
```

Nothing else. Do not grant `write-all` or omit the block (a workflow with no
`permissions:` key inherits the repo's default, which may already be
read-only and silently fail the comment step with a 403 — fail loud instead
by declaring the block explicitly).

## 4. The full workflow

`.github/workflows/fathom-review.yml`:

```yaml
name: fathom

on:
  pull_request:

permissions:
  contents: read
  pull-requests: write

jobs:
  review:
    name: review
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # review diffs against origin/<base_ref>; a shallow clone has no such ref to diff against

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: build fathom
        run: go build -o bin/fathom ./cmd/fathom
        env:
          CGO_ENABLED: 1
      - run: echo "$PWD/bin" >> "$GITHUB_PATH"

      - name: ingest a throwaway index for this checkout
        run: |
          mkdir -p workspace/db
          fathom ingest --service ${{ github.event.repository.name }} \
            --path . --store workspace/db/ci.sqlite

      - name: render the review comment
        run: |
          fathom review --format github --no-persist \
            --service ${{ github.event.repository.name }} \
            --since origin/${{ github.base_ref }} \
            --store workspace/db/ci.sqlite \
            --out review-comment.md

      - name: upsert the PR comment
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const body = fs.readFileSync('review-comment.md', 'utf8');
            const marker = '<!-- fathom-review -->';
            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              per_page: 100,
            });
            const existing = comments.find(c => c.body && c.body.includes(marker));
            if (existing) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: existing.id,
                body,
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body,
              });
            }
```

`--no-persist` (cmd/fathom/review.go) matters here: a CI runner's throwaway
store is discarded after the job, so writing findings to its claims store
would only be lost work — the same reasoning the pre-push hook's
`--no-persist` summary already relies on.

`--out review-comment.md` writes the rendered comment straight to that file
(stdout then carries only a one-line confirmation) instead of a shell `>`
redirect — the same bytes either way, but `--out` also fails the step
loudly if the file cannot be written, rather than silently truncating it.

`fetch-depth: 0` is load-bearing: `actions/checkout`'s default shallow clone
fetches only the PR's head, so `origin/${{ github.base_ref }}` does not exist
in the runner's git history and `git diff --name-only` (which `fathom review`
shells out to) fails with "unknown revision". A full checkout costs more
runner time on a large repo; if that matters, `fetch-depth` can be narrowed
to a value that reliably covers the PR's actual commit count instead of 0.

## 5. The `fathom / review` status check under a Ruleset

The job above is named `review` inside a workflow named `fathom`, so the
check GitHub reports on a PR — and the `context` string a Ruleset requires —
is **`fathom / review`**, following the same `<workflow name> / <job name>`
rule `github-rulesets.md` §2 documents for `fathom / verify` and
`fathom / layer-rules`. Add it to that page's ruleset JSON
(`required_status_checks.parameters.required_status_checks`) to require it
alongside them:

```json
{ "context": "fathom / review", "integration_id": null }
```

Requiring `fathom / review` means "a receipt was posted for this diff" is a
merge gate — not "the receipt is clean" (§1 above explains why this check
never fails on findings). If your repo wants a required check that DOES fail
on a real regression, that is exactly what `fathom / verify` and
`fathom / layer-rules` are for; combine all three.

## 6. Verifying it actually bites (the stranger's proof, not a claim)

1. Open a PR that changes at least one Go symbol with an existing caller.
   The `fathom / review` check must appear and turn green, and a comment
   starting with `<!-- fathom-review -->` must appear naming the changed
   symbol, its caller(s) and their resolution tier, and the test gap.
2. Push a second commit to the same PR. The SAME comment must update in
   place (check its "edited" timestamp) — a second comment must NOT appear.
3. Delete the `pull-requests: write` line from `permissions:` and push
   again. The `upsert the PR comment` step must fail with a 403 (or
   equivalent "Resource not accessible" error) — proving the permissions
   block in §3 is load-bearing, not decorative.
4. Revert step 3. Open a PR against a `docs/plans/verbs.md` row whose
   anchored symbol you touch — the comment's `### Verify` section must name
   that row and the `fathom verify` command to run.

If any of these does not hold, check the exact `context` string GitHub
reports on a real PR's Checks tab against §5's JSON — the same byte-for-byte
requirement `github-rulesets.md` §3 documents.

## 7. Committing the review-kit outbox (optional)

The job above is fathom's OWN PR-comment review. A SEPARATE, optional step —
in this workflow or another — lets the review kit's own findings reach a
LATER coding session with no daemon and no network hop: `leadsman review`
(the review kit's CLI, a peer process, never a fathom dependency — vision
30 §2) writes its projection outbox, and the workflow commits it back to
the branch:

```yaml
      - name: run the review kit and commit its outbox
        run: |
          leadsman review --target fathom \
            --outbox .reviewkit/outbox/projection.json
          if ! git diff --quiet -- .reviewkit/outbox/projection.json; then
            git config user.name "github-actions[bot]"
            git config user.email "github-actions[bot]@users.noreply.github.com"
            git add .reviewkit/outbox/projection.json
            git commit -m "chore(reviewkit): update findings outbox"
            git push
          fi
```

The next `fathom findings import --service S --subject ORG/REPO/COMPONENT
--file .reviewkit/outbox/projection.json` (by hand, or automatically —
`docs/configuration.md`'s `[findings] outbox` setting makes the SessionStart
hook run it once per session) is what actually gets these findings served
at the edit; committing the outbox only gets them OUT of the CI runner.
`docs/adoption/findings-loop.md` covers the full loop, including the local
variant (no CI at all — `leadsman review` then `fathom findings import`
from a workstation) and every honest failure shape (no subject provisioned,
an unknown producer, a stale or conflicting replay).

## What this page does not claim

This is a template and a walkthrough, not a measurement. Whether the
comment's findings actually help a real reviewer catch something a plain
diff view would miss is VB-24's job (`docs/plans/verbs.md`) — a replay over
fifty to sixty historical merged-then-fixed PRs this page does not run.
Nothing here is evidence that a posted comment changes reviewer behavior; it
only proves the mechanism wires up the way this page describes.
