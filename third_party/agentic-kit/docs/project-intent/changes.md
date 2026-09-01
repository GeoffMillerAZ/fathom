# Change records — commits, changelog, breaking changes, and where the rest goes

Sibling to the Project Intent practice ([README.md](README.md)). Intent records say why
the system is shaped this way. Change records say what changed, for whom, and what it
costs to follow. They link to each other and never repeat each other.

## 1. What AI coding actually changes

Git already records every change. The changelog's job was never to record; it is to
tell a specific audience what they must know. Agents raise the commit rate tenfold, and,
left alone, they also write changelog entries that carry rationale, evidence, and
verification narrative because those have nowhere else to go. Volume times verbosity is
what makes the readable log unreadable. The fix is not a second log. It is a routing
rule per layer with a budget per layer.

Evidence from this repo at the time of writing: the `[Unreleased]` block was 248 lines,
individual entries ran to 500 words, and `v0.7.0` had been tagged without rolling the
block. Every one of those is a routing failure, not a diligence failure.

## 2. The layers

| Layer | Answers | Audience | Lives in | Budget | Lifetime |
|---|---|---|---|---|---|
| L0 session | what was tried, rejected, verified in this session | the next session's agent | thread notes, plan dispositions, PR body | unbounded | dies with the plan |
| L1 commit | what changed and why, at the smallest reviewable unit | reviewer, bisect, blame | git | subject ≤ 72 chars, body ≤ ~10 lines | permanent, never rewritten after merge |
| L2 changelog | what the audience must know since the last release | consumers, the owner catching up | `CHANGELOG.md` `[Unreleased]` | ≤ 2 sentences per line | collapsed while unreleased, frozen at release |
| L3 migration | how to cross a breaking change | upgraders | `docs/upgrades/<version>-<slug>.md` | as long as needed | permanent |
| L4 decision | why, and when to reopen | future designers | `docs/intent/` decisions | ≤ 80 lines | superseded, never rewritten |

**Routing rule.** Each fact lives in exactly one layer; the others link to it. Rationale
goes to L4, or to the L1 body when it is below the decision threshold. Evidence goes to
L1's `Verified:` line and to L0. The user-visible effect goes to L2. Migration steps go
to L3. A changelog line that explains *why* is a decision record leaking; a commit body
that explains *what a user sees* is a changelog line leaking.

## 3. Commits (L1)

```
<type>(<scope>)<!>: <subject in the imperative, ≤ 72 chars>

Why: <one or two lines, or "see ADR-007">
Verified: <test names | task check | fathom verify <plan> row X | not run: <reason>>
Refs: <plan rows, decision ids, spec ids>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `build`, `chore`. `!` marks a
breaking change. Scope is a package or a surface. Trailers: whatever the harness adds and
whatever the repo's own rule says; nothing hand-written beyond that.

- **One commit per reviewable unit.** A plan row is the natural unit. WIP and fixup
  commits are squashed before merge. History is never rewritten after merge.
- **Solo on main** (no merge step): agents batch until the unit is done and commit once.
  No WIP commits on main.
- **`Verified:` is mandatory and honest.** `not run: <reason>` is allowed. Silence is not.
  This is the AI-era addition: the commit carries its own proof pointer, so "done means
  proven" survives into history.
- **Several agents at once**: one unit per agent, one worktree per agent, the
  orchestrator integrates, shared files are append-only rows, worktrees are removed on
  completion ([operating.md](operating.md) §4).

## 4. Changelog (L2)

**Threshold.** A line exists when the change is observable to the changelog's audience:
behavior, a CLI verb or flag, an API or exported type, a config key, a file format, a
default, an exit code, a dependency with user impact, perceptible performance, or
anything on the compatibility surface. Not: refactors, tests, CI, internal renames, docs
(unless docs are the product), tooling wiring.

**Format.** Keep a Changelog categories, `### Breaking` first, then Added, Changed,
Deprecated, Removed, Fixed, Security. One line, at most two sentences: what changed, for
whom, and a link to the decision, spec, or plan that holds the why and the evidence.
Written in the same change as the code, by the agent that made the change.

