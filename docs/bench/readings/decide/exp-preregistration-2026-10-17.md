# Stage 0's second pre-registration: E2b's items and E3's arms

EXP-1 amended 2026-10-17 at exp0 commit bd9d24ea, before any E2b or E3 call. It changes only what it names. Everything else in `exp-preregistration.md` (2026-10-16) holds unchanged, every bar included, as that page's section 1.3 requires.

## Verdict, plainly

E1 and E2a ran under the first page and both read NONE (`exp-e1.md`, `exp-e2.md`). E2b
did not run: its source gave 7 items of the 50 needed. Of the 45 fix commits in the
300-commit window, 41 add or change a `_test.go` file, so the regression test seldom
exists at the parent, and the tests that do exist pass there. No E2b request was sent.
This page moves E2b to the failures the test-run hook logged, which are the failures
F35's question would be asked about in use, and fixes the E3 arm rules the first page
left open. The questions, options, bands, baselines and bars are unchanged.

## 1. E2b's items: test failures the test-run hook logged

- **Source.** The test-run hook's logs, `workspace/tmp/test-run/*.log` in the
  maintainer's fathom checkout (728 files on 2026-10-17). They are copied into scratch
  first and read only there.
- **Kept.** A log with at least one `--- FAIL: Test` line and a line
  `FAIL<tab>github.com/geoffmilleraz/fathom/...`. Only fathom's own packages qualify, so
  no other repository's output is ever sent to a provider.
- **The item.** It is built from the log's first `--- FAIL:` block. The block runs from
  that line up to the next line that starts at column 0 with `--- `, `=== `, `FAIL`,
  `ok` or `PASS`, at most 60 lines. Absolute paths become repository-relative and the
  user name becomes `user`. Each item carries:
  - `test`, the failing test's name;
  - `file`, the `_test.go` file that declares the test, looked up in `subj/fathom` at
    fd091c4f first and then in the exp0 clone at f6bf2904;
  - `test_src`, the test function's source from that same tree;
  - `output`, the block.

  A test found in neither tree is excluded and counted. Fields and limits are those
  `exp_e2_test.go` takes.
- **One per failure.** Items that share the package, the test and the first non-blank
  output line after the `--- FAIL:` line (digits normalised to 0) count as one. The
  log whose file name sorts first is kept.
- **Secret check.** An item whose text matches `sk-[A-Za-z0-9_-]{20,}`,
  `gh[pousr]_[A-Za-z0-9]{20,}`, `AKIA[0-9A-Z]{16}`, `xox[abprs]-`,
  `-----BEGIN [A-Z ]*PRIVATE KEY` or `(?i)(api[_-]?key|secret|token)\s*[:=]\s*\S{12,}` is
  dropped and counted.
- **The draw.** Candidates are sorted by the sha256 of their output, then shuffled with
  `ask.SampleSeed` (20261016) on the stream `e2-failure-log/fathom`. The first 50 are
  kept and get ids `e2b-001` to `e2b-050` in draw order. If fewer than 50 remain, E2b
  runs on all of them and the reading names n. Building twice gives byte-identical
  files.
- **Labels.** The first page's protocol applies, with one difference: there is no fix
  commit, so there is no fix diff. The Claude reviewer reads the output, the test
  function and the code under test in the tree `test_src` came from, and may read
  fathom's git log around the log's modification time. Labels are written before any
  E2b call, with provenance `frontier`.
- **Cap.** E2b's run caps at US$0.30.

## 2. E3's arms

- **Sessions.** Each arm answers each question in its own fresh agent session, so
  there are 10 sessions. Both arms run the same model, Claude Sonnet 5.5. Neither arm
  may use the live daemon's `mcp__fathom__*` tools or the web.
- **Table arm.** Its only commands are `bin/fathom` against `stores/fathom.sqlite`: the
  `serve-mcp` tool calls in `scratch/e3-tooling.md` (`find_symbol`, `find_callers`,
  `blast_radius`, `test_impact`) and `tests select`. With those it may write the spec
  and rows files and run `TestDecideExpE3` live once per question (a dry run first is
  allowed). It reads no source file, `.go` or otherwise, and it does not use fathom's
  `read_source` or `search`. Its answer comes from fathom's outputs and the receipt.
- **Reading arm.** It reads the files of `subj/fathom` at fd091c4f with cat, sed, grep,
  rg or a file reader. It runs no fathom command and makes no model call.
- **The answer.** Each arm writes `scratch/e3-answers/<q>-<arm>.md`, one element per
  line as `<symbol, path or test> <file>:<line>`, and nothing else.
- **Context tokens.** These come from the session's transcript: the largest per-turn
  context (input plus cache-read plus cache-creation tokens) minus the first turn's
  context. That is what the arm loaded beyond the shared prompt. Sessions are found by
  their label, `exp0b-<q>-<arm>`, in the agent metadata.
- **Dollars.** The table arm's Jev spend for each question comes from its receipt. The
  reading arm's is $0. Subscription tokens are recorded beside them, not priced.
- **Grading.** The first page's rule applies: correct, partial or wrong against
  `scratch/e3-reference.md`, which was written on 2026-10-16 before any arm ran. A Claude
  reviewer (Opus 5.5) grades the answers blind: for each question the two answers are
  relabelled A and B, ordered by the sha256 of `<q>` plus `ask.SampleSeed`. The
  reviewer records each grade before unblinding.
- **Cap.** Each question's table-arm run caps at US$0.10.

## 3. Spend so far

E1 US$0.0176 and E2a US$0.0029, so US$0.0205 of the $2 cap is spent. The caps above
add at most US$0.80, so stage 0 cannot pass US$0.83.
