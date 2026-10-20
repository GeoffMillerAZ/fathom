---
name: intent-release
description: >
  Run the release ritual for a repo with a Keep-a-Changelog CHANGELOG.md: verify the
  Release impact line against the [Unreleased] contents and the compatibility surface,
  roll [Unreleased] into a versioned block, confirm a migration note exists for every
  Breaking line, advise the tag (never tag before rolling the block), then run the
  intent refresh (goals done, futures promoted, questions answered). Use when cutting a
  version or when a tag exists without a matching changelog block.
---

# intent-release

Part of the `intent-*` family. Rationale: `agentic-kit/docs/project-intent/changes.md`
§4–§6. Tagging and pushing tags are usually ask-first actions in `autonomy.md`; check
before step 6.

## Procedure

Fragments repo (the `[Unreleased]` block is one pointer line to a change-note
fragments directory, `changes.md` §4): steps 1 to 5 read and fold the fragments, below.
A repo without fragments follows the steps as written.

1. **Baseline.** `git describe --tags --abbrev=0` (last tag) and `git log <tag>..HEAD
   --oneline`. Every observable change since the tag has a line in `[Unreleased]`;
   every line corresponds to something still in the tree (not reverted). Fix gaps. With
   fragments, the lines are the fragment files.
2. **Impact.** With fragments, `release-notes Unreleased` prints the fold and its
   `Release impact:`; run `release-notes -check` first. Otherwise recompute `Release impact:` as the max category present (Breaking →
   major, Added or Deprecated → minor, else patch; pre-1.0: Breaking → minor). Any
   change to an item on `scope.md`'s compatibility surface that is not under
   `### Breaking` is a defect: move it.
3. **Collapse.** Apply `intent-prune` pass 5 so the block is the delta, not the journey
   (with fragments: collapse the fragments).
4. **Migration notes.** Every `### Breaking` line whose fix exceeds one sentence links to
   `docs/upgrades/<version>-<slug>.md`, and that file exists. Write the missing ones.
5. **Roll.** With fragments: `release-notes -roll x.y.z -date YYYY-MM-DD` writes the
   block under the pointer line, `### Breaking` first, and deletes the fragments; commit
   both together. Otherwise rename `## [Unreleased]` to `## [x.y.z] — YYYY-MM-DD`. Open a new
   `## [Unreleased]` above it with `Release impact: patch` and empty categories.
6. **Tag** (ask first if `autonomy.md` says so): `git tag -a vx.y.z -m "vx.y.z"` on the
   commit that contains the rolled block. The block moves first, then the tag, never
   the reverse. If a tag already exists with no block (the failure this skill exists
   for), roll the block retroactively under that version and date, and say so.
7. **Refresh intent.** Run `intent-audit` passes 1–6 quickly: goals with `Done when`
   met, futures with `Promote when` met, questions answered by this release.
8. **Report**: version, impact, number of lines per category, migration notes written,
   intent edits made, and the tag command run or awaiting approval.
