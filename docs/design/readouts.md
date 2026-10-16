# Readouts: a command's output as a compact, enriched or raw readout, with a receipt

Status: accepted 2026-10-16 (ADR-071). Plan: `docs/plans/readouts.md` (RO-). Goal G39. Roadmap:
`docs/vision/38-proof-layer-roadmap.md` §9.6 (the next priority program after the RT-M1 reading; the Claude Code mods
program follows). Constraints carried in: the harness port's §5.3 inject-only contract, narrowed once by ADR-038
(deny) and once by ADR-071 (view); SEC-1 and SEC-2 (ADR-042: no hook runs a command a repository names); I8, I9, I14,
I35, I38, I42, I53, I55, I61; ADR-063's overlap rule; the table freeze until RT-M1 reads PASS; the hold on model
spend; ADR-011 (built-ins name generic tools only); the determinism contract (`internal/AGENTS.md`); ADR-066 (pure Go).

## 1. The question, and what the numbers say

The owner asked for a more powerful version of rtk (a tool that rewrites an agent's shell command to run inside its
own wrapper and filters the output), extensible by users in a better way, with modes: savings first, and a mode that
spends tokens when the information is worth more, chosen per call by a decision model such as Jev. The answer here
keeps the goal and changes the frame: the bytes are in a few shapes, and fathom's edge is knowing what the output
means, not knowing more commands.

The corpus: 30 days of every project's local transcripts on the owner's machine, read by a throwaway script on
2026-10-16 (aggregates only, not committed; RO-M0 re-reads it with `fathom readout replay`), deduplicated by
tool-use id because resumed sessions copy history into new files.

| Shape (Bash, 145,825 calls, 334.9 MB of result text) | Calls | Share of bytes | p50 bytes |
|---|---|---|---|
| `sed -n` | 18,478 | 25.2% | 3,445 |
| `grep` | 28,884 | 16.0% | 998 |
| `cat` | 10,834 | 13.2% | 1,446 |
| `ls` | 6,014 | 6.8% | 1,963 |
| `bash -c` and scripts, `python3`, `for`, `echo` | 27,963 | 15.1% | 406 to 1,839 |
| `git diff`, `git show`, `git log` | 4,610 | 5.3% | 1,047 to 2,501 |
| `tccutil reset` (a compound prefix, mostly) | 4,579 | 2.1% | 635 |
| `go test` | 3,255 | 0.9% | 394 |

Kinds: shell reads of a source file 37.2% of bytes (reads over 200 lines 10.6%); grep, rg and git grep 16.5%
(bare-identifier greps 13.5% of grep calls). Failures: 2.7% of calls, 1.2% of bytes. Exact repeats: 0.03%. Each
result is capped near 30,000 characters by the harness, so totals understate real output.

Six deterministic strategies applied without overlap remove at most 12.3% (41.2 MB): long-read spans 8.5% (an
unvalidated ceiling: it assumes 60 lines of a long read are enough), grep to symbol 1.8% (refused: the grep-blocking
non-goal), test-success collapse 1.2%, grep grouping 0.6%, ANSI and progress 0.2%, dedupe 0.02%. Without the span
assumption, about 3.9%.

rtk's own figure for this machine, 41.9M tokens saveable from `rtk discover --all --since 30`, multiplies a static
per-rule percentage by 351,726 calls that include session copies; it is an estimate, not a saving (P12).

Three conclusions shape everything below:
1. The savings are in long source reads, then grep, `ls` and git output; a long command list buys little.
2. Lossless transforms are worth under 1%; any real saving is lossy, so the contract (section 6) is the product.
3. Failures are small in bytes and large in value: compact successes, enrich failures.

## 2. Components