**Collapse while unreleased.** The `[Unreleased]` block describes the delta from the last
release, never the journey. A fix to an unreleased addition folds into the addition. An
extension rewrites the line. A removal deletes the line. Released blocks are frozen.

**Release impact line.** The first line of `[Unreleased]`:
`Release impact: major | minor | patch`, maintained as the highest category present
(Breaking → major, Added or Deprecated → minor, otherwise patch). Pre-1.0 repos state
`pre-1.0: breaking bumps minor` on the same line. The agent that adds a line updates it.
This is the semver advisement; it costs one edit.

## 5. Breaking changes and migration (L3)

Breaking means any change to the **compatibility surface**, which the repo names in
`docs/intent/scope.md` (the `--json` envelopes, schema keys, on-disk formats, verbs and
flags, whatever that repo promises). That list is intent: a promise about what will not
change without ceremony.

- The commit carries `!`. The changelog line sits under `### Breaking` and names what
  breaks and who is affected, in one line.
- When "how to migrate" exceeds one sentence, it goes to `docs/upgrades/<version>-<slug>.md`
  and the line links to it. The readable log stays readable.
- **Potential breakage is not a log.** If a real consumer may be affected, the line gets
  a `Watch:` clause. If the concern is structural (a promise the project might not keep),
  it is a decision record with a `Revisit when`. Everything else is L0.
- **Deprecation** is announced under Deprecated with its removal condition (a version, or
  a `Revisit when`). The removal, when it comes, is Breaking.

## 6. Release ritual

1. Check the `Release impact` line against the block's contents.
2. Rename `[Unreleased]` to `[x.y.z] — YYYY-MM-DD`. Open a new empty `[Unreleased]` with
   `Release impact: patch`.
3. Every `### Breaking` line has its migration note.
4. Tag. The block moves first, then the tag, never the reverse.
5. Run the intent `refresh` walk: goals with `Done when` met, futures with `Promote when` met.

## 7. The owner catching up

Not a file. The owner who did not write the code reads three things:
`git log --oneline --since='7 days'` filtered by type, the L0 thread brief, and the
`[Unreleased]` block. If catching up needs a fourth document, the commit subjects are the
defect to fix.

## 8. Per-repo wiring

The intent router does not change. Change records need to be known at *every* commit,
so their rule lives in the repo's always-loaded conventions, four lines at most:

```markdown
Change records: conventional commit with a `Verified:` line, one commit per reviewable
unit. A `CHANGELOG.md` line in the same change only for observable behavior (compatibility
surface: `docs/intent/scope.md`); `### Breaking` first; collapse `[Unreleased]` to the
delta, never the journey. Rules: `agentic-kit/docs/project-intent/changes.md`.
```

Plus `CHANGELOG.md` from `templates/CHANGELOG.md` and a **Compatibility surface** section
in `scope.md` (the template has it).

## 9. Adopting in a repo with a verbose changelog

Do not rewrite released blocks. Apply the budget from the next entry on. As entries are
touched, move their rationale into decision records and their evidence into the commit
that already holds it. A verbose changelog is usually a repo without decision records;
fix the cause.

## 10. Considered and rejected

- **A micro changelog file.** Git is the micro log. A second one drifts.
- **A changelog line per commit.** Below the threshold it is noise; above it, the line
  already exists.
- **A separate "concerns" log.** Concerns without a consumer are L0; concerns with a
  structural cause are decisions. A third file would collect the ones nobody acts on.
- **Generated changelogs (git-cliff, conventional-changelog).** The agent writes the L2
  line in the same change, so generation solves a problem this workflow does not have.
  Adopt when lines are being missed, not when volume feels high.
- **A "what agents did" digest file.** Section 7. The commit subjects are the digest.
