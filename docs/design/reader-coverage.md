# Coverage before conclusion — the reporting layer gets a read manifest

**Status.** Proposed (ADR-027). Plan: `docs/plans/reader-coverage.md`.
**Applies:** law 3 (never silent), law 7 (instruments get existence checks),
I25 (silent-zero ingest), ADR-026 §"an audit that silently yields nothing is
indistinguishable from a healthy one". **Precedent:** IT-14/IT-15 (v0.5.0)
already decided this for `fathom ingest`; nothing here is new doctrine.

## 1. The defect, measured

`fathom intent audit` on a corpus with one unreadable file
(`chmod 000 docs/intent/invariants.md`, the state a wedged macOS TCC verdict
produces for a whole process tree on an external volume):

| | readable | `invariants.md` unreadable |
|---|---|---|
| invariants | 37 | **0** |
| unproven claims | 169 | **85** |
| unparsed lines | 0 | 0 |
| anything in `uncomputable` naming the file | — | **nothing** |

The repository with a broken corpus reports **zero invariants and half the
debt**, silently. Its degraded state is its healthiest-looking state.

This is not hypothetical. The wedge is a documented, recurring condition on
this machine (`~/.claude/CLAUDE.md`; O16; INC-3's neighbourhood), it fails
`open()` while `stat()` keeps working, and it takes the whole process tree.
It fired twice during the session that wrote ADR-026.

### Where it comes from

Eight readers in the reporting layer do `if err != nil { continue }` or
`return false`, conflating three different facts into one:

- the file is not there (a real, reportable absence),
- the file is there and could not be read (an instrument failure),
- the file is there, readable, and empty (a real, reportable emptiness).

Sites: `doctor.go:321,390,493,585`, `intent/corpus.go:202`,
`intent/parse.go:403`, `custodian/adrrefs.go:58`, `intentaudit.go:377`.

### What is NOT affected, and why that matters

`fathom verify` — the gate — is clean: an unreadable plan exits **2**
(operational), never 0. The `ingest` path is clean: IT-14/15 gave it a
`read-failed:` column with errno and a refusal to overwrite a good index with
a plausible empty one.

So the discipline exists and is proven in this repository. It was simply
never applied to the layer built last: the layer whose entire job is
reporting honesty.

## 2. Three more findings from the same root

**F2 — two instruments, two numbers.** `fathom doctor`'s `unproven_claims`
row reports **12**; `fathom intent audit` reports **88** on the same tree.
The doctor row builds a partial `AuditInput` (no plan contracts, no guard
notes, no symbol oracle) and then prints its count as if it were the total.
Its zero-claims path names what it could not compute; its non-zero path drops
that list entirely. U12 and U13 need no index and were simply never wired in.

**F3 — a message that states the opposite of the truth.** `binary_fresh`
reports "`<root>`'s go.mod does not declare github.com/geoffmilleraz/fathom"
when `go.mod` was merely unreadable. Observed on this repo, whose go.mod
declares exactly that on line 1. The check then SKIPs instead of FAILing —
degrading, again, toward looking fine.

**F4 — a remedy that destroys correct configuration.** `guard_dead_scopes`
calls `cmd/fathom/ui/**` a dead scope "matching zero indexed files". The
directory holds three real CSS files; fathom does not index `.css`. The
remedy reads "narrow or remove the scope", so a user who follows it deletes
a working guard. "Not indexed" is not "not there".

**F5 — the honesty section drowns in its own repetition.** U12 emits eleven
near-identical `NOT COMPUTED` lines, one per surface, restating a fact its
first line already gives. The section a reader most needs is the least
readable one.

## 3. Decision

**An instrument may not report a conclusion it did not earn. Where it could
not see, it says so — and no failure mode may make the repository look
healthier than it is.**

Mechanically, four rules:

1. **Every read in the reporting layer is recorded.** A small
   `internal/readset` returns content or a named failure and accumulates a
   `Manifest`: files read, files skipped with a category, files that FAILED
   with their errno text. This mirrors `fusion.ReadFailure` deliberately;
   it is not shared code because the ingest path's refusal semantics are
   its own, and coupling the audit to the ingest pipeline buys nothing.
2. **A count over an incomplete manifest is a lower bound, and says so.**
   `37 invariants` becomes `≥ 0 invariants (2 source(s) unreadable)`. The
   number is never presented as a total. Downstream (doctor rows, the
   attention line, the fleet column) carries the same qualification.
3. **Absence is distinguished from blindness at the point of report.** The
   three facts in §1 get three different messages, always naming the path
   and the errno.
4. **The direction of degradation is checked by a test.** For each affected
   instrument, a test makes an input unreadable and asserts the output does
   NOT improve. This is the regression test the defect above never had.

### What this does not do

- **No refusal.** Unlike `ingest`, the reporting layer does not refuse to run
  on partial coverage: a `doctor` that dies when one file is unreadable is
  worse than one that reports honestly. The doctor rows keep their
  never-FAIL contract (ADR-026) and go WARN with the reason.
- **No new always-on cost.** The manifest is per-invocation and in memory.
  Nothing is written, nothing is injected into a session, no new channel.
- **No change to the gate.** `fathom verify` is already correct.

### Alternatives rejected

- **Fail hard on any unreadable file.** Wins if the reporting layer were
  load-bearing for correctness. It is not — it is advisory — and a hard
  failure on a transient wedge would train users to ignore or disable it.
- **Log to stderr and carry on.** The status quo plus noise. The number in
  the report is what is read and believed; a warning elsewhere does not
  change what the number claims.
- **Share `fusion.ReadFailure` directly.** Rejected: it drags the audit's
  dependency into the ingest pipeline and carries refusal semantics the
  reporting layer explicitly does not want. The vocabulary is copied, the
  coupling is not.

## 4. Uplift: what belongs in agentic-kit, and when

The kit already owns this doctrine for LLM-derived evidence: **D10** —
`ABSENT_PROVEN` claims absence within a declared scope at a named snapshot,
never global absence, and every verdict carries a `CoverageManifest`
(`foundations/evidence/evidence-ladder`, `evidence-signature`).

Fathom's deterministic readers had no such manifest. That is the whole
finding, stated in the kit's own accepted vocabulary: **D10's discipline has
no mechanical counterpart for non-LLM reads, and three Project Intent repos
now depend on one.**

The kit is the right eventual home and **not yet**: D13 requires two
consumers, and today fathom is one. So the sequence is the promotion rule —
build it here, file the ask with this evidence, promote when a second
consumer names the need. Filed as FA-20/FA-21.

## 5. Kill criteria (law 5)

Delete this if, after 60 days: no manifest has ever been incomplete in a real
run on any adopting repo (the discipline is theatre for a condition that does
not occur), **or** every incompleteness recorded was the same single cause
and a narrower fix would have covered it.
Measurement: an `incomplete_coverage` count on the existing `intent-cli` and
`doctor` beats — no new channel, no new file.