| Component | Planned home | What it does |
|---|---|---|
| Command lexer | new package internal/shellcmd (RO-3) | One quote-aware lexer for every Bash consumer: segments, pipes, redirects, heredocs, subshells, `bash -c`, env prefixes, `FATHOM_READOUT=`, an `rtk ` prefix. It never re-emits a command (readouts never rewrite), so it only classifies, and it refuses what it cannot read. It replaces four splitters: `splitShellSegments` (`cmd/fathom/testrunhook_declared.go`), `shellClauseFields` (`cmd/fathom/bashwrite.go`), `bashObservedReadPath` (`cmd/fathom/discoveries_observed.go`), `firstCommandToken` (`cmd/fathom/discovery.go`) |
| Classifier | internal/readout | Maps a lexed command to a rule or to raw. Machine-bound shapes are raw (ADR-071 decision 3e) |
| Rule set | internal/readout plus embedded rule files; user scope `~/.fathom/readouts.toml`, `~/.fathom/readouts.d/` | Built-ins in the user format; each rule's fixtures run at load, and a failing rule is refused and named |
| Engine | internal/readout | A pure function: tokens, stdout, stderr, the interrupted flag and the harness cap in; a readout, typed records and a receipt out. Stages, then joins, then the contract check, then never-worse |
| Policy table | internal/readout | Rich mode's per-call choice of raw, lean or enriched (section 12) |
| Hook verb | `fathom readout-hook` (RO-16) | PostToolUse(Bash) and PostToolUseFailure(Bash); one process, with the test-run hook folded in as its rich stage when both are on (RO-21) |
| Codec view | `HookOutput` gains a view field (RO-19) | Claude Code encodes it as `hookSpecificOutput.updatedToolOutput` `{stdout, stderr}` on PostToolUse only; every other codec and event refuses it and the verb stays silent (ADR-038's deny pattern, `internal/harness/codec.go:92-112`) |
| Evidence | the existing hook log, channel `readout` | One row per readout, never output text (section 10) |
| CLI | `fathom readout replay`, `explain`, `test`, `report`, `learn` | Explicit verbs (law 10); no MCP tool (the net-tool-growth non-goal) |

## 3. Data flow

```
Claude Code runs the agent's Bash command (unchanged, under its own permissions)
  └─ PostToolUse(Bash) ──► fathom readout-hook (one process, 5 s descriptor, 4 s deadline under I53)
        ├─ mode off, or no [readouts] block ──► exit 0, nothing read beyond the envelope
        ├─ lexer + classifier ──► raw? ──► exit 0 silent (the agent sees the raw output)
        ├─ engine: stages ──► records ──► local joins ──► contract check ──► never-worse
        ├─ hook log: one `readout` row (rule@version, bytes in and out, dropped by class, trips, ms, mode)
        ├─ observe ──► emit nothing
        └─ lean or rich ──► view {stdout: readout + receipt, stderr: kept or summarized under the contract}
  └─ PostToolUseFailure(Bash) ──► never replaced; rich adds one injected line (inside I35), test-run's tier folded in
Any error, timeout or unparseable envelope ──► exit 0, the raw output stands (I9)
```

PostToolUse(Bash) carries the output text capped at 30,000 characters, with larger output persisted and named by
`persistedOutputPath`, and carries no exit code; an unpiped failure arrives on PostToolUseFailure led by `Exit code N`,
a piped one on PostToolUse (`docs/bench/readings/test-run-hook/hook-facts.md`, Q32). So "success" means "the harness
delivered it on PostToolUse", which includes a piped failure: the protected-line rule (section 6) is what keeps a
piped failure visible, not the event.

## 4. The exec path: a view after the run, never a rewrite before it

| Vehicle | What it changes | Verdict |
|---|---|---|
| PreToolUse rewrite to a wrapper that runs the command (rtk's design) | The command; fathom runs it | Refused. Fathom would run commands (SEC-2) and tests (ADR-043 decision 5); Claude Code evaluates permission rules against the rewritten input, so an allow rule written for `git` stops matching; and the defects rtk carries come with it: masked exit codes (#3267, #3492), dropped stdin (#2431), `;;` broken (#3197), machine output corrupted (#2487), a verdict on a different command (#3970) |
| PostToolUse view (`updatedToolOutput`) | Only what the agent is shown of a successful result | Chosen. The command ran natively: exit status, stdin, pipes, redirects and the permission verdict are untouched, and a pipe consumer already got the raw bytes. A schema mismatch makes Claude Code keep the original output (fail open) |
| Mods `tool.call` (Claude Code plugin hooks module) | The result, in process, 10 s budget per hook | Later (F78): the same engine behind the mods program's vehicle, which saves a process spawn per call |

No top shape in the corpus needs a flag injected before the run; the one reason a rewrite could earn its cost (for
example asking `go test` for `-json`) is a revisit trigger, not a plan.

## 5. Latency budgets

| Step | Budget (p95) | On a miss |
|---|---|---|
| Hook process start and envelope decode | measured by RO-M1 (startup baseline UNVERIFIED) | nothing to do: the raw output stands |
| Lexer and classifier, non-matching command | 2 ms after decode | exit 0 |
| Engine over a 30,000-character result | 20 ms | the raw output stands; the row says `deadline` |
| Outline join (one file, in-process extractor) | 150 ms | the readout drops the outline and says "outline: not computed (deadline)" |
| Every local join together | 300 ms | each missed join is named in the receipt |
| Internal ceiling for the whole hook | 1 s | emit nothing; the row says `ceiling` |
| Hook deadline (I53) | 4 s | the harness's own kill; never reached by design |
| RO-M1's bar | added p95 at most 100 ms, fail-open at most 1% | lean waits for the fix or the mods vehicle |

A daemon join (phase 3) gets its own deadline inside the 1 s ceiling and a named receipt; today a graph call on the
hook path is abandoned (EW-75: localize 385 of 437 at 2 s, `docs/plans/efficiency-wave.md`), so no readout depends on
one until EW-76 brings abandonment under 5%.

## 6. The contract: lossless, lossy, and recovery

Every readout, from any rule at any scope, keeps these (ADR-071 decision 3, I38):

1. **Protected lines.** Error, FAIL, panic, fatal and warning lines; a `file:line` frame; `Exit code N`; the
   harness's truncation or persisted-output notice. A rule whose stages would drop one serves the raw output, and
   the row and receipt name the trip.
2. **Never worse.** A lean readout not smaller than the raw is not served.
3. **Computed lossiness.** Each stage has a fixed class, and a rule's lossiness is the strongest of its stages:

| Stage | Lossiness | Inverse checked on fixtures |
|---|---|---|
| `ansi` (escape sequences; carriage-return progress overwrites) | none (display-equivalent) | the rendered terminal text matches |
| `collapse` (identical consecutive lines to `line ×N`) | none | expand restores the bytes |
| `group` (a repeated key such as `path:` factored to one header) | none | ungroup restores the bytes |
| `extract` (output to typed records; changes nothing by itself) | none | — |
| `join` (adds lines from a built-in join) | none (additive) | removing the added lines restores the bytes |
| `fields` (keep named columns) | columns | — |
| `keep`, `drop`, `cap` (head, tail, per group) | lines | — |
| `summarize` (records to a template line), `outline` | whole | — |

   A stage that claims `none` must round-trip every fixture through its inverse (RO-6): lossiness is tested, not
   declared.
4. **A recovery for every lossy rule**, or the rule is refused at load:
   - `rerun`: the command is read-only and reproducible (reads, `ls`, `grep`, `find`, `git log|show|diff` at a
     stamped HEAD); the receipt prints `FATHOM_READOUT=raw <cmd>`.
   - `span`: a file read; the receipt prints `sed -n A,Bp <file>` recipes for what was dropped.
   - `persisted`: the harness already saved over-cap output; the receipt repeats its path.
   - `saved`: a non-reproducible class (a test or build run) whose raw output fathom saves to a count-bounded file
     under the state directory (the test-run hook's precedent, `cmd/fathom/testrunhook.go`); only if the owner
     answers Q-RO-1 yes, otherwise those classes stay lossless until RT-M1 PASS.
5. **Deterministic bytes**: sorted groups, no wall-clock text in the readout, the same input giving the same bytes.
6. **Machine-bound commands raw**, and **refuse what cannot be read** (ADR-071 decision 3e and 3f).

The receipt is the last line of every lossy readout, for example:
`[readout ls-long@1 lean: 214→31 lines; columns dropped (perms, owner, size, date); 0 entries dropped; raw: FATHOM_READOUT=raw ls -la internal/]`.
A lossless readout carries a short receipt (`[readout grep-group@1: regrouped, nothing dropped]`) so the agent never
mistakes a regrouped view for the raw text.

## 7. The rule schema

A rule file is TOML with `schema = 1` and one or more `[[rule]]` tables; the embedded CUE schema (schema/readouts,
with valid and invalid testdata pairs per `schema/AGENTS.md`) validates it; unknown keys are refused.

| Field | Meaning |
|---|---|
| `id`, `version`, `about` | identity; the row and receipt carry `id@version` |
| `match.argv0`, `match.sub`, `match.flags_any`, `match.flags_none` | token matcher over the lexed command, never a regular expression over the raw string |
| `match.compound` | `simple` (default), `display_tail` (a producer piped only into `head`, `tail` without follow, or `cat`), or `after_quiet` (after a prefix from the quiet list: `cd`, `export`, `mkdir -p`, `sleep N`, plus the user's own list in `[readouts] quiet_prefixes`) |
| `when.bytes_gt`, `when.lines_gt` | below the floor, the output stays raw |
| `mode` | the mode this rule starts in; every new rule starts `observe` unless the user sets otherwise |
| `[[rule.stage]]` | `kind` from the closed set (section 6) and its parameters: `pattern` (a Go regular expression), `captures` (named groups to record fields), `key`, `keep`, `head`, `tail`, `per_group`, `template`, `record` (the record kind an `extract` stage emits), `join` (a name from the built-in table) |
| `recovery` | `rerun`, `span`, `persisted` or `saved`; required when the computed lossiness is not `none` |
| `[[rule.test]]` | `input` (inline or a file beside the rule) and `expect`; at least one per rule; a lossy rule also needs one fixture that carries a protected line |

There is no field that names an executable, an argv, shell text or a URL, and joins are names from a closed table:
`outline` (one named file through fathom's extractors), `head_stamp` (`cmd/fathom/edithook_pinstamp.go`),
`last_failures` (the previous run's failing ids from the hook log), `changed_files` (the read-only git wrapper),
`edited_in_session` (the session's edit set, `cmd/fathom/editset.go`), `guard_for_command` (the command guards'
matcher). A rule may render a command as text in a `template` (a recovery, a next step), never run it.

### 7.1 Worked example: observe (a user's new rule)

Every new rule starts here: computed and logged, never shown. A user adds `make`'s directory chatter:

```toml
schema = 1

[[rule]]
id = "make-chatter"
version = 1
about = "make: drop Entering/Leaving directory lines"
match.argv0 = ["make", "gmake"]
mode = "observe"
recovery = "rerun"

[[rule.stage]]
kind = "drop"
pattern = '^make(\[\d+\])?: (Entering|Leaving) directory '

[[rule.test]]
input = "make[1]: Entering directory '/src/a'\ncc -c a.c\na.c:3:1: error: expected ';'\nmake[1]: Leaving directory '/src/a'\n"
expect = "cc -c a.c\na.c:3:1: error: expected ';'\n[readout make-chatter@1 observe: 4→2 lines; 2 directory lines dropped; raw: FATHOM_READOUT=raw make]\n"
```

`fathom readout test ~/.fathom/readouts.d/make.toml` runs its fixture and the contract (the `error:` line is
protected and kept); `fathom readout report` shows its hits and dropped-line reference rate; the user moves it to
`lean` when the report says the bar is met.

### 7.2 Worked example: lean (a built-in)

```toml
[[rule]]
id = "ls-long"
version = 1
about = "ls -l and -la: one entry per line, columns dropped, every entry kept"
match.argv0 = ["ls"]
match.flags_any = ["-l", "-la", "-al", "-lah", "-lha", "-lA"]
match.compound = "display_tail"
when.bytes_gt = 1500
mode = "observe"           # built-ins ship in observe; RO-M0 and RO-M3 promote per class
recovery = "rerun"

[[rule.stage]]
kind = "fields"
pattern = '^([dl-])\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(.+)$'
captures = ["type", "name"]
template = "{name}{type:d=/,l=@}"

[[rule.stage]]
kind = "drop"
pattern = '^total \d+$'
```

The receipt states the entry count and that no entry was dropped (rtk's `ls` drops `node_modules` and `.git` with no
count, #3434; this one never drops an entry).

### 7.3 Worked example: rich (a built-in with typed records and joins)

```toml
[[rule]]
id = "go-test"
version = 1
about = "go test: collapse passing packages; failures as records with delta and flake joins"
match.argv0 = ["go"]
match.sub = ["test"]
match.flags_none = ["-json", "-v"]
mode = "observe"
recovery = "saved"         # needs Q-RO-1 yes; otherwise the collapse stage is disabled and the rule is lossless

[[rule.stage]]
kind = "extract"
pattern = '^--- FAIL: (?P<id>\S+) '
captures = ["id"]
record = "test_failure"

[[rule.stage]]
kind = "summarize"
pattern = '^ok\s+(?P<pkg>\S+)'
template = "ok: {count} packages"

[[rule.stage]]
kind = "join"
join = "last_failures"     # renders: fixed 2 (A, B), still failing 1 (C), new 1 (D)

[[rule.stage]]
kind = "join"
join = "head_stamp"        # a flake flag when the same id passed at this HEAD earlier in the session
```

A passing run reads `ok: 14 packages [readout go-test@1 rich: 41→1 lines; raw saved: <path>]`. An unpiped failing run
arrives on PostToolUseFailure, is never replaced, and gains one injected line: `go test: 2 failing (TestA new, TestB still
failing since a1b2c3d; TestC fixed); TestB failed then passed at this HEAD earlier (flake?)`.

## 8. Rule tests and the lossiness contract

- `fathom readout test [file...]` runs every fixture, the contract (a protected-line fixture per lossy rule; the
  inverse round-trip for every `none` stage; never-worse; determinism by running each fixture twice) and prints
  each rule's computed lossiness and recovery. Exit 0, 1 (a failure), 2 (operational).
- Built-ins' fixtures run in `task validate` through the internal/readout tests, synthetic inputs only.
- At load, a rule whose fixtures fail is not served; `fathom doctor`'s readouts row names it and its first failing
  fixture. A rule cannot be served on the strength of its author's word.
- `fathom readout replay --rules <file>` replays a rule over the local corpus (section 9's reader) and prints
  aggregates only: hits, bytes in and out, protected trips, and the offline dropped-line reference rate. It never
  prints or commits transcript text.

## 9. Learned rules

`fathom readout learn` is an explicit verb (I14 is about egress; this makes none and uses no model). It reads the
local transcripts instrument B already reads, groups Bash results by lexed shape, and for shapes above a byte-share
floor proposes draft rules: lines that recur across most outputs of one shape and carry no protected class become
`drop` candidates; repeated key prefixes become `group` candidates. Each draft is replayed on a held-out 30% of that
shape's results (seeded, the seed printed) and printed with its would-remove bytes, protected trips (must be 0) and
offline reference rate. Output is a TOML draft on stdout, sorted and deterministic; nothing is installed. The user
reviews it, saves it under `~/.fathom/readouts.d/`, and it starts in observe like any rule.

## 10. Evidence per rule

One hook-log row per readout on channel `readout`: `rule`, `version`, `class`, `mode`, `bytes_in`, `bytes_out`,
`dropped` (counts by line class), `trips`, `joins` (ran, missed), `ms`, `session`, `tool_use_id`, `bypass` (the
command carried `FATHOM_READOUT=`). Never output text (the test-run hook's TR-7 precedent; no table).

`fathom readout report` reads those rows plus the session's following calls and prints, per rule:

| Column | Definition |
|---|---|
| hits, bytes removed | sums over rows; bytes, labelled bytes |
| est. tokens | bytes/4, labelled an estimate (instrument A's convention, `docs/design/production-tokenomics.md` §3) |
| dropped-line reference rate | a later call in the same session, within five, names a path, `path:line` or identifier that appeared only in dropped lines; computable in observe mode, because the hook saw both texts |
| recovery rate | in lean: a raw rerun of the same command or a read of the same file within five calls |
| net bytes | removed minus the bytes of recovery calls (a re-read costs; #3492's lesson) |
| verdict | `promote`, `keep`, `demote` or `insufficient`, by the bars below |

Promotion to lean: at least 200 hits and a 95% Wilson lower bound of at least 0.90 that no dropped line was
referenced (ADR-056 R3's pruning bar). Demotion: a lean recovery rate whose Wilson lower bound exceeds 5%. The report
recommends; the user's config changes (P11). This is not a token dashboard: no totals across rules, no per-person
view, and nothing a client already reports.

## 11. Distribution and trust

| Scope | Where | Trusted because | May | When |
|---|---|---|---|---|
| Built-in | embedded in the binary, user format | fathom's gate runs their fixtures | everything the contract allows | phase 0 |
| User | `~/.fathom/readouts.toml`, `~/.fathom/readouts.d/*.toml` | the user put them there | everything the contract allows | phase 0 |
| Repository | the repository's `.fathom/` | the user lists the file's sha256 in user scope (`[readouts] trust`) | extract and add only; never drop or replace; observe until trusted | phase 5, after ADR-061 |
| Pack | a `readouts` key in a tech pack | the repository's pin puts the pack on disk (ADR-061) and the user trusts it | as repository | phase 5 |

A repository can never set `[readouts] mode` or a rule's mode (user-only, `internal/config/scope.go`), and a repository
rule can never hide a line, because the risk a cloned repository's rule carries is hiding output from the agent.
Sharing is copying a file; there is no registry, no download (I56) and no import of another tool's format (the owner
removed the rtk filters item, 2026-10-16).

## 12. Modes and the decision inputs

| Mode | Successes | Failures | Cost |
|---|---|---|---|
| `off` (default) | raw | raw (the test-run hook unchanged) | none |
| `observe` | raw; readout computed and logged | raw | hook time only |
| `lean` | readout where the rule is promoted, else raw | raw | fewer bytes, a recovery risk |
| `rich` | the policy table chooses raw, lean or enriched | raw plus one injected line | may cost more bytes than raw where the table expects value |

Per call: `FATHOM_READOUT=raw|lean|rich <cmd>`. Per rule: `[readouts.rules.<id>] mode`. Both user-only.

The policy table (provider `none`, deterministic, I42):
1. A failure (PostToolUseFailure or `interrupted`): enriched injection when a record names a session-edited file or a
   test in the last run's failing set; otherwise raw.
2. A success with protected lines present (a piped failure): raw, plus enrichment as in 1.
3. A success over the rule's byte floor, rule promoted: lean.
4. A whole-file read of a session-edited file: raw (the agent is about to edit it).
5. Otherwise: raw.

Inputs every decision sees, and the only inputs a model would ever see (I61): class, byte and line counts, record
counts by kind, overlap with the session's edited files, the rule's measured recovery rate, the event, and whether the
session is a sub-agent (`agent_id`). The contested band, where a model could add anything, is rules 1 and 4 plus long
reads of files the session has not edited: failures are 3,918 calls and reads over 200 lines 2,682, at most about 4.5%
of Bash calls.

## 13. What the Jev or llm adapter would add, and the A/B that proves it

The decision port site `readout_mode` (ADR-042 as amended by ADR-071) asks a `choice{raw, lean, rich}` in the
contested band only, through a loopback daemon endpoint the hook calls under its deadline (the `POST /route`
precedent; a hook process never calls a provider, I14), over the features above and never the output text (I61), with
the table's answer on any miss, timeout or unsure band (I42). It is named in user scope only.

What it could add: the table cannot tell which long read of an unedited file the agent will need whole, or which
failure is worth an enrichment line; a typed choice over features might. What it costs: one provider call per
contested call (about 6.6k per 30 days on this machine at the corpus rate; tokens per call UNVERIFIED) and latency
inside the hook's deadline (provider p50 UNVERIFIED; `docs/bench/readings/decide/provider-smoke-2026-10-09.md` is a
smoke, not a reading).

The proof, in order, after the owner lifts the budget hold (Q-RO-4):
1. RO-M5, calibration: at least 100 contested calls with the provider answering in shadow (logged, not used); the
   label is computed from what happened next (lean was right if nothing dropped was referenced and nothing was
   recovered within five calls; rich was right if the next call touched a file or symbol the enrichment named). Pass:
   a 95% Wilson lower bound of at least 0.90 at the shown confidence (R3). Route failed this bar at 45.7% (ADR-042's
   2026-09-24 amendment), so the prior is against it.
2. RO-M6, instrument C (`docs/design/production-tokenomics.md` §5): pre-registered arms, rich with provider `none`
   against rich with `typesafe`, at least 20 units per arm (the arm-homogeneous A/B unit); primary metric tokens per
   passing task, with pass rate non-inferior; secondary the recovery rate. Kill: no gain at n ≥ 3 (ADR-042's own
   trigger), and the site stays unbuilt.

A cheaper first decision model needs no provider at all: tune the table's byte floors and the contested band from
`fathom readout report`'s recovery and reference rates, offline and deterministic.

## 14. Integration

Hooks on Bash after readouts:

| Event | Today | With readouts |
|---|---|---|
| PreToolUse(Bash) | `edit-hook --pre`: command guards (deny when enforced), discovery beat, pin stamp | unchanged; readouts never touch it |
| PostToolUse(Bash) | `session-hook --event test-run` (opt-in, inject on a failure) | `fathom readout-hook` (mode on): the view, and the test-run tier folded in; `fathom init` writes one entry; the old marker stays accepted |
| PostToolUseFailure(Bash) | `session-hook --event failed-test-run` | the same verb with `--event failure`: never replaced; rich injects one line |
| Stop | `stop-hook` | unchanged; the session ledger may gain a per-session readout byte sum (additive telemetry field) |

`fathom doctor` gains a readouts row: mode, rules loaded and refused (with the first failing fixture), the harness's
view support (Claude Code yes; Codex and Cursor unsupported, named), and any other wired Bash output rewriter.

Claude Code mods: the engine's input is the lexed command, stdout, stderr, the interrupted flag and the harness cap,
so a mod's `tool.call` (result replacement validated against Bash's output schema, a 10 s per-hook budget, plugin
typings of Claude Code 2.1.301) can call the same engine through a loopback endpoint and skip a process start per
call. That vehicle belongs to the mods program (F78), which also settles how a mod module reads the No Node toolchain
non-goal. A mod's `ui.render` could show the human the receipt beside the row (row redraw for tool results is
UNVERIFIED).

## 15. Coexistence with rtk

Under ADR-063's rule, applied to an external tool: **keep, separate, with a boundary**; no delegate, no merge.
- Fathom does not install, configure, read, write, import or ship rtk configuration (owner, 2026-10-16).
- A command starting `rtk `, and an output containing rtk's recall marker (`[full output: rtk recall`), stay raw: two
  lossy views are never stacked, and one tool rewrites a command at most.
- If rtk's PreToolUse rewrite is wired, commands reach the readout hook as `rtk …` and stay raw, so readouts do
  nothing; `fathom doctor` names the other rewriter so the user can choose.
- No rtk code is ported. Credited ideas, re-derived from this specification: a never-worse guard, a lossy and recovery
  declaration per stage, trust by a file's content hash. rtk's BUSL-1.1 engine and proxy are not read for design.
- Where readouts differ on purpose: lossiness computed and tested, not declared; protected lines for every scope; a
  receipt on every lossy readout (rtk #2204, #3434 are silent caps); a repository rule can only add (rtk trusts a
  project file to strip once its hash is accepted); a token matcher, not a regular expression over the command
  (#3838, #3197); fixtures required to serve; typed records and joins.

## 16. Honest measurement

| Figure | What it is | What it is not |
|---|---|---|
| Corpus bytes | UTF-8 text of tool results after the harness cap, deduplicated by tool-use id | real output size; tokens |
| Would-remove bytes (observe, replay) | bytes a readout would have removed | a saving: the agent may re-read |
| est. tokens | bytes/4, labelled | a tokenizer count |
| Net bytes | removed minus recovery-call bytes | a cost saving |
| Dropped-line reference rate | an offline and observe-mode harm proxy | proof that nothing was lost |
| Instrument C | tokens per passing task and pass rate, arms on and off | free (model spend; the budget hold) |

No saving is claimed before instrument C (P12). An independent evaluation of rtk found its defaults cost 16% more per
passing run with a lower pass rate (#3492); readouts must beat that bar in the open or stay off by default.

## 17. Built-in rules, in value order from the corpus

| Rule | Bucket (share of Bash bytes) | Lossiness, recovery | Gate before lean |
|---|---|---|---|
| `read-outline`: a whole-file source read over 200 lines becomes an outline from fathom's extractors, the first 40 lines, and `sed -n` span recipes; an explicit `sed -n A,Bp` range stays raw (the agent chose it) | reads over 200 lines 10.6%; whole-file share unknown | whole, span | RO-M0 whole-file share at least 3%, then RO-M3 |
| `grep-group`: matches grouped by file, `path:` factored out; above 200 matches, a per-file cap with exact totals | 16.5% (grouping 0.6%) | none, or lines with rerun | RO-M0 |
| `ls-long`: section 7.2 | 6.8% | columns, rerun | RO-M0 |
| `git-log`, `git-show`, `git-diff`: trailers and signature lines dropped in log, every commit kept with an exact count, never a silent commit cap (#2204); diff and show grouped by file with hunk headers kept, bodies capped only above a floor with span recipes | 5.3% | lines, rerun at the stamped HEAD | RO-M0 |
| `go-test` and the other declared test commands: section 7.3 | 0.9% (collapse ceiling 1.2%) | whole, saved | Q-RO-1, RO-M3 |
| `base`: ANSI, progress and repeat collapse on every matched rule | 0.2% | none | none; it runs inside other rules only |

Agent-written scripts (`python3`, `bash -c`, `for`, `echo`: 15.1%) get no rule: their output is the agent's own
report. `tccutil reset` (4,579 calls) is mostly a compound prefix whose bytes come from the command after it
(UNVERIFIED; RO-M0 measures the compound share), and its repetition is environment friction for `fathom observe`, not
a readout.

## 18. New capabilities this inspires

| Capability | Why | Where |
|---|---|---|
| Test delta and a flake flag at the commit pin | "fixed 2, still failing 1, new 1" and "failed then passed at this HEAD" localize faster than any output text, from rows the hook log already holds | RO-24 |
| Compile and vet errors grouped by root cause and tied to this session's edits | "37 errors, 1 root: the signature of `Foo` you edited two calls ago" turns a wall of errors into one action | RO-25 |
| Command obligations at output time | the guard matcher already knows `go test` without `-race` misses what the gate runs; saying so beside the output is the cheapest correction | RO-26 |
| One command lexer for guards, test-run, discovery, bash-write and readouts | four approximate splitters disagree today; one tested lexer with a fuzz corpus removes a defect class | RO-3, RO-4 |
| Typed extractor rules, with joins supplied by fathom | an author writes one pattern and gets delta, outline, guard and edit-set joins for free; text filters cannot do that | RO-7, section 7 |
| Learned rules from the user's own transcripts | drafts with held-out replay beat hand-written rules for the long tail, with no model | RO-28 |
| The dropped-line reference rate | a zero-spend harm proxy that calibrates any compaction, readouts or not, before an A/B | RO-22, RO-M0 |
| Instrument B counts shell reads | F75's gap: the largest discovery share (`sed -n`, `cat`, `head`, `tail`) is invisible to the denominator today | RO-30 |
| Witnessed runs | an observed test result (command, pin, tree state, result; no text) shown beside attested runs, never proving anything | F76 |
| A session map from readouts | reads and greps become anchors a parent or the next worker can use without re-reading | F77 |
| Readouts as a mod | the same engine in process, no spawn per call | F78 |
| An index hint beside a code-discovery shell command | the original F75 idea, still behind the grep-blocking non-goal's condition | F75 |

## 19. Risks and the measurement that kills each

| Risk | Measurement | Kill |
|---|---|---|
| A readout hides a failure (rtk #2204, #2317, #3267) | the contract fixtures in the gate; RO-M1's trip count | any lossy readout that drops a protected line in the field: the rule is refused and a test pins the case |
| Compaction makes the agent re-read (#3492) | reference rate (observe), recovery rate (lean), net bytes | decision 9's bars; a class over them returns to observe |
| The saving is illusory | instrument C | no gain at n ≥ 3: off by default, saving lines retracted |
| Hook latency at about 4,860 Bash calls a day | RO-M1 | added p95 over 100 ms or fail-open over 1% |
| The channel misbehaves (size cap on `updatedToolOutput` UNVERIFIED; `is_error` and exit text after replacement UNVERIFIED) | RO-M2 | any loss of exit text or `is_error`: replacement stops |
| Two rewriters race (which parallel hook's output wins is undocumented) | doctor's row | readouts stay raw on `rtk ` commands; one rewriter per command |
| An outline hides the needed line | RO-M3 for `read-outline` | its own demotion bar; RO-M0's 3% floor drops it first |
| The program costs more than it returns | RO-M0's program bar | under 4% of Bash bytes in classes that pass the harm bar: the lossy track stops, rich stays, and the mods program starts |
