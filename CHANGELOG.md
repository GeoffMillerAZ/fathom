# Changelog

All notable changes to fathom. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

**How to read this file.** Each release opens with what it was for, then
one block per day: a bold topic naming the program and its plan rows
(`SH-1..6`, `O16`, `ADR-025` are durable reference points -- `fathom plans
archive` checks that every archived row id is absorbed here, in `ADR.md`, or
in the per-release narrative), followed by what changed. The full narrative
record -- root causes, measurements, rationale as originally written -- lives
per release under `docs/changelog/` and is never normative; plans, ADRs, and
observations are.
From 2026-08-25 the `[Unreleased]` block follows the change-records rules
(`third_party/agentic-kit/docs/project-intent/changes.md`): a `Release impact:` line first,
one line per observable change at most two sentences, `### Breaking` first
when present, the delta not the journey; released blocks stay as written.
Compatibility surface: `docs/intent/scope.md`.

## [Unreleased]

Release impact: minor

### Breaking

- `fathom intent render --playbook` is removed after its 0.15.0 deprecation: `fathom intent render` now exits 1 as an unknown `intent` subcommand (ADR-074, IV-10). Migration: drop any call of it from scripts and gate steps; `docs/intent/README.md` is the router.

### Added
- JSON Schema documents (a `.json` whose root `$schema` names json-schema.org, or a `*.schema.json`) extract at L3 (L1 excellent, L2 excellent, L3 excellent), measured against `@apidevtools/json-schema-ref-parser` on SchemaStore and the JSON-Schema-Test-Suite remotes: `$defs`, `definitions` and root `properties` are symbols, `$ref`s to them are references, and a `$ref` to another schema file is a cross-file edge. A `.json` such a schema was read as generic JSON before, so a store ingested earlier re-reads on the next epoch bump.
- `fathom doctor` has a `project` row: every key of the `.fathom/*.cue` files with the file that sets it, its status and its entry count, plus any key fathom does not read (a warning). It fails on a file that does not parse, a key its reader refuses, or two files setting one key.
- `fathom init` and `fleet reconcile` write `.fathom/.gitignore` as a receipted, undoable region naming `owned.json`, `leases.jsonl`, `scip/`, `receipts.jsonl` and `memories.jsonl`, so the local zone is never committed; lines you wrote outside the region are untouched.
- Elixir (.ex .exs) extraction at L3 (L1 excellent, L2 good, L3 good), measured against Elixir's own parser and the grammar's tags query on phoenix and ecto: modules, protocols, implementations, functions, macros, guards, delegates, callbacks, types and ExUnit tests, with `alias`, `import`, `require` and `use` resolved to the file that defines the module.
- Bazel / Starlark (BUILD, BUILD.bazel, *.bzl, WORKSPACE, MODULE.bazel) extraction at L3 (L1 good, L2 excellent, L3 good), measured against `bazel query` (tier A) on bazelbuild/examples and grpc/grpc: targets, rules, macros and functions, `:label` references, and `load()` edges from a target or def to the `.bzl` file defining the macro or rule it calls; the Starlark grammar adds 107,943 bytes (gzip) to the embedded set.
- Tiltfile extraction at L2 (L1 good, L2 good), measured against tree-sitter-python's tags query on the Starlark grammar (tier C) on tilt-extensions and tilt-example-go: functions, and the resources, images, manifests and settings its Tilt calls declare.
- `fathom export receipt --service S --symbol NAME [--kind symbol_location|callers] [--no-excerpts] [-o FILE]` writes the receipt `fathom receipt verify` reads, with anchored facts: each anchor names the commit, blob id, excerpt digest, the file's `fathom_fingerprint` and a bounded excerpt (40 lines; `--no-excerpts` keeps digests only), and a symbol with no callers carries the read set its absence was counted over, its count a named lower bound when a read is missing.
- Non-Go test entries are test symbols in a file the role classifier calls a test: pytest `test*` functions and `Test*` classes, jest, vitest and mocha `describe`, `it` and `test` blocks (now symbols named by their quoted titles, `"sum" "adds"`, that own the calls in their callbacks), JUnit `@Test`, xUnit `[Fact]` and `[Theory]`, NUnit `[Test]`, MSTest `[TestMethod]`, PHPUnit `test*` methods and `#[Test]`, and Rust `#[test]` (a Rust function's signature now starts at its outer attributes). `blast_radius`'s and the test portfolio's tests name them once the service is ingested again (TQ-27).
- `fathom init --dry-run` lists every file init would write or change (repository, home and state directory) and writes none; the same steps decide each write, and a test holds the list equal to what the real run then changes on disk.
- `fathom init --demo` wires a throwaway copy of the bundled fauxtown fixture fleet under a temporary HOME, ingests a service, verifies a two-row fixture plan and prints the time to that first proven answer as one line; it never touches the real HOME, `~/.fathom` or a running daemon.
- Markdown extraction at L3 (L1 excellent, L2 good, L3 good), measured against universal-ctags (headings) and lychee 0.24.2 (links) on rust-lang/book and facebook/docusaurus: a relative link to another `.md` or `.mdx` file (inline, reference-style or an HTML anchor) is a file edge from the section holding it, a fragment an edge to the heading it names, and `.mdx` reads as Markdown. Heading names keep their inline markup and YAML front matter is no longer a heading.
- `[readouts] mode = rich`: a failing test run's line names the tests fixed, still failing (since which HEAD) and new against the session's previous run, `not run` for a previous failure outside this run's scope, and flags a possible flake when a test failed and passed at the same HEAD and working tree.
- Lua extraction at L3 (L1 excellent, L2 good, L3 good), measured against luaparse (definitions), the grammar's own tags query (calls) and Lua's own `package.searchpath` over `require` statements (imports) on Kong 3.9.2 and telescope.nvim v0.2.2: a literal `require "a.b"` or `pcall(require, "a.b")` resolves to `a/b.lua` or `a/b/init.lua` under the repository root, `lua/`, `lib/` and `src/`, and every Lua file now has a `<module>` symbol that its top-level requires and calls are made from.
- Jupyter notebook extraction at L2 (L1 excellent, L2 good): the code cells of a `.ipynb` (format 4) are read with the python dialect, IPython magics made Python first, and each class, function, method and variable lands on the line of the notebook file that holds it, measured against nbformat plus universal-ctags (definitions, tier B) and Python's `ast` (references, tier C) on jakevdp/PythonDataScienceHandbook and microsoft/ML-For-Beginners, 2026-10-19
- CircleCI extraction at L2 (L1 good, L2 good): `.circleci/config.yml` jobs, workflows, commands and executors, with the jobs a workflow runs or requires, the commands a job or command invokes and a job's executor as edges to names the file declares, measured against `circleci config process` (definitions and workflow jobs, tier A; command and executor use a source walk, tier C) on circleci-cli, cimg-base, hasura/graphql-engine, cypress and openapi-generator, 2026-10-19
- Luau extraction at L2 (L1 excellent, L2 good), measured against Luau's own parser (definitions: functions and type aliases) and tree-sitter-lua's tags query run on the Luau grammar (calls) on luau-lang/luau 0.743 and dphfox/Fusion v0.3-beta: `.luau` files are read with the Lua dialect's patterns plus type aliases and typed signatures, as `LuauFunction` and `LuauType` symbols.
- R extraction at L2 (L1 excellent, L2 good), measured against universal-ctags (definitions) and the grammar's own tags query run by py-tree-sitter (calls) on tidyverse/dplyr v1.2.1 and r-lib/testthat v3.3.2: `.R` and `.r` files are read for function assignments (`<-`, `<<-`, `=`, string and backtick names, nested helpers) and their plain and namespaced calls; S4 and R6 classes and `library()`/`source()` edges are not read.
- Perl extraction at L2 (L1 good, L2 good), measured against universal-ctags (definitions; it writes no end line for Perl) and PPI (calls) on mojolicious/mojo v9.50 and Perl-Critic v1.154: `.pm` files, `.pl` and `.t` files whose content is Perl's (Prolog shares `.pl`) and scripts with a perl shebang are read for packages, subs, the 5.38 `class` and `method` forms, and the function and method calls they make (a function named like a Perl builtin is no call); the `use` graph is not read.
- GraphQL extraction at L2 (L1 excellent, L2 excellent), measured against graphql-js on octokit/graphql-schema and gqlgen: operations, fragments, type extensions, directive definitions and enum values index, with the type references each definition makes, where a file holding an operation or an extension used to return nothing; a type's Signature is now its header as written.
- The decision port's per-site rails: `[decide] max_asks_per_site_per_session` (default 200, the minimum across layers, so a repository can lower it and never raise it) caps one site's asks per session, after which the site answers on its deterministic path with one `capped` line; a site whose last 100 asks in `decisions.jsonl` hold more than 10% misses turns off for the process's lifetime with one `killed` line (`kill_source: misses`), and `fathom doctor`'s `decision_provider` row names it. Provider `none` is unchanged.
- Answers shaped by a configured decision provider now say so: the obligations envelope, `POST /route` and `certify suggest` carry `decided_by` (`judged(<site> · <provider>/<model> · conf <x> · calibration <status> <n>/100)`, `computed`, or `computed · not consulted (<class>)` when the provider missed), the degrade line is written once per session, site and class, and `fathom statusline` adds `J:off`, `J:on(<n>)` or `J:killed`; provider `none` output is unchanged.
- `fathom readout report --baseline` reports the share of the test-run hook's failing-run rows whose named file or test is touched by one of the next three calls, with a Wilson 95% interval, the rows it cannot place counted apart, and the window from the first row.
- `fathom doctor` gains a `readouts` row: the `[readouts]` mode and roots, rules loaded and refused (naming the first failing fixture of each refused rule), the harnesses with no readout view, and another Bash output rewriter wired in the client's hook settings (named by its command's first word). It reads ok or warn, never fail.
- `[readouts]` config section (`mode` off|observe|lean|rich, `rules.<id>.mode`, `roots`), every key user-only so a repository's config that sets one is dropped and named by doctor.
- `fathom readout-hook` on PostToolUse(Bash) and PostToolUseFailure(Bash), off by default: with `[readouts] mode` set and not off (and the directory the command acts in, a leading `cd` or `pushd` target, else its `cwd`, under `roots`), it computes the readout of the command's output and writes one `readout` row to the hook log (rule, sizes, dropped lines by class, `cmd_sha` and `rule_sha`, never output or command text) and emits nothing, in every mode for now; `fathom init`, `plugin render`, `fleet reconcile` and doctor write and expect its two rows only while the mode is on (RO-16).
- `fathom config audit --loaded [--days N]` reports, per instruction file loaded, its bytes at HEAD (est. tokens), its loads and the served claims compiled from it (RO-41, H11).
- `[hooks] claim_check` (default off): at Stop and SubagentStop, a test or gate claim in the agent's final message ("all tests pass") is checked against the session's recorded test runs, one advisory line when the last matching run failed, read unknown or ran at an older tree, at most once per stop chain; at SubagentStop it reads the repository of the agent's last Bash or file tool call in its own transcript; with it on, `fathom init` writes a SubagentStop hook entry (RO-38, RO-39, H2).
- `[hooks] gate_preflight` (default off): before a Bash `git commit` or `git push`, one advisory line naming a claimed plan row the verify engine refuses and, on a push, a missing passing test run at the current tree, read in the repository the command commits or pushes from (a leading `cd`, `git -C`); never a deny (RO-40, H6).
- `[hooks] read_outline_hint` (default off): before a Read with no offset or limit of a source file over 200 lines this session has not edited, one line naming up to 12 top-level symbols with line ranges from the file on disk and the Read offset and limit recipe; the read runs unchanged and waits at most 150 ms for the hint, and with it on `fathom init` writes a PreToolUse(Read) entry, `fathom readout-hook --event pre-read` (RO-44, H8).
- `fathom readout replay|explain|test`: the readout rules over the local Claude Code transcripts as aggregates only (hits, would-remove bytes, protected trips by stream, the offline dropped-line reference rate, shapes and their compound share), one command's output explained through the engine, and rule files held to their fixtures and the contract (RO-14).
- Built-in readout rule `read-outline`, observe only: a whole-file read (`cat <file>`, `sed -n 1,$p <file>`) of a source file over 200 lines reads as its first 40 lines, an outline of its top-level symbols with line ranges from fathom's own extractors run in process over the text as printed (150 ms, else `outline: not computed (deadline)`) and `sed -n A,Bp` span recipes; an explicit range, several files and a read of a file the session edited or that differs from HEAD stay raw (RO-8).
- `fathom readout report [--since DURATION] [--json]`: per readout rule, hits, bytes removed, est. tokens (bytes/4, an estimate), the dropped-line reference rate replayed from the local transcripts (a row it cannot replay counted apart as not computable, never as clean), the lean recovery rate and net bytes (`no lean rows` until lean runs), and a promote, keep, demote or insufficient verdict by ADR-071's Wilson bars; per rule only.
- `fathom statusline [--subagent] [--json]`: one status-line segment from Claude Code's status-line JSON on stdin (`fathom ● @<sha>` with `+uncommitted` or `dirty unknown`, `◐ <n> behind`, `○ daemon down`, `· not checked (<verdict>)` when the hook guard sent no request, `? not indexed`; an unknown never reads as 0 or clean), and `/statusz?format=json&root=<dir>` answers the service whose root holds `dir` with `head_commit`, `behind` and `dirty` (null with a reason when unread); the JSON without `root` is unchanged.
- The core plugin carries the `fathom-scout` subagent (`agents/fathom-scout.md`: the default surface by plugin-scoped tool name, `omitClaudeMd`, low effort, routing and a 40-line answer contract with a `file:line` receipt per claim) and a `settings.json` holding only `subagentStatusLine`, which runs `fathom statusline --subagent` or prints `fathom: not on PATH`; plugin 0.3.14.
- The repository is a Claude Code marketplace: `.claude-plugin/marketplace.json` (`fathom-plugins`) and `plugins/fathom/`, rendered by `fathom plugin render --out plugins/fathom --marketplace .` (`--marketplace` now takes the root its sources are relative to, and such a render stamps no build version) and held to a fresh render by a test; the README installs it with `/plugin marketplace add GeoffMillerAZ/fathom` and `/plugin install fathom@fathom-plugins` after the binary.
- `fathom readout replay` gains three sizing sections, aggregates only, each with its bar evaluated as `verdict_projection`: `dedupe` (Bash and Read result bytes that exactly repeat or are contained in an earlier result since the last compaction, within a batch and across turns, recovery repeats excluded), `spawns` (workflow and other subagents whose first three tool calls are code discovery) and `compactions` (over the latest 14 days, compactions a day and the pre-set files re-read after each).
- serve-http's /metrics gains `fathom_cache_events_total{group,event}`: every memo cache group's hits, misses, flights, uncached builds, `abandoned` (the last waiter left), `refuse_cancelled` (a refuse drop ended the build), `declined` and `evicted_<reason>`; every existing family is unchanged (RT-188).
- `fathom resources --days` prints an RT-M13 verdict line over the last seven complete UTC days (shed + refuse minutes at most 201.6, 2% of the week, and no host-trigger rung entry at normal pressure; a day logged before rung entries were recorded reads INCOMPLETE, named), each day row gains `rung_entries` by rung, trigger (own mark, pressure, swap growth, swap-volume floor) and the host pressure at entry plus `shed_minutes_by_trigger` (each shed or refuse minute counted against the trigger that entered the rung the ladder was on; a step-down returns to the trigger that entered the lower rung), and `--json` adds `rtm13` with the week's split (minutes logged before the split named `unattributed_shed_minutes`); a FAIL line adds the pressure share; the RT-M1 verdict is unchanged.
- Under `[readouts] mode = "rich"` (user-only, default off), the readout hook adds one line beside a failing `go build` or `go vet` run: its errors grouped by the identifier or file they share, the root a symbol the working tree changed named at its definition, and the group touching a file this session edited first (`go build: 8 errors, 3 roots: the signature of money.Round, money/money.go:3, edited this session (3 errors); ...`); the raw output is never changed, and a miss adds nothing and is named in the hook row (`rich_missed`).
- A command guard in `.fathom/guard.cue` may declare `requires`, the argv tokens a command matching its `command_scopes` must carry (`requires: ["-race"]` beside `command_scopes: ["go test *"]`); under `[readouts] mode = "rich"` a matching command that lacks one gets one advisory line naming the token, the guard's `.fathom/guard.cue:N` and its note. A guard without it behaves as before, and nothing is denied for it.

### Changed

- The extractor epoch is derived from per-wave bump files (`internal/fusion/epochs/<wave>.md`) and a digest of the embedded dialect manifests, queries and grammar lock, so a dialect data change re-extracts without a hand bump; `extractor_epoch` in `fathom version --json` and SARIF run properties changes form from `68` to `<number>.<12 hex>`, and the first ingest after upgrading re-extracts every file once.
- `PROVENANCE.md` drops its `**total**` row, and generated files merge by regeneration: `task merge-drivers` sets the `fathom-regen` driver `.gitattributes` names, `task regen` rebuilds them after a merge.
- With `[readouts] mode` on and `[hooks] test_run` on, `fathom init` writes one PostToolUse(Bash) and one PostToolUseFailure(Bash) entry, `fathom readout-hook`, which runs the test-run hook in its own process (same line, same row); the old `session-hook --event test-run` and `--event failed-test-run` entries stay accepted and stay silent while the readout mode is on, so the tier runs once (RO-21).
- The test-run log row gains additive `head`, `tree` and `scope` keys on every matched run, passing or failing; a key that could not be computed in the hook's deadline is absent (RO-35).
- Instrument B (the discovery observer) now counts `cat`, `head`, `tail` and `sed -n` of a source file as `shell-read` beats, and a Bash command it cannot read as an `unreadable` beat, never as zero reads and never as a discovery action (the research report names the count and keeps it out of the share's denominator); it reads commands through the one command lexer (RO-29, RO-4).
- The fathom-activate skill's step 6b no longer names the removed `fathom intent render --playbook`: there is no INTENT.md, delete any root one and read `docs/intent/README.md` (plugin 0.3.15).
- A hook's graph-backed call that cannot finish inside its caller's deadline now answers at once with a named partial (`navLoadNote`'s declined note, verify_plan's stage-building partial) instead of starting a build it abandons; a call with no deadline, a nested build, or a deadline that would keep the build for the next caller still starts one. A hook RPC's `X-Fathom-Deadline` is now the shorter of the hook's remaining deadline and the call's own client timeout (RT-190, RT-191).
- Memory pressure at `warn` now sheds only while fathom's own footprint + children is a real share of its budget: at or above `[tenancy.host] allowance` × budget (default 0.45, 0.6 × `shed_at`; `allowance = 0` restores shedding at any footprint), and a rung held by pressure steps down after 60 s under 0.6 × the allowance; pressure `critical` and the swap-volume floor are unchanged, and `fathom resources --json` names the allowance in `host_triggers`.
- A restarted daemon or a repeat `fathom ingest` reads only the files whose stats changed: the stat gate's prior record is the store's `file_stats`, whose rows carry the extractor epoch they were written under, so an epoch bump still reads every file (RT-180).

### Fixed
- A managed daemon started by hand no longer keeps launchd's or systemd's own instance down: the daemon record and start row say whether a supervisor runs it, a relaunch that finds an unsupervised same-label holder exits 75 (`port_held_unsupervised`, never counted by the breaker) so the supervisor takes the port over once it is free, and the hand start and the daemon-down text name the supervisor's restart command (O1363).
- Ingest stores the file-resolved edges of a file a dialect claims by basename, not only by extension: a `BUILD`, `WORKSPACE` or `MODULE.bazel` file's `load()`s, a `meson.build`'s `subdir()`s and a `CMakeLists.txt`'s `add_subdirectory()`s, which the file-resolve pass skipped before (O379).
- The Stop hook's verify ends after one daemon call when the first answer is a stage partial that verified no row, instead of sending elision follow-ups that can only return the same partial.
- `fathom doctor` and `fathom daemon status` name the right reload step for a drifted daemon plist: `fathom daemon install --reconcile` run from your own terminal reloads the agent itself, and the `launchctl bootout` and `launchctl bootstrap` lines are only what an agent shell prints instead (O1252).
- `fathom certify suggest`, `fathom assess` and `fathom plans context --preflight --score` ask the decision port from their own site (`certify_suggest`, `assess`, `preflight`), so `[decide] max_asks_per_site_per_session` and the per-site miss kill cover them (O1246).
- The edit hook's obligations re-rank asks for its caller's ceiling less the time its joins took, never its own `rerank_timeout_ms` past that ceiling; joins that spent the ceiling ask nothing, and the call's `rank_note` names `kill_source deadline` (KS-10).
- `fathom daemon status` and doctor read a binary's signing authority with `codesign -dvv`: on macOS 27 one `-v` prints no `Authority` line, so a binary signed with fathom-dev read as "signed, no authority named".

- The test-run hook reads the repository a leading `cd` or `pushd` moves the command into (its config, declared commands and row keys), not only the session's `cwd`, which inside a workflow or Agent subagent working in a scratch clone is the parent's project (O1230).
- The test-run hook's saved outputs under `workspace/tmp/test-run` are bounded at 50 files and 8 MiB, oldest removed first and only its own `<16 hex>.log` files (RO-36).
- On macOS the managed daemon stopped starting after many deploys: Background Task Management tracks the launchd agent by its program's hash, and every deploy changed it. `fathom daemon install` now writes a `fathomd` launcher beside the binary that execs the sibling `fathom` and that deploys never rewrite, sets `WorkingDirectory` to `~/.fathom`, and `fathom doctor` and `fathom daemon status` report a plist that still runs the binary itself; `fathom daemon install --reconcile` migrates it and `--direct` keeps the old shape (O1233).
- `fathom verify`'s wired criterion counts a method that only a standard-library interface calls, such as an `http.RoundTripper`'s `RoundTrip` that net/http runs, when reached code converts its type to that interface (O1237). The next ingest of each Go service re-runs its typed pass.
- A `fathom ingest` that finds the certifications layer already compiled at the current commit, and that read a file, found one gone or picked up a new run, re-checks every certificate: one whose test moved to another file goes `stale`, one whose test is gone goes `contradicted`, and a new attested run picked up from `[tests] results_dir` at the same commit updates its state. An ingest that changed none of those skips the re-check and says so, which brings a repeat ingest of fathom itself from about 9 s back to 1.4 s (O1270).
- With `CLAUDE_CONFIG_DIR` set, `fathom config audit` reads Claude Code's auto-memory, and doctor, init, reconcile and the fleet report read the user settings, `CLAUDE.md` and `.claude.json`, from that directory instead of `~/.claude` (O1194).
- The prompt hook's line for a pasted stack trace, failing test or error names `localize` only when the session's fathom registration serves it (`/rpc/extended`, `/rpc/companion`); a session on the default `/rpc/agent` surface is told to `search` the most distinctive frame or error text, then `symbol_context` the top in-repo symbol (O615).
- A plan row whose quoted value carries `\"` parses under its own id whatever the count: an escaped quote is one the value carries, never a delimiter, so an odd count no longer leaves the row read as a positional `md-NNN` with no contract; and `fathom verify`'s lint names, as an error with its line and the reason, any `{id:` line still read as no row (O842).
- `fathom verify`'s `edge_wired` failure tells a `reachable_from` that names no symbol, or one several symbols carry (with up to five of their keys and how to write the name), from a missing typed path, which is all it said before; the verdict stays a failure (O1258).
- `fathom tests select --safe` counts the tests whose only reach to a changed symbol is a dispatch into error's `Error`, `fmt.Stringer`'s `String` or `fmt.GoStringer`'s `GoString`, in one line naming the method and in `--json` under `stdlib_dispatch_only`, and still selects them; the opt-out `--skip-stdlib-dispatch` leaves them out (one edited `Error` method in `cmd/fathom`: 626 tests selected, 625 of them counted, 1 with the opt-out) (O887, O1288).
- `fathom tests select` says when the index it reads holds another commit than HEAD, in one line and in `--json` under `index`, instead of printing an empty selection with no reason; the wave guide's worker step names the scratch ingest to pass with `--store` (O1199).
- The observatory Config panel reports a repository's platform-module bindings from `.fathom/modules.cue`, the file `fathom init` writes and the server reads; it checked `.fathom/bindings.cue`, which nothing writes, so it always reported no bindings.
- The daemon's ingest (the `ingest` MCP tool and auto-refresh) now refreshes a service's certificates the way `fathom ingest` does: it rescans `// certifies:` markers, then compiles or sweeps, so a certificate whose test moved goes stale without a CLI verb. A service that declares no certificate pays one declaration read.
- `task stranger-check` and `fathom doctor --stranger` run `fathom init` on a stranger's repository, the fauxtown fixture fleet as its own git repository, instead of the whole copied tree, whose first index outlasted init's wait on a loaded host and failed the gate with nothing wrong; the copy still builds standalone, doctor must read `binary_fresh` as PASS against it, and the first `find_symbol` must find the stranger's code. The check fell from 392 s to 95 s on one host (O1339).

### Security
- Requests to a configured decision provider now carry only the fields each site declares, redact secret-shaped strings (counted as the decision row's `redactions`), honour the new `[decide] exclude` path globs (every layer's union), refuse a question that names a security property, and can be printed without sending (`FATHOM_DECIDE_PLAN_ONLY=1`); provider `none` is unchanged.

## [0.15.6] -- 2026-10-18

Release impact: patch

**What this release is.** A fix. The test-run hook never injects a line its
work finished past the hook deadline, and the observatory console verifies a
plan in one in-process `verify_plan` call.

### Fixed

- The test-run hook drops a line its work finished at or past the hook deadline, as TR-10 says: it checked only its context's own timer, which a second timer set to the same instant (the hook guard's transport) could beat on a loaded machine.
- The observatory console verifies a plan with the same declared dispatch budget as the CLI, so a big plan answers in one in-process `verify_plan` call instead of ~13 rows per call against the 90 s deadline. (O1169)

## [0.15.5] -- 2026-10-17

Release impact: patch

**What this release is.** A fix. Daemon-mode `fathom verify` and
`fathom plans status` get a whole plan's verdicts in one `verify_plan` call,
and a daemon that cannot read the typed call graph leaves reachability rows
Unknown (exit 2) instead of failing rows that hold.

### Fixed

- Daemon-mode `fathom verify` and `fathom plans status` get a whole plan's verdicts in one `verify_plan` call: they declare a dispatch budget sized to what they read (`X-Fathom-Max-Bytes`, capped at 4 MiB), where the 16,000-byte agent default cut a long plan into one graph-rebuilding call per few dozen rows. Agent surfaces' default answers and schemas are unchanged; an answer still cut falls back to the follow-up calls. (O1014, O1169)
- A daemon `verify_plan` that cannot read the typed call graph (a read cut at its shed or refuse mark) no longer fails reachability rows that hold: they read Unknown naming the cause with a `partial`, and strict `fathom verify` exits 2 when every unproven row is one (index-stale, graph fallback, unanswered verdict) (O1162).

## [0.15.4] -- 2026-10-17

Release impact: patch

**What this release is.** A fix. `fathom plans status` and `plans next`
report every plan the daemon could verify and name the ones it could not,
daemon-mode `fathom verify` stops re-verifying rows it cannot receive, and
a hook's hand-built daemon requests name its agent session.

### Fixed

- `fathom plans status` and `fathom plans next` no longer fail whole when the daemon cannot verify one plan, for example when the call misses its deadline. That plan's row reads `not measured: <the error>` (`--json`: `not_measured`), every other plan still reports, a summary line counts the unmeasured plans, and the verb exits 2 (operational). (O1161)
- Daemon-mode `fathom verify` follow-up calls for a long plan carry no more rows than the last cut answer held. The daemon used to re-verify every missing row on each call: about 20,000 row checks for language-breadth's 768 rows. The graph rebuild on each call remains (O1014). (O1161)
- A hook's `/identity`, `/indexed`, `/refresh-request` and session live-example requests now name the hook session in `X-Fathom-Caller`, like its `/rpc` calls (O1151).

## [0.15.3] -- 2026-10-17

Release impact: patch

**What this release is.** A fix. A strict verify held back only by a stale
index exits 2 with one aggregate line instead of failing every `gone:` row,
the prompt hook stops resolving prose acronyms to files, and the stop and
session hooks name their agent session.

### Fixed

- `fathom verify --strict` exits 2 (index-stale), with one aggregate line, when a dirty tree or a commit after the ingest is the only reason claimed `gone:` rows are unproven; any other unproven row still exits 1 (O1160).
- The prompt hook no longer turns a prose word such as "ADRs", "APIs", "URLs" or "SDKs" into a file and injects its rules: a plural acronym is not a symbol candidate, and a symbol resolves only on an exact `find_symbol` hit (a case-insensitive hit only when backticked or called). (O1114)
- The stop and session hooks now name their agent session in `X-Fathom-Caller`, completing O1151 for every hook that calls the daemon.

## [0.15.2] -- 2026-10-17

Release impact: patch

**What this release is.** A fix. The edit, prompt and test-run hooks now
name their agent session to the daemon, so a killed decision port is
disclosed once per session and site in practice, not only by key.

### Fixed

- The edit, prompt and test-run hooks now name the hook payload's `session_id` in `X-Fathom-Caller`'s `session=`, so the daemon's `/rpc` and `/route` see the session and a live kill is disclosed once per session and site (O1151).

## [0.15.1] -- 2026-10-17

Release impact: patch

**What this release is.** Fixes. `fathom obligations` and `/explain` now
withhold exactly the stale rules the `obligations` tool withholds, a
withheld adr-rules claim's changed design section is served raw in its
place, and a killed decision port says so once per session and site rather
than once per daemon.

### Fixed
- `fathom obligations` and `/explain` withhold and count (`stale_withheld`) exactly the rules the `obligations` tool withholds for a changed section or a deleted source file, through the tool's own predicate, instead of listing them as served (O1149, I38).
- `obligations` serves a withheld adr-rules claim's changed `docs/design` section raw in its place: the raw fallback ranks each withheld layer's stale sections from that layer's own corpus, within the same reserve (O1125, I38).
- Under `~/.fathom/decide.off` or the user-only `[decide] enabled = false`, a `serve-http` daemon now writes one `killed` line per session and site (the `/route` prompt hook, the obligations re-rank, the compile pre-filter, each session its `X-Fathom-Caller` names), not one for all of them, and the line names both (O1131, KS-14).

## [0.15.0] -- 2026-10-17

Release impact: minor

**What this release is.** Intent practice v2 (ADR-074). The `INTENT.md`
playbook region is retired, `docs/intent/README.md` is the one router, and
`fathom intent render --playbook` is deprecated. The router's
sensitive-surface table now generates `.fathom/guard.cue`'s intent entries:
an edit under one is served its invariant ids and rungs in at most two of the
six slots, and the session-start primer names the repo-wide invariants once.
`fathom intent index --check` and the futures vet keep the decisions index and
the futures honest, `obligations` withholds a rule whose source file is gone,
and the audit's U4 reads the repository's own tests.

### Breaking
- The `<!-- fathom:intent-playbook -->` region of a root `INTENT.md` leaves the compatibility surface: `INTENT.md` is deleted, the `intent-render-check` gate step goes, and `config audit`'s `pointers_resolve` no longer scans the file (ADR-074). Migration: delete any root `INTENT.md`; read `docs/intent/README.md`.

### Deprecated
- `fathom intent render --playbook` now prints a notice naming ADR-074 on stderr, writes nothing and exits 0; it is removed one minor release later (`docs/plans/intent-v2.md` IV-10).

### Added
- `fathom intent audit` names a future whose fields fail `schema/intent` (U16): a Confidence or Horizon that is missing or off the list, a Value without its why, or a measured Basis that cites no path, instrument or commit (ADR-074, `docs/plans/intent-v2.md` IV-5).
- `fathom intent export --guard --write` writes `.fathom/guard.cue`'s intent entries from the router's sensitive-surface table inside fathom's own markers, every hand-written entry untouched, and stops on a region edited by hand; `--check` exits 1 naming the first entry that drifted from the router, and runs in this repository's `task validate` (ADR-074, IV-3a).
- `fathom intent index --check` fails, naming each id and the side that lacks it, when an `ADR.md` heading and its `docs/intent/decisions/README.md` row drift apart; it is a `task validate` step.
- The fathom-plan skill's plan skeleton opens with a Request line, the requester's words verbatim and dated.

### Changed
- The pre-edit guard of a file under a sensitive surface names the surface's invariant ids with their rungs (`I19 [test]`) and its Done-needs tests in at most 2 of the 6 rule slots, the governing rules taking the rest, also when a hand-written guard matches the file first; the provenance stamp no longer reaches the agent, and an id is matched whole (`I1` no longer rides along with `I12`) (IV-3b).
- The session-start primer (and `fathom prime`) lists once, by id, the invariants no sensitive-surface row names: repo-wide, so no edit carries them; the line is bounded and the whole-primer cap holds (IV-3c).
- The stanza's obligations sentence, where the intent channel is wired, names what is delivered: the governing rules and the invariant ids of the surfaces `.fathom/guard.cue` covers, no longer "matched invariants on every edit"; `fathom fleet reconcile --apply` refreshes a wired stanza (IV-4, O1134).

### Fixed
- `obligations` withholds a compiled rule whose source file no longer exists at the indexed commit, counts it in `layer_coverage.stale_withheld`, and names the recompile in `remedy` (new `layer_coverage.missing_sources`), instead of serving it as governing (O1135, I38).
- `fathom intent audit` U4 checks a named test against the repository's own `*_test.go` files, so a daemon index that lags HEAD no longer reports an existing test as missing (O1136, I38).

## [0.14.1] -- 2026-10-17

Release impact: patch

**What this release is.** Fixes. A decision-port kill now lifts without a
restart, and your consent opens every provider a repository names, not only
`llm`. Navigation names the real caller of a package variable's
initializer, `blast_radius` deletes on a Go type list the type's users,
`test_impact` ranks the nearest tests first, and `obligations` judges a
stale section by the layer that compiled it. Releases are published as
prereleases.

### Security
- A daemon or CLI started while `~/.fathom/decide.off` or the user-only `[decide] enabled = false` closed the decision port now reopens it on the first ask after the kill lifts, no restart; while killed it still reads no key, execs nothing and dials nothing, and the startup line and `fathom doctor` say so (O1132, KS-2).
- `[decide] allow_repo_provider = true` now opens a repository's `typesafe` or `openai` provider, not only `llm`, with a key `~/.fathom/config.toml` names beside the transport it is for (`transport = "openrouter"`); a key named with no transport still never follows a repository's provider, and a transport's well-known variable is still read only when your config names the provider (O1133, KS-4).

### Changed
- `fathom verify` with several `--plan` files now prints `plan <file>` before the first failing row of each plan, so one call over every plan names which plan failed.

### Fixed
- `obligations` withholds an `adr-rules` rule whose section the adr-rules layer reports stale, a `docs/design` section included, and no longer withholds a rule fresh in its own layer because another layer's compile of the section is stale (O1122); `pkg/client` adds `Client.LayerCoverage` for any layer.
- `list_services` and the primer no longer list `javascript` for a service with `.json` files and no JavaScript: the `JSONKey` kind names no language (O1123).
- `find_callers` and `symbol_context` name a call made inside a package-level variable's initializer (a function literal kept in a var as a test seam, a route table) as that variable at its own file and line, not as every `init()` of the package (O1124: 117 direct callers of `openDecisionPorts` on fathom itself, 11 real; now those 11); an `init()` is listed only for the file its call is in. The next ingest re-derives every Go package's typed edges.
- `blast_radius` `kind: "delete"` on a Go type or interface lists every symbol that uses it (a parameter, a result, a field, a composite literal, a conversion, a `pkg.T` from another package), not only its callers, and every referent row carries `key`, the symbol's own key (O1127); a source scan cut short by its budget is named in `type_scan`.
- `test_impact` lists its tests nearest first (hops from the symbol, then package distance, then name) instead of in key order, so a test that calls the symbol directly is never cut while a farther one is shown; it budgets its own page, and the `elision` line names how many were cut and the `offset` argument (new) that lists the next page (O1128).

## [0.14.0] -- 2026-10-16

Release impact: minor (pre-1.0: breaking bumps minor)

**What this release is.** A security fix for the decision port: a
repository can no longer open a paid decision provider on your key, and you
hold a kill that closes the port before any key read, a running daemon
included. It is a minor release because a repository's own `[decide]
provider` now needs your consent. Releases are published as prereleases.

### Breaking
- **A `[decide]` provider only a repository's `.fathom/config.toml` names no longer opens (security, O1113, I66).** The port stays `none`, reading no key, unless `~/.fathom/config.toml` sets `[decide] allow_repo_provider = true`, and a transport's well-known key variable is read only when your config names the provider; `fathom doctor` names the refusal. Migration: name the provider in `~/.fathom/config.toml`, or set `allow_repo_provider = true` there.

### Added
- `FATHOM_DECIDE=off` closes the decision port in its process, and `~/.fathom/decide.off` or the user-only `[decide] enabled = false` closes it in every process, a running daemon on its next ask; the user-only `session_sites` and `deny_sites` gate session sites and refuse a site, and `fathom doctor`'s `decision_provider` row names the closing source and calls nothing (KS-1 to KS-7, ADR-073).
- `task validate` fails a record from ADR-071 on in `ADR.md`, or a `docs/design/*.md`, without a decision-models section (DP-41, ADR-073 decision 12); the design docs that predate the rule sit on a grandfather list whose length can only go down.

## [0.13.1] -- 2026-10-16

Release impact: patch

**What this release is.** Fixes for answers that disagreed with the index's
own state: discovery gaps for files the repository excludes on purpose, a
rule served from a section the same response reports stale, and a TypeScript
service listed as JavaScript. Releases are published as prereleases.

### Fixed
- A typed edge into a file excluded by a declared policy (`.fathomignore`, a skip dir, the default cache list) is counted as policy-excluded, not a discovery gap: ingest of fathom itself no longer prints ~130 `discovery gap` lines and `fathom doctor` no longer WARNs for them.
- `obligations` no longer serves a compiled rule from a section the layer reports stale: it is withheld, counted in `layer_coverage.stale_withheld`, and the section's current text is served raw.
- The primer and `list_services` list `typescript` for a service whose `.ts` files hold only functions (O1086): languages are read from the stored file classes, not the shared JS kinds.
- Releases stay prereleases: GoReleaser overwrote the flag the release was created with, so v0.13.0 first published as Latest (`.goreleaser.yaml` now says `prerelease: true`). The install docs find the newest release through the releases API, since `/releases/latest` skips prereleases and the old lookup produced no version.

## [0.13.0] -- 2026-10-16

Release impact: minor (pre-1.0: breaking bumps minor)

**What this release is.** fathom installs without a clone: it is one
self-contained Go module, so `go install .../cmd/fathom@v0.13.0` and
`go get .../pkg/client@v0.13.0` need no credentials and no `replace`, and each
GitHub release carries prebuilt darwin and linux archives. The app-kit mirror
shrinks to fathom's build closure, and `plans status` names an archive the
absorption check would refuse.
Detail: `docs/changelog/0.13.0.md`.

### Breaking
- `pkg/client`'s kit-typed parameters (`WithDecision`'s `decision.Decider`, `WithLLM`'s `llmclient.LLMClient`) now come from `github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/...`: a consumer that built against the private `github.com/geoffmilleraz/agentic-kit` path changes that import (ADR-070).

### Changed
- fathom is one self-contained Go module: `go.mod` requires no private kit and carries no `replace`, the agentic-kit and app-kit mirrors are packages of it (their `go.mod` kept as `go.mod.upstream`, `.vendor-commit` the pin), so `go install github.com/geoffmilleraz/fathom/cmd/fathom@<tag>` needs no credentials; a local `go.work` no longer substitutes a sibling kit, `task sync-kit` does (ADR-070, DS-1, DS-2).
- The module no longer ships app-kit's design-asset data (the TailwindPlus manifest and indexes), docs, spikes or other apps: the app-kit mirror holds only fathom's build closure, computed by `scripts/kit-closure.sh` into `third_party/app-kit.keep` and checked by `task scripts-check` (1,806 files to 317; nine `go.mod` requirements dropped) (ADR-070, DS-4).

### Added
- Each GitHub release gets prebuilt `fathom` archives for darwin and linux on arm64 and amd64 plus a sha256 `checksums.txt`, built by GoReleaser from `.github/workflows/release.yml`. They are not signed or notarized; the install docs list the download, `go install` and source paths.

### Fixed
- `fathom plans status` prints `archivable:` only for plans `plans archive` would accept; a plan with nothing open whose row ids are not absorbed prints `archive blocked: <plan> -- <n> row ids not absorbed (first: ...)`, and `--json` adds `archive_blocked` and `unabsorbed_ids`. A plan whose absorption check cannot run (an unreadable `CHANGELOG.md` or ADR) prints `archive blocked: <plan> -- absorption check failed (...)` with `archive_check_error` in `--json`, never `archivable:`.
- A `serve-http` that finds its port held records its `exit{port_held}` lifecycle row even when the holder's command line is tens of KiB long: the holder's command is clipped to 240 bytes, where before the overlong ledger line was skipped on read and the exit went unrecorded.

## [0.12.1] -- 2026-10-16

Release impact: patch

**What this release is.** Fixes for answers that read a failure as an empty
result: a call graph that could not be loaded, a graph build held back by the
memory ladder, and review findings a daemon dropped. `fathom tests select --safe`
selects fewer tests on the same diff.

### Fixed
- `blast_radius` names a call graph that could not be loaded, with the tenancy level and budget knob at shed or refuse, instead of answering "no symbols for service" for an indexed service; `find_callers` and `localize` name it too (O1013).
- At shed or refuse, `verify_plan`'s deadline partial names the tenancy level and the budget knob (`FATHOM_TENANCY_BUDGET`, `[tenancy].budget`) instead of saying the call graph "is still building" (O1073).
- A daemon persists `review_diff` findings (`findings_persisted:true`), and `search` `kind: "docs"` and `staleness_report`'s doc-link drift work through it: the auto-refresh wrapper forwards them, and a test fails when it misses any capability the client has (O1072).
- The session primer and `list_services` list `typescript` for a service holding TypeScript interfaces, type aliases or enums (they read `javascript` only).

### Changed
- `fathom tests select` counts a Go symbol as changed only when its own text changed since `--since`, never because lines moved above it, and follows an interface call into a type's method only for a test whose binary constructs that type. `--safe` on a diff of fixes to fathom's own repository now selects 1,850 tests in 13 packages instead of 3,314 in 67, with no failing test missed in the mutation check (TQ-29, TQ-30, TQ-M6, O1001).

## [0.12.0] -- 2026-10-16

Release impact: minor

**What this release is.** The beta made friendlier to a first visitor and
wider in what it reads. A fresh install reads green in `fathom doctor`, the
hooks are silent where fathom is absent, help and doctor text carry no
internal ids, and the README leads with a quick start read on a clean clone.
Kubernetes workloads, Jenkins pipelines, GitHub Actions, Helm and PowerShell
read L3. `read_source` reads a file at a commit, `fathom version --json`
prints the index's cache-key parts, the findings importer hashes cited lines
at the finding's revision, and a store on read-only media mounts. The hook
paths run to their own deadlines, and a spending compile asks the prefilter
once per section. Not yet read: RT-M1; the index-snapshot timing read CUT, so
cache-from snapshots stay deferred.
Detail: `docs/changelog/0.12.0.md`.

### Added
- Kubernetes manifests store each workload's per-container facts (requests and limits, the three probes, init containers apart, replicas) from all three pod paths, matched field for field by `k8s.io/api`'s typed structs; a templated value, env, args and command are never read (LB-F451). Kubernetes L3 reads good with its ceiling written, the Service selector joins graded at tier A by apimachinery's label-selector matcher (LB-F456).
- `symbol_context` on a Kubernetes workload names, in `typed_edges.observed_by`, the ServiceMonitor and PodMonitor objects whose selectors reach it, and a core Service's `spec.selector` is stored as a route-ref edge to the workloads it fronts (TL-15).
- Jenkins pipelines read L3: a literal `load` step is an edge beside the `vars/` library-step calls (LB-Q-jenkinsfile). GitHub Actions and Helm read L3 good, each with its ceiling written (LB-F457, LB-F458).
- PowerShell reads L3: dot-sourcing, `Import-Module`, `using module` and a module manifest's modules resolve as file edges, and a script's top-level call is stored when it binds to a definition of the repository (owner ruling S5; LB-F521..LB-F523, LB-F571, LB-Q-powershell).
- `fathom version --json` adds `extractor_epoch`, `typed_edge_epoch`, `format_version` (the store's `PRAGMA user_version`) and `fact_kinds`, the fathom-side parts of an index snapshot's cache key; `fact_kinds` is `{}` until a fact-kind list exists, and every existing key is unchanged (WAE-2).
- `read_source` takes an optional `ref` and serves the file as committed at that commit through git, never the working tree, adding `ref` and the resolved `commit` to its answer; every read filter holds and names itself, and a symlink or submodule entry at the ref is refused (WAE-3).

### Changed
- `ExtractorEpoch` is 64, so the first ingest after upgrading re-extracts every Kubernetes manifest, Jenkins pipeline and `vars/` file, and PowerShell file (wave lf8).
- The SessionStart and SubagentStart primers and the stanza `fathom init` writes name only the eight default-surface tools and no internal row ids. A wired repository reports its stanza behind template once; `fathom fleet reconcile --apply` refreshes it.
- `fathom init`'s printed manual steps run plain `fathom` instead of `bin/fathom`, no longer suggest fathom's own commit-msg `Verified:` check, give Codex a stdio line that shares the daemon, and link docs by URL.
- The shipped skills follow the v0.11.0 install path (plugin 0.3.11): fathom-onboard is cut to build, daemon, register, init and a checklist, and no skill names a tool that no surface serves.
- A spending `fathom compile` through the daemon sizes its timeout from an `estimate_only` dry run (a new `compile_layer` argument, default off) that asks the decision port's compile prefilter nothing, so `[decide] compile_prefilter` costs one prefilter question per section instead of two (EW-77).
- The test-run hook no longer asks the daemon `localize`. Its first candidate restated the failing test, and its call-graph build outlasted the hook, which abandoned 385 of 437 fleet calls. The hook's remaining calls run to its own deadline instead of a fixed 2 s cap (EW-76).
- The post-edit hook's `obligations` call runs to the hook's own deadline less 500 ms instead of a fixed 2 s. A pre- or post-edit answer the daemon could not give, and a test-run hook that missed its deadline, now name `daemon-error` as their silence reason (EW-76).
- `fathom module pack` clears the copy's machine-local fields (`service_meta.path`, `ingest_state.path`, `file_stats.mtime_ns`, `file_stats.ino`), which the store spec now lists in full, with no store-format change; `fathom remap` points a packed store at the reading machine's corpus (IXS-0).
- `fathom findings import` hashes a finding's cited lines at the event's `sourceRevision` through git, never the working tree; an empty, unknown or unreadable revision writes no cites link and the receipt's additive `cites_skew` names the finding, path, revision and reason (WAE-1).

### Fixed
- A pipeline named `<x>.Jenkinsfile` is read by the jenkinsfile dialect; before, ingest keyed the extension case-sensitively and left it unread (LB-F562, O1042).
- A fresh install reads green in `fathom doctor`: `lefthook_gates` is SKIP when the repository has no lefthook config, `obligations_served` is SKIP when no model provider is configured (its compile remedy now says it spends model budget), and `surface_traffic` is WARN until the first recorded call. A wired repository with a broken gate, or a configured provider with no compiled layer, still FAILs.
- Doctor row text, remedies, `fathom --help` and every subcommand's help no longer show internal row ids or section references (pinned by a test).
- `fathom init --help` lists every harness init accepts (cursor and copilot were missing), read from the table init parses.
- The typed-edge pass's over-budget error names `FATHOM_TENANCY_BUDGET` and a value to retry with (`2GiB`).
- `fathom verify` and `fathom plans status` through the daemon return the verdicts `--store` returns over the same index: `gone:` rows are evaluated instead of reading `?`, `runtime_pass` reads the attested run, and a self-skipped row no longer carries a waived pass beside its refusal (O1068).
- A `gone:` row whose corpus file could not be read is Unknown with the file named, never proven absent (O1068).
- MCP `verify_plan` reads `plan_path` from the service's indexed root; an absolute, escaping or missing path is an error naming the accepted form, never an empty result (O1069).
- MCP `search` keeps the match in a hit's text when the hit's receipt is long, instead of cutting the line to its first character (O1070).
- MCP `blast_radius` names a changed file the index does not hold: an error when no changed file is indexed, a `not_indexed` list beside the radius when some are (O1071).
- Every hook in the repo's `.claude/settings.json` and in the block `fathom init` writes is a silent no-op (exit 0, no output) when `fathom` is not on PATH, so a visitor without fathom sees no failing hooks. `fathom fleet reconcile --apply` rewrites an older unguarded entry to the guarded form.
- `fathom init` writes the canonical bare `fathom` into hooks (unless `--bin` is given), so `doctor` no longer reports false path drift on a fresh install.
- MCP `initialize` reports the fathom release in `serverInfo.version` (the version `fathom version` prints) and `fathom_meta` the build's version, instead of a fixed `1.0.0`.
- A store on read-only media mounts: `serve-http --module` and every other read-only mount open a cleanly closed WAL-mode store in a directory nobody may write with `immutable=1`, and refuse one with an uncheckpointed `-wal` beside it by name (`wal-on-read-only-media`); a store in a writable directory opens as before (IXS-12).

## [0.11.0] -- 2026-10-11

Release impact: minor (pre-1.0: breaking bumps minor)

**What this release is.** A beta for early adopters: the index reads a
polyglot repository at a measured level, with TypeScript, JavaScript, Python,
Java, C#, C, C++, PHP, Rust and Dart at L4 and HTML, CSS, Bash, Ruby, Kotlin,
Go, React, Vue and Astro at L3 in the generated board of `docs/languages.md`.
New dialects read Scala, SCSS, Vue, Svelte and Astro components, Jenkins
pipelines and the observability formats (Prometheus, Alertmanager, Grafana,
the OpenTelemetry Collector), framework routes are read from source, and
`localize` takes a stack trace, an error payload or a log line to the code
that wrote it. The proof got stricter: `fathom verify`'s reach passes only
over typed call edges, a repository's config can no longer name a command, a
key file or a path outside it, and every SARIF fathom writes names its commit
and fingerprints its cited lines. Linux builds are pure Go. Not yet read:
RT-M1 and the cloud-session readings in a real VM.
Detail: `docs/changelog/0.11.0.md`.

### Breaking
- **`fathom verify`'s `reachable_from` and `wired` pass only through typed call edges (RF-5, I57).** A row reached only through a heuristic edge now fails naming that edge and its tier, so a non-Go row's `reachable_from` cannot pass until its language gains a typed pass, while `test_reaches` still reads every edge ([migration](docs/upgrades/0.11.0-typed-only-reach.md)).
- `api_key_cmd`, `api_key_file` and `api_key_env` in `[decide]` and `[profiles.*]` are read from `~/.fathom/config.toml` only, and a repository's `.fathom/config.toml` that sets one has it ignored, named by file and field in `fathom doctor`'s `key_source_scope` and `fathom config` (SEC-1, ADR-042 amendment 2026-10-09). Migration: move the key source to your user config.
- A repository's `.fathom/config.toml` chooses behaviour only: `profiles.*.bin`, `profiles.*.endpoint` and `daemon.store`, `addr`, `call_log`, `auto_refresh` and `memory_limit` are read from `~/.fathom/config.toml` only, a repository's value ignored and named by file and field (SEC-2, the scope table in `docs/configuration.md`). Migration: move them to your user config.
- `[tests] results_dir`, `[findings] outbox`, `[connector] include` and `[[work.ci_summaries]] path` refuse an absolute path, a `..` escape or a symlink out of the repository, by name, and the session hook's outbox sync imports into the user config's `[daemon] store` (SEC-2). Migration: use a repository-relative path.

### Added
- `fathom intent render --playbook --check` compares INTENT.md's marked region with a render in memory, exits 1 naming the first differing line and never writes; `task validate` runs it (HP-39, HP-40).
- `[decide] provider = "openai"` and Jev over OpenRouter open through the kit's adapters, with new keys `base_url`, `transport` and `api_key_file` and a literal `api_key` refused at load (DP-27..DP-36, ADR-042 amendment 2026-10-09). `fathom doctor`'s `decision_provider` row names provider, model, transport, endpoint and the key's source, never the key, and `provider = "none"` stays the byte-identical default (I42).
- **The observability formats read L3: Prometheus rules, configuration and promtool tests and Alertmanager configurations excellent, OpenTelemetry Collector configurations, and Grafana dashboards and provisioning good (tier T7, ADR-062, ADR-065; LB-F307, LB-F313..LB-F320, LB-F361..LB-F365).** A PromQL selector joins the service it names, an alerting rule reaches the receivers it pages under Alertmanager's own routing, a Grafana panel joins the recording rule it reads, and the Kubernetes route pass reads Istio and prometheus-operator references.
- **No secret-bearing field is stored or served, and documents embedded in Kubernetes objects are read through their dialect (LB-F366, LB-F367, SR-76, ADR-065).** `symbol_context` answers a definition in a secret-bearing file with `snippet_withheld` and `references` with `text_withheld`; `read_source` and literal `search` stay raw-bytes readers.
- Composer manifests (`composer.json`) read L2 excellent (LB-Q-composer).
- **`localize` reads stack traces and error payloads with one frame parser, and a telemetry identity names the service (TL-1, TL-5, ADR-062).** PHP, V8, JVM, Python and Go frames and Bugsnag, OTLP, Lambda and New Relic payloads seed the throw site (top-1 on the TL-7 replay 0.259 to 0.984), and a payload's message and user fields are never read.
- **`localize` takes a pasted log line to the logger call that wrote it (TL-4, TL-10..TL-13, ADR-064).** Logger calls in Python, Java, JavaScript/TypeScript and PHP are stored as `LogTemplate` nodes, and top-1 reads 92 of 96 logged cases.
- A living intent doc split behind a router reads as one document in `fathom intent audit`, the console and the intent tab, and a new unproven kind U15 names a routing break (AG-30..AG-38, ADR-063).
- `fathom resources --days N` prints per-day footprint summaries with self-caused and host-starved refuse minutes and one RT-M1 verdict; `--json` adds `days` and `verdict` (RT-184, RT-185, RT-187).
- `fathom doctor` names the compile path's silent failures: `profile_bins`, `compile_layer` and `layers_stale` (GL-27).
- **Framework routes are read from source through one reader registry, with mounts, directory mounts included, composed across a service's files (LB-F321..LB-F336, LB-F351..LB-F353, LB-F355).** Spring MVC, JAX-RS, Nest, Fastify, Express, Next.js, Laravel, Symfony, Django with DRF, FastAPI and Flask, each scored against the running application's own route table by `indexbench route-oracle`, with counted ceilings named.
- React Router v6 and v7 routes, including a path written as an imported same-repo constant, are GET page routes handled by the component they render (LB-F378, LB-F380, SG-46, SG-47).
- Doc dialects gain opt-in value predicates (`where.not_has_key`, `where.matches`, `kv:<KEY>`, `refuse_templates`, `unique`); a dialect that sets none extracts byte for byte as before (K20, LB-F337..LB-F340, TL-9).
- **Python reads L4 excellent: every `.py` file has one module symbol, `<module>` (`PythonModule`), that owns its top-level calls and imports (K18, LB-F346..LB-F349, ADR-048 amendment 2026-10-10).** It answers an exact query and keeps every edge, but fuzzy tiers and keyword rankings never offer it unasked, and `blast_radius` `kind:"delete"` on it names the file's importers (SR-69..SR-75).
- **Jenkins pipelines are a dialect read at L2, and a pipeline's shell steps and shared-library calls reach what they run (LB-Q-jenkinsfile, LB-F373..LB-F376).** `Jenkinsfile`, `*.jenkinsfile` and `vars/<name>.groovy` read by path; scripted pipelines are read and tolerated, never claimed.
- **CloudFormation reads L3: a Lambda function binds to its handler's symbol, and CloudWatch alarms join the metric filters and functions they watch, across templates and with Terraform (LB-F368..LB-F372, LB-Q-cloudformation).** A handler file's change re-runs the pass only where a Lambda function is declared (EW-57, EW-58).
- **React components, class components, hooks and props are symbols, and a JSX element is a call (LB-F377, LB-Q-react).** React reads L3 good against react-docgen-typescript under the oracle's tier-C cap.
- **TypeScript and JavaScript read L4: each file has one module symbol (`JSModule`), and module-level `const`, `let` and `var` bindings, CommonJS value exports, class fields, interface properties and enum members are definitions (K18, LB-F381..LB-F386, LB-F411, LB-F413..LB-F415, LB-F470..LB-F472, LB-F481, LB-F482, LB-F486, LB-F511, LB-F512).** TypeScript reads L4 with every level excellent on a TSX holdout read once and JavaScript L4 good on openlayers, its eslint holdout still reading no level (O788); a CommonJS export of a function or a wrapped one binds a node Lambda handler.
- **Scala files are extracted by a tree-sitter Scala dialect and read L2 against scalameta, and named, renamed, wildcard and `given` imports resolve (LB-F396..LB-F400, LB-F438..LB-F440, LB-Q-scala).** `ScalaConstructor`, `ScalaType`, `ScalaVal` and `ScalaVar` are new kinds, and L3 waits on file edges (file_edge_recall .0112).
- An extensionless script is read by the dialect its `#!` line names (bash, sh, python, ruby, node), at ingest and on a one-file refresh; perl and zsh stay unread (LB-F417, LB-F418, LB-F494, O756).
- **Bash reads L3: the bash tags dialect extracts `.sh` and `.bash`, `source` and `.` resolve to the files they name, and a script's top level is its module symbol (`BashModule`; LB-Q-bash, LB-F391, LB-F520, LB-F524, LB-F525, owner ruling S1).** A top-level call to an external command stores no call row; a repository's shell scripts store about a third more (O969).
- **Vue (`.vue`) and Svelte (`.svelte`) components are indexed: Vue reads L3 excellent, and Svelte reads L2 and L3 excellent with L1 unmet (LB-F531..LB-F540, LB-Q-vue).** A component's script is read as TypeScript or JavaScript, its style as CSS or SCSS, and a component element in its template is a call.
- **Astro components (`.astro`) are indexed and read L3, and Angular templates are refused at the grammar gate (LB-F526, LB-F541..LB-F550, LB-Q-astro).** A component's hosted TypeScript and CSS are read on the parsers the repository's own `.ts` and `.css` files use.
- HTML's same-document ID references are calls of the page's module symbol, and html.json reads L3 excellent at every level (LB-F441..LB-F446, LB-F455, LB-F506, LB-F507).
- SCSS (`.scss`) is indexed and reads L1 good and L3 excellent against Dart Sass; the indented `.sass` syntax is disclosed as unsupported (LB-F465, LB-F508, LB-F509, LB-Q-scss).
- The `ingest` tool names the run that serves each refresh answer, and `ingest {service, run}` reads that run's state without queuing anything (EW-66, G4).
- `fathom verify`'s `runtime_pass` reads the newest attested run of a row's certifying tests, informational only (TC-29, ADR-067).
- `fathom doctor`'s `rule_outcome_writer` row names a rule-outcome log that wrote nothing in 72 hours while the edit hook served claims (GL-24, INC-27).
- The edit hook's beat records a destination class (`clone`, `worktree`, `gitignored`, `scratch` or `outside`) and the target path for every file-target answer (XM-36).
- `[typed] skip_modules` names the nested Go modules the typed pass does not load, read from the repository's own config only (EW-68, EW-69, EW-71).
- **Every SARIF fathom writes names the commit it was produced at and fingerprints each cited excerpt (EVX-1..EVX-3, ADR-068).** `versionControlProvenance` names a repository only when `[receipts] repository_uri` configures one, and each output validates against SARIF 2.1.0 and is byte-identical across runs.
- The edit hook serves the indexed repository's rules for a tracked file in a clone or linked worktree of it, and an untracked path stays silent (XM-37).

### Changed
- Help exits 0 on every verb and lists its flags with their types and defaults; `fathom --help`, `-h` and `--version` work (K11 S1, CL-10..CL-12).
- INTENT.md fits one screen (50 lines), keeps wrapped goal titles and ends in a sha256 of the corpus it read (HP-37, HP-38).
- GitHub Actions workflows and `action.yml` files are extracted by the github-actions dialect: a run step reaches what it runs, and a remote `uses:` stores a pin node (LB-F306, LB-F309..LB-F311).
- A remote decision provider's key goes only to the provider's own host, a loopback host or a host the user config names; anything else turns the port off with one `warn:` line, and a declined question degrades like a failed call (DP-32, DP-33, DP-39).
- **C# reads L4 excellent: fields, events, enum members, delegates and indexers are symbols, `new X(..)` is a call, and a Unity MonoBehaviour's messages are entrypoints (LB-F286..LB-F290).** A property is named by its type, so it no longer hides a same-named type (O595).
- **Java reads L4 excellent at every level on the tree-sitter Java dialect (LB-F301..LB-F305, LB-F230).** An annotation type is a `JavaAnnotation`, `new X(..)` is a call, and a single-type import no longer binds a same-package class.
- **C and C++ read L4: the C++ tags dialect replaces the CGO walker, fields, enumerators and file-scope variables are symbols, and a C++ file is a module whose `#include` is a file edge (LB-F117, LB-F291, LB-F294, LB-F295, LB-F434, LB-F435, LB-F473, LB-F474).** C reads every level excellent, and C++ L1, L2 excellent with L3 and L4 good.
- `fleet reconcile --apply --rebaseline` re-records a changed skill only when its content is a render fathom shipped; any other edit under the `fathom:owned` marker stays `DRIFT` (FA-24).
- The fathom-doctor skill names `cc-plugin-agents-md@builtin` with `agents-md@builtin` as the legacy id; plugin version 0.3.9 (AG-39).
- **`ExtractorEpoch` is 63 (52 in 0.10.0) and `TypedEdgeEpoch` 5, so the first ingest after upgrading re-extracts every source file whose dialect changed, which is most of them, and re-runs the typed pass once per Go service.** The bumps carry this release's dialects, symbols and edges (waves lf1 to lf7, c1 to c3 and h3).
- Every `go` command fathom runs on its own uses `GOTOOLCHAIN=local`: a go.mod that asks for a newer go is refused and named, with syntax edges standing (K22, LB-F341).
- The Go typed pass loads the repository's own nested modules, each from its own root, one package at a time under `GOMEMLIMIT=1GiB`; a module named otherwise keeps its syntax edges and is named (EW-59, EW-60, EW-62, EW-63, EW-67).
- Rust files are extracted by the Rust tags dialect and read L4 excellent at every level; `mod x;` and a module `use` path are file edges (LB-F387..LB-F390, LB-F422, LB-F475..LB-F477).
- `.sql` files are extracted by the sql tags dialect in place of the regex: tables, views, indexes, functions and columns carry end lines, and a foreign key is a call (LB-F420, LB-F421, O794).
- Kotlin reads L3 good: a member import binds no wrong import symbol, and `Companion` members bind their class (LB-F431..LB-F437).
- fathom builds as pure Go on Linux with `CGO_ENABLED=0`, and macOS keeps cgo for the daemon's two memory readers (ADR-066, CS-9).
- CSS reads L3: `var()` reads and animation names are references, and a stylesheet's `@import` is a file edge (LB-F459..LB-F466, LB-Q-css).
- The risk rank reads coverage from a named source, never a store, and says which (`coverage_supplied`, `coverage_source`) (TQ-26).
- Test and vendor paths are read by one classifier, `internal/repofacts`, in `search`, certify, conserve, the route readers and the impact walks (RF-2).
- `fathom review --receipt` and `review_diff`'s `receipt_sarif` report a result built on a link below the typed tier as a `note` under `link_below_typed_tier`, never an `error` (EVX-3, I57).
- `fathom compile --dry-run` prices output from the service's own completed runs, names where the ratio came from, and prints `unpriced` instead of a zero (EW-74).

### Removed
- The `express-regex` route reader; the Express reader replaces it (O696).

### Fixed
- `fathom doctor` reads Claude Code's agents-md `instructionFiles` setting under `cc-plugin-agents-md@builtin` as well as `agents-md@builtin` (AG-27, O655).
- Where `AGENTS.md` exists, `fathom doctor`'s remedies name only the native path and no longer tell you to create a root `CLAUDE.md` (AG-28, O656).
- `fathom tests select` counts a change to a file a `//go:embed` directive embeds as a change to what reads it (TQ-21, O658).
- Every fathom write of wiring records its baseline in `.fathom/owned.json` from the bytes it wrote, so fathom's own skill render no longer reads `DRIFT` (FA-23, O657).
- `fathom verify` reads a plan moved inside the range at the path it had, so an archived plan no longer fails `--strict --since` as self-skipped.
- The edit hook stays silent, and sends no daemon request, for a file git ignores, unless a guard's scopes name it (EW-55, EW-56).
- `docs/languages.md`'s "Engine / dispatch" column is generated from the dispatch registry (LB-F356..LB-F358, O687).
- A one-file refresh of a Makefile, a script, a Dockerfile or a Jenkins file clears the doc-xref pass, so no edge to a removed target survives (O730).
- `blast_radius` `kind:"delete"` lists file-resolved callers as `heuristic` referents instead of "proven zero", and `find_callers` summarises `typed_callers` when `callers` is empty (SR-75, O743).
- `fathom tests select --safe` selects a test that reads a changed file's source by a literal path (TQ-24, O783).
- A qualified exact `find_symbol` query no longer ignores a wrong qualifier (SR-77, O770).
- The edit hook writes a rule-outcome row for every served answer, the Bash-write path and a guarded edit's post call included (GL-24, PR-11, INC-27).
- A Bash write's relative target resolves against the directory the command ran in (GL-25, O820).
- `fathom receipt verify`'s toolchain rung runs offline and names a module missing from the cache (EVX-6, I56).
- `fathom init` waits for the ingest run the daemon names and stops with a `MANUAL` line when it fails (EW-70).
- The commit pin check finds a commit made quietly, inside a compound command or through `git -C` (PR-13).
- The prompt hook writes exactly one answered beat per prompt, and a silent one names its reason (GL-26, O822).
- A spending `fathom compile` through the daemon waits in proportion to the sections it compiles, between 120 s and 2 h, instead of a fixed 120 s (EW-73).

## [0.10.0] -- 2026-10-09

Release impact: minor (pre-1.0: breaking bumps minor)

**What this release is.** fathom became ready for its first outside testers:
`docs/early-testers.md` says what to install, what works and what does not, and
cloud sessions (Claude Code on the web, Codex cloud) run fathom inside the VM.
The index now sees more than calls: reference, route, policy and Docker image
edges reach what they name in `blast_radius`, and a generated excellence board
in `docs/languages.md` names the level every language and tool format meets,
with CUE, Timoni, Rego, Kyverno, Gatekeeper, Terragrunt, the proxy
configurations, OpenAPI, Meson, vcpkg, Conan and more among the new dialects.
Stewardship verbs (`plans lines`, `report used`, `gc` strays and binaries) show
what a repository carries, `fathom ingest` names each pass's time and parses a
file once, and a compile no longer loses its work to one section's deadline.
Detail: `docs/changelog/0.10.0.md`.

### Breaking
- `decisions.jsonl` and the assess ledger (and `fathom assess --json`) name the yes/no decision kind `predicate` at `schema_version` 2, where version 1 rows say `noul`; `fathom decide log` and the console read both as `predicate`, and no provider request changes ([migration](docs/upgrades/0.10.0-decision-kind-predicate.md)).
- The edit hook's rule-outcome events `complied` and `violated` are renamed `top_changed` and `still_served`, nothing writes `violated` any more, old ledger rows are read under the new names, and `fathom obligations`, `/explain`, `/rules` and `fathom claim effectiveness --json` carry the renamed fields (JF-1, JF-2; ADR-056, ADR-057). Migration: [docs/upgrades/0.10.0-rule-outcome-events.md](docs/upgrades/0.10.0-rule-outcome-events.md).
- `fathom observe` writes each entry to its own file, `.fathom/observations/O<n>.md`, and never appends to `.fathom/observations.md`, which every fathom reader still reads beside the directory (EW-32). Migration: commit `.fathom/observations/`, and point any tool of your own that reads the legacy log at both.

### Added
- Cloud sessions (Claude Code on the web, `claude --cloud`; Codex cloud) run fathom in the VM: a committed `scripts/cloud-setup.sh` provisions tools at the pins in `scripts/cloud-tools.env`, builds `bin/fathom` and pre-builds the index, and a SessionStart wrapper starts a per-session daemon so the repository's hooks and MCP tools work there (CS-1..CS-7, [docs/adoption/cloud-sessions.md](docs/adoption/cloud-sessions.md)). A local session is unchanged.
- `fathom serve-mcp --url <daemon rpc url>` is a stdio front for a running `serve-http`: it opens no store, forwards each JSON-RPC line and answers every request with an id even when the daemon is down, so a stdio-only harness shares the daemon's one writer (CS-2).
- The custodian kind `agents_md_reaches_claude` takes `import_only: true`: a CLAUDE.md holds only as exactly `@AGENTS.md` beside an AGENTS.md, and once a root one exists every nested AGENTS.md needs its own (CS-5).
- `fathom ingest` names each pass's wall and CPU time: one `pass-times` line, and `pass_times` (`name`, `parent`, `wall_ns`, `cpu_ns`) in `--json`, each sub-pass after the pass it is part of; CPU is the process's (EW-48, O362).
- `fathom plans lines [--json]` classes one repository's local branches, linked worktrees and stashes as landed (with the removal command it would take, never run), at risk, idle or active, each with its age, and reads no plan file (SV-1, SV-2, EW-40; ADR-060, I63).
- `fathom report used <dir> [--json]` prints a vendored or mirrored dependency's used closure from the product's main packages (Go packages, embedded files, CSS `@source` and `@import` roots, TS/JS modules, module files), each with its entry point and first importer, and the unreached rest (SV-6, SV-7; ADR-060).
- `fathom gc` lists two report-only classes for the repository (`--repo`, default the enclosing one): `strays`, untracked non-ignored files outside `workspace/` and the build-cache roots, and `binaries`, executable binaries committed or lying in the tree; `--apply` never removes either (SV-4; ADR-060, I63).
- `fathom plans status` marks a plan with no open and no claimed-unproven row archivable and names its `fathom plans archive` command (`--json`: `archivable`, `archive_command`) (SV-5; ADR-060).
- `[decide] sites` accepts the opt-in site names `triage_failure`, `ask`, `explore`, `compact_advice` and `judge_<name>` (JF-4); no call site asks one yet, and each is refused, whatever `provider` is set to, unless the repository's own `.fathom/config.toml` names it.
- `list_services` discloses Cedar (`.cedar`, `.cedarschema`) and Sentinel (`.sentinel`) files as unsupported languages, and `docs/languages.md` carries a generated excellence board (the queue's tiers T1..T6, level met and next gap per item), held by `TestLanguagesTrackerIsGenerated`.
- Reference edges: an import-bound type or identifier reference (an annotation, `new X`, extends or implements, a bare or qualified name) is a file-resolved edge to the file defining it, following re-exports, so callers, `blast_radius` and test reach see it (F-D, LB-F74..LB-F78; `ExtractorEpoch` 42). File-edge recall rose on Python, TypeScript, Java, C#, Dart and Ruby at precision above .90, and protobuf imports resolve under `buf.yaml` and `buf.work.yaml` module roots.
- A route reaches the service it names: ingest stores a file-tier `route-ref` edge from a Kubernetes Ingress, Gateway API route, Istio VirtualService or DestinationRule, Traefik IngressRoute, the six proxy dialects and the launch arguments of a container or compose service that runs mongobetween to the Kubernetes Service or compose service its host names, so a change to a Service reaches every route in front of it in `blast_radius` (LB-F96, LB-F97, LB-F100). A target that resolves to no declared service draws no edge and is counted by reason.
- A policy reaches what it governs: Kyverno (L3 good) and OPA Gatekeeper (L3 excellent) files read through their own dialects, and ingest stores a file-tier `policy-ref` edge from each rule, policy, exception or constraint to the Kubernetes objects it matches by kind, namespace and label selector (LB-Q-kyverno, LB-Q-gatekeeper, LB-F134..LB-F137; `ExtractorEpoch` 46).
- Docker images resolve to the Dockerfile that builds them: `FROM`, `COPY --from`, a compose service's `image:` and `build:` and a Kubernetes container's `image:` are doc-xref edges to the root stage of the in-repository Dockerfile, through an image-tag index each ingest builds, and a templated tag or a tag two Dockerfiles build joins nothing (LB-X-docker-image-ref, LB-F87). Compose files read through the docker-compose dialect with `include:` and `extends.file` resolved, and a directory a Taskfile include or kustomize base names resolves to that dialect's own file (LB-F88, LB-F89, O165).
- Terraform ingest stores cross-file doc-xref edges between a module's `.tf` files (variables, locals, resources, data sources, module calls and their inputs, outputs, `module.m.o` reads), measured against OpenTofu 1.13.0 at L1 and L2 excellent and L3 good (LB-Q-terraform, LB-F84..LB-F86). `Calls` no longer name a for expression's variables, a dynamic block's iterator or the `provider`/`providers` meta-arguments.
- CUE (`.cue`) extracts at L2 (L1 and L2 excellent) against the cue v0.16.1 binary: every field is a symbol at any depth, a package clause is an entity, a field calls every field and definition it references, and imports and cross-file references are doc-xref edges (LB-Q-cue, LB-F211..LB-F215; `ExtractorEpoch` 50).
- Timoni modules and bundles extract through a native dialect over the CUE extraction, measured against the timoni 0.35.0 CLI: module, values, object template, bundle, bundle instance and runtime entities, and a bundle instance reaches its local module's object templates as cross-file edges (LB-Q-timoni, LB-Q-timoni-rest, LB-F216..LB-F218). It reads L1 good; L2 and L3 read under .90 precision, because the composed CUE references fall outside the oracle's truth.
- fathom's own `.fathom/config.toml` (L2 good) and `docs/plans/*.md` rows (L3 good) extract through their own dialects: a key that names a profile references it, and a plan row reaches the rows its `needs:` names and the code symbol its `symbol:` and `reachable_from:` name, so `blast_radius` on a function reaches the plan rows that claim it (LB-Q-fathom-config, LB-Q-fathom-plan-rows, LB-F219..LB-F222; `ExtractorEpoch` 50).
- Rego (`.rego`) extracts at L3 (L1 and L2 excellent, L3 good) against `opa parse` 1.18.2, and `import data.x.y` resolves by the new `rego-package` strategy to the files declaring the package, so a call or reference through an import is a file-resolved edge (LB-Q-rego, LB-F131, LB-F132; ADR-048).
- AWS IAM policy documents, Azure Policy definitions and initiatives, and Cloud Custodian policy files extract by content at L2, each emitting `policy_ref` references for the policy resolver (`ExtractorEpoch` 46). The doc engine gains `$index`, `..{key=value}`, `scan` and `target_map`.
- Terragrunt extracts at L3 and HashiCorp HCL tool configs (Packer, Nomad, docker-bake) at L2, both by native dialects on hclsyntax (LB-Q-terragrunt, LB-Q-hcl-tools; `ExtractorEpoch` 47). A `.hcl` file no sniff claims is counted ambiguous, and terraform definitions carry an extra `terraform.module` label.
- Azure Resource Manager templates (L3), Azure Bicep (L3), Serverless Framework (L2), Pulumi YAML (L2) and AWS CDK `cdk.json` (L2) are extracted, measured offline against each tool (`ExtractorEpoch` 47). Doc-engine references gain a regex `pattern` and a cross-file `via`.
- Argo CD and Workflows (L3 excellent), Flux CD, Crossplane and Ansible (L3 good) extract through their own dialects: a file whose every document is Argo's, Flux's or Crossplane's reads through its dialect, a mixed file stays Kubernetes, and Ansible playbooks, roles, task files and YAML inventories extract (`ExtractorEpoch` 47).
- Proxy and gateway configuration is extracted: nginx (L3 good), HAProxy and Caddyfile (L3 excellent) by native dialects, Envoy (L3 good), Kong (L3) and Traefik (L2) by doc-engine dialects, each claimed by name, glob or content sniff (LB-F101, LB-F102, LB-F104..LB-F107; `ExtractorEpoch` 43). A `.conf` or `.cfg` that no sniff claims is counted ambiguous, never extracted.
- OpenAPI and Swagger documents (YAML and JSON) extract through the doc-engine `openapi` dialect at L3 against swagger-parser: operations, schemas, parameters, responses, request bodies, headers and security schemes are symbols, an internal `$ref` is a reference and a `$ref` to another document is a file-tier edge (LB-Q-openapi, LB-F90..LB-F92; `ExtractorEpoch` 43).
- Gradle build scripts in the Groovy DSL extract at L2 (excellent) against Gradle's own evaluation; project edges (L3) are measured and unmet, Kotlin DSL scripts stay with the kotlin dialect, and Groovy sources stay disclosed as unsupported (LB-Q-gradle-rest, LB-Q-groovy-rest).
- vcpkg manifests (`vcpkg.json`, `vcpkg-configuration.json`) read through a doc dialect at L3 excellent against `vcpkg x-package-info` and `vcpkg depend-info`: a port or project is a package, each feature a symbol, each dependency a reference, and a dependency on a port the repository holds a file-resolved edge.
- Conan files read through a dialect at L1 and L2 excellent against `conan graph info`: a `conanfile.py` that names itself is a recipe whose requirements reference the recipes they name, and `conanfile.txt`, `conan.lock` and `conandata.yml` are documents. A `conanfile.py` is no longer read as plain Python, and the requirement-to-recipe edge (L3) is not drawn (O370, O371).
- Meson builds (`meson.build`, `meson.options`, `meson_options.txt`) and `compile_commands.json` are indexed at L2 excellent, against Meson 1.12.1's own parser and libclang's CompilationDatabase (LB-Q-meson, LB-Q-compile-commands, LB-F127). A CMake tree-sitter dialect is built and measured but not dispatched; `cmakeextract.go` stays.
- The C and C++ include resolver reads a compile database from `build/`, `builddir/`, `_build/`, `cmake-build-*/` or `out/build/*/`, resolves a quoted include rooted at an ancestor directory, and binds a call through a header prototype to the one `.c` file that defines the function (LB-F121, LB-F122, LB-F123). A `.h` routed by the CT-3 sniff gains `file_imports` rows and file-resolved edges.
- Go extraction: `find_symbol` and `symbol_context` answer unexported package-level consts and vars, which were symbols only when exported (LB-F60; `ExtractorEpoch` 40).
- PHP properties, class constants and enum cases are symbols (`Owner.$name`, `Owner.NAME`, `Owner.Case`), and an enum's methods are indexed (LB-F81).
- Rust struct and union fields, enum variants and associated consts and types are symbols (`Owner.name`), a trait's required and default methods are indexed, and `use` trees nested more than one group deep resolve (LB-F82, LB-F83).
- Python class attributes and fields are symbols named `Owner.name` once per class (an annotation without a value is the new kind `PythonField`), and Python reads L4 good against scip-python (LB-F241, LB-F242; `ExtractorEpoch` 52).

### Changed
- `route` is off by default: the default `[decide] sites` drops it (JF-7, it failed calibration), so a configured provider answers `POST /route` only when `sites` names `"route"`, and the deterministic precheck is unchanged. The default list is now frozen.
- The vendored agentic-kit is pinned at 0c08348 (`v0.7.1-0.20261001071505-0c083484abe6`), the decision port's hexagonal hardening that the `predicate` rename above follows.
- `fathom ingest` of a large polyglot tree costs about a third of the instructions it did, and then 16% fewer again, with the stored output unchanged: the doc-link pass reads symbols and links once per pass, a file is parsed once for its symbols and imports, SQLite's statement journal stays in memory, and content sniffs and manifest globs refuse early (EW-49, EW-50, EW-52, EW-53, LB-F208..LB-F210; O361, O362). On the 17-repository LB-M5 set that is 1,311.6 G to 485.8 G instructions, then 451.8 G to 379.4 G.
- `fathom ingest` of an HTML-heavy or mixed C and C++ tree costs less with the stored output unchanged: a header of the other language is parsed only when an include reaches it, and a query pattern may name text a source must hold (`#requires-text!`) (LB-F179..LB-F181, O451..O453).
- Go callers and `blast_radius` no longer serve a dependency's call bound by bare name, and a type's consumers are references: `symbol_context` lists them under `referenced_by` and `references`, and `blast_radius` keeps them in the radius with an `edge_kind` (LB-F199, LB-F200; ADR-048). `symbol_context`'s callers, callees and related tests read the served graph, and Go's L2 reads excellent.
- Call sites are stored qualified and attributed where the code puts them: a Python call keeps its receiver chain and a decorator's call belongs to what it decorates, Go records a method called on a field or on a call's result and no longer records a constant as a call, and the resolver binds `self.x`, `cls.x` and `Repo.create()` to the class member (F-B, LB-F65..LB-F67; `ExtractorEpoch` 41).
- TypeScript and JavaScript bare imports (`@scope/pkg/sub`, `#internal/x`) resolve inside the repository as the compiler resolves them, through tsconfig or jsconfig `paths`, `baseUrl`, `rootDirs` and `extends`, workspace packages and their `exports`, `imports`, `types` and `main`, so callers, `blast_radius` and file edges cross a monorepo's packages (LB-F223..LB-F228; `ExtractorEpoch` 51). An edit to a tsconfig, jsconfig, package.json, pnpm-workspace.yaml or lerna.json alone re-resolves on the next ingest, and TypeScript reads L3.
- TypeScript and JavaScript calls: `new X(...)` is a call to X, a decorator's call belongs to the definition it decorates and a class field's to its class, and call text is stored compacted; both read L2 excellent (LB-F171..LB-F173; `ExtractorEpoch` 48).
- JavaScript and TypeScript function-local and object-literal definitions are no longer indexed, so a call written in one belongs to the definition around it; TypeScript reads L1 excellent against scip-typescript (LB-F69; `ExtractorEpoch` 41).
- A `package.json` package records the dependency names it writes (dependencies, devDependencies, peerDependencies, optionalDependencies) as references, as a `Cargo.toml` package does, so they reach the call graph.
- C files, and `.h` files the header router reads as C, are extracted by the tree-sitter C dialect, which indexes `#define` macros (`CMacro`) and unions (`CUnion`), signs every function and no longer turns an `else if` that opens an `#ifdef` branch into a function named `if` (LB-F119; ADR-048; `ExtractorEpoch` 45). C++ files extract as before.
- C and C++ type, typedef, macro and qualified-name references bind through the include graph to the header that defines them, and Rust `use` paths resolve to the defining file through crate, workspace and Cargo target roots and `pub use` re-exports (LB-F195..LB-F198; `ExtractorEpoch` 49). C's and Rust's L3 readings are good.
- PHP files are extracted by the tree-sitter PHP dialect, which indexes a `PhpNamespace` per namespaced file, signs every definition and draws file-resolved edges for qualified calls, so PHP's L3 reads excellent (LB-F183; `ExtractorEpoch` 49). A PHP ingest costs about 1.8x the instructions it did (O459).
- C# files are extracted by the tree-sitter C# dialect, which indexes properties (`CSharpProperty`), signs every definition, names a generic callee without its type arguments, drops `nameof` and draws file-resolved edges for usings and type references (LB-F232, LB-F233, LB-F189..LB-F191; `ExtractorEpoch` 51). A C# ingest costs 1.8x to 2.2x the instructions it did (O515).
- Rust `macro_rules!` items are `RustMacro` symbols, and a `#[macro_use] mod` reaches its macros through a facade's `pub use`; Rust reads L4 good (LB-Q-rust-rest, LB-F250..LB-F252, O471, O543; `ExtractorEpoch` 52).
- Dart reads L4 excellent: a library and its `part` files share their names, so a name defined in a part is a file edge from every file that reads the library, and a comment before a `=>` body no longer drops the callable (LB-F245..LB-F248; `ExtractorEpoch` 52).
- A Ruby bare identifier that is no local in scope (`helper`, or `params` in `params.fetch`) is a call from the method around it, read through a dialect manifest's new `queries.locals`; Ruby stays at L3 good (LB-F254, LB-F255; `ExtractorEpoch` 52).
- `fathom ingest -scip`'s import report (`client.ImportSCIP`'s `Report.Skipped`) counts a reference to a parameter or type parameter under a new key, `parameter-target`, and stores no edge for it, so a rust-analyzer or scip-dart `local N` parameter no longer stores a self-loop (LB-F239).
- Eleven more tool formats read excellent at every level through their target against each tool's own code (LB-F153..LB-F170); no extractor changed, the oracles did. The excellence board in `docs/languages.md` reports 39 of 58 tool formats at target, 22 of them excellent at every level through their target.
- The shipped `fathom-onboard`, `fathom-custodian` and `fathom-plan` skills (`fathom init --skills`) now cover cloud sessions, the import-only `CLAUDE.md`, ingest pass-times and exit codes, and plan archival.

### Fixed
- A Go typed pass that loads 0 packages although the connector found Go files is a disclosed failure: `fathom ingest` prints the typed-edge warning, exits 1 and says so in `--json`, as any failed typed pass already did (EW-39). A repository with no loadable Go files stays silent.
- An ingest whose only change is a Go file keeps the stored cross-file edges of tool formats, which the typed Go pass used to drop until a tool-format file next changed (O335).
- A one-file refresh no longer drops the refreshed package's Go typed edges from the cached call graph until the next ingest: a package patch with no new typed edges keeps the ones it had (LB-F202).
- `fathom ingest -scip` stores typed edges for an index whose producer sets no enclosing_range (scip-php, scip-clang, scip-dotnet) and stops counting references to local variables as unmapped (LB-F54, LB-F55, O235). `client.ImportSCIPResult` names the fallback count (`EnclosingFallback`) and the local targets (`Skipped["local-target"]`).
- A C++ test macro's name is no type reference: `TEST(xchar_test, ostream)` no longer draws an edge to fmt::ostream (LB-F238; `ExtractorEpoch` 51).
- The pre-push `verify` step runs under dash, the `sh` of Debian, Ubuntu and a Linux cloud session VM: its body, and the copies in the adoption guides, are POSIX sh (CS-8).
- A held port names its holder on Linux without lsof: serve-http's `exit{port_held}` row and `fathom daemon stop`'s listener fallback read the listening pid from procfs's socket tables when lsof is missing or silent (CS-8).
- The cached git toplevel misses when the .git nearest the cwd carries the recorded identity but `<top>/.git` does not: a Linux file system can hand a removed .git's inode to a nested `git init`, which the cache read as the old repository for up to 10 minutes (CS-8).
- A compile no longer dies on one section's deadline: the section is counted failed in `section_errors` and the loop continues, and a caller's abort stores the claims compiled so far and records the run (EW-33, O297).
- A long `compile_layer` over /rpc answers its result and not an empty body (EW-36, O299). The daemon's logging response writer hid the connection from the ingest-class detach hook, so the 150 s write deadline was never lifted.
- `layer_coverage` serves `uncompiled_reasons` (section to failed, prefiltered, refused, budget, cancelled, corpus_grew or stale), and a failed section now appears in `uncompiled_scopes` (EW-35, O298); `compile_layer` answers the same map. One added `compile_runs` column.
- A claude-cli profile takes `timeout` (a Go duration; `FATHOM_CLAUDE_TIMEOUT` on the env lane) for one model call, default 5 minutes (EW-34).
- A refusal names the ladder's cause: at the refuse rung the daemon's refusal, the refresh gate and the abandoned-by-governor outcome say `swap volume free X < Y` (or the host-pressure trigger) when that fired, and keep "over its memory budget" for a footprint mark (EW-37).
- `fathom tests select` no longer names a package under a nested `go.mod` (`calibration/oracles/*`), which cannot run from the repository root (EW-38).

## [0.9.0] -- 2026-09-30

Release impact: minor (pre-1.0: breaking bumps minor)

**What this release is.** fathom became a responsible tenant on the machine it
runs on, and its agent surface shrank to what is used. The daemon bounds its
own memory (a budget with a shed, refuse and exit ladder), admits every call
in a cost class, keeps one refresh queue, ingests off the request path and
trips a crash-loop breaker (ADR-047). The default MCP surface is an eight-tool
floor, and 39 tools and four prompts were removed or folded into others
(ADR-053). `AGENTS.md` is the one instruction file fathom writes (ADR-059),
and "done means proven" now means proven and wired (ADR-018 amendment).
Around those: the Observatory became a console (vision 35), refactor
conservation and test certification arrived as proof planes (visions 34, 33),
exploration maps, governance that learns and doc retrieval joined the agent
surface, and nineteen new languages and manifest formats were added
(ADR-048). Detail: `docs/changelog/0.9.0.md`.

### Breaking
- **`fathom init`, `fleet reconcile --apply` and the console apply write the fathom stanza into `AGENTS.md` only, for every `--harness` (ADR-059).** A root `CLAUDE.md` gets one marked `@AGENTS.md` import region, and fathom never creates a CLAUDE file. Migration: `docs/upgrades/0.9.0-agents-md-one-instruction-file.md`.
- **`fathom gen-claude-md` is renamed `gen-agents-md`, writing `workspace/artifacts/AGENTS.md` by default (ADR-059).** The old name still runs through 0.9.x with its old default output and a deprecation line on stderr.
- **`fathom verify --strict` (the pre-push gate) refuses a claimed row whose Go function a test reaches but no product entry point does: done means proven and wired (ADR-018 amendment 2026-09-28).** Such a row is `library-only` unless it declares `library: true` with a `reason:` that predates its claim.
- **`search` refuses a bare `root` outside every indexed service's root (`-32602`) and bounds its reads: 1 MiB of ripgrep output, 512 columns a line, 200 Go-fallback hits (RT-88).** Pass `service`, or a `root` under an indexed one.
- **`references` searches the caller's own service unless `services` names others; `services: ["*"]` is the fleet, and a call with neither a `services` argument nor a resolvable caller is a `-32602` (RT-86, RT-159).** Migration: `docs/upgrades/0.9.0-references-own-service.md`.
- **A read of a drifted service answers from the index as it stands and queues the re-ingest; every daemon ingest runs on one refresh queue (RT-97..RT-100, RT-111).** The envelope's `refresh` names the queued state, and `post-commit-hook` queues through `POST /refresh-request` instead of ingesting in its request.
- **The daemon records every start and exit in a lifecycle ledger and trips a crash-loop breaker (RT-37..RT-40).** Three counted exits in 30 minutes start safe mode, five in 60 minutes stop the daemon until `fathom daemon start`.
- **`serve-http` binds before it opens its store and waits for an unavailable store instead of exiting; a held port and SIGTERM exit by rule (RT-41..RT-44).** A port held by another process exits 75, and a managed daemon's SIGTERM drains within 12 s and exits 0.
- **`serve-http` bounds its own memory and exits 70 over budget (RT-29..RT-36; ADR-047 §2).** The budget (auto RAM/8, 1 to 4 GiB; `[tenancy]` in the user config only) drives a shed, refuse and exit ladder, and an unmanaged daemon's budget is 512 MiB (`FATHOM_TENANCY_BUDGET` raises it).
- **Work the ladder stops fails by name, and only a completed explicit `ingest` tool call lifts a quarantine (RT-153, RT-155, RT-172).** A halted ingest keeps its prior watermark, and a cancelled call answers `data.outcome: "abandoned-by-governor"`.
- **Discovery skips the default cache directories (`.cache`, `.tools`, `target`, `dist`, `build`, `__pycache__`, `.venv`, `.gradle`, `.next`, `.turbo`, `coverage`) and names each one it pruned (RT-145).** Migration: `docs/upgrades/0.9.0-default-cache-dirs.md`.
- **An unmanaged `serve-http` is a bounded scratch daemon (RT-20..RT-22).** It exits within 10 s of its parent's death, after 20 minutes idle or 8 hours, defaults to `--auto-refresh=false` and refuses the managed daemon's store; a supervisor running the long-lived daemon must add `--managed`.
- **`concerns.cue` (both scopes) refuses what it used to ignore: an unknown top-level field, a `where:` on a kind that reads no corpus, a check field the kind does not read, and an optional concern.** The load error names the field; remove it.
- **`fathom fleet report --json` omits a service's `calls_total` and `calls_agent` when the call log could not be read, and `session_count` counts distinct sessions from the ledger `FATHOM_SESSION_LEDGER` names.** Migration: `docs/upgrades/0.9.0-fleet-report-honest-columns.md`.
- **`serve-http` watches its own binary for self-restart only when it is the managed daemon (`--managed`, or launchd's installed label), and adopts a new binary by exiting 75 instead of 0 (RT-11, RT-12).** A daemon under your own supervisor needs `--managed`.
- **Every git read through `internal/gitro` runs with `GIT_NO_LAZY_FETCH=1` and `GIT_TERMINAL_PROMPT=0` (IM-F2).** A partial clone reads a missing object as an error instead of fetching it, and a read that would prompt for a credential fails.
- **`serve-http` admits every tool call in its cost class and serves through a hardened `http.Server` (RT-53..RT-65, RT-152; ADR-047 §3).** Migration: `docs/upgrades/0.9.0-admission-and-call-log-v3.md`.
- **`POST /rpc/agent`, the path the user-scope MCP registration points at, serves an eight-tool floor instead of the 25-tool companion allowlist (SR-33, F-86).** Migration for this and every removal below: `docs/upgrades/0.9.0-surface-reckoning.md`.
- **The `api_diff` MCP tool is removed — the calibrated API delta stays in `conserve`'s `surface` plane (surface reckoning SR-26, F-20).**
- **The `diff_impact` MCP tool is removed — it was a thin unbounded-walk alias for `blast_radius`, and `review_diff` already runs the same unbounded walk in-process for a diff/PR verdict (surface reckoning SR-25, F-16).**
- **The `find_callees` MCP tool is removed — `symbol_context` already returns direct callees, so the two-call `find_callees`-then-`symbol_context` pattern collapses to one (surface reckoning SR-21, F-05).**
- **The `entrypoints` MCP tool is removed — `repo_map`'s response now carries the same detector as a header (surface reckoning SR-22, F-10).**
- **The `capabilities`, `server_stats` and `list_sources` MCP tools are removed — one `status` tool answers all three, byte-compatible per section (surface reckoning SR-23, F-11).**
- **The `rename_impact` and `safe_delete_check` MCP tools are removed — `blast_radius` answers both through a `kind` argument, byte-compatible per kind (surface reckoning SR-24, F-13).**
- **The four canned MCP prompts (`investigate_incident`, `review_change`, `onboard_service`, `plan_requirement`) are removed — zero lifetime calls (surface reckoning SR-20, F-78).**
- **The MCP `brief` tool is removed — zero lifetime calls, overlapping `pack_context` (surface reckoning SR-15, F-53 refined).**
- **The `fathom narrate` and `fathom brief` CLI verbs are removed — zero lifetime calls (surface reckoning SR-14, F-53).**
- **The `dep_drift` and `cross_repo_blast_radius` MCP tools are removed — zero lifetime calls (surface reckoning SR-10, F-17/F-18).**
- **The `stale_docs` MCP tool is removed — `staleness_report` now carries the same scan as an additive `doc_drift` section, byte-compatible per row (surface reckoning SR-28, F-56).**
- **The `discoveries` MCP tool is removed, and `fathom routes` is no longer its own top-level verb (surface reckoning SR-29, F-47).**
- **The `arch_conformance`, `spec_coverage` and `spec_refs` MCP tools are removed (surface reckoning SR-27, F-35).**
- **The `http_client_calls`, `fleet_drift`, `reachable`, `traverse` and `history` MCP tools are removed — zero lifetime calls each (surface reckoning SR-12, F-22).**
- **The module MCP tools (`list_modules`, `module_obligations`, `module_query`, `module_staleness`), the `POST /rpc/modules` surface and hosted-modules mode (`serve-http --modules-token-env`) are removed, and the `module_query` wire contract v0 leaves the compatibility surface (SR-17, F-68, ADR-053).**
- **Anchored memory is removed — the `memory_search` MCP tool, the `fathom memory` verbs (`add`/`list`/`review`/`show`), the edit hook's `🧠 fathom memory` block and `internal/memory` — 0 memories in any repository (surface reckoning SR-13, F-45, ADR-053).**
- **The twelve legacy analyzer MCP tools are removed — `complexity`, `god_object`, `community`, `dead_code`, `cloc`, `secret_scan`, `shell_out`, `proto_conformance`, `traceability`, `evidence_summary`, `contradiction_scan` and `batch` — zero lifetime calls each (surface reckoning SR-19, F-77).**
- **A plan row wrapped onto indented continuation lines keeps its contract (G2-5).** Migration: `docs/upgrades/0.9.0-plan-grammar-wrapped-rows-and-needs.md`.
- **Deferred is not done: a `needs:` target that is a `[-]` row no longer satisfies it, in the same plan or across plans (G2-6).** Migration: `docs/upgrades/0.9.0-plan-grammar-wrapped-rows-and-needs.md`.

### Added
- **`review_diff` gains an additive `receipt` section: the change risk receipt over data the call already resolves (CX-5).**
- **`fathom review --receipt --format sarif|check` renders the change risk receipt through the shared SARIF/check-run emitter instead of the full review (CX-6, GL-20/GL-21).**
- **`review_diff` additionally emits its `receipt` section as SARIF and a GitHub check-run payload (`receipt_sarif`, `receipt_check`; SR-49).**
- **The request path is a gate (RT-91..RT-95, RT-137; ADR-047 §4.7).** Concurrent cold `obligations` reads share one read, `read_source`, `review_diff` and `search` read through the metered corpus reader, and `task validate` runs a tenancy lint and a per-tool cost harness.
- **Admission observes and names (RT-53..RT-65).** `/metrics` gains `fathom_inflight`, `fathom_queue`, `fathom_slots` and the rejected, would-refuse and calls counters, and egress-class calls are skipped with a disclosure at the refuse rung.
- **`fathom resources [--json] [--since 24h]` shows the daemon's budget, footprint history, lifecycle, quarantine, disk and last capture from files, so what the daemon takes is visible when it is down (RT-47..RT-52).** It exits 1 over budget, crash-looping or breaker-stopped.
- **The operator can switch the daemon off, and launchd no longer relaunches it forever (RT-3..RT-6, RT-13..RT-15; ADR-047).**
- **`fathom daemon ps` and `fathom daemon reap` list and reap scratch `serve-http` daemons (RT-23, RT-24, RT-166).** `reap` is a dry run unless `--apply` and never touches the managed daemon.
- **The config custodian audits the gate chain, not only agent config: named corpora and shipped packs (CC-11, CC-12, vision 22 §7).**
- **`fathom config propose` offers a unified diff for each violated concern it can mend mechanically and names the remedy for the rest; it writes nothing (CC-9).**
- **The Observatory becomes a console (wave F0; vision 35, ADR-045; OC-1..OC-16).** Every page renders on one app-kit shell with grouped navigation, a theme control and search at new routes (`/`, `/fleet`, `/projects/{svc}`, `/impact`, `/sessions`, `/brief`, `/wiring`, `/statusz`, `/search`); the old `/observatory*` HTML routes answer 308.
- **Now and the fleet (console wave B; OC-17..OC-20).** `/` becomes the Now page (attention queue, "since you last looked", fleet pulse) and `/fleet` one sortable, filterable table of every project.
- **The console's Proof pages (wave C; OC-21..OC-26, RG-5, SD-2..SD-4).** `/plans`, `/plans/{svc}/{plan}` and its rows show proof counted from the gate's verdicts, never the checkboxes, with a read-only board, a drawer of a row's evidence and a burn-up.
- **The project workspace (console wave E; OC-35..OC-40).** `/projects/{svc}` becomes a workspace of nine tabs that are URLs: Overview, Index, Intent, Activity, Wiring and more.
- **Console panels say "Updated — refresh this panel" when their data changes, without moving (OC-14, the kit's `interaction.live-freshness`).**
- **⌘K (Ctrl+K elsewhere) opens a command palette over every console page (OC-12, the kit's `interaction.command-palette`).**
- **Search and the plan page answer "which rows of plan P remain" within three clicks (OC-55..OC-57, UX-M1).**
- **The Knowledge pages (console wave D; OC-27..OC-34).** `/layers`, `/layers/{layer}`, `/claims/{id}`, `/packs` and `/explain` show the strata, compile runs and cost, a claim's lifecycle and why an agent saw a rule, with `fathom layers strata|coverage|show` and `fathom claim show` printing the same JSON.
- **The console's Evidence and System pages (wave F; OC-41..OC-45, SH-9).** `/impact` answers Q1 to Q4 in order with each counterfactual arm drawn as a range beside its session count.
- **The console's Learn group (wave G; OC-46..OC-48).** `/learn` draws the pipeline from source to evidence for a project, `/learn/ask` maps nine question shapes to the tool that answers each, and `/learn/guides` renders the use-case guides.
- **Add opt-in, loopback-only `theme-guide` on app-kit's exact `dc77e66` mirror (re-vendored from the side-branch pin `bcfeb8e`) with synthetic Observatory states and source-preserving preview/reset/export over an app-owned baseline.**
- **`fathom intent doctrine [--kind visions|bets] [--json]` shows where each vision of the series the router names stands (DP-3, DP-4; ADR-026 amendment 2026-09-25).**
- **`fathom conserve` reports the reading set: changed functions no attested test reaches (CV-60..65, ADR-044 amended 2026-09-25).**
- **`fathom.conserve/v1` findings and `fathom.certify.gaps/v1` targets now carry a source location (RK-36 anchors).**
- **`fathom conserve --since <ref>` is the refactor-conservation report (vision 34, ADR-044).** Six self-registering planes report `conserved`, `losses` or `unproven`, never a score, and `--strict` exits 1 on any `losses`.
- **The proof-integrity plane catches a certifying test tampered with, not just deleted.**
- **`conserve` extracts a capability inventory (`api:`, `json:`, `route:`, `env:`, `rpc:`, `cli:` and configured-pattern keys) at base and head and reports the delta.**
- **`gone:` plan rows prove an import, module, lexical pattern or kept-but-unused symbol is gone, and `conserve` flags a Blindness copy: absence proofs and the anti-Blindness detector.**
- **`fathom refactor plan <target>` writes a gate-ready plan for retiring one symbol, import or package (a pinned baseline, ratcheted `gone:` rows); `fathom refactor status` reports each ratchet's live count (CV-33, CV-34).**
- **`fathom report evidence` gives, per service, typed edge share, edge tier per language, certificate density and inventory counts, ranking no service by tooling maturity (CV-39).**
- **The pre-push hook and `review`/`review_diff` carry conservation too.**
- **Test certifications through stage 3 (vision 33, ADR-043): `fathom certify status|gaps|manifest|orphans|suggest`, `fathom tests ingest`, `fathom verify certified_by` and a `certifications` review dimension over test-kit declarations and attested runs.**
- **The daemon's memory is observable and returns freed heap to the OS on a schedule (O108).** Correction (INC-23, INC-24): an earlier draft claimed it was "bounded at the peaks"; the real bound is the responsible-tenant program (ADR-047), and the instrument is `phys_footprint` (`fathom resources`), not `heap_sys`.
- **The decision port, off by default (vision 32, ADR-042): `[decide] provider = none | llm | typesafe` in `.fathom/config.toml`, and with `none` nothing is called.**
- **Claims gain bi-temporal validity (ADR-041 decision 7, SU-19).**
- **Claims gain an explicit supersedes chain (ADR-041 decision 7, SU-20).**
- **Claims gain symbol-bound anchors (ADR-041 decision 7, SU-21).**
- **Claims record who asserted them (ADR-041 decision 7, SU-22).**
- **Typed edges beyond Go by import (ADR-041 decision 5; SU-9, SU-12, SU-13).** `fathom ingest --scip PATH` imports an external indexer's SCIP output as `typed_edges`.
- **Language lookups corpora and the push gate (ADR-041 decisions 9 and 10).** `lookups-ts` and `lookups-py` corpora are committed, not run, and `task push` refuses to push unless the lefthook pre-push hook is present (INC-20, I40).
- **Findings-loop learning telemetry (vision 31 §3 D8, ADR-041 decision 8).** `findings candidates` scores rule candidates by ownership weight, `findings status` reports `rules_never_fired_30d` and `top_fired`, and `fathom claim add` mints one human-authored claim.
- **Typed edges beyond Go: CUE native, TypeScript/JavaScript/Python file-resolved (ADR-041 decision 5 §B).**
- **Portable bounded build cache (ADR-039).** `FATHOM_CACHE_ROOT` relocates the build caches without a workstation path, and `FATHOM_CACHE_MAX_MIB` bounds the regenerable Go build cache.
- **`task stranger-check` times a first proven answer, not just build/init/doctor (RH-9).**
- **`fathom plugin render --client cursor` also writes `.cursor-plugin/plugin.json`, Cursor's own manifest (RH-8).**
- **`fathom plugin render` stamps the live default surface and its opt-in groups into the rendered tree instead of leaving them undocumented (RH-5).**
- **`fathom map show ID --focus <path|package>[,...]` lists the entries your task touches first (XM-35, O177).**
- **Each hook-latency record carries the invocation's CPU time (EW-29).**
- **`/progress` gains a cost-per-proven-row panel: each session the proof join attributes rows to is priced by its recorded model through `~/.fathom/prices.toml` (`cost.LoadPriceTable`).**
- **`/sessions` serves the live session feed: the 25 newest sessions with harness, model, repository, window and the rows the proof join attributed to each, read-only and refreshed over the SSE channel's `sessions` source; no cell is a session id or a per-person figure (PE-19w).**
- **The session-ledger row carries `tool_calls_by_name` (the 20 most-called tool names plus `other`), and the console shows the tool-call mix and cache hit rate on `/sessions/{id}` for the session and on `/plans/{svc}/{plan}` summed over the sessions the proof join attributes to the plan (PE-18w).**
- **`fathom stop-hook --harness codex` folds the session's Codex rollout (`transcript_path`) on the harness port, incrementally from the session's offset stamp, into the ledger row's token, cache, `tool_calls` and `model` fields; turns and duration are named unavailable for a rollout (PE-14).**
- **The session-ledger row records `harness` (the Stop hook's `--harness` codec id) and `model` (the last model the transcript names), so `/work`'s scorecard and the progress views split by agent identity instead of one `unknown` cell; rows from older binaries stay `unknown` (WI-9w).**
- **The Stop hook's session-ledger row carries `tool_calls`: every tool call the session made, any tool name, from the same transcript fold as its token and cache fields; absent when no transcript was parsed (PE-13w).**
- **`.fathom/config.toml` gains `[[work.ci_summaries]]`: each CI summary file (JUnit XML or a workflow-run JSON export) and the commit it built, which `/work` reads into CI minutes per zone (WI-2w).**
- **`staleness_report`'s `doc_link_drift` now reports: a doc span whose linked symbol or path changed since the ingest linked it is listed with the commit that changed it (KR-7).**
- **An ingest links each document section it (re)indexes to the code it names (KR-6): a backticked symbol that resolves to exactly one indexed symbol, or a repository path, becomes a `doc_links` row with the target's hash at link time.**
- **Every decision `fathom claim learn --apply` persists is announced with its signals: an intent-ledger row the session attention line counts (`learning N`) and `fathom intent audit` lists (GL-14).**
- **`fathom claim learn [--service S] [--store DB] [--apply]` promotes and retires claims under `.fathom/learning.cue` from the rule-outcome log (GL-13).**
- **The edit hook's verdict ask names the rule's claim id, and the user's one-key answer is recorded (SR-48w, F-31).**
- **`fathom obligations` marks each `.fathom/guard.cue` layer rule governing the path checkable or not, and `fathom obligations promote <rule> [--write]` compiles a checkable one to an `enforce: true` guard (SR-45w, F-26).**
- **`fathom refactor status --migration` prints each repository's burn-down and completion verdict, and takes `--conserve-since <ref>` (CX-12w).**
- **`fathom refactor status --migration` names each remaining site's blocker (CX-11w).**
- **`fathom tests coverage --foreign <format>=<file>` joins a non-Go coverage report to the service's indexed symbols (TQ-14w).**
- **`fathom rules replay --format sarif` emits the layer-rule violations a replayed range introduced as SARIF 2.1.0 (SR-47w, F-29).**
- **`fathom conserve --strict-plane integrity` exits 1 on a silent pin loss or an uncited retirement, and this repository's pre-push `refactor-conserve` step now blocks on it (PR-9, ADR-057's PR-8 decision; PR-12, its K4 clause).**
- **`fathom conserve`'s integrity plane guards a test named in a `done needs:` segment of a `.fathom/guard.cue` note, under the new reason `guard_done_needs` (PR-2, ADR-057 decision 3).**
- **Exploration maps arrive by hook (XM-8, XM-11 to XM-13, ADR-056): the `SubagentStart` primer carries one map id inside RC-28's 1,200-byte cap, a prompt or edit that touches code inside a map's anchored entries gets one pointer line once per session per map, the session brief names at most three maps.**
- **Exploration maps, CLI only (XM-1 to XM-7, XM-9, XM-10, XM-14, ADR-056): `fathom map new|append|name|include|compose|show|list` (`thread note --map` is the same append) files anchored, content-hashed entries, composes maps by reference as a DAG (cycles refused, depth capped at 4).**
- **`fathom config audit` gains the check kind `agents_md_reaches_claude` (optional `allow_import: false`): AGENTS.md exists and no root CLAUDE file switches it off, or the first one imports it; the claude-code pack's `claude_code_stanza_present` now uses it under the same id (ADR-059).**
- **The test-run hook, opt-in behind `[hooks] test_run` (default off; TR-1..TR-11, ADR-056 decision 6).**
- **The commit pin line, `[hooks] pin_check` = `off`, `record` (the default) or `feedback` (PR-3..PR-7, ADR-057 decision 4).** A `git commit` the edit hook sees is checked from its parent for a lost pin inside a 250 ms sub-budget, leaving one `pin` beat in the hook log and a byte-identical payload; `feedback` adds at most three short lines naming the lost pins (`docs/configuration.md`).
- **`pkg/client` exports `RankedDoc`, `DriftedDocLink` and `ErrDocIndexUnavailable`, aliases of the doc-index result types and sentinel that `SearchDocs` and `DocLinkDrift` return.**
- **An ingest indexes the service's Markdown for `search kind: docs`, hash-gated per file, inside a per-service byte budget that names what it cut (KR-8, KR-9).**
- **`fathom tests select` leads with the certifying-first selection: the minimal tests that re-prove every live certificate the diff touched, `--safe` adding every test that reaches a changed symbol (TQ-15w, SR-50).**
- **`test_impact` takes `rank: "risk"`: the service's untested, dangerous symbols ranked by uncovered share x centrality x churn x severity x incidents, each factor cited (TQ-1..TQ-5).**
- **`fathom verify --converge` reports claimed-unproven tasks, unanchored tasks and changed code no task anchors, and `--write-fixups` writes fix-up tasks only inside the spec's fathom converge block (SI-5, SI-6, ADR-049).**
- **`fathom verify --plan` reads Spec Kit and OpenSpec task files in place (SI-1..SI-4, SI-9, ADR-049).**
- **Plan grammar gains `weight:` and `accept:` (SI-7, SI-8, ADR-049).**
- **Plan grammar gains a `Proof: certified` header line (SI-14, ADR-049).**
- **`fathom verify --plan` reads Kiro's `.kiro/specs/<name>/tasks.md` in place too (SI-10, ADR-049).**
- **`fathom verify --dialect gh-tasklist --plan <file>` reads an exported GitHub issue or PR task list in place (SI-13, ADR-049).**
- **`fathom verify --plan` reads Task Master's `.taskmaster/tasks/tasks.json` in place too (SI-12, ADR-049).**
- **The verify ledger writes schema 3: every reading carries a `digest` of the plan's per-row verdicts, and a `rows` array (`id`, `state`, `criterion`, `weight`) only when that digest changed since the plan's previous reading (PE-1).**
- **`fathom plans status --weighted [--json]` rolls proven weight over total weight up by fleet, project, plan and section (PE-3).**
- **New console page `/progress` (Proof; PE-2, PE-4..PE-12): goal gauges, weighted progress, a read-only derived kanban, the frontier, a burn-up with a projected finish, proof regressions with their commit, time-to-proof, claim honesty per agent identity and a proof heatmap, filterable by service, plan.**
- **New console page `/work` (Proof; WI-8, WI-11, WI-13): per repository, the model and harness scorecard (descriptive unless G2 admits a comparison), the interest rate and cost per zone with the first-attempt split, the friction map with proposed remedies, and hotspots.**
- **`.fathom/config.toml` gains `[zones]`: zone name to repo-relative path globs, the unit work-intelligence views aggregate to (WI-1).**
- **`scripts/tenancy-stress.sh`, the stress harness for the operator's RT-M3/M5/M10 runs (RT-136, RT-168).**
- **Doctor gains `hook_budget` and `disk_retention`, and the custodian's config walk is bounded (RT-134, RT-135).**
- **Hooks fan out less (RT-130..RT-133, RT-169).**
- **`fathom gc` and the daemon's own disk bounds (RT-125..RT-128).**
- **`fathom gc` gains the owner-run class `ledger-unregistered` (PE-29): verify- and intent-ledger rows whose `repo_root` is not a registered service root are counted per root, and `--apply` moves them to `$FATHOM_HOME/archive/<ledger>-unregistered-<date>.jsonl`.**
- **Diagnostics are captured at threshold crossings, pprof is guarded, and `/metrics` carries every tenancy family (RT-122..RT-124).**
- **`[tenancy]` config (schema `#Tenancy`), `/metrics` ladder gauges and `/statusz` `quarantined` and `tenancy` blocks expose the budget, footprint and ladder level (RT-122..RT-124).**
- **`fathom assess` (DO-3, IR-3; ADR-046) asks a model typed decision-port questions about one session's committed work products, or with `--interventions` about the retro rows mechanics cannot decide, and never reads its transcript.**
- **Judging spend is isolated under `IR-4@2026-09-25` (I46): the session ledger gains an additive `spend_label`, and a labelled row is never an arm, thread-bet or flywheel unit.**
- **`fathom research bundle --include-retros` (IR-5) adds an optional, counts-only `retros` group to the bundle's closed allowlist, from the retro corpus under the state directory's `artifacts/retros/`.**
- **`search` is a new agent-surface tool: grep with a receipt (enclosing symbol and kind, test/generated/vendored flags, ident/comment/string classification), excluding vendored and `workspace/` trees by default (RC-24).**
- **Kotlin extraction (`.kt`/`.kts`): classes/objects/interfaces, functions, properties, imports, same-file call edges.**
- **npm, pnpm and yarn `package.json` extraction lands (LB-Q-package-json, doc engine):** package, script, bin and export entities and workspace and dependency cross-file references.
- **HTML extraction lands (LB-Q-html):** `id` attributes, custom elements, forms, injected `<script>` and `<style>` content and `src`/`href` file edges.
- **CSS extraction lands (LB-Q-css):** selectors, `@keyframes`, custom properties and `@import` file edges.
- **`pyproject.toml` extraction lands (LB-Q-python-packaging, doc engine):** PEP 621 and Poetry package entities, scripts and entry points, `[tool.*]` inventory and dependency-group targets.
- **Terraform / OpenTofu extraction gains resource reference edges, module source recording and `.tfvars` keys (LB-Q-terraform).**
- **`summary` field (a deterministic ≤200-char answer sentence) on nine agent-surface tools: `find_callers`, `references`, `blast_radius`, `rename_impact`, `safe_delete_check`, `test_impact`, `localize`, `search`, `obligations`.**
- **`#Guard`/`#LayerRule` gain `enforce: bool | *false` (ADR-038): an enforced rule denies a violating Edit/Write/MultiEdit/Bash call outright instead of only advising after the fact; escape hatch `FATHOM_ENFORCE=off` or `.fathom/config.toml`'s `[guards] enforce = false`.**
- **Codex verified against codex-cli 0.153.1 (hooks and MCP surface confirmed live); `fathom harness hooks --harness ID` prints paste-ready hook JSON for a harness (like Codex) that only loads a user-global hook file.**
- **Seven advisory hook events (`SubagentStart`, `PostToolUseFailure`, `FileChanged`, `WorktreeCreate`, `CwdChanged`, `DirectoryAdded`, `Setup`), all fail-open; `doctor hooks_coverage` reports wiring against what the detected harness offers.**
- **`fathom intent render --playbook` writes a root `INTENT.md` inside fathom markers, held to existence by `config audit`'s `pointers_resolve`.**
- **`fathom compile --max-tokens-per-run`/`--max-usd-per-run` overrides a resolved layer's budget for one compile call (a positive number or `"all"` for unlimited).**
- **`review_diff`/`fathom review` and `fathom tests coverage` accept one Go coverage profile per test layer (`coverage_profiles: [{layer, path}]` / repeatable `--coverage-profile [layer=]<file>`), reporting per-changed-symbol coverage broken out by layer instead of one blended number.**
- **`read_source` gains `max_bytes` (default 8KB); `find_symbol` on a Go interface carries `implementations`.**
- **The findings loop's governed import lands (ADR-040, FL-1..FL-14).** `fathom findings import` (and `pkg/client.ImportFindings`) derives one content-addressed claim per finding under a provisioned `[findings] subject`, and a producer catalog caps the served tier and refuses a `fathom`-origin producer.
- **The findings loop (ADR-040): `fathom init` scaffolds a commented `[findings]` section in `.fathom/config.toml`, and a new `fathom-findings` skill documents import, inspection and refutation.**
- **Measurement corpus `testdata/scenariobench/findings-loop.json` (FL-24, `task bench-scenario-findings`): four tasks, each a real gap this repo's own tree carries, anchored to a review-kit finding served (full arm) or withheld (control arm) — committed only, not run.**
- **Findings are served at the point of work (ADR-040): the pre- and post-edit hooks print an "open findings on this file" advisory block, capped at 3 findings and 1,200 bytes, with drifted anchors flagged and superseded or refuted findings never shown.**
- **The edit hook asks a one-key verdict when a served governing rule is ignored (surface reckoning SR-48, F-31).**
- **`fathom doctor --stranger` times install to first proven answer against G16's 5-minute bar (surface reckoning SR-53, F-51).**
- **The console fleet page gains a "Check wiring drift" panel, `GET /fleet?reconcile=1` (surface reckoning SR-55, F-57).**
- **`fathom trace export --standard <id> --out <dir>` writes an evidence pack on request, and `fathom trace verify <dir>` re-verifies it byte-identically at its commit (CX-14..CX-18).**
- **Each ingest of a repository with `.fathom/rules/archconformance.cue` records the violation count per deny rule, with the commit, the previous reading's commit and the change (CX-20).**
- **`fathom intent audit` evaluates computable revisit and promote triggers when the operator runs it (CX-21).**
- **The Intent tab shows the drift radar (CX-22).**
- **`fathom intent adr-rules` proposes an archconformance deny rule for each dependency statement in the decision records, cited by ADR and `file:line` (CX-19, F24).**
- **`search` with `kind: "docs"` takes `pkg: pkg:<ecosystem>/<name>@<version>` and answers from that exact dependency version's README and doc comments, read from the local toolchain cache and never fetched (KR-11, KR-12).**
- **`fathom plans context <plan> --row <id> --store DB --preflight` prints a row's task preflight, and `fathom plans outcome` records how it went (WI-16..WI-19 U6).**
- **`FATHOM_HOME` relocates the whole `~/.fathom` (ledgers, logs, stamps, daemon records, caches), and `--ledger <path>` on `fathom verify` and `fathom intent audit` names one ledger file (PE-26..PE-28, O121).**
- **`fathom doctor` gains a `service_roots` row: a registered service whose recorded root no longer exists is named with the `ingest` call that re-points it (EW-20, O127).**
- **`ingest` gains `remap_from` (EW-19, O127).** `remap:true` alone no longer re-points a service whose recorded root is a live checkout of the same repository; the call must also name that root.
- **PowerShell (`.ps1 .psm1 .psd1`) extraction at L2 good (LB-Q-powershell).**
- **Makefile (`Makefile`, `GNUmakefile`, `*.mk`, `*.make`) extraction at L2 excellent (LB-Q-makefile).**
- **GitLab CI (`.gitlab-ci.yml`, `.gitlab/ci/**`) extraction at L3 good (LB-Q-gitlab-ci).**
- **Maven POMs (`pom.xml`) extraction at L3 good (LB-Q-maven).**
- **Dart (`.dart`) extraction at L2 excellent (LB-Q-dart).**
- **Emacs Lisp (`.el`) extraction at L1 good (LB-Q-emacs-lisp).**
- **Common Lisp (`.lisp .lsp .asd`) extraction at L1 good (LB-Q-commonlisp).**
- **Cargo manifests (`Cargo.toml`) extraction at L3 excellent (LB-Q-cargo).**
- **MSBuild projects (`.csproj`, `.fsproj`, `.vbproj`, `.props`, `.targets`) extraction at L3 excellent (LB-Q-msbuild).**
- **MATLAB extraction (`.m` by content sniff; LB-Q-matlab):** classes, properties, methods and functions, with no support level claimed.
- **Taskfile (`Taskfile.yml` and its spellings) extraction at L3 excellent (LB-Q-taskfile).**
- **Ada (`.adb`, `.ads`) extraction at L1 good (LB-Q-ada).**
- **VHDL (`.vhd`, `.vhdl`) extraction at L1 excellent (LB-Q-vhdl).**
- **Git hook managers (`lefthook.yml`, `.lefthook.yml`, `lefthook.yaml`, `.lefthook.yaml` and their `-local` overrides; `.pre-commit-config.yaml` and `.pre-commit-hooks.yaml`) extraction at L1 good (LB-Q-git-hooks-config, doc engine).** `ExtractorEpoch` 39 re-reads every such file, each of which was skipped as unclassified YAML before.
- **AWS CloudFormation and SAM templates extraction at L2 excellent (LB-Q-cloudformation, doc engine; target L3), read as a template by its content, never by its name.** `ExtractorEpoch` 39 re-reads every such file: a YAML template was skipped as unclassified, a JSON one read as bare keys.
- **New `fathom support [--kind language|tool|client|spec|rules|coverage|contract|ecosystem] [--json]` (SM-10): a table and one-line summary per kind, or the `fathom.support/v1` JSON, joining each kind's registry, declared targets and committed readings into the computed matrix; no MCP tool is added.**
- **New console page `/support` (System; SM-20..SM-25): the support matrix as a heatmap whose cells show each computed state as a shape and a label (a click opens the reading and the plan row verdict), the dialect directories with their grammar pin and embedded size against the 40 MiB cap.**
- **Every fathom receipt now carries an estimated token count beside its response bytes, and the project workspace's Recent-calls table shows it per answer (PE-20/PE-21).**
- **A `RulesSource` port feeds every client's rules files into one compile path, and `fathom obligations --format sarif` and `fathom verify --format check-run` render served obligations, findings and gate refusals as SARIF 2.1.0 or a GitHub check-run (GL-1, GL-20, GL-21).**
- **`review_diff` answers `layers`: the changed files grouped by blast radius in review order — `leaf`, `shared-callee` (another non-test file calls it), `cross-service-provider` (another service calls a route it registers) — each with its caller-file count and consuming services.**
- **Cursor and GitHub Copilot get hook codecs; `fathom harness hooks --harness cursor|copilot` prints their FLAT hook files (GL-7, GL-8).**
- **The rule-outcome signal source and rule effectiveness (GL-10, GL-11, U4).**
- **Promotion and retirement thresholds are published in `.fathom/learning.cue`'s `#Learning` (GL-12).**
- **Rule remedies as proposals, with evidence (GL-19).**
- **Validity windows reach doc links, and a compiled claim's window opens when it was compiled (GL-16).**
- **`as_of` reaches `staleness_report`, `review_diff` and `findings_near`, through one resolver shared with `obligations` (GL-17).**
- **New console page `/rules` and `fathom claim effectiveness`: rule effectiveness, per governing rule (GL-18, U4; Evidence).**
- **`search` gains `kind: "docs"`: ranked full-text search over the indexed doc corpus, no new tool (KR-1..KR-5).**
- **New `fathom coverage ingest <file> --format lcov|cobertura|jacoco|coverage-py|istanbul|llvm-cov [--commit SHA] [--json]` (TQ-9..TQ-14 T3): coverage for every language, into the one shared block model.**
- **The pre-edit guard hook adds a one-line blast-radius trailer — "N callers, M tests reach this" — when the edited file has callers (SR-60, F-12).**
- **`fathom obligations` and the console's `/explain` show each served rule's record from the edit hook — served, complied, violated over the last 30 days (SR-61, F-27/U4, the reader half).**
- **`fathom init` runs `scip-typescript` / `scip-python` when they are already installed and has the daemon import the index (SR-65, F-74).**
- **`fathom refactor status --migration "<kind> <from> -> <to>" [--store DB] [--services a,b,c]` shows the fleet-wide migration view (CX-13): remaining/done sites per service, tiered typed or lexical.**

### Changed
- **The typed pass records package initialization as a caller (TypedEdgeEpoch 3).** A non-test `init()` and package-level variable initializers are keyed `<pkg>.init`, so `find_callers`, `blast_radius` and verify reach what they register; the first ingest after upgrade re-resolves every Go package.
- **The daemon's derived caches share one budget, and whole-repository facts come from ingest (RT-66..RT-68, RT-72..RT-79, RT-82, RT-160, RT-163; ADR-047 §4–§6).**
- **Query tools answer from indexes and bounded scans (RT-71, RT-84, RT-85, RT-87, RT-88, RT-89, RT-90; ADR-047 §4.5, §4.6, §6).**
- **The console runs under the tenancy contract (RT-112..RT-119, RT-121; ADR-047 §14).** A page past the model's 30 s TTL is served the last model while one rebuild runs outside the lock, and the call log and session ledger are read as tails.
- **Interrupted ingests and deferred auto-refresh (RT-109, RT-150, RT-45, RT-46, RT-162).** An ingest records its watermark only at its final commit and writes `ingest_state = in_progress` first, so a killed ingest never looks fresh, and auto-refresh defers by named reason at shed and above.
- **Ingest is off the request path and streamed (RT-97..RT-111; ADR-047 §7).** An ingest stats every file first and reads only those that moved (`--rehash` reads everything), streams through a 64 MiB in-flight window and runs the typed Go pass only when a Go file changed.
- **Auto-refresh re-ingests survive their requester and run one at a time (RT-149, RT-151; ADR-047).**
- **The self-restart watcher stats its binary and hashes it only when it changed (RT-12).**
- **fathom as a responsible tenant — list_services, discovery and whole-tree reads bounded (vision 36, ADR-047; RT-1, RT-2, RT-27, RT-69, RT-70, RT-80, RT-144, RT-146).**
- **Tests never reach the operator's daemon (RT-16..RT-19, RT-164, ADR-047 §10.4).**
- **The console's numbers mean one thing each (OC-H2..OC-H4).** Calls for an unregistered service are named and left out of per-service figures, and the Now and Fleet pages show one metric per question (`plans.proven`, `claims.current`, `certify.live`).
- **`fathom doctor`'s `lefthook_gates` and `stanza_present` rows render the lefthook, claude-code and codex packs' findings instead of re-deriving them (CC-13, O27).**
- **The realization ladder reads its window and the verify ledger (RG-5, OC-25; `fathom fleet realization`, the project overview, the fleet grade).**
- **The plan board, its drawer and the Proof pages' controls are reworked (wave C's review).** Lanes wrap instead of hiding behind a scroll, targets are 44 px, and the drawer's verb chip gets a Copy button.
- **Console polish from the F0 browser proof: repository paths wrap after `/`, `/statusz` fits a 390 px screen, and the top bar keeps a `fathom` brand below `md`.**
- **The console moves to app-kit `9fa58e6`, carrying four more of fathom's kit returns (FATHOM-S0-KIT-01/02, FATHOM-F0-KIT-05/06), and the mirror now carries app-kit's FSL-1.1-ALv2 licence.**
- **The console moves to app-kit `d9dd789`, the kit revision carrying fathom's own vision-35 contributions (FATHOM-KIT-01..08) and wave F0's four returned defects (FATHOM-F0-KIT-01..04).**
- **The Observatory's light `positive` role adopts app-kit's contrast-safe `oklch(0.51 0.14 155)` (was 0.55), so the generated theme is byte-identical to the kit's.**
- **`fathom verify` lint names a claimed row with no contract: it verifies `no-contract`, which only `--strict` blocks.**
- **The reading set now prefers execution over static reach (CV-67..72, ADR-044 amended 2026-09-25, CV-M13).**
- **`fathom refactor plan` refuses a dirty working tree.**
- **`fathom refactor plan` refuses rather than seeding a ratchet's `max:` at 0 from a partial corpus read.**
- **Certificates bind to the commit their run exercised (O114).** `live` now requires a passing run bound to a commit after which nothing the certificate reaches has changed; an unbound run is `unproven`.
- **Self-declaration spans the whole change range (I45).** `fathom verify --since <ref>` reports a skip, disposition, `retires:`, `surface_change:` or raised `max:` added anywhere across base..head as self-declared.
- **The decision port's `route` site fails its own calibration check (CV-M11, ADR-042 amended) — no threshold fixes it.**
- **`fathom compile` with no `--layer` now compiles `obligations` plus `adr-rules` and `spec-bindings` whenever their own corpus matches a file, and a citation whose lines moved re-anchors by its own anchor text instead of going stale (CV-41, CV-40).**
- **The edit-hook payload ceiling is the measured one: 320 tokens (Q21, ADR-016 amended).**
- **`fleet reconcile`'s drift report and `--rebaseline` now cover hook entries, not just MCP/skills (PC-28).**
- **The agent-surface `tools/list` schema tax is cut 62.7% (O101, TD-2).**
- **Bug reports route to `localize` (O102).**
- **The push gate verifies the tree it pushes (INC-21).**
- **Vectors are removed, not parked (ADR-041 decision 2, SU-8).** `internal/vector`, the query engine's `WithVector` option and fathom's import of agentic-kit's embedding client are deleted; keyword and tier ranking is unchanged.
- **`localize` walks the graph (ADR-041 D4, SU-14).**
- **Substrate hygiene and freshness (ADR-041 decisions 3 and 4).** Secondary indexes serve the obligations claim query and typed-edge lookups, claims are prefiltered in the backend, and every `tools/call` result passes a dispatch-level byte budget.
- **Observatory pages render through the vendored app-kit dashboard shell (mirror `bcfeb8e`), and `task ui:css` builds the CSS from that exact mirror with `GOWORK=off`.**
- **Tree-sitter and doc-engine dialects for C, C++, PHP, Rust, SQL, Java, Bash, C#, GitHub Actions, Kubernetes, Docker Compose and Helm are built and measured (LB-Q-c, -cpp, -php, -rust, -sql, -java, -bash, -csharp, -github-actions, -kubernetes, -docker-compose, -helm), and fusion's dispatch stays on the incumbent extractors until each meets its merge gate.**
- **Tree-sitter dialects for Swift, Tcl and Vimscript are built, measured and refused: `.swift`, `.tcl` and `.vim` stay disclosed as unsupported (LB-Q-swift-rest, LB-Q-tcl-rest, LB-Q-vimscript-rest).**
- **Every reader of the verify and intent ledgers counts only rows recorded at a registered service root, and names the rows it ignored (PE-23..PE-25, O121).**
- **Fathom-on versus off is read only when G2's admissibility test passes for both arms' units (PE-17w).**
- **`/work` passes the friction remedy table its inputs, so every pattern can fire (WI-7w): the served text of each rule the rule-outcome log names, the call-site count of each function a search loop ended at, and the repository's doc-link drift.**
- **`/work`'s friction map reads every signal it counts (WI-6w): search loops and lookups from the MCP call log (sequenced by caller session, ended by an edit in the rule-outcome log), rediscovery against the discovery pointers an earlier session established or an agent filed.**
- **`/work` reads each proven change's rework (lines changed again in its files within 7 days) and the later commits that revert it, from one bounded read-only `git log` (WI-4w).**
- **A Stop that claims nothing costs one git process and no daemon request, however many plans the repository has (O140).**
- **The console's What's next (`/support`) follows G2-6: a need is met only by a claimed row.**
- **Kustomize (`kustomization.yaml`, `kustomization.yml`) extraction at L3 excellent retires `kustomizeextract.go` (LB-Q-kustomize).**
- **`go.mod` and `go.work` extraction at L3 excellent (LB-Q-go-mod, native engine).**
- **Lua extraction at L2 (L1 excellent, L2 good).**
- **Kotlin extraction at L2 (L1 excellent, L2 good).**
- **Ruby extraction at L2 (L1 excellent, L2 good).**
- **The in-tree agentic-kit mirror moves from `beb53a4` to kit main `f006f08` and carries the kit's FSL-1.1-ALv2 licence (it was MIT at `beb53a4`).**
- **The in-tree app-kit mirror moves to app-kit main `d34067a`, whose `appkit adoption check` accepts the router in `AGENTS.md` alone (app-kit ADR-0088), so fathom's adoption check passes with no `CLAUDE.md` (`docs/app-kit.md`).**
- **The obligations layer compiles `AGENTS.md` at the root, under `.claude/` and nested, beside `CLAUDE.md`; fathom's own stanza region is no longer compiled as a rule, and a `CLAUDE.md` section byte-identical to its `AGENTS.md` twin compiles once (ADR-059).**
- **A compile keeps the claims of a section moved to another path, re-cited to its new path and lines, with no model call (AG-26).**
- **`fathom doctor` `stanza_present` checks the stanza in `AGENTS.md` and WARNs, with the remedy, when Claude Code will not load it (ADR-059).**
- **`.claude/skills` is the only skills copy; `.agents/skills` is a relative link to it, written only for `--skills` (bare), `--skills=codex` and `--skills=agents` (ADR-059).**
- **In `fathom harness capabilities --json` and `harness list`, Claude Code reads `AGENTS.md` natively (≥2.1.277) and `CLAUDE.md`, `.claude/CLAUDE.md` and `CLAUDE.local.md` in the new `import` mode; `schema/harness` retires `mirror` (ADR-059).**
- **The stanza `fathom init` writes no longer tells agents to call `ingest`, and its routing/opening-moves guidance names only the eight-tool default floor (surface reckoning SR-34).**
- **`fathom gen-status-cue` and `fathom gen-plan-cue` leave `fathom help`'s rendered output (surface reckoning SR-30, F-44).**
- **`fathom research` and `fathom assess` leave `fathom help`'s rendered output (surface reckoning SR-16/SR-18, F-53/F-70).**
- **`fathom metrics` gains `instrument`, `flywheel`, `retro`, `rate` and `arm` as five more subverbs alongside its existing `spend`/`traffic`/`channels`/`interventions`/`series`/`rollup`/`rotate`/`impact`/`catches`/`sessions`/`load`.**
- **`fathom doctor` gains a `providers` row folding `fathom providers`'s own zero/near-zero-token auth probe in; `providers` and `smoke` leave `fathom help`'s rendered output (surface reckoning SR-32, F-71).**
- **TypeScript's language-breadth reading lands (LB-Q-typescript):** `level_met` is `none` (sym_precision 0.7852 against scip-typescript), an oracle-side gap.
- **`unsupported_languages` (ingest report, `list_services`, doctor) now names every queued language without a dialect, generated from the language-breadth queue (LB-F25): HTML, CSS, SCSS, PowerShell, R, Fortran and some 40 others join Swift, Dart and the rest.**
- **A JavaScript, TypeScript, Python, Ruby or Kotlin file that is minified (average line over 500 characters in its first 64 KiB) or carries a `@generated` / `Code generated … DO NOT EDIT` comment in its first 2 KiB is no longer extracted (LB-F10).**
- **Tree-sitter extraction holds at most six grammar modules process-wide (LRU, each idle module closed after two minutes and at the refuse and idle rungs), and compiled grammars are charged to the cache budget, trimmed at shed and dropped at refuse (LB-F15).**
- **The managed daemon keeps compiled grammar code in `~/.fathom/cache/wazero/<wazero version>/`, and `fathom gc` ages it out (class `wazero-cache`: another version's directory, or an entry unwritten for 30 days) (LB-F26).**
- **JavaScript, TypeScript, Python, Ruby and Kotlin symbols carry `EndLine` and, for callables, a `Signature` (LB-F3).**
- **`task validate` takes a machine-wide gate lease and runs niced (`nice -n 10`) with `go test -p 4` (RT-129).** `FATHOM_GATE_NICE=off` runs it at normal priority and `=background` under macOS's `taskpolicy -b`; the pre-push hook's scratch ingest and `verify --strict` runs take the same lease.
- **`task test` runs `cmd/fathom` as four race shards of one build beside the other packages, and `task validate` runs `internal/scenariobench` once.** `scripts/test-shards.sh` partitions the tests over the measured weights in `scripts/test-shards.weights.tsv` (`task test:weights` re-measures, `FATHOM_TEST_SHARDS` sets the count), and a test dropped or run twice fails the gate.
- **JavaScript's `tags.scm` gains two assignment-based definitions it previously dropped silently — a prototype method (`Foo.prototype.bar = function(){}`) and a single-level member assignment (`app.foo = function(){}`, `exports.foo = function(){}`).**
- **Python extraction at L1 excellent (sym_recall .9732, sym_precision .9999, fastapi 0.119.1 + flask 3.1.3) after `tags.scm` gains a class-level attribute with a type annotation and a default value (`count: int = 0`) and PEP 695 `type X = ...` aliases.**
- **Dockerfile extraction at L2 excellent retires `dockerextract.go`'s line-scanner (LB-Q-dockerfile, tags engine).**
- **The GC soft limit defaults to the budget's exit mark instead of a fixed 3 GiB, and a managed daemon runs at `GOMAXPROCS` max(2, NumCPU/2) unless the env sets it (RT-36).**
- **Agent-surface tool descriptions now carry only what a caller needs to call correctly.** Ids, ADR cross-references and migration notes moved to `docs/mcp-tools.md`, and the five tools whose result can be definitive keep a short "this result IS the answer" stop sentence (SB-4). Argument schemas are unchanged, pinned by a pre-diet golden; `tools/list` schema bytes fell from 60,918 to 47,480.
- **`find_callers`/`blast_radius` return only edges the Go typed pass resolved by default (heuristic edges behind `include_heuristic: true`); `references`' definition half gets the same treatment; every row carries `edge_confidence` and each response names `edge_filter`.**
- **`blast_radius` defaults to a depth cap of 2 and a 500-symbol listing cap (both overridable, reported alongside `total_symbols`/`truncated`/`depth_capped`); `diff_impact` stays the unbounded both-kinds walk for PR review.**
- **`references` defaults to `exact: true` where the service has typed edges (text mentions behind `include_heuristic`), renders one occurrence under 100 bytes (default cap 25), and refuses a query that is not an identifier, a dotted path, or a declared regex.**
- **`read_source`'s byte budget (`max_bytes`, default 8000) bounds the `lines` array as sent — each row's `{line, text}` envelope and JSON escaping count, as every other budgeted tool counts its rows — so a call returns fewer lines at the same budget.**
- **`find_callers` renders one caller under 100 bytes by default (`with_signatures: true` restores them) and now unions every candidate a bare name shared by two packages could mean, instead of silently falling back to the syntax answer.**
- **`find_callers`, `outline`, `symbol_context`, `staleness_report`, `repo_map` are now budgeted: a truncated answer keeps whole rows plus a one-line `elision`; `find_callers`/`references` add a `scope_required` escape hatch for a fan-in too large to page through at all.**
- **`obligations` orders served rules fresh-before-stale, then by layer role, then specificity; `#Layer` gains `role: "governs" | "claims"` (default `governs`) so external packs fill the serving cap first.**
- **A claims-layer rule is now scoped at serve time to its own row's `{package, symbol}` contract instead of binding by prose keyword; the tool inlines each rule's cited lines as `snippet` (`include_snippets`) and, called with no `file`, answers "what governs this repository" from the root scope.**
- **`references` on a query naming a method exactly one indexed interface declares now carries `implementations` first, one row per implementing type.**
- **`fathom init --hooks-scope project` (or `FATHOM_HOOKS_SCOPE=project`) forces every fathom hook to project scope regardless of Claude Code user-scope settings, so a clean-room or CI harness that loads only project settings still gets them wired.**
- **`fathom init --harness=codex` no longer writes `.codex/hooks.json` — codex-cli 0.153.1 never loads a project hook file — and prints the entries instead.**
- **`core.SymbolNode.EndLine` is now set by the Go extractor and most tree-sitter/AST-backed extractors, not only Markdown; `outline`/`query_symbols`/`symbol_context` render `end_line` for these symbols.**
- **Every Claude Code MCP entry fathom writes now sets `alwaysLoad: true`, avoiding a wasted `ToolSearch` turn per session on a headless run.**
- **The CGO Java extractor now also emits field and import declarations; `ExtractorEpoch` bumps 6→7, forcing re-extraction of every already-indexed `.java` file on next ingest.**
- **`fathom review --format github`'s "Changed symbols" caller listing now walks to depth 2 (direct callers plus callers-of-callers), labeled `[tier, depth N]`, instead of stopping at direct callers only; call-site completeness counts stay direct-callers-only.**
- **C# file-resolved edges follow the ordinary `using X;` then `Y.Z()` idiom: an unaliased `using` is a wildcard import found through the files that declare the namespace.**
- **TypeScript extraction now emits abstract classes (qualifying their members), interface and abstract method signatures, and function overload / `declare function` signatures, all reported by scip-typescript and universal-ctags and missed before.**
- **SQL extraction no longer invents a column named `ATTACH` for PostgreSQL's `ALTER TABLE … ATTACH PARTITION` (only a CREATE TABLE column list or an explicit `ADD [COLUMN]` defines a column).**
- **Language readings re-measured with round-2 instruments (owner decisions D1–D7): TypeScript L1 good and JavaScript below L1 against scip-typescript (after the D3 audit tightened oracle-miss admissions to module-scope definitions), C# L1 excellent against a Roslyn dump.**
- **`fathom harness capabilities` says what a Codex rollout is for (PE-14).**
- **agentic-kit's `evidence-producer` catalog (`foundations/evidence/evidence-producer`) is vendored into `third_party/agentic-kit` (pin `2639212`); `internal/semantic.CatalogFromKit` builds an `evidenceproducer.Catalog` from `schema/semantic/producers.cue` under the shared engineering tier ladder.**
- **The obligations compile reads every major client's rules files, and a file's own declared scope becomes the claim scope (GL-2..GL-6).**
- **`references` no longer refuses a free-text query (SR-40, F-02): it is answered by `search` instead, escaped as a literal phrase across the same scope `references` itself would have used.**
- **`repo_map`'s default budget is cut to 1,200 tokens (was 2,500), and every answer carries `label: "orientation-only"` (SR-41, F-07).**
- **`query_symbols`' default `top` is cut to 10 (was 20), and every answer opens with a one-line `summary` (SR-43, F-24).**
- **`fathom prompt-hook` routes a stack trace, a failing test name, or an error string to `localize` deterministically, no `POST /route` call (SR-44, F-25).**
- **`fathom init`'s printed pre-push manual steps now include a non-blocking `fathom conserve` step beside the verify gate (SR-63, F-42).**
- **The edit hook records served, violated and complied outcomes per governing claim, not only refuted and corroborated (SR-61, F-27).**

### Fixed
- **A failed post-commit refresh says nothing was ingested (RT-170).**
- **Hooks no longer pile up on a slow or dead daemon (RT-7..RT-10, ADR-047).**
- **`fathom verify` against the daemon checks every row of a long plan.**
- **An unreadable source is named, never counted as nothing (I38).**
- **The proof ledger's pages say what they count (wave C's review).**
- **`/certifications` and `/refactors` name the store they read, and `/refactors` never writes the repository (wave C's review).**
- **The console's own reads are no longer counted as usage, and the brief's figures are the shared ones (the F0 proof's two defects).**
- **Wave F's pages and verbs keep every truth state (the wave F fix round).**
- **Registry counts disclose incomplete coverage; unreadable call logs and session ledgers no longer appear as zero activity.**
- **CSS generation requires a successful content read/hash instead of a separate access predicate, preserves read-error causes and still refuses missing, escaped, symlinked or concurrently changed sources before publication.**
- **The observatory story page counts sessions, not ledger rows.**
- **A service root that is itself a symlink is now walked (O39).**
- **`fathom report evidence` no longer reports zeros from a store it just created.**
- **Reading set: a comment, blank line or reflowed line inside a function body no longer makes the function `modified`.**
- **CV-M4 fixed and re-measured: the Go `api:` surface delta now agrees with apidiff (0.960 precision / 1.000 recall over 50 pairs).**
- **Fixed: the Go `api:` surface delta now tracks an interface's own embedded-interface fields.**
- **CV-M5 fixed and re-measured: `conserve.Resurrected` no longer flags long-lived, independently-written twins as a Blindness copy.**
- **`review_diff` (MCP) now wires the `integrity` dimension.**
- **`fathom compile --dry-run` no longer prints a confident zero cost when it has no price for the model.**
- **A layer's very first compile no longer refuses outright just because its whole corpus exceeds `max_tokens_per_run`.**
- **`fathom verify --store` can now prove a `gone:` row.**
- **A resurrection `moved` finding's text no longer contradicts itself.**
- **`fathom ingest -scip` resolves real scip-typescript/scip-python output (O115).**
- **Fixed: `fathom conserve`'s default plan discovery — its only real invocation — no longer loses every `retires:`/`surface_change:` declaration (O117).**
- **Truncated MCP answers point at refinements that work.**
- **The daemon's logging, fleet cost and doctor correctness are fixed in one sweep (O112, DG-1..DG-21).**
- **A refreshed file reaches the next graph query (SU-4, O99).**
- **Observatory display truth.** Freshness states reserve positive language for complete evidence, partial coverage reads as an explicit lower bound, and an unreadable call log never shows as zero cost.
- **A map read never closes a map its reader did not create (XM-34, O176).**
- **`fathom doctor` and `fathom fleet reconcile` check each hook row by its own command (FA-7, O175).**
- **The test-run hook names the test of a piped panic whose `--- FAIL:` line fell outside the tail (TR-13, O175).**
- **TypeScript, JavaScript and C# imports resolve where the language resolves them (O145).**
- **C# symbols in a file whose `#if` groups break the parse keep their namespace (O148, LB-F53).**
- **`fathom map append` of an anchor the map already carries files no second entry (XM-33, O149).**
- **`fathom map append` (and `thread note --map`) resolves a symbol anchor the way `find_symbol` does, and names an index it could not reach instead of blaming the anchor (XM-31, O147).**
- **`fathom map name` makes a map durable whatever condition it was filed under (XM-32, O143).**
- **`fathom doctor`'s `stanza_loaded` no longer says Claude Code loads `AGENTS.md` where `stanza_present` warns it will not.**
- **The Claude Code harness descriptor no longer claims `.agents/skills` discovery: Claude Code reads skills from `.claude/skills` only (ADR-059).**
- **The prompt hook drops a path or package token that names nothing in the working tree, and serves nothing for a message another agent relayed (EW-21, O126).**
- **The Bash-write hook no longer reads quoted text as a write, and serves one line, not the nearest guide's rules, when it cannot name what a command wrote (EW-1, O122).**
- **The PostToolUse Bash hook no longer panics (exit 2, a Go trace shown to the agent) on a command that ends in an unmatched `(`, and an unmatched subshell keeps its last byte, so `(cat > a.go` names `a.go`, not `a.g`.**
- **The build-cache guard (`FATHOM_CACHE_MAX_MIB`) trims the Go build cache by entry age instead of running `go clean -cache`, and never removes an entry used within the last day.**
- **The `wired` check stops counting four over-approximations the 2026-09-29 re-audit corpus exposed (EW-15).**
- **The first `verify_plan` after an ingest or a daemon start answers within its deadline (EW-22, O130).**
- **The tree-sitter engine's parse and query budget is elapsed time, as documented.**
- **An ingest in which every file failed to read, over a service with no index to protect, is no longer recorded complete at HEAD with zero fingerprints (which auto-refresh then never retried): it writes nothing, records no watermark.**
- **An ingest onboarding a new service no longer fails for every file (`stat <service>/<path>: no such file or directory`) when a read names that service while it runs: a read's Discover of the not-yet-recorded root no longer replaces the ingest's root.**
- **`obligations` no longer reads the whole tree on every call: layer coverage and the raw fallback read only the layer's guides (paths from one names-only walk per ingest, each guide under a 2 MiB cap) and the edited file, the pack union compiles once per pack digest.**
- **`obligations` stats the edited file before its index-freshness check and never reads one over 2 MiB; the response names the skip as `index_unchecked` instead of reporting the file fresh or stale, and both edit hooks print `index freshness not checked for this file (…)` for it.**
- **The daemon answers its own service registry in process: Observatory pages no longer round-trip `list_services` over loopback HTTP, which timed out under load and rendered project pages as "store unreadable", and no longer write a call-log row for a page render.**
- **`fathom plugin render` writes `"license": "FSL-1.1-ALv2"` into the Agent Plugins `plugin.json`, the licence `LICENSE.md` grants (ADR-052); it wrote `MIT`, which fathom is not released under.**
- **`wiring_current` (config audit, doctor's wiring row, the session attention line) no longer reports a fathom-owned skill, owned file or hook entry as drifted when it is exactly what the running binary ships: `.fathom/owned.json` is per checkout.**
- **`fleet reconcile --repo <root>` renders the stanza under the registry's service id (else the one the stanza already names), not the directory's name: `--apply` on a checkout whose directory differs from its service would have rewritten CLAUDE.md and AGENTS.md to name a service that does not exist.**
- **`fathom daemon install` now writes `GOPATH`/`GOCACHE`/`GOMODCACHE` into the launchd plist and the typed-edge loader repairs them when unset, empty, or relative — previously a Go service could silently fall back to syntax-only call edges while `fathom doctor` reported a healthy index.**
- **The `WorktreeCreate` hook entry is withdrawn: Claude Code read its stdout as the path of the worktree it was meant to create, blocking every worktree-isolated agent spawn.**
- **`obligations` now compiles every `**/CLAUDE.md`, not only the repo root's, floored to its own subtree at compile time — a rule about one subtree can no longer widen scope to a sibling directory.**
- **`ingest` (CLI and MCP) and `list_services` report `typed_edges: {packages_seen, packages_processed, edges, failed, error?, fallback?}`; `failed: true` now flips the outcome to `degraded`/exit 1 instead of a silent clean success when packages were seen but none processed.**
- **The daemon `ingest` tool refuses a `path` that differs from a service's registered root unless `remap: true` is passed; `post-commit-hook` checks the registered root first and skips on a mismatch instead of silently re-pointing the service.**
- **Service-scoped `search` now loads the same `.fathomignore` `read_source`/ingest use and fail-closes rather than serve a file either would refuse; both also exclude `.env`-pattern files and never follow a symlink.**
- **`obligations` accepts `path` as an alias for `file`; its governing-scope ranking no longer lets a nearer-tier candidate's bare ancestor-directory-name match block a farther-tier, more relevant rule.**
- **`fathom compile`'s `max_usd_per_run <= 0` now means unlimited, matching `max_tokens_per_run`'s own convention.**
- **`internal/guardspec.LoadGuards` no longer rejects a `forbidden_paths`-only guard as "governing nothing".**
- **`fathom review` gains an explicit `--mode report|persist` (`--no-persist` is now an alias for `--mode report`), `--manifest FILE` for an explicit changed-path digest set (mutually exclusive with `--since`; a current-content mismatch refuses with `manifest mismatch: <path>` and exit 2).**
- **A stale/contradicted obligation rule no longer renders as if binding: every hook (`edit-hook`, its `--pre` guard, the prompt hook) moves it into a trailing "stale rules — not binding until recompiled" block with one `fathom compile` remedy line.**
- **`.mjs`/`.cjs`/`.mts`/`.cts` now extract: the "javascript" tree-sitter engine already spanned the JS pair, but neither the connector's language classification nor fusion's extractor dispatch table ever named any of the four.**
- **`symbol_context` on a bare name matching more than one definition, or resolving to a single definition with more than 50 callers, returns `{ambiguous|scope_required, candidates, count, hint}` and no neighbourhood.**
- **`fathom instrument --adoption`, `fathom research report` and `fathom flywheel` (which gains a `--service` flag) now resolve a named service's call log the same way — its recorded corpus path, then that root's own `.fathom/config.toml`, then the daemon's default for that root.**
- **`fathom layers install <dir> --root DIR` (the documented flag order) is no longer rejected as too many positional arguments; flags are now accepted in either position relative to the pack directory.**
- **`fathom verify`'s CLI store path merges typed Go call edges into its reachability graph, as the MCP `verify_plan` tool already did, so `edge_wired` and `test_reaches` stop falsely reporting "does not reach" for a method name two receivers share (O40).**
- **A `fathom ingest -scip` re-import clears a package's stale typed edges when the package drops to zero edges or vanishes from the index entirely (O116).**
- **Fixed (GL-15): compile reconciliation's supersede loop left a superseded claim's validity window open.**
- **A long `ingest` over `/rpc` answers with the refresh queue's state instead of an empty body (EW-18, O128).**

## [0.8.0] -- 2026-09-02

**What this release is.** fathom stopped being a Claude Code program and
started being a harness-agnostic one: the harness is now data behind one seam
(ADR-029) -- Claude Code byte-identical by golden, Codex and Gemini CLI
shipping with real codecs and an honest `unverified` stamp -- and the shipped
skills moved to the `.agents/skills` path 13 of 15 surveyed harnesses read.
Three ways the gate could report a clean result over a broken reading were
found and closed: a verdict that recorded nothing about which binary built the
store it read (O20), a third `package main` that collapsed a symbol key and
silently refuted five untouched rows (O21), and a worktree that re-rooted the
daemon's service while every ingest answered `ok` (O22). The verify gate
gained a lexical oracle (ADR-030) so a documentation or measurement row proves
itself instead of carrying a skip note, and the bench gained the two things it
needed to be believable by someone who is not its author -- a third-party
derived public corpus and reliability statistics on every ledger row
(ADR-031). Nothing here licenses a savings claim; the A/B is still one-armed.
Detail: `docs/changelog/0.8.0.md`.

### Breaking
- **`fathom init --skills=codex` writes `<repo>/.agents/skills/<name>/SKILL.md`
  (the path Codex documents) and no longer `~/.codex/prompts/`; bare
  `--skills` (= `all`) writes `.agents/skills` + `.claude/skills`, both
  repo-scoped.** The deprecated prompts twin is `--skills=codex-prompts`, the
  only user-global write (RP-1; migration:
  `docs/upgrades/0.8.0-skills-on-the-standard.md`).
- **A plan row's `package:` names ONE directory and no longer matches
  sub-packages by prefix** (PC-32), so a row anchored at a parent directory
  stops matching. Migration: name the symbol's own package directory
  (`internal/contextbench/publicloc`, not `internal/contextbench`).
- **`fathom intent audit --json` nests the read outcome in
  `coverage.unreadable[]`** -- `{path, outcome: {kind, detail, bytes}}` where
  `outcome` was a bare string (PC-34, ADR-027). Migration: read
  `outcome.kind` for the old value.

### Added
- **The harness is data behind one seam (ADR-029).** `internal/harness` holds a
  descriptor registry (Claude Code, Codex, Gemini CLI, the generic `agents`
  pull tier) plus per-harness codecs, and Claude Code's hook table, stanza and
  hook envelopes are derived from it byte-identically (HP-1, HP-2, HP-3, HP-9,
  HP-10, HP-16); `schema/harness/schema.cue` validates the exported registry
  (HP-11).
- **`fathom harness list | capabilities | matrix | ladder`** prints the
  registry, each harness's per-capability vehicle table (HP-4), the adoption
  matrix `docs/adoption/other-harnesses.md` pins by test (RP-18), and the tier
  B/C/D rows -- Windsurf, OpenCode, Amp, Zed/ACP, Junie, Warp, Continue, Aider
  -- each with its source (HP-25). Every row of an unverified descriptor says
  `unverified on this machine`, and `docs/adoption/codex.md` §6 is now that
  table, test-pinned (HP-24).
- **`--harness <id>` on `session-hook`, `edit-hook`, `stop-hook`,
  `session-end-hook`, `pre-compact-hook`** (HP-5, HP-6, HP-7, HP-8): absent ⇒
  `claude-code`, and an unknown id or a harness with no codec exits 0 with one
  stderr line. Hooks never detect their caller -- the wired command line
  spells it.
- **Codex and Gemini CLI codecs** (HP-17, HP-19, HP-22, HP-23). Each speaks its
  harness's documented envelope (Codex: `apply_patch` → the patch's first file,
  `additionalContext` cut at 8,000 bytes with a named cut; Gemini CLI:
  `systemMessage` where `additionalContext` is unshown, flat decision never
  emitted), round-tripped against fixtures that cite their source; both
  descriptors ship `unverified` with the 60-day demotion clock started
  2026-09-02.
- **`fathom init --harness=claude-code|codex|gemini-cli|agents|cursor|copilot|none`**
  writes only that descriptor's instruction files and `.fathom/`, generates the
  stanza's hook sentences from the descriptor, and prints the harness's own
  `mcp add` line (HP-12, HP-18). Codex and Gemini CLI hook entries go into
  `.codex/hooks.json` and the `hooks` key of `.gemini/settings.json` through
  the wiring engine with a receipt `fleet undo` reverses; `cursor` and
  `copilot` write stanza and skills only and report `hooks: not wired`
  (HP-20).
- **`fathom doctor` accepts every registry id**, resolves `auto` through the
  registry's presence markers, adds a `gate_posture` row from the descriptor's
  fail mode (HP-13, HP-14), and reads MCP registration or a non-Claude
  harness's hook files as `skipped: unverified`, never a guessed PASS (HP-15,
  HP-21).
- **Store provenance on gate verdicts (O20).** `/statusz` (schema 2) carries
  the daemon's own `binary_commit`, `binary_dirty` and `built_at`, every
  verify-ledger row carries `daemon_commit`, `store_commit` and
  `binary_commit`, and `fathom verify` prints one stderr `provenance:` line
  when the daemon's build is not this binary's -- never compared to the cwd's
  HEAD (PC-1, PC-2, PC-3). Doctor gains `daemon_fresh` and `binary_fresh` says
  it knows the local binary only (PC-4, PC-6); `plans status`, `intent audit`
  and doctor's `unproven_claims` mark a stale daemon's readings
  `measured-stale` -- a coverage gap, never a failure (PC-5).
- **Claim-moment verify (PC-22).** The PostToolUse `edit-hook` runs the verify
  engine for a depth-1 `docs/plans/*.md` the moment an edit adds a `[x]`
  relative to git HEAD and injects the refusal as `additionalContext`, silent
  on a clean claim, an edit that claims nothing new, or any operational
  failure. `~/.fathom/verify-ledger.jsonl` rows gain an additive `origin`
  (`cli` | `stop` | `claim-moment`), and the Stop hook now leaves its own rows.
- **Two more hook rows.** `InstructionsLoaded` → `fathom session-hook --event
  instructions-loaded` writes one silent `instructions-loaded/loaded` beat
  `{file_path, memory_type, load_reason}`, the first harness-side evidence the
  stanza reached a session (PC-23); `ConfigChange` → `fathom config audit
  --hook` runs the custodian audit on the changed repo and writes one
  `custodian/config-change` beat, never a block (PC-24). Doctor's
  `stanza_loaded` PASSes when a beat names this repo's `CLAUDE.md`/`AGENTS.md`
  and SKIPs -- never FAILs -- naming the wiring date otherwise (PC-25).
- **Wiring drift over MCP entries and owned skills.** The law-15 ledger
  baselines each harness descriptor's MCP server entry (argv and env/header
  NAMES, never values) and every `fathom:owned` skill file; `fleet reconcile`
  reports drift by kind and path and never merges (`--apply` records missing
  baselines, `--apply --rebaseline` re-records a drifted one) (PC-28, PC-29).
  Doctor gains `wiring_drift` (PASS / FAIL naming each / SKIP on an unreadable
  file), the custodian a `wiring_current` kind, and the session attention line
  a `wiring drift N` fragment (PC-30).
- **The lexical oracle (ADR-030).** A plan row may carry `{oracle: lexical,
  path: <repo path>, pattern: <regexp>}` and `fathom verify` proves it from the
  working tree -- path exists, non-empty, pattern matches -- so a measurement,
  changelog or design row verifies and leaves U2 (PC-10, PC-18, PC-19). Lint
  and the gate refuse a markdown path without a pattern and a row carrying both
  a symbol and an oracle (PC-12, PC-20).
- **`needs: <sibling>.md#<id>`** -- a row may wait on a row in another plan;
  `fathom verify` lints a missing sibling or row and `plans next` reads the
  sibling's authored state (PC-17).
- **`--format sarif`** on `fathom verify`, `fathom intent audit` and `fathom
  config audit`: SARIF 2.1.0 of the refusals, one rule per criterion / claim
  kind / concern, located at the plan line or the cited file:line (PC-21).
- **One verify-gate figure (PC-13).** `fathom research report` prints a `verify
  gate: figure:` sentence computed from the verify ledger with its window and
  denominators inline (`verifyFigure`), and the adoption gauge reads the same
  computation.
- **`task config-audit`** (`fathom config audit` against this repo) joins `task
  validate`; `TestNoOrgSpecificNames` (ADR-011, with a local
  `FATHOM_ORG_DENYLIST`) and `TestMaintainedLinesParse` lift invariants I19 and
  I36 from `stated` to tested (PC-14, PC-27).
- **`--json` on `ingest`, `init`, `thread brief|list`, `daemon status`, `layers
  list`** (RP-14, RP-15, RP-16, RP-17): ingest carries counts and
  `read_failures` with an errno class and `outcome: ok|degraded|refused` so CI
  can assert `read_failed == 0`; init carries every write, the receipt id and
  the manual steps as data. Envelopes: `docs/usage.md`.
- **`fathom plugin render` emits the Agent Plugins 1.0 `plugin.json` and
  `mcp.json` beside `.claude-plugin/plugin.json` over one `skills/` tree**
  (RP-6). Both manifests vet against the vendored `schema/plugin/schema.cue`
  under `task cue-vet` and the pair is golden-pinned (RP-7); `scripts/ci.sh`
  runs `claude plugin validate --strict` over the rendered tree when a `claude`
  binary is on PATH and prints a named skip otherwise (RP-8).
- **`schema/skills/schema.cue`** vendors the Agent Skills frontmatter spec, and
  every shipped `SKILL.md` vets against it under `task cue-vet` (RP-2).
- **`task stranger-check`**, inside `task validate`: the tracked tree, built
  with `GOWORK=off`, runs `fathom init --harness=none` and `fathom doctor`
  against a throwaway daemon and fails on any FAIL row (RP-5, RP-23).
- **Blast depth.** `find_callers` and `references` rows carry `depth` and
  `edge_confidence` (`resolved` = typed pass, `heuristic` = syntax-only;
  `core.EdgeConfidenceOf`) with a `by_depth` summary, `find_callers` takes an
  optional `depth` that adds `transitive_callers`, and
  `blast_radius`/`diff_impact` add `by_depth` (BD-1, BD-2, BD-3, BD-6).
  `review_diff` and `fathom review` compose `test_impact` -- `affected_tests`
  (cited file:line) and `test_gap`, the changed symbols no test reaches (BD-4,
  BD-5); all fields additive.
- **Served MCP contract.** Every agent-surface tool description names its own
  required arguments and a "Reach for it when" sentence, the 17 tools serve
  `annotations` and a generated `outputSchema`, `tools/call` returns
  `structuredContent`, `tools/list` is sorted by name, `capabilities` reports
  `schema_bytes`, and `initialize` echoes the client's protocol version from a
  supported table (`2025-11-25` … `2024-11-05`) with the
  `MCP-Protocol-Version` header tolerated and echoed (RP-9, RP-10, RP-11,
  RP-12). The per-session schema budget grows from ~12.4 KB to 28.7 KB (16.0
  KB of it the description sentences) and
  `TestServeHTTPNegotiatesWithKitClient` proves `/rpc/agent` against
  agentic-kit's Streamable-HTTP client (RP-13).
- **`contextbenchscore import`** derives the public Go localization corpus
  (`testdata/contextbench/tasks-public-go.json`) from pinned local
  Multi-SWE-bench (CC0-1.0) and SWE-Explore-Bench (MIT) snapshots with no LLM
  and no network, under a pre-registered selection rule, emitting per-task
  `derivation`, `requires`, `subject` and a `corpus_sha256` that reproduces
  bit-for-bit (PB-14, PB-15, PB-17). `cmd/loctruth` computes the `find_callers`
  ground truth with `go/types` -- never fathom's own index (PB-16) -- and
  `docs/bench/public-loc.md` is the runbook with the cost statement and
  criterion H2-2 registered before run 1 (PB-18, PB-20).
- **`docs/bench/ledger.jsonl`** is the per-run bench history, one line per run,
  created by the first run and appended never rewritten; it holds one
  backfilled n=1 row (the 2026-07-24 second swing, indeterminate under the
  floor) and `task bench-ledger` re-renders `docs/bench/README.md` from it for
  free (PB-2, PB-3).
- **Reliability figures on every ledger row (PB-10).** `ComputeReliability`
  (agentic-kit `reliability-stats`) records `ci_success_diff` -- a seeded,
  task-clustered paired bootstrap 95% CI on the per-task success difference --
  plus `wilcoxon_p` on per-task mean tokens, `a12`, and per-arm `pass_at_1` /
  `pass_pow_3`, rendered as a `Δsuccess 95% CI` column. Reported, never
  gating; additive, so older rows render `n/r`.
- **`contextbenchscore regress` / `task bench-regress`** compare the newest
  ledger row against the previous same-corpus row task by task and exit
  0 / 1 / 3 (no regression / a task fell / not comparable); flaky tasks are
  listed and added tasks are not regressions (PB-4, PB-6, PB-13, BC-16).
- **Criterion H2-2** -- precision-aware: a fenced answer block within
  `max_predicted`, precision and recall recorded -- is registered beside H2-1
  in the home corpus, whose verdicts stand (PB-8, PB-11). The corpus also
  commits a leak rule: a task the new no-tools `none` arm passes in 2 of 3
  trials is excluded from the verdict and named on the row (PB-9).
- **Runner flags** `--resume <run-dir>` (score only the missing cells),
  `--subject-root` / `--service` / `--fathom-url` / `--prompt-subject` (bench a
  repo other than this one), `--scratch-daemon` (ingest the subject into a
  store under `workspace/`, never the fleet store), `--env`, `--scoring
  precision`, `--criterion` and `--stream` (PB-5, PB-7, PB-12). `memory_search`
  is excluded from the fathom arm by design and the README says why.
- **`task bench-home` / `bench-lookup`** run three trials per (task, arm) cell
  instead of one and score the run against the criterion committed in the
  corpus (BC-14). Both spend real LLM tokens and are deliberately absent from
  `validate`, hooks and CI.
- **The home corpus is true at HEAD again** (`truth_at_head_revision`, PB-1):
  the moved workspace rule, the inverted ADR-010 staleness verdict, three moved
  premises and the bare `29` token are re-derived, and every ground-truth token
  now carries a `truth_at_head` anchor that `TestHomeCorpusTruthAtHead`
  re-checks in `task validate`, so corpus drift fails the gate.
- **`docs/guides/openviking-side-by-side.md`** -- how to run OpenViking
  alongside fathom without the two fighting, and the authority rule that keeps
  conversational memory from asserting facts about code (ADR-028, untested as
  written).
- **ADR-016 carries a PROPOSED amendment (Q21, PC-16)**: a measured obligation
  ceiling (~320 tokens, the post-edit p90) in place of the unmeasured 150 --
  recorded in `ADR.md` as proposed; nothing changes until the owner decides.

### Changed
- **One read manifest (PC-34, ADR-027).** `internal/readset` is deleted onto
  agentic-kit's `foundations/telemetry/read-manifest`, which the kit
  generalised from it; fathom keeps the rendering adapter
  `internal/readcoverage` and the pinned lower-bound wording is unchanged.
- **`fleet reconcile` plans the shipped skills through the wiring engine**
  (RP-4): both repo-scoped shapes, rewritten only under the `fathom:owned`
  marker and receipted; a hand-authored file by a skill's name or a hand edit
  under the marker is BLOCKED by name. Doctor's `skills_installed` reads
  `.agents/skills` for every non-Claude harness and names each missing skill
  (RP-3).
- **Adoption docs match the binary** (RP-19, RP-20, RP-21, RP-22): the
  `--skills` shapes and `fathom plugin render` in the Claude Code and Codex
  guides, a dated *Verified against* line and the resolved Codex-hooks
  contradiction in `docs/adoption/codex.md` §6, the nine shipped skills
  re-rendered, and `TestNoStaleHelpPhrasesInDocs` forbidding the two stale
  sentences under `docs/`.
- **`contextbenchscore ledger`** pairs the arms, evaluates the corpus's
  `machine_criterion` and exits 0 pass / 1 fail / 3 indeterminate (BC-15). A
  run where a task ran in only one arm, or where the thinnest cell fell below
  the registered replication floor, is indeterminate -- never a win (ADR-031).
- **The home corpus registers a replication floor of 3 trials** it did not
  previously have. The two n=1 runs of 2026-07-24 are indeterminate under it
  going forward; their recorded verdicts stand as history of an n=1 instrument
  and are not rewritten.
- **Bench results report two token ratios**: the aggregate the criterion reads,
  and the per-task median an outlier cannot move. The median is interpretation
  only and cannot become the gate.
- Ledger rows carry `per_task`, `corpus_sha256`, `model_id`, `cost_usd`,
  `dirty_tree`, `cells_lost` and `leaked_tasks` -- additive, so older rows
  parse and read as not-recorded (PB-3, PB-19).
- The U2 remedy in `fathom intent audit` now says what the counter does: a
  `skip:` reason does not lift U2, a symbol or a lexical oracle does
  (`TestU2RemedyMatchesCounter`, PC-7, PC-8, PC-9).

### Fixed
- **A third `package main` silently refuted five untouched plan rows (O21,
  PC-32).** `core.SymbolNode.Key()` collapsed every `func run` in every
  `package main` into one node, so `callgraph.Build` kept whichever was
  ingested last and the verdict depended on ingest order; a symbol under a
  directory is now keyed `service::<dir>/<package>.Name` and `TypedEdgeEpoch`
  2 rebuilds stored typed edges on the next ingest (the old
  `service::package.Name` form still resolves as an input wherever it names
  exactly one symbol).
- **A worktree's post-commit hook re-rooted the daemon's service while every
  ingest answered `ok` (O22, PC-33, I30).** `ingest` -- the MCP tool and
  `fathom ingest` -- now refuses to re-root an existing service at a git
  worktree of its recorded root or any `.claude/worktrees/` path, naming both
  paths, and its `ok` envelope carries `root`; the post-commit hook run inside
  such a worktree refreshes nothing and says which service and root it left
  alone.
- **`-store` / `-call-log` defaults resolve under the enclosing repository's
  `workspace/` from any subdirectory, or `~/.fathom/` outside a repository --
  never the bare cwd** (invariant I31, O11/O12, PC-15). Explicit flags,
  `[daemon].store` / `[daemon].call_log` and `FATHOM_CALL_LOG` still win.
- Doctor's `binding_modules_mounted` / `binding_module_readiness` rows name the
  real reason `list_modules` is absent -- no `--module` store was mounted --
  instead of "tool not found" (PC-11).
- `fathom init`'s manual steps no longer claim Codex has no PreToolUse hook;
  the sentence predated the Codex codec, and the replacement says what is wired
  and that it stays unverified until a Codex session writes beats.

## [0.7.0] -- 2026-09-01

**What this release is.** fathom became something a second person can adopt.
A fresh clone did not build at all — `go.work` was committed pointing at two
private sibling checkouts, and one of those siblings had no in-tree mirror —
and every "the daemon is down" message named one developer's launchd label.
Both are fixed, both are now gated, and the evidence for what fathom does and
does not prove is written down for a reviewer to argue with.
Detail: `docs/changelog/0.7.0.md`.

### 2026-09-01

### Breaking
- **`go.work` is no longer committed** and is gitignored. It listed
  `../agentic-kit` and `../app-kit`, so any clone without those siblings failed
  before compiling a line — and it masked missing `go.sum` entries underneath.
  Contributors who want a workspace now create their own; nothing else changes.

### Added
- **`fathom daemon install | status | uninstall`** (macOS/launchd). `install`
  writes the agent with your own resolved binary, store, call log and a PATH
  carrying your Go toolchain, then bootstraps it — replacing a documented
  copy-paste plist with four paths to hand-edit. It refuses to stack a second
  agent on the same port, and says so out loud when no Go toolchain is on the
  PATH it wrote (without one, auto-refresh silently degrades Go services to
  syntax-only edges).
- **`third_party/app-kit`** — the second private module is now mirrored in-tree
  on ADR-019's terms, so a clone with no private access builds. Each mirror
  records the commit it was cut from in `.vendor-commit`; `task sync-kit`
  refreshes both; `vendor-check` asserts pin and stamp agree.
- **`task clean-clone-check`**, in `task validate`: copies the tracked tree,
  drops the workspace, and builds with `GOWORK=off`. The adopter build broke
  silently once and stayed broken because every machine that ran the gate had
  the siblings.
- **`docs/adoption/README.md`** — a start-here that leads with what is proven
  (the verify gate has caught 540 claimed-but-unproven rows across 2,494 runs
  over 167 plans) and what is not (no savings claim is admissible; the A/B is
  one-armed).

### Changed
- Every daemon-restart hint — `fathom doctor`'s remedy, the SessionStart line
  when the daemon is down, `fathom init`'s closing note, `task deploy` —
  resolves **this machine's** launchd label by reading `~/Library/LaunchAgents`
  instead of naming a fixed one. A machine with no agent is told to install one;
  off macOS, to run `fathom serve-http` under its own supervisor.
- agentic-kit pin advanced 25 commits (2026-07-23 → 9a44f9b).
- The `fathom-doctor` and `fathom-onboard` skills cover the daemon verb; the
  doctor skill's stale "the remedy spells a raw copy" note is corrected.

### Fixed
- **An unreadable input no longer makes a repository look healthier than it is** (ADR-027). `fathom intent audit` reported 0 invariants instead of 37, and half the unproven claims, when one corpus file could not be read — naming nothing. Counts over incomplete coverage now render as named lower bounds (`≥ 85 (1 source(s) unreadable: …: permission denied)`) and blindness leads the NOT-COMPUTED list.
- `fathom doctor`'s `unproven_claims` row and `fathom intent audit` reported different totals for the same tree (12 against 88). The row built a partial audit and printed the result as a total; U12 and U13 need no index and are now included.
- `binary_fresh` reported "go.mod does not declare `github.com/geoffmilleraz/fathom`" against the repo whose go.mod declares exactly that, when the file was merely unreadable. Unreadable is now its own message and a WARN.
- `guard_dead_scopes` called a guard dead when its files exist but are not INDEXED, sending the reader to a remedy that deletes working configuration. It now looks at the disk first.

### Added
- Claim kind **U14**: a sensitive-surface row whose globs match no file has silently stopped guarding. It found one on this repo — `internal/instrument*` guarding "the numbers the program steers by", pointing at a path that does not exist.

## [0.6.0] -- 2026-08-27

**What this release is.** The repository learned to audit what it declares about
itself. `fathom intent audit` reads a Project Intent corpus -- invariants and their
enforcement rungs, conditions, sensitive surfaces, decision records -- checks it
against the index and the gates, and lists every claim nothing proves, with a remedy
per line and no score. Two doctor rows carry it into a session through ONE shared
attention line rather than a second always-loaded one. Two checks were narrowed
because running them against this repo showed they fired on healthy rows, and the
narrowings are the findings. The obligation ceiling was finally MEASURED (missed on
47% of payloads) and its enforcement deferred to an owner decision rather than
shipped blind. Detail: `docs/changelog/0.6.0.md`.

### 2026-08-27

### Added
- `fathom intent audit` reads a repo's Project Intent corpus and reports what it declares, what an instrument can show, and what nothing shows — unproven claims by kind, each with a remedy, and every source it could not compute named. `fathom intent export --guard` proposes `.fathom/guard.cue` entries from the router's sensitive surfaces; fathom never writes that file (ADR-026, `docs/use-cases/intent-corpus.md`).
- `fathom doctor` gains `intent` and `unproven_claims` rows. Neither can FAIL: not adopting the practice is a choice, and honest debt is not a broken repo. Both are silenced by the `intent-corpus` capability, which never silences the verb.
- The SessionStart primer's custodian line became one shared attention line — `⚠️ attention: config N · unproven M (+k) · fired F — fathom doctor` — rather than a second always-loaded line. It reads recorded ledger rows only and goes silent when the last audit is over 14 days old.
- `fathom post-commit-hook` refreshes this repo's index through the daemon instead of a shell ingest against the shared store (INC-3).
- Custodian check kind `adr_references_resolve`: every path a decision record cites must exist, scoped to this repo's own record-id shape so a sibling repo's records are named rather than reported missing.

### Changed
- **`fathom fleet report` `schema_version` 1 → 2**: an `unproven` column read from the intent ledger. A repo with no recorded audit renders `n/a: no audit recorded`, never a `0`.
- The four MCP group surfaces (`coding`, `investigation`, `shared`, `platform`) are retired; `agent`, `companion`, `modules` and bare `/rpc` remain (ADR-014 amendment 2, on law 4).
- The whole SessionStart primer is capped at 3,200 bytes with any cut named; the thread brief keeps its own 2,400-byte budget.
- `fathom help` renders from the dispatch table, so a dispatched verb can no longer be missing from it.

### Fixed
- `fathom plans archive` reads the git history of commits that touched non-plan paths, so a proven plan whose row ids were absorbed by code commits can finally be archived; range and slash-list ids (`SH-1..6`, `IT-14/15`) expand.

## [0.5.0] -- 2026-08-25

**What this release is.** The frontier closed and the instrument told the truth
about itself. Reachability became real (function values are edges, closures
count, discovery gaps are named), ingest became honest (unreadable files are
counted and a wedged reader can no longer replace a good index with an empty
one), law 15 reached the production write path (init and reconcile write only
through the wiring engine, with receipts and undo, and the observatory can
preview and apply behind a local-only gate), threads scope to the repo you are
in, the custodian polices the stanza against the daemon and handoff notes
against their evidence, memories reach the edit loop and the agent surface,
budgets are project-settable, and the observatory renders absence as absence.
The repository also adopted Project Intent (`docs/intent/`), archived its first
proven plans, and re-swept every guide and shipped skill against the binary.
Detail: `docs/changelog/0.5.0.md`.

### 2026-08-25

**Frontier closed: ingest honesty, project budgets, law 15 on the production path, repo-scoped threads, custodian concerns, memory surfacing, observatory series (IT-14/15, BA-9, SH-7/8, TH-6, CC-14/15, AM-6/8, RG-6..8, INC-12)**
- IT-14/IT-15 (O16 fixed): `fathom ingest` counts and names unreadable files in a `read-failed:` column with their errno (plus the external-volume TCC hint), still indexes what it could read, and exits 1; a run that could read nothing refuses to replace a populated index, and the daemon's `ingest` tool returns the same cause as a JSON-RPC error.
- BA-9: a `budgets` block in `.fathom/capabilities.cue` or `~/.fathom/capabilities.cue` sets per-class answer budgets (`list`, `composite`, `map`; tokens; project → user → built-in); `capabilities set` preserves it.
- INC-12, SH-7, SH-8: `fathom init` and `fleet reconcile --apply` write only through the wiring engine — v3 digest markers, receipts, `fleet undo`; a hand-edited tracked region is refused and named while a v1 region is adopted. The observatory gains preview → typed diff → apply with a plan digest (409 on drift) behind `requireLocalMutation` (loopback, same-origin, registered services only).
- TH-6: `thread note|point` default to the current repo's thread; `thread list|brief` show this repo's items and count the rest (`--all-repos`, `--all`).
- CC-14, CC-15: custodian kinds `stanza_tools_served` (every tool the stanza names is served by the daemon; daemon down reads unable-to-check) and `handoff_cites_instruments` (a handoff note claiming shipped work must cite a commit, plan row, verify run, or retro).
- AM-6, AM-8: the edit-hook appends a labeled, budget-respecting memory block for memories anchored to the edited file; `memory_search` joins the agent surface (17 tools), every result carries tier and state, retired memories never surface.
- RG-6..8: the service page renders 30-day per-dimension series and the fleet page a grade distribution from real call-log rows; an absent day renders `stat.Absent` with its reason, never a zero or a line across the gap, and unreadable sources render Absent.
- `task store-purego` joins `task validate` (the pure-Go store is proven on every push); `fathom plans archive` also scans `docs/changelog/*.md` for absorbed row ids and reads the changelog's range and slash-list reference points (`SH-1..6`, `IT-14/15`) as every id they denote; sibling-project names left the fixtures (ADR-011).
- Operator rows recorded with evidence: GKE-W1-14 (live-store format markers), SW-7 (registration at `/rpc/agent`), H1-17 (injection coverage 99% over 168 h).
- Project Intent adopted: `docs/intent/` (router, living docs, decisions index over `ADR.md`) and the stanza in `CLAUDE.md`/`AGENTS.md`.

**Reachability truth: function values are edges, the gate goes quiet (VS-5..10, O18, O19)**
- VS-6: a function referenced as a value is an edge in both passes (`core.EdgeFuncValue`): `mux.HandleFunc(p, h)`, route-table literals, `http.HandlerFunc(h)`, middleware, method values. An httptest that builds a server now statically reaches every handler it registers; webui rows no longer need direct-call tests (consumer finding G15). Building it caught a phantom-edge bug: a selector's trailing ident bypassed the interface check.
- VS-7: the "call it at the test's top level" folk rule is retired with evidence -- both passes already descend into `t.Run`/`defer`/goroutine closures; pinned by `TestClosureCallsReachFromEnclosingTest`.
- VS-5: the syntax resolver already refuses ambiguous same-named methods (never first-wins), proven stable across node orderings; the 08-23 C++ flip's cause was VS-9's gap, not the resolver.
- VS-9: the typed pass counts edges dropped because their target file was outside the discovered set and, when that file exists on disk, names it as a discovery gap on stderr -- a partially wedged discovery can no longer void reachability rows as a clean "0 failed".
- VS-10: `internal/fusion/testdata/reachfix`, the end-to-end reachability harness (mux, route table, HandlerFunc, middleware, method value, closure-only caller, same-named methods) every future reachability claim runs against.
- VS-8: `fathom verify --quiet` prints failing rows and the summary only; the pre-push gate, the init guidance for consumer repos, and the fathom-verify skill carry it (consumer finding G14).

## [0.4.0] -- 2026-08-24

**What this release is.** Ambient fathom, measured. The capability
registry, self-healing fleet, anchored memory, and the flywheel check
(phases T-Y) made governance a delivered configuration; the impact
observatory, exploration ledger, and continuity threads made the program
answer for itself in instruments; the config custodian made "instructed
means true" mechanical. The 08-21 evidence audit then turned the
instruments on the instruments (the verify gate proven, the A/B one-armed,
the gauges caught lying under the TCC wedge and the OFF arm) and the
closing waves shipped instrument truth, the work frontier (ctx packs,
eligibility, leases, `fathom prime`), the verify spine (CUE joins the call
graph), the vision-05 registry prune (34 tools deleted on evidence), and
budgeted answers. Detail: `docs/changelog/0.4.0.md`.

### 2026-08-23

**Verify spine, surface prune, budgeted answers (VS-1..4, SP-B1..3, BA-1..8)**
- verify-spine (VS-1..4): O4 annotation tokenizer is quote-aware with a loud lint on malformed blocks; CUE joins the call graph via `cueReferenceEdges` (unification/field-type/list-element/import edges), fixing O3 and ending the skip_test-only exit for CUE rows.
- surface-prune (SP-B1..3): vision-05 kill clock executed, 34 of 59 zero-call registry tools deleted, 25 exempt with per-tool evidence; `docs/mcp-tools.md` synced with a registry-doc bidirectional test; `instrument --adoption` gains IT-1 read-error classification, fixing O17 (no more "88 of 88 zero" from a wedged reader).
- ambient-fathom (BA-1..8): budgeted answers ship, `elisionFor` with a registry-iteration disclosure test (caught `repo_map` disclosing nothing on truncation), `applyBudget` with strict-prefix and never-refuses rules, per-tool defaults, per-call size accounting, and the observatory Cost panel.
- CUE extractor migrated off deprecated `f.Imports` ahead of its mid-2026 removal; a fourth reachability silent-void confirmed (same-named methods do not wire, the Extract collision), worked around with direct wire tests.

### 2026-08-21

**Evidence audit, work frontier, instrument truth (IT-1..13, WF-1..11)**
- 12-agent audit of every feedback channel produced the first full evidence verdict: verify gate PROVEN (180 of 436 gate runs/5d caught claimed>verified), thread bet COLLECTING at 4/20, A/B NOT ADMISSIBLE (ON arm: 0 admissible units); instruments themselves lied under the TCC wedge and the OFF arm.
- instrument-truth (IT-1..13), 24/24 rows gate-verified: flywheel read-errors OPEN never IDLE with wedge hint; `ArmStateForWindow` arm-aware; counting beats survive `FATHOM_CHANNELS=off`; doctor surface_traffic falls back to persistent call log; session-hook fetch failure never "not indexed" (O2); observation ids mint under flock; adoption verify figure derives from verify ledger, retiring the 215/4940 double-count (O6).
- work-frontier (WF-1..11): `ctx:` pointers + `plans context` materializer, `needs:` + derived eligible/blocked_on + `--eligible-only`, TTL-only advisory leases (.fathom/leases.jsonl), `fathom prime` for Codex, dispatch boundary registered in vision 24.
- Found while shipping: a wedged ingest writes a zero-symbol/zero-edge index while reporting "0 failed", a fourth false-signal shape; measurement arm flips ON now that IT-1..5 are live, OFF floor exceeded 5x.

### 2026-08-14

**Completion pass: recommendations shipped (O29, SD-1, CC-8..10, DO-1)**
- O29 interim fix: `plans status` fans per-plan verifies out 8-wide (was serial at ~4s x ~40 plans = 2m35s measured).
- SD-1: the verify ledger, every gate run appends per-plan rows to `~/.fathom/verify-ledger.jsonl`, closing O14/O30 (flywheel verify-gate loop read only session-scoped runs; a repo gating on every push read "never ran").
- CC-8: the custodian flywheel loop (IDLE/OPEN/CLOSED), fed by a new custodian audit beat; CC-10: `fathom init` scaffolds a commented concerns.cue and fleet reconcile reports zero-coverage repos informationally.
- DO-1/2/4: A/B criterion's outcome half now derives from disk (median commits per unit beat-window); `fathom rate` demoted to optional human override.
- DP-1: the doctrine catalog design doc. Spillover named: SP-10, SH-7..9, AM-6, AM-8, IR-3..5, DO-3, CC-9/11/12/13, SD-2..4, DP-2..4.

### 2026-08-13

**Adoption surfaces catch up to shipped fathom**
- Refreshed continuity threads, lifecycle hooks, capability registry, scope/purge, config custodian, and plans status/next/archive across all adoption surfaces, every claim checked against the binary.
- Shipped skills updated: onboard teaches the full hook stack including SessionEnd/PreCompact and the nine-skill set; doctor's row table gains capabilities + concerns; a new ninth skill `fathom-custodian` added.
- Docs synced: Claude Code guide (7.1 brief+attention injection, new 7.7 continuity threads, 8 concerns.cue + capabilities.cue, 11 nine skills), Codex guide, usage.md, mcp-tools.md.
- Install demo caught a real defect before commit: `skillsdist.Names` is the canonical list and the embed directive alone does not ship a skill, nine skills now proven installed by demonstration.

**Config custodian ships its audit (vision 22, CC-0..7)**
- "Instructed means true": agent configuration joins the evidence plane via `internal/custodian`, loading falsifiable wants from CUE at two law-13 scopes (`~/.fathom/concerns.cue` user defaults, `.fathom/concerns.cue` governing).
- Four mechanical check kinds run from file reads only, no daemon, no LLM: forbidden_pattern, command_documented_and_exists, max_lines, pointers_resolve.
- Surfaces: `fathom config audit` (text/JSON, exit 1 iff violated, zero concerns exits 0 with absence named), a doctor row, and a session-brief attention line that exists only while violations exist.
- Falsification runs confirmed red for project-governs and file:line-evidence mutations; vision 23 and vision 22 section 7 land as specs with plan rows CC-8..13, DP-1..4, SD-1..4 open.

**Brief learns from its first reader (O28)**
- First real-world contact: a fresh session reported the brief was just "session ended with N dirty files", no substantive handoff.
- Deposits supersede per repo (stable `se-<repo>` id): the fact closes itself when the tree comes clean (`repo_clean`).
- Brief now leads with the spec plane (local plans summary, checked vs open) before this repo's items, then a collapsed foreign-telemetry line.
- A thin brief teaches: when no handoff notes exist, it says exactly what to run before `/clear`. Supersede confirmed RED under mutation.

**Reconcile detects stanza-content drift, fleet refreshed**
- Stanza template gained the continuity line, but `fathom fleet reconcile` reported "wiring current" for all 17 repos, it checked stanza PRESENCE, never content against the current template.
- `staleStanzaFiles` now compares each marker-owned region against the template rendered with the registry's service id, dirname only as a daemon-unreachable fallback.
- First apply using dirname mis-rendered exactly one repo, sextant (registry id `fathom`); the corrected binary flagged and repaired its own mistake.
- Applied fleet-wide: 17 repos, 0 drifted, every stanza now carries the continuity instructions.

**Phase V lands: scope hides, purge proves (SP-3)**
- All six rework defects re-reproduced against the new code and confirmed dead: fleet-wipe purge, inert suppression, VERIFIED-without-probe, constructor-blind audit, fleet-wide exclusions, home-reading gate.
- `read_source` served purged content from disk after a purge printed VERIFIED; fixed so `read_source` and `symbol_context`'s snippet path load service-scoped exclusions and consult suppressions per call, refusing with the suppression named.
- The disclosure channel (SP-3) was inert in deployment: `ApplySuppressions` produced disclosures no shipped surface collected; fixed by wrapping MCP dispatch's ctx with the collector, rendering a `suppressed` block into the envelope.

**TH-5: continuity thread promote and handoff**
- `fathom thread promote <thread>` scaffolds a gate-ready plan from a thread's open items with stable uppercase ids and origin session pointers; no row born checked, no contracts claimed.
- Promoted items gain `plan_row_checked` conditions on their own rows so checking a row closes its source item; promote refuses to overwrite an existing plan.
- `fathom thread handoff` renders open work as a harness-neutral markdown artifact with full session ids, grouped by thread, unbudgeted.
- Stanza template gains the Codex path (SP-1, SP-9, SP-11, SP-12): harnesses without SessionStart are instructed to run `fathom thread brief` at start; note/point/close accept the natural `"text" --flags` order.

**Phase V rebuilt: scope/purge defects made structural (V1-V6, AM-12, SP-10)**
- `fathom scope list|suppress|unsuppress|preview` (reversible, disclosed suppression) and `fathom purge ... | history` (irreversible, verified removal) rebuilt so each of six verified defects (V1-V6) is impossible by construction.
- V1: `PurgeReport` carries a tri-state `PurgeVerdict` (verified/bytes-unchecked/failed) computed in one place (`purgeVerdict`), mapped to distinct exit codes.
- V2/V4: ingest-time exclusions are service-scoped by construction via `~/.fathom/ignore.d/<service>`; V3: chokepoint audit derives constructor list from the store package's own source, catching `MountReadOnly`.
- V5: suppression applies at the query layer, re-read per call via `scope.Source` stat-based reload; V6: no init()-armed globals, resolved only through `$FATHOM_SUPPRESSIONS`.
- Everything deleted and proven per location, closing AM-12 (`.fathom/memories.jsonl` purge for quoting memories); SP-10 (observatory Scope panel) stays open by instruction.

**Continuity threads live (vision 21, TH-1..4)**
- The thread, one strand of intent spanning sessions, is now first-class: `fathom thread brief|note|point|list|close`, an append-only store, items that close themselves when their awaited artifact materializes (`plan_row_checked` / `file_exists`).
- SessionEnd hook deposits one mechanical item only when work is actually in flight; PreCompact files a pointer before context dies; neither reads transcript content.
- The brief is capped at 2,400 bytes (~600 tokens), injected only on startup/clear; `resume` and `compact` print nothing, both properties confirmed RED under mutation.
- Measurement loop: every ledger row carries cohort labels, `threadBetCheck` evaluates the registered bet, flywheel gains a continuity loop. TH-5 (promote/handoff, Codex stanza) is next.

### 2026-08-12

**Arm-covered windows count as measurement unit (MI-5)**
- Owner pushed back on a 40-fresh-session baseline: his practice is one session resumed across days, so session-level units produced a control arm that could never admit anything.
- Amendment registered before any data existed under it: the unit becomes an arm-homogeneous unit, a fully-covered session or the arm-covered window of a longer one, restricted to lines timestamped after the arm's `set_at`.
- Compared quantity becomes median real tokens per assistant turn, with a >=10-turn floor per unit and >=20 units per arm; window read is tail-bounded, a truncated window is excluded and named.
- Both leak paths (since-filter removed, whole-session figures admitted) confirmed RED under mutation; flywheel's arm loop now admits windowed units.

**Daemon self-updates its own binary (AU-1..3)**
- launchd `KeepAlive` means the daemon exiting IS a restart; it now watches its own executable and steps aside for a verified newer binary instead of waiting for a human to run `launchctl kickstart`.
- AU-1 adds four safety properties: verify the candidate executes and reports a version before yielding, require the same bytes across two consecutive polls (settle), rate-limit to one adoption per cooldown, and log every adoption/refusal loudly.
- Guards against a real incident: an unsigned binary once installed over the daemon's path and macOS refused to run it (`OS_REASON_CODESIGNING`); exiting on file-change alone would have looped forever.
- `FATHOM_SELF_RESTART=off` pins the binary, and the choice is announced rather than silently honoured.

**Dropped instrument fields caused zero admissible A/B sessions (MI-4)**
- `transcriptSummaryFrom` hand-copied 3 of 12 parser fields; every field added later (including `FirstTS`) was computed then discarded, so `arm_coverage` read `unknown` and not one OFF-arm session was admissible (EX-1, IQ-2).
- Consequences: no arm ever had a spend-shaped median (in/out/cache split dropped), warm-up (IQ-2) forever reported 'rows predate the instrument', and exploration-ledger per-question cost (EX-1) never filled in.
- Fix: whole-struct assignment so new fields carry by construction; `TestTranscriptSummaryCarriesEveryField` asserts via reflection so reverting the fix requires naming all twelve fields.
- Also fixed: `TestFlywheelSurvivesWithoutALocalCallLog` used process-global `os.Chdir`, breaking later tests resolving relative paths; replaced with a `FATHOM_CALL_LOG` override.

### 2026-08-09

**Session-count measurement bug inflated denominators 25x (MI-1..3)**
- Claude Code's `Stop` event fires per turn, not per session close, so 200 ledger rows were only 8 distinct session ids; one resume_as_code session wrote 74 rows alone, inflating 'of 188 sessions' denominators roughly 25x.
- `discovery_actions_total` is a per-session cumulative, so summing rows double-counted: 17,727 collapsed to 1,038 after fix; the pre-registered '>=20 sessions per arm' floor would have been met by about one real session.
- Fixed by deduping at read time, keeping the LAST row per session id, while still reporting the raw stop count alongside for auditability (MI-1).
- New `fathom rate <1-5> [--note]` command instruments the self-rated outcome half of the criterion, joined to the ledger by session id at read time; an unjoinable rating is refused rather than stored.
- `evaluateABCriterion` now states in words which parts are unmet, e.g. 'the OFF arm has 0 of the 20 sessions registered as the minimum; the ON arm has 5 of 20'.

### 2026-08-08

**Realization grade replaces wiring grade, ladder of 5 rungs (RG-1..4)**
- `wiringGrade` could score a repo A from configuration alone (one call ever, no obligations firing, no gate running); `realizationGrade` (RG-1) instead folds a 5-dimension ladder: wired, fresh, queried, preventing, gating, each a precondition for the next.
- 'Queried' now counts calls in the current window rather than ever, so the signal can actually go down; 'preventing' and 'gating' are new and are the only dimensions measuring fathom doing something rather than being available.
- The letter grade names the first broken rung (e.g. 'B -- not preventing') instead of a bare letter.
- A repo with no plans grades gating not-applicable rather than failing; a repo with no sessions in the window grades idle, never F, avoiding false alarms that bury the real one.
- Fleet page now sorts by unrealized value (unmet rungs weighted by sessions) instead of call count, ties breaking on sessions then name.
- Refs: SH-1, SH-6.

**Self-healing fleet engine, fathom edits repos under law 15 (SH-1..6)**
- `fathom fleet reconcile --apply` sat unrun beside 9 of 17 repos for weeks since it required a human to act; `internal/wiring` (SH-1..6) makes it a safe button via three ownership tiers.
- Marker format v3 (`<!-- fathom:begin v3 sha256:... -->`) hashes the region body as fathom last wrote it, so a hand edit inside fathom's region is detectable with no history; pre-v3 markers are recorded as unknown provenance, distinct from drift (law 3).
- `.fathom/owned.json` ledgers keys fathom added to files it doesn't own; a losing ledger makes fathom own nothing, never a duplicate.
- Plan -> apply -> receipt: the plan is always computed before mutation, and one BLOCKED item does not block others in the same run; law 15 is one-sided -- a drifted region is never merged, only reported BLOCKED with both versions attached.
- New verbs `fathom fleet receipts`, `fathom fleet undo --last`, `fathom fleet undo <receipt-id>`; undo restores from receipt (never inverts) and refuses if the file changed since, itself receipted.
- Also fixed: `fetchServicesOnce` dereferenced a nil `*http.Client`, so `fathom flywheel` panicked outright when the local call log was absent.
- Refs: AM-1, AM-5, AM-7, SH-6.

**Anchored memory, refusal as the product (AM-1..5, AM-7)**
- Phase W of `docs/plans/ambient-fathom.md`: a memory with no anchor is refused outright (not stored at low tier), with the reason and destination stated, e.g. 'put it in CLAUDE.md'; the store stays empty on refusal by test.
- Tier is derived structurally from evidence kind, never asserted by the caller; `WriteMemory` routes claimed confidence through agentic-kit's `scoring.ApplyCeiling` plus `TierCeiling` (AM-1's first caller), proven against a lying caller claiming verified/5 that lands capped at assumed/1.
- Drift is evaluated at read time and cached per index generation, no sweep or timer, so it can't be missed while the daemon is down.
- Lifecycle: fresh -> suspect (anchor content changed, tier drops one step) -> orphaned (anchor disappears, stops surfacing, never deleted); `fathom memory review` resolves as still-true/no-longer-true/rewrite, with model-confirmed landing strictly below human-confirmed automatically.
- New `internal/memory` package and `fathom memory add|list|review|show`, stored at `<root>/.fathom/memories.jsonl`, project-scoped, loopback only (law 13); AM-6 (obligations injection) and AM-8 (`memory_search` tool) deliberately deferred.
- Refs: AM-5.

**`fathom flywheel` verifies feedback loops are actually closed (FW-1..3)**
- New command checks 7 loops (session ledger, counterfactual arm, call log, obligations, verify gate, exploration ledger, observations) and reports each CLOSED (produced, arriving, and consumed), OPEN (a defect, exit code 1), or IDLE (nothing to measure yet, never fails the exit code).
- Motivated by two same-day silent measurement failures: the OFF arm burned 14 hours recording nothing, then produced rows that were all inadmissible.
- The arm loop distinguishes 'armed but recording nothing' from 'rows arriving, none admissible' -- the latter is subtler because the producing side looks alive.
- Caught a bug in itself on first run: the obligations loop read `ObligationCalls` off the impact model where it is always zero instead of the call log via `buildObservatoryModel`, wrongly reporting IDLE on a loop carrying 6,994 injections.
- Live fleet result at time of writing: 4 closed, 1 open (counterfactual arm), 2 idle.

**Arm coverage labeling fixed after OFF-arm rows leaked pre-arm activity**
- First two OFF-arm rows after the earlier fix were corrupt but unlabeled: their transcripts began 10 days before the arm was set (duration_ms 243h and 256h), one carrying tokens_total 933,232,702, yet stamped arm: off.
- Such rows would drag the OFF arm toward the ON arm in a paired comparison, manufacturing a false null result ('fathom saves nothing') from mislabeled evidence.
- Producer side: `sessionLedgerEntry` gains `arm_coverage` (+note) via `armCoverage`, valued full (session began at/after arm set, the only value admissible), partial (transcript predates the arm), unknown (no timestamped line, not assumed clean per law 3), or empty (no arm active).
- Consumer side: `buildImpactModel` excludes partial/unknown rows from both arms and names the excluded count with reason rather than silently dropping them; a row stamped off with no coverage field is treated as unknown.
- Also fixed: the brief promised a median comparison whenever both arms had any sessions, even without tokens on both sides; every new test was confirmed RED against the ungated fold first, without which the 933M row leaks into the OFF median.

**Capability registry reported switches it never enforced (CR-7)**
- `fathom capabilities set guards off` wrote config and the table reported 'switched off by this repository' as fact while guards kept firing on every edit -- a config read rendered as behavioral fact, violating law 14 inside the package built to enforce it.
- Fix distinguishes `Capability.Switchable` (obligations, guards, verify-gate: actually gated in `edithook.go`, `edithook_pre.go`, `stophook.go` before daemon traffic) from Status capabilities (index, layers, modules, agent-surface, session-ledger, narrate), which `set` now refuses and names the real control instead.
- `verify-gate` off still records the ledger row with `verify_catches` as nil (not a fabricated zero), since the gate did not run and its result is unknown (law 3).
- `session-ledger` is deliberately status, not switchable, so measurement cannot be silently disabled from a config file; `FATHOM_CHANNELS=off` remains the documented kill switch.
- `TestEverySwitchableCapabilityHasAnEnforcementCase` fails the build if a capability is marked switchable without a test proving observed behavior changes; all enforcement tests were confirmed RED against the shipped defect first.

**URGENT: OFF arm was silencing its own measurement, zero ledger rows**
- `fathomChannelsDisabled()` returned `env == "off" || armOffActive()` and every hook called it first, so an active OFF arm made the session hook exit before its announcement/decrement and the stop hook exit before `recordSessionLedgerEntry`, leaving zero ledger rows.
- Measured cost: an arm set 2026-08-08T07:11:47Z produced no ledger rows for the rest of that coding day, with `sessions_remaining` stuck at 10 of 10 since the decrement never ran.
- `TokensOffSessions` could never rise, so the counterfactual pilot could never leave 'collecting' -- the one instrument able to license a savings claim (law 12) was structurally unable to produce one.
- Fix separates measurement from assistance: `fathomChannelsDisabled()` is now the env kill switch alone; new `assistiveChannelsDisabled()` = env or active OFF arm, used only by edit hooks (obligations, guards, pre-edit channel); session and stop hooks use the env gate so they always record, and the stop hook returns before the verify pass (an assistive gate) leaving `verify_catches` nil rather than 0.
- A test, `TestArmStamping/channels_kill_switch_honors_the_arm`, had asserted the buggy conflation as required behavior for weeks; it and a new `armmeasurement_test.go` now pin the correct contract, confirmed RED against the old gate first.

**Capability registry, one typed on/off surface (CR-1..6)**
- `internal/capability` is a typed registry over four extension seams; `fathom capabilities` is the new verb answering "why isn't X happening here?"
- Detection is observation, never configuration: `guards: on` with no rules compiled reports OFF with the reason (law 3 applied to config).
- Every registry entry must state why it is off, enforced by a test iterating the registry against a ZERO probe; absence and breakage are kept distinct so only breakage fails doctor.
- Law 13 enforced loudly: a user-scope file setting a project-only key is refused with the key named, unknown ids get nearest-match suggestions.
- Fixed three defects found by tests: unbound module read as breakage, duplicated `daemon_reachable` diagnosis, and index freshness collapsed from three states (unknown/stale/fresh) to two; also renamed ambiguous anchor `Validate` to `ValidateSettings` (O22, O23).

**Value brief rewritten as a readable story (BR-1..4)**
- The brief now opens with a plain-language short version (what fathom is, its scale, what it has and has not shown) plus five numbered questions each with an explicit Reading line.
- All counts route through the observatory's existing `fmtThousands` (`17,727` not `17727`); a duplicate helper added in a first draft was deleted in favor of the incumbent.
- The closing section became a work order: each entry names what's switched off, what enabling it would let a future brief say, and the exact command or a reason none exists.
- Exposure is not outcome: the prevention section now states that 6,992 rule injections and 17,727 discovery actions count things fathom did, not mistakes avoided.
- Brief page rendering escapes markup before tag emission so an instrument note can never inject markup.

**Observatory readable pass: token medians, chart values, index-only honesty**
- The 596M-token finding: `tokens_total` summed all flow including cache reads, inflating a single row's "median session" to 596M tokens; the ledger now splits in/out/cache at instrument, model, and render layers.
- Rows predating the split count as unknown rather than feeding the median an inflated total; OFF arm now reads "median real tokens --" instead of "no token totals".
- The tool-load chart now renders comma-grouped value labels (`ShowValue: true`) instead of relying on bar-width eyeballing.
- Every non-A wiring card now shows a remedy via fallback to `GradeRaise` when `FreshnessRemedy`/`WiringRemedy` don't cover the "wired but unused" case.
- A C/D-graded service with zero ledger sessions and zero agent calls now renders a neutral "index-only" chip instead of a scary amber letter; learn-tooltips added for "(N agent)" traffic figures.

**Exploration ledger: the 90x measured, dashboard readable (vision 19, EX/FR/TD)**
- Every session transcript now records both raw exploration (Grep/Glob/Read sizes) and indexed discovery (fathom answers), measuring per-question token cost and ratio, folded fleet-wide (EX-1/2/3); impact page leads with this story (EX-4).
- Dashboard readability: every page opens with a plain-English lead and no-JS explainer; `/observatory/brief` renders a management-ready summary with markdown download (EX-5/6); service page shows layers, guards, bindings, and obligations-compile condition (EX-7/FR-5).
- Field report fixes: plans status/next now discover plans in the service's own repo, not blindly cwd, fixing a cross-repo wrong-verdict bug (FR-1); the 264,487-catches artifact retired in favor of per-session verify_catches (FR-2).
- Fleet reconcile detects and `--apply` installs missing shipped skills (FR-3); session hook now teaches the first query with a live symbol from the repo's own index so the first query lands (FR-4); O13 filed: CompileLayer ignores the provider registry, deferred to FR-6.
- Tool diet (TD): agent surface pinned at <=16 tools (~12.3 KB schemas, ~3.1k tokens/session) by test; the alarming 88-tool count is the full /rpc surface agents never load; wiring grades stop reading invisible-from-here as absent (EX-8).

**Observatory v2: the four questions leaders ask (vision 18, IQ-1..11 + IQ-15)**
- Q1 (what did the agent not have to do?) stays DARK until OFF-arm sessions exist, law 12 rendered as product; Q2 (what did fathom prevent?) shows verify catches plus 6,985 obligations served live from the call log, with conformity checking deferred to IQ-13.
- Q3 (where does the agent flounder?): call-log rows carry result_count and matched_via (IQ-1) so hits/misses/did_you_mean are distinguishable per tool; Q4 (is knowledge compounding?): ledger learns first_edit_turn (IQ-2).
- New story layer: `/observatory/session?id=S` replays one session as a Timeline with a ledger verdict card (IQ-4); `/observatory/story` composes the trailing week with session-replay links (IQ-5); landing page leads with the four questions (IQ-6).
- Fleet tiles gain 14-day sparklines, top-tools bar chart, and a WIRING GRADE chip (A-F) with the one command to raise it (owner ask); brief reorganizes around the four questions (IQ-11).
- `--for 14d` parses (IQ-9); `fathom init` auto-arms a 10-session narrate window, self-expiring (IQ-10), ON fleet-wide until 2026-08-22; deferred: session-start context pack (IQ-12), obligation conformity (IQ-13), LLM weekly (IQ-14).

**Observatory: value realization at a glance, built on app-kit (vision 17, OB-1..12)**
- Daemon grows a full loopback front door: GET /, `/observatory` fleet tiles, `/observatory/service?id=S` detail, `/observatory/impact` verify catches/discovery actions with denominators and a counterfactual pilot state machine (law 12); `?format=json` everywhere.
- `fathom brief` renders the impact model to a dated local markdown value brief with an explicit "Not measured" section, keeping sharing a human verb (law 13).
- Built entirely on app-kit per owner directive: dashboard.RenderWidget/Grid, stat.RenderBand, viz.Sparkline/Gauge replace hand-rolled tiles deleted the same day; the honest-absence tile was promoted up into the kit as `stat.Absent` (app-kit af47262, D13).
- CSS builds via `task ui:css` from `cmd/fathom/ui/input.css` using the kit's checksum-pinned Tailwind binary, no Node; UI doctrine wired into CLAUDE.md/AGENTS.md and .fathom/guard.cue.
- Every page verified by Playwright screenshots in both themes; fixes included absence-note duplication, missing thousands separators, and a duplicated law-14 line.

**Full C++/CMake/clang/LLVM/Ninja support (cpp-toolchain, IO-27 trigger fired)**
- Symbols get one spelling: out-of-class definitions ("Widget::render") normalize "::" to "." so call edges join inline-method spelling (CT-1); templates, operators, destructors, and C++17 nested namespaces pinned by tests (CT-2).
- Polyglot `.h` extension now routes per-file through a C++-signal sniffer so LLVM-style headers keep classes/namespaces/methods instead of being flattened by the C grammar (CT-3).
- CMake becomes a dialect: CMakeLists.txt/*.cmake extract projects, targets, functions, options, find_package deps, with target_link_libraries as target-to-target edges (CT-4/5), via TierPreScan hand scanner (no variable-indirection guessing).
- languages field stops lying for C-family services: cpp/c/cmake/rust/csharp/cue (CT-6); doctor's `cpp_toolchain` check warns (never fails) when compile_commands.json is missing, naming the Ninja+clang export command (CT-7); init excludes build/, out/, cmake-build-*/ (CT-8); build.ninja refused as indexable input (CT-9).
- ExtractorEpoch 5->6 forces re-extraction of all stored C/C++ observations; calibrated live against fmtlib/fmt: 5,470 symbols (424 classes + 181 namespaces from .h headers), 24 CMake targets, zero "::" contamination (CT-10). Precise clang mode, WASM engine migration, Conan deferred (CT-11/12/13); O12 filed: ingest CLI's cwd-relative -store default indexes into the void.

**Activation plan closes: exit proofs run live (IO-30/31/32)**
- IO-30: `fathom instrument --adoption --service app-kit` from app-kit cwd resolved the daemon call log (69 calls, bare=12/agent=57 split), proving the gauge is cwd-independent.
- IO-31: doctor on app-kit ran 24 checks, 20 pass/3 fail/1 skipped, and every FAIL was true (lefthook_gates, skills_installed, surface_traffic) -- no false doctor row survives on a consumer repo.
- IO-32: the counterfactual pilot was pre-registered before any OFF session, N=10 matched sessions per arm, primary metric tokens_total from the ledger, medians reported as a range with denominators, kill criteria filed per law 12.
- O9 closed (fixed by the IO-9 budget fix); O10 filed: a redeploy rolls the TCC dice, new binary hash under launchd silently loses the removable-volume grant; O11 filed: CLI call log resolves cwd-relative.
- Final sweep `fathom verify --service fathom --plan docs/plans/impact-observatory-activation.md`: 32 rows, 29 claimed, every contract-bearing row verified, three deferred rows carry triggers.

**Skills pack: eight skills, three renderings, one plugin (vision 16, IO-23/24/25/26)**
- fathom-rewire and fathom-activate join the embedded skills pack, promoted from the reconcile pass and activation catalog prompts.
- `--skills=agents` renders the same SKILL.md set to the shared `.agents/skills` path Codex and Gemini CLI both walk natively, one spec-conformant write for multiple harnesses.
- `fathom plugin render` generates the Claude Code plugin from the same embedded sources: plugin.json, all eight skills, hooks/hooks.json spelled via the shim-stable `fathom`, .mcp.json, and optionally a marketplace manifest.
- IO-26 integration proof run live: both kit packs (app-kit ui-appkit, agentic-kit agentic-kit-standards) install via `fathom layers install` and list with honest not-yet-eval-gated warnings.

**Narrate v0: enhanced-understanding dial, story rows only (vision 14 section 6, IO-21/22)**
- `fathom narrate on --for D|--sessions N` is bounded, auto-expiring, and off by default; an unbounded narrate call refuses.
- While active, hooks append version-stamped story rows per intervention (obligation-served, verify-catch, guard-served) to `<repo>/workspace/artifacts/narrate/story-<date>.jsonl`.
- Story rows are per-repo, local, never bundled, and a lens over mechanism-labeled events, never a source of truth (law 14 candidate) -- nothing downstream may consume them as evidence.
- Pinned by TestNarrateExpiry and TestStoryRows, confirming inactive narrate writes nothing.

**Fleet reconcile and session tripwire: wiring is a diff (vision 14 section 7, IO-17/18/19)**
- `fathom fleet reconcile` diffs every repo's installed hooks against the canonical template (fathomHookSpecs), naming missing entries, stale absolute binary paths, and pre-rename stanza branding per repo.
- `--apply` executes only the SAFE subset: repointing stale commands to the shim-stable `fathom` in the project file, merging missing entries through init's idempotent machinery, one scope, never both.
- The session hook is now the tripwire: a drifted repo's session opens by naming the drift and the exact remedy.
- Pinned by TestReconcileDiff/TestReconcileApply (drift -> apply -> clean) and TestSessionTripwire.

**PATH shim: hooks spell `fathom`, never a checkout path (IO-20)**
- `fathom init` installs `~/.local/bin/fathom` pointing to the deployed libexec binary, falling back to the running executable, refusing to clobber a foreign real file, and idempotent on rerun.
- doctor gains a `binary_on_path` check after a 2026-07-31 case where `fathom verify ...` printed command-not-found as one token and the pipeline exited 0, a verification failure wearing a pass.
- One stable name on PATH ends the class of bugs where hook behavior depends on when a checkout was last rebuilt.

**Fleet report: the audit as a verb (IO-16)**
- `fathom fleet report` folds daemon registry, per-service call-log traffic, local wiring state, open observations, ledger session counts, and three-state freshness (indexed commit vs HEAD via statusz) into `~/.fathom/fleet/report-<date>.{json,md}`.
- Local files only, no daemon writes, no network beyond loopback; collection stays per-repo, consumption is explicit and human-run, not silent aggregation.
- Every unmeasured cell is a dash, never a fabricated zero.
- First live run folded the real fleet's 18 services.

### 2026-08-07

**Ledger attribution: verify catches ride the session row (IO-14, IO-15)**
- Stop hook now runs its verify pass first; the session-ledger row carries `verify_catches`, the proven-failing count for newly-claimed tasks, as a nullable field (nil = UNKNOWN when verify never ran, not a fabricated zero).
- Together with `arm` (IO-14) the ledger now holds the counterfactual program's join keys per session.
- The vision's other two attribution fields, `injected_rules_used` and `discovery_substitutions`, need obligation-file tracking and call-log/beat correlation that don't exist yet; deferred, not dropped.

**Measurement arm: FATHOM_CHANNELS=off with a memory (IO-14, O15)**
- `fathom arm off --for D --sessions N [--note]` is the counterfactual program's OFF switch: STAMPED (schema+fathom version, set-at, note), BOUNDED (unbounded OFF refuses), ANNOUNCED (doctor's `channels_env` and the session hook declare it), RECORDED (session-ledger `arm: on|off` field).
- Addresses the O15 lesson that a silent kill switch is a measurement bug.
- `fathomChannelsDisabled()` composes both switches; hooks treat them identically. State at `~/.fathom/arm.json` (0600); `fathom arm on` ends it early.
- Pinned by TestArmStamping (bound refusal, session countdown to expiry, wall-clock expiry, clear, kill-switch composition).

**Version-stamped telemetry: every record names its producer (IO-11)**
- New `internal/buildinfo` package (VCS revision + -dirty, "unknown" floor, never empty) stamps call-log rows (`schema_version 2` + `fathom_version`), hook-beat meta, session-ledger rows, and daily rollups.
- Hook-beat meta also gains `project_dir`, the per-repo attribution the user-global beat log lacked; caller keys win on collision, caller maps never mutated.
- Legacy rows without the fields read as version-unknown, never backfilled or guessed.
- The 251,834-catch-headline class of cross-semantics aggregation becomes a filterable query instead of an investigation.

**Registry surgery: two verbs, one-root-one-service enforced (O1, IO-1, IO-2, IO-3)**
- `remove_service` (idempotent every-row deletion, claim_events cascaded through claims) and `rename_service` (atomic re-key across eleven service_id tables, typed refusals, claim ids stay minted as historical evidence) both delegate store to client to AutoRefresh to tool and bust the identity cache on success.
- The ingest handler now enforces ValidServiceID at the daemon (the geoffmiller-cloud twin was re-fed for seven weeks past the client fix), plus EvalSymlinks canonicalization, duplicate-root and nested-root refusal.
- With IO-1's deterministic resolution, every O1 fix (a)-(d) is now landed.

**Adoption gauge: scoped to any repo, agent-surface aware (O6, IO-10)**
- `instrument --adoption` gains `--service` (case-insensitive filter; unattributed rows excluded AND counted beside the filter, never silently dropped) and cwd-independence, reusing doctor's call-log resolution when the relative default log is absent.
- O6's headline defect closes: the zero-call list is partitioned into "zero-call AGENT-surface tools (actionable)" vs "registered but not exposed" (different remedy).
- New `mcp.AgentSurfaceTools()` exports the allowlist the partition scopes against.
- Pinned by TestAdoptionServiceScoped.

**compile --dry-run: cost as an honest range (O8, IO-8)**
- Dry run once reported input-only tokens and `cost_usd: 0.000000` for a run that actually cost $0.64 and 21 minutes (output was 87% of tokens).
- Now carries an estimate band (EstTokensOutLow/High = input x [2, 8], observed obligations layer ran ~6.7x); human rendering prints tokens-in as the bytes/4 heuristic it is, output as unknown-with-a-band, cost as a labeled range, duration band at ~100 output tok/s.
- Real runs print exactly as before.
- Pinned by TestDryRunEstimateRange (positive ascending cost band, no LLM configured needed).

**Explicit-empty bindings/guards accepted as valid (O7, O22, IO-7)**
- O22's fix accepted the ABSENT field but kept `bindings: []` a loud error; the fleet's activate skill, kit packs, and review fix-pass all standardize the explicit literal, so doctor FAILed an adopter file fathom's own tooling prescribes.
- Now one vocabulary: `bindings: []` and `guards: []` load clean and pass ValidateStrict (schema `[_, ...]` constraints dropped in schema/binding + schema/guard); only present-and-malformed entries fail.
- Live proof: platform-engineering's standing `binding_file FAIL` is now PASS.
- Pinned by TestExplicitEmptyBindings / TestExplicitEmptyGuards; prior rejection-asserting negatives inverted.

**review --budget-seconds bounds the advisory channel (O9, IO-9)**
- Root cause: nothing prompts and nothing hangs, a six-file pre-push review measured 22.6s and lefthook buffers output until completion, so scripted pushes read the silence as a stall and killed it at their caps.
- `fathom review` gains `--budget-seconds N`: the whole body races the deadline and on excess prints a named advisory skip, exits 0.
- Flag requires `--no-persist` since a run that can be cut off must never be a store writer; pre-push step now passes `--budget-seconds 20`.
- Pinned by TestReviewNonInteractive. Follow-up filed in O9's fix note: why a six-file review costs 22.6s at all.

**doctor: lefthook gates recognized through variable indirection (IO-5, O23)**
- `checkLefthookGates` no longer requires the literal substring "fathom verify"; `lefthookInvokes` matches literal, path-suffixed, or shell-variable spellings (`"$FATHOM" verify`, `${FATHOM} verify`).
- Closes the audit's live false negative where a correctly-gated repo (platform-engineering's pre-push loop) was told to add a gate it already had.
- The audit's other two doctor findings (`binary_fresh` HEAD comparison, `hooks_firing` repo-relative call log) were already fixed at HEAD by the O23 wave; the audit ran a stale repo binary, the deploy-skew lesson the reconcile design exists to close.

**Plan grammar: quoted braces preserved, unparseable blocks loud (O4, GI-7, IO-6)**
- `parseAnnotation`'s brace pairing is now quote-aware: a quoted value containing the notation's own vocabulary `{symbol, package}` no longer terminates the block, so the row keeps its author-owned id and full contract (TestParseAnnotation_BracesInQuotedValues).
- GI-7's trailing-block rule is preserved: pairing runs over outside-quote braces only, last close wins.
- A brace block that parses to zero keys but contains contract keywords now surfaces as `Task.AnnotationSuspect`, and LintPlan names it "VOID" instead of silent md-NNN downgrade (TestLintPlan_UnparseableAnnotationIsLoud).

**Session hook: enumeration failure is not "not indexed" (O1, O2, IO-4)**
- `fetchServices` now returns an error alongside the map, with one retry on its own 4s timeout; every failure mode (transport, non-2xx, unmarshal) becomes an error instead of an indistinguishable empty map.
- Session hook grew the third branch O2 asked for: a healthy daemon whose `list_services` fails renders a degraded notice naming the failure, never the confident false "not an indexed service" instruction.
- A genuinely empty index now says "none yet" instead of the dangling "Indexed services: ." join artifact.
- Hook's inline candidate dedup now uses mergeCandidates (the O1-c fix). Pinned by TestSessionHookDegraded and TestSessionHookZeroServices.

**observe race window before foreign-file refusal**
- `fathom observe`'s foreign-file refusal now waits the same 200ms `reReadAfterLostCreateRace` window before concluding a file is genuinely foreign, instead of misreading a concurrent creator's O_EXCL-create-to-header-write window as a foreign file.
- Previously the loser refused and lost the entry (exit 1); `TestObserve_ConcurrentFirstCreationLosesNothing` now green at `-count=10 -race`.
- Found by the impact-observatory activation gate run, not by a user losing an observation.

**Deterministic service resolution and dedup (O1 a-c, IO-1, IO-2)**
- `resolveFromServices` no longer picks duplicate-corpus-root winners by Go map iteration order, which had zeroed obligations on 18/50 measured edits; winner is now chosen by richness (symbol count), then basename affinity, then lexicographic order, pinned by `TestResolveFromServicesDeterministic` (200-iteration determinism per case).
- `candidatesWithDaemon`'s dedup is extracted to `mergeCandidates`, which drops only exact name+root duplicates so a name-equal candidate at a different root survives (`TestMergeCandidatesNeverCollapses`).
- O1 fix (d), daemon-side duplicate refusal, lands separately with plan row IO-2.

## [0.3.0] -- 2026-07-30

**What this release is.** The rearchitecture and the proof. The store
went pure-Go SQLite and DuckDB was deleted outright (ADR-025); the review
verb, findings-as-claims, semantic layers, expert modules and the layer
catalog landed; the MCP surface split into a curated 16-tool agent surface
over an expert registry (ADR-014); the deterministic SDD reconciler,
`fathom plans`, and the done-means-proven gate went live in the repo's own
hooks; and the benches were run honestly -- the lookup x10 claim retracted
under its pre-registered bar, then fathom winning its home turf 13/13 vs
11/13 at 0.570x tokens. Detail: `docs/changelog/0.3.0.md`.

### 2026-07-24

**T5-8 second bench swing, fathom wins home turf (O10, O12, O15)**
- Home bench (H2-1 criterion): PASS, success 13/13 vs grep 11/13, tokens 1,594,762 vs 2,795,948 = 0.570x, turns 47 vs 72; fix wave drove it, `read_source` (T5-4) cut H1-T13 from 601,132 tokens/16 turns to 106,954/4, DocChunk `end_line` (T5-5) flipped H1-T08 to a pass.
- Lookup bench (B1's <5x bar): the x10 claim retraction STANDS; 15/15 vs 14/15 success, tokens 0.511x = 1.96x reduction (was 1.38x), still under the pre-registered 5x bar.
- Pre-run corpus revision committed first (8a847d3); bench runner's fathom-arm allowlist gained `read_source` to match the live 16-tool `/rpc/agent` surface.
- `list_services` now returns each service's recorded corpus `path`; session/edit/stop hooks resolve service identity by matching project directory against those paths, fixing O15 where obligations injection was silently dead on the dogfood repo for six days.

**Citation drift re-anchoring and eviction visibility (T5-1, T5-2, O13, O14, ADR-017, OP-6)**
- O13 fixed: `core.ClaimLink.AnchorText` is now persisted alongside the hash by both live "cites"-link writers; `semantic.Sweep` re-anchors a drifted citation when its `AnchorText` resolves to exactly one new location, appending a `ClaimEvent{Verdict:"re-anchored"}` audit entry.
- Fleet re-sweep (T5-1b): 340 claims checked, 73 wrong citations found (all fathom-service), 0 healed / 73 staled since no existing claim carries `AnchorText` yet; healing activates on the next fleet recompile.
- O14 fixed: `obligations` reports `displaced: N` when `MaxObligations` drops served-eligible claims, and every served rule carries a `layer` label; the scope-mapping half is filed as `prompt.PENDING-scope-precision.md` pending a deliberate flag-day alongside OP-6.
- Once deployed, ~73 fathom obligations begin serving `[stale]`-labeled fleet-wide, an expected honesty event, not new breakage.

**read_source tool, DocChunk line offsets, symbol_context snippet (T5-4, T5-5, T5-6, ADR-014)**
- New `read_source` MCP tool (`{service, file, start_line?, end_line?}`), bounded/line-numbered/read-only, added to the 16-tool agent surface; capped at 200 lines / 16000 bytes, refuses absolute paths, traversal, out-of-corpus files, and binaries; answers H3-1 where recovering cited text cost 16 turns / 601k tokens.
- DocChunk gains an additive `EndLine` field threaded through fusion, SQLite schema (`end_line` column + migration), and serving paths; `MarkdownExtractor.Extract` now computes each heading's real section extent, closing the H3-1 T08 class of guessed cited lines.
- `symbol_context` gains an inline `snippet` field (source, line-numbered, capped at 40 lines), with `snippet_truncated:true` when capped and `snippet:false` to opt out; honest-absence when source isn't retrievable.
- Hardening round closed three proven holes: symlink containment via `filepath.EvalSymlinks`, a 2MB `os.Stat` ceiling plus 16000-byte line truncation, and enforced `.fathomignore`/skip-dir/`.env` refusal reasons; all "15-tool" doc references updated to 16 with `read_source` named.

**Home-turf bench run: fathom does not win its own ground (H3-1, O13, O14, SS-8)**
- Wave-3b home-turf context-bench (13 index-native tasks x 2 arms, 26 live `claude -p` runs) failed both halves of the H2-1 criterion: success 12/13 vs 12/13 (equal, not strictly greater), tokens 1.071x grep's (7% premium where <=1x was required).
- Fathom alone failed H1-T08 (DocChunk granularity can't pin a sentence-start line); grep alone failed H1-T13 (quoted an ADR identifier outside the required line range); fathom cheaper on 8/13 tasks.
- Run's product findings filed as dogfood O13 (sweep-missed citation drift) and O14 (top-6 serving cap silently evicts previously-served claims after recompile).
- Pre-run re-derivation found the recompile's three-layer blend silently drifted T01/T02's citations and SS-8's DuckDB deletion broke T06/T07/T13's targets outright.

### 2026-07-23

**DuckDB backend and CGO deleted outright (ADR-025, SS-8, XT-6b, ADR-013)**
- SS-8's deferred trigger fired after a full green day on SQLite; `internal/store/duckdb.go` (`DuckStore`, `go-duckdb/v2`) and its test file are deleted outright, no DuckDB code remains in the repo.
- `migrate-store` removed rather than kept, since reading a pre-SS-8 store now needs a pre-SS-8 build; shared row scanners moved into `sqlite.go`.
- `store.BackendForPath` now refuses `.duckdb` loudly with a typed error naming the file, instead of a silent fallback; empty-path in-memory convention now opens `SQLiteStore`.
- `go mod tidy` removed `go-duckdb/v2` and its Arrow-only indirect deps; `internal/store` now builds/tests with `CGO_ENABLED=0`, while the full binary still needs CGO solely for tree-sitter (ADR-013), a separate not-yet-triggered removal (XT-6b).

**The review verb: review_diff and fathom review (Phase C, ADR-016, ADR-018, FT-4, FT-6, FT-7)**
- C1-1: new `review_diff` MCP tool composes diff impact, scoped obligations/findings, architecture conformance, and spec coverage into one cited verdict; CLI mirror `fathom review --store DB --service S --since <ref>`; arch-conformance edge normalization and rule-pack resolution shared via new `internal/analysis/archconformance` and `internal/projectrules` packages.
- C2-1: review findings persisted as content-addressed claims on the `findings` layer (verified tier, deterministic, fires FT-7's pre-armed trigger); refutation durability proven at three layers of tests.
- C2-1 write-path fixes before release: real cited-line hashes prevent sweep from falsely flipping 57 review claims to stale/contradicted; only deterministic-oracle dimensions persist (never `obligations` or `impact`); `--no-persist` flag added and used by the pre-push hook; reconcile supersedes fixed findings; verified-tier admission recorded in layer.cue (ADR-018/ADR-016 M1).
- C3-1: `lefthook.yml` pre-push gains a non-blocking `fathom review --no-persist` summary step (~12s worst case on a 1777-file diff); `review_change` MCP prompt now calls `review_diff` directly instead of an 8-step manual sequence.

**Serving-layer hygiene fixes (Wave 3a P1-6, P1-9, O11)**
- O11 closed: `migration_targets`'s caller-supplied `deprecated_pkgs` list now merges with the built-in default list (deduplicated, deterministic order) instead of silently replacing it, matching the tool's documented behavior.
- Sweep event vocabulary fixed: the post-compile sweep's `claim_events` verdict for anchor-dead claims is now `"contradicted"` (matching `ClaimContradicted` status) instead of `"refuted"`, reserving "refuted" for deliberate human/agent judgment.

**Pulled-tools excellence: O12's product gaps and C4-4 method-edge gap (P1-1..P1-5, P1-8)**
- P1-1 closes O12(b): `find_symbol`, `query_symbols`, and `references` now accept `symbol` as an additive parameter alias, erroring only when both native and alias values disagree.
- P1-2 closes O12(a): `symbol_context` and `find_callers` gain `cross_service_callers`/`cross_service_callees` fields when a fleet backend is wired, additive and never merged into same-service results.
- P1-3 closes O12(c): `find_symbol`'s case-insensitive tier now returns a package-match group alongside the name-match group so a package-naming query is no longer hidden behind an unrelated same-named symbol.
- P1-4 closes C4-4: `arch_conformance` edge endpoints are normalized to their declaring Go package so package rules catch method-origin and method-target edges, not just function-to-function ones.
- P1-5 re-run: B1-T09 (the O12 flagship case) improved from 504,785 tokens/20 turns to 321,100/9 turns (6.6x to 4.2x vs grep), still 4.2x more than grep, tracked as residual under O12. P1-8: dispatch now enforces declared `required` fields, so `license_policy`, `spec_coverage`, and `spec_status` refuse with -32602 when a required field is missing instead of silently running empty.

**Spec-driven development adoption guide (AD-3)**
- Added `docs/adoption/spec-driven-development.md`, the third and final adoption guide alongside `claude-code.md`/`codex.md`.
- Documents the full SDD loop: author a plan in `{id, symbol, package, ...}` grammar, implement, `fathom verify` proves rows, pre-push gate blocks over-claiming, computed progress, archival lifecycle, `spec-bindings` layer binding, oracle classes.
- Written against this repo's own live adoption program (`docs/plans/activation.md`, `docs/plans/adoption-docs.md`) with real `fathom verify`/`gen-plan-cue` output.
- `fathom plans status`/`next`/`archive`/`restore` documented from committed design and marked "(landing this wave)", not yet built in this worktree.

**Wave-1 adoption-friction fixes (ADR-014)**
- `task build` wrote to `workspace/tmp/fathom` while hooks and daemon-install expected `bin/fathom`; `Taskfile.yml`'s `build` now emits `bin/fathom`, with the fast inner-loop variant renamed `build-dev` (still `workspace/tmp/fathom`).
- `docs/use-cases/getting-started.md` documented the removed `set_profile` tool, retired by ADR-014's surfaces-week1 amendment; rewritten against `docs/mcp-tools.md` covering path-scoped HTTP surfaces, `off_surface:true` gate semantics, and the real `capabilities` response shape.
- `docs/usage.md`'s CLI reference stopped at `session-hook`; added entries for thirteen shipped verbs (`init`, `compile`, `layers`, `verify`, `config`, `providers`, `smoke`, `instrument`, `edit-hook`, `stop-hook`, `claim`, `remap`, `report`), flags verified against `cmd/fathom/*.go`.

**`fathom plans` (E3/E4), reverse-drift report (E6-1), verify_plan SLO (B3-1)**
- `fathom plans status` (E3-1): per-plan rollup (proven, claimed-unproven, open, deferred-with-trigger, skipped-other) computed fresh from the verify engine, `--json` plus a human table.
- `fathom plans next` (E3-2): ordered work frontier including claimed-but-unproven rows as `claimed-failing`/`claimed-unknown`, ordered by plan file then document order.
- `fathom plans archive`/`restore` (E4-1): governed lifecycle requiring all rows proven or skipped, task IDs absorbed into CHANGELOG.md/ADR.md, and a fresh re-run; writes a tombstone and `git mv`s the plan to `docs/plans/archive/`.
- Pre-push/CI gate scoped to active plans (E4-2): `lefthook.yml` and `scripts/ci.sh` walk only the top level of `docs/plans/`, so archived plans drop out of gate scope.
- `fathom report --analyzer reverse-drift` (E6-1) computes ungoverned public API surface per service; `verify_plan` p95 SLO (B3-1) set at 5s with a Stop-hook timeout degrading to advisory, never a hard block.

**context-bench and the x10 tokenomics retraction (B1/B2-1)**
- context-bench built (B1-1/B1-2/B1-3): 15-task committed corpus with grep-derived ground truth, tested scorer (`internal/contextbench`, 16 tests; `cmd/contextbenchscore`, 7 tests), two-arm `claude -p` runner comparing fathom-MCP-only vs grep/find/cat-only.
- The x10 tokenomics claim is RETRACTED (B1-3/B1-4): first full run measured equal task success (15/15 both arms) at 1.38x aggregate / 1.79x mean per-task token reduction and 2.81x mean wall-time reduction, below the pre-registered <5x retract criterion.
- All docs stating x10/<=1/10th updated to measured magnitudes.
- B2-1 existence-check wiring: `scripts/check-surface-traffic_test.sh` proves ok/zero-traffic/missing-dependency verdicts against a local fixture server; law-8 adoption clock explicitly not started.

### 2026-07-22

**Relative corpus paths at ingest boundary (dogfood O10, ADR-020)**
- Typed edges were silently absent for relative `--path` (CLI default `.`): the connector-scope filter rel-mapped absolute file positions against a relative `repoPath`, excluding every package with zero edges and zero failures.
- `EnrichTypedEdges` now normalizes `repoPath` to absolute at entry; added a never-silent stderr guard and typed-edge counters in `cmd/fathom ingest` output.
- `ServiceMeta.Path` was recorded verbatim-relative, causing wrong-corpus refresh from the daemon's working directory; now recorded absolute, with both repos' post-commit hooks passing `--path "$PWD"`.
- Added `scripts/migrate-fleet-store.sh`, the executable ADR-020 DuckDB-to-SQLite runbook, decoupling the daemon from post-commit rebuilds and stamping the SS-8/XT-6b bake clocks.

**Tree-sitter extraction engine via wazero/WASM (ADR-021 s4, XT-1..6b)**
- `internal/extractor/tsengine`: a pure-Go tree-sitter execution engine, C core + one grammar's parser.c/scanner.c compiled to `wasm32-wasi` and run via `wazero`, no CGO in the engine and no Docker at runtime.
- `NewTSExtractor` (XT-1) implements `extractor.Extractor` for javascript/python/ruby via real tree-sitter `.scm` queries; qualification handles "Class.method" and Ruby's "Outer::Inner::Widget".
- Pathological-input bounds (critic B1): `({[` x 50,000 previously ran 52.9s unbounded vs ~49ms on the CGO incumbent; now bounded via guest-side parse budgets, a host-side hard backstop, and parser re-creation on fault, counted in `fusion.Report.TSEngineFaults`.
- Calibration (XT-2/XT-3/XT-4) on the 12-repo corpus: coverage never regressed; JS/TS gains largest (lodash 0.41 to 1.26); Python/Ruby tie their CGO incumbents; corpus ingest measured at 1.69x the incumbent, exceeding the plan's 1.5x line, flagged for owner gate call.
- `fusion.ExtractorEpoch` bumped 4 to 5, forcing re-extraction; `capabilities` MCP tool gains `extraction_engines` (XT-5); binary-wide CGO deletion (XT-6b) remains PENDING on a 7-day green bake, per ADR-013.
- Refs: ADR-020.

**Typed Go call edges resolve O9 ambiguity residue (ADR-021 s3, CT-4..6)**
- `internal/fusion/typedcalls.go`'s `EnrichTypedEdges`: additive Go-only pass via `go/types` producing `EdgeDirect`, `EdgeIfaceDispatch` (one edge per in-scope implementer), and `EdgeUnresolved`, scoped to `.fathomignore`-respecting Discover set with per-package caching and reconcile.
- `core.Edge` gains `Kind`; new store surface round-trips typed edges through SQLite, DuckDB, and the memory fake, contract-tested.
- verify (CT-4): `test_reaches` now verifies paths crossing interface-dispatch edges, labeled "reached via interface dispatch (N implementer(s))"; three real O9 residue shapes proven flipping from refusal to typed reach.
- Serving (CT-6): `blast_radius`/`diff_impact` traverse typed edges; `find_callers` adds `typed_callers`, `symbol_context` adds `typed_edges`, `server_stats` reports per-kind typed-edge counts.
- Performance gate (CT-5): first-run typed load measured against a true syntax-only baseline; cached re-ingest is delta-only.

**Tool identity critic round fixes (ADR-021 s2, F1-F6)**
- ToolKind/Generated were write-only against real stores (F1, HIGH): sqlite/duckdb column lists dropped them while the memory fake round-tripped them, so asserting tests passed against a lie; both columns now persist in both backends and are covered by the shared store CONTRACT suite.
- CRD tagging is now per-document (F1e/P15) via `core.SymbolNode.ToolKind`, fixing a multi-doc YAML stream that tagged every observation "k8s-crd" file-wide.
- Guard-wrapped Helm templates extract (F2): whole-line `{{- if ... -}}` actions are dropped rather than substituted; measured 7 to 23 symbols on the 10-file demo corpus.
- image-Dockerfile keying fixed (F3): `*.dockerfile` files now key by stem instead of directory name; root-level lone Dockerfile falls back correctly.
- IsGenerated (F4) accepts Go's "Code generated ... DO NOT EDIT." marker anywhere before the package clause (bounded 4KB); compose signal and classified-but-zero visibility fixed (F5) with new `classified_zero_files` counters; docs updated (F6) to clarify `server_stats` counters are event counters since process start.

**Tool identity classifier for yaml/cue tooling (ADR-021 s2, LT-4, TI-9/10)**
- Pass-3 tool-identity classifier (`fusion.Classify`) content-classifies ambiguous yaml/cue files into a `ToolKind` (k8s, CRD, compose, helm-template-yaml, kustomize, openapi, cue-module, timoni, unclassified) using a per-ingest sibling index.
- `HelmTemplateYAMLExtractor` substitutes `{{ ... }}` action spans with placeholder scalars before YAML decode, recovering document shape from previously-failing helm charts; `KustomizeExtractor` and OpenAPI/Swagger extractor added, previously silently zero-output.
- Compose files recognized by content (top-level `services:` key) regardless of filename, not just four hardcoded names.
- CUE module/timoni awareness tags `.cue` files under `cue.mod/` ancestors, layered on the existing LT-4 CUE extractor; generated-file detection adds `Generated` flags and a `cloc` bucket.
- `fusion.ExtractorEpoch` bumped 3 to 4 forcing re-extraction; Selector-Service and cross-repo image resolution deferred (TI-9), jsonnet/.ipynb extraction deferred (TI-10).

**Pure-Go SQLite store backend replaces DuckDB (ADR-020)**
- Added `internal/store/sqlite.go`: `SQLiteStore` (`modernc.org/sqlite`) implements the full 26-method `Store` interface with fresh final-column schema, WAL pragmas verified per-connection by `TestSQLite_WALActuallyEngages`.
- `TestSQLite_ReaderDuringWriter` proves cross-process concurrency, a reader completes a `SELECT` while another connection holds an open write transaction, unlike DuckDB's cross-process lock.
- `store.BackendForPath` dispatches `.duckdb`/`.sqlite`/`.db` replacing four hardcodes; `PlatformStore` loads mixed-backend fleets; same-stem duplicates resolve to `.sqlite` with a loud warning.
- `fathom migrate-store --from X.duckdb --to Y.sqlite` ports the eight non-derivable tables read-only with per-table row count + verified content spot-hash; deferred DuckDB removal tracked as SS-8.
- CLI `--store` defaults flip from `workspace/db/fathom.duckdb` to `workspace/db/fathom.sqlite` across all commands; added dependency `modernc.org/sqlite v1.52.0`.

**Language truth: one registry, binary detection, honest CUE (ADR-021 §1)**
- Deleted `pkg/client.langFromPath`'s stale 10-language table; `internal/connector.extClassMap` is now the single ext-to-language source, parity-tested against `fusion.New`'s extractor registrations via `TestLanguageRegistryParity`.
- `cloc` reports an explicit `unclassified` bucket instead of a silent `other`; lockfile heuristics with no extractor (`go.sum`, `Gemfile.lock`, `Cargo.lock`, `requirements.txt`) now honestly report `unclassified`.
- `connector.IsBinary` sniffs for a NUL byte in the first 8000 bytes; fixes a 423K-line "other" bucket that was actually an untracked 103MB `fathom` binary.
- CUE extraction v2 recurses struct literals for nested field names, reads `Package` from the actual `package` clause, and captures imports as `core.KindCUEImport`; `fusion.ExtractorEpoch` bumped 2 to 3 triggers a one-time full re-index per service.
- Fixed root-anchored `.fathomignore` patterns (leading `/` was a silent no-op) and a CUE malformed-import crash in `literal.Unquote`.

**Build fathom without agentic-kit access (ADR-019)**
- Added `third_party/agentic-kit/`, a committed full mirror of the private kit tree at the pinned commit; `go.mod`'s `replace` points there while `go.work` still wins for local co-development.
- `task sync-kit` (`scripts/sync-kit.sh`) re-archives the sibling checkout at the pinned commit; `task vendor-check` (`scripts/vendor-check.sh`) content-hashes the mirror against a fresh archive and is now part of `task validate`.
- Air-gapped `go mod vendor` now succeeds standalone; new `docs/distribution.md` documents the build path.
- Size note: the pinned commit archives to ~13MB (1,153 files) including an accidental ~4.1MB Mach-O binary (`kit-new`), archived faithfully and flagged for the kit maintainer to prune.

### 2026-07-17

**Sextant back-compat surface removed (ADR-015)**
- Owner overrode the original 2026-07-07 "kept by design" back-compat decision and asked for all of it removed, not deprecated.
- Removed `withNameAliases` sextant/fathom service-name aliasing from `session-hook`, `edit-hook`, `stop-hook`, collapsing each hook's double lookup to a single lookup.
- Removed `internal/connector.LegacyIgnoreFileName` (`.sextantignore`) fallback; only `.fathomignore` is honoured now, covered by `TestLocalConnector_Discover_NoLegacyIgnoreFallback`.
- Behavioral consequence: stores ingested under service name "sextant" and repos relying on `.sextantignore` are no longer recognized; re-ingest or rename required.

**Layer catalog: prompt-as-data, pack shape, adr-rules + spec-bindings (LC-1..8,13, OP-6)**
- LC-1: every `#Layer` carries `prompt_file`; `promptForLayer` is generic over `LayerDef`, removing the silent obligations fallback for an unwired layer name.
- LC-2: `serving.drift_tracked` schema field replaces the `kind=="finding"` Go hardcode at all three remediation-drift sites; LC-3 unions embedded packs with a project's `.fathom/layers/`, LC-4 migrates built-ins to `layer.cue`/`prompt.md`/`eval/`/`KILL.md` pack shape.
- LC-7 adds `fathom layers list|show|install`; two new embedded packs `adr-rules` and `spec-bindings` each cleared precision 1.00 / recall 1.00 live against `claude-cli`.
- LC-8: a third-layer end-to-end proof surfaced and fixed a real drift-tracking resolution gap for project-local packs.
- Hardening: a broken project pack no longer breaks the union (warned, surfaced via `LoadError`); LC-13 (real `layers uninstall`) deferred; obligations layer re-run recorded recall straddling its bar (0.71/0.86), escalated as OP-6.
- Refs: ADR-018.

**Configuration experience: profiles, ~/.fathom/config.toml, health, smoke**
- Added `internal/config`: TOML-authored, CUE-validated runtime config merging user (`~/.fathom/config.toml`) and project (`.fathom/config.toml`) layers field-by-field with per-field `Provenance` tracking.
- `#Profile` is a closed CUE struct with no raw `api_key` field; only `api_key_env` or `api_key_cmd` (argv-exec, no shell, 10s bound) supply credentials.
- Added `fathom config [--explain]`, `fathom providers [--profile X]` (auth-health probe via `llmclient.Pinger`), and `fathom smoke --profile X|--all` (one real completion, cost estimate, requires explicit selection).
- `serve-http` now loads `[daemon]` config at startup, closing the gap where `compile_layer` was silently disabled on the launchd daemon with no env; AWS GovCloud and Azure Government ship as profiles.
- Review hardening: modelless bedrock profile fails fast, `compile --profile` without `--store` errors naming the constraint, TOML/CUE errors carry line/column and grouped keys.

**Provider matrix: openai, azure-openai, bedrock, codex-cli**
- Rounds `buildLLMClient` out to six `FATHOM_LLM_PROVIDER` lanes, adding `openai`, `azure-openai`, `bedrock` (full AWS credential-chain resolver, no static keys required), and `codex-cli`.
- `codex-cli` lane also Pings the constructed client before returning it, since undocumented config keys could silently drop system-prompt mapping and revert to the CLI's ~13k-token built-in persona.
- CLI `--store` mode no longer hardcodes the Anthropic model string into `WithLLM`; both construction sites now share `compileModel()`.
- Docs add a provider matrix table including the `codex-cli` cost line, roughly 10k input tokens of ambient overhead per call (~300k for a 31-section compile), ~60% cache-discounted steady-state.

**Subscription-backed compile provider via Claude CLI**
- `FATHOM_LLM_PROVIDER=claude-cli` shells out to a locally logged-in Claude Code CLI instead of `ANTHROPIC_API_KEY`, via new `agentic-kit` `foundations/llmclient/claudecli` package.
- `cmd/fathom/compile.go`'s single construction site renamed `buildLLMClient`, switching on the provider; `FATHOM_CLAUDE_BIN` overrides the resolved `claude` binary path (e.g. for `serve-http`'s launchd daemon).
- Reported `cost_usd` is the same API-equivalent accounting under either provider, but actual marginal spend under `claude-cli` is $0 (subscription plan limits, not a metered key).
- `cmd/fathom/compile_test.go` covers `buildLLMClient`'s switch: default/explicit anthropic without a key, claude-cli construct plus one real `Complete` through a hermetic fake-shim, unresolvable `FATHOM_CLAUDE_BIN`, unknown provider value.

**Findings-as-claims tracer bullet (D3, FT-1..FT-6)**
- Kind:"finding" `#Layer` instance added alongside `obligations`; `internal/semantic.Compile` becomes layer-aware, resolving name/kind/tier/tier_profile/prompt from `Compiler.Layer` (FT-1).
- `pkg/client.CompileLayer(ctx, service, layer, dryRun)` generalizes `CompileObligations`; fixes finding F7 where `cmd/fathom compile --layer` and the `compile_layer` MCP tool silently discarded the requested layer name and always compiled "obligations".
- `semantic.ObligationsFor` now reads claims across every compiled layer, and served `Obligation` gains a `kind` field (FT-2).
- `fathom claim refute|corroborate --store DB --service S --claim ID --note "..."` wires the previously-dead `claim_events` machinery; `refute` flips status to `ClaimRefuted`, `corroborate` is provenance-only (FT-3). `PutClaims` retains `ClaimRefuted` status across upsert of the same `claim_id` (FT-4).
- `fathom report --store DB --service S --analyzer security_rollup --out findings/` writes deterministic markdown findings, verified by double-run byte-compare, excluding its own `--out` directory from the scan (FT-5); remediation drift tracked via `ClaimLink{Rel:"scope-fp"}` fingerprints and `semantic.DriftedFindings` sets `code_drifted:true` at serve time (FT-6), tied to O8 precision work's compile guard on bare `**` scopes.

**O8 precision: serve-time trust (OP-1..OP-5)**
- Prompt v3 forbids bare `**`/repo-wide catch-all scopes, requiring file-class scoping; `PromptVersion` rotates to `obligations/c02b1aa1`, forcing full recompile under O8.
- Over-broad-scope guard in `internal/semantic/compile.go` `verifyDraft` rejects a claim whose scope glob matches more than half of `IndexedFiles`, counted in `ClaimsRejected`; single-file-class extension globs (`**/*.go`, `docs/**/*.md`) are exempted from the breadth check as a review fix.
- Edit-hook root refusal (`cmd/fathom/edithook.go` `cmdEditHook`): a file not relativizing under any resolved service-candidate root skips that candidate instead of querying the daemon with a raw out-of-repo path, fixing an O8 addendum; symlink resolution and `..`-escape checks added as review fixes.
- Spec-line injection appends `"▸ implements [ID] — defined at <file:line>"` for cited spec IDs via `spec_refs`, capped at 3, firing independently of obligations under a shared ≤150-token budget (OP-1..OP-5, OP-6 recompile).
- New `plan` project skill (`.claude/skills/plan/SKILL.md`) scaffolds gate-ready grammar-v2 plans with stable ids, `{symbol, package}` contracts, typed dispositions, and the `fathom verify` command.

**Coding-agents canonical doc and staleness sweep (IA-4/IA-5)**
- `docs/use-cases/coding-agents.md` becomes THE adoption guide covering the loop, hooks, gates, CLAUDE.md/AGENTS.md doctrine, agent-surface registration, and the honest cost story.
- `usage.md` `set_profile` guidance retired; June `claude-code-integration.md` banner-superseded (assessment content kept); `getting-started`/`coding-context` scoped and pointed at the canonical doc.

**`fathom init`: one-command coding-agent wiring (IA-1/IA-2/IA-3)**
- `fathom init` wires a repo end-to-end, idempotently and additive-only: creates `.fathomignore` if absent, calls the daemon's `ingest` MCP tool and confirms the service is indexed, fail-soft exit 2 when the daemon is unreachable.
- JSON-merges project-level `.claude/settings.json` hooks (SessionStart `session-hook`, PostToolUse `edit-hook`, Stop `stop-hook`), writes the fathom operating-contract stanza into CLAUDE.md and AGENTS.md between `<!-- fathom:begin/end -->` markers.
- Every mutation is create-if-absent or marker-replace, so a second run changes zero bytes and reports "already present".
- `mergeHookSettings` preserves every unrelated top-level key, hook event, and sibling hook entry; detection is by command substring so re-running never duplicates an entry.

**MCP surfaces week-1: execute-and-flag, agent surface (amends ADR-014)**
- Gate semantics change from hard-gate to execute-and-flag: an off-surface `tools/call` or `batch` sub-call now executes instead of erroring, logged with `off_surface:true` in the JSONL call log; old hard gate stays behind `Server.strictSurfaces`.
- The `companion` allowlist becomes the 15-tool **agent** surface (minus `batch`/`brief`, plus `verify_plan`/`capabilities`), served at `POST /rpc/agent` with `POST /rpc/companion` kept as a compatible alias.
- `set_profile` removed entirely, amending ADR-010/014's stdio clause, since stdio now always serves the full registry.
- Four zero-call alias tools deleted (`platform_callers`, `test_debt`, `api_routes`, `deps`) in favor of canonical `cross_repo_callers`, `markers`, `routes`, `dependencies`; `serve-http`'s default listen address changes from `:7575` to `127.0.0.1:7575` (loopback).

**Gap-1 channel instrumentation: denominator, liveness, origin**
- Hooks beat a central log (`FATHOM_HOOKS_LOG`) via agentic-kit v0.7.0 telemetry/heartbeat: edit-hook seen/answered/error, session-hook fired, giving injection coverage a denominator.
- `fathom instrument` computes coverage (answered/seen vs the ≥90% gate), per-day rollup, and liveness alarms (sessions without edits in 7d; edits unanswered in 1d); advisory, exit 0 always.
- `X-Fathom-Origin` header lets every hook/CLI HTTP call declare its channel, recorded in the daemon call log as `origin` to separate plumbing from agent choice.
- Grammar hardening (GI-7): the plan-annotation parser takes the trailing brace block, so prose braces and code literals in titles no longer hijack contracts.

**M1: the manifest goes live (semantic layers, ADR-016 amendment)**
- `schema/semantic/schema.cue` becomes a runtime artifact: `internal/semantic.LoadLayer` embeds a byte-identity-tested copy and decodes `#Layer` instances via `cuelang.org/go`, retiring the hand-synced `ObligationsInclude` var and `maxRuleWords` const.
- Content-addressed prompt identity via agentic-kit v0.7.0 `prompttemplate`: `PromptVersion` derives as `obligations/<hash8>` from `Template.TextHash()`, so any prompt edit is automatically a new compile identity, preventing the bug class that left 27/32 sections stale on v1 claims after the v1→v2 bump.
- Typed compile failures via agentic-kit `llmclient/structured`, classifying truncation/malformed output as `structured.ErrTruncated`/`structured.ErrMalformed`, retiring the hand-rolled `errBadResponse` classifier.
- `enforceBudgets` (ADR-016 §4.1) refuses to start a compile whose pre-flight cost estimate exceeds `max_usd_per_run`, and stops mid-corpus when the running token total crosses `max_tokens_per_run`, recorded on the `compile_runs` ledger.
- Adversarial-review fixes: (F1) structured-outputs schema made object-rooted (`{"claims":[...]}`) for Anthropic compatibility; (F2) superseded/refuted claims no longer feed the digest gate; (F5) budget-stopped runs surface `budget_stopped`/`refusal_reason` at `compile_layer` and CLI printers. Spec `docs/plans/m1-manifest.md` (M1-1..M1-8, all landed); `#Layer.tier` widens to admit `verified` (ADR-018).
- Refs: O8.

**Extractor-epoch fingerprint salt fixes stale observations (O9)**
- The pass-3 hash-gate keyed fingerprints on file content only, so a binary with improved extraction (e.g. H1's aliased-selector canonicalization and method-edge forms) silently kept unchanged files' old observations, breaking `test_reaches` on genuinely-tested symbols (dogfood O9).
- Fingerprints now salted with `fusion.ExtractorEpoch`, bumped on any extraction-semantics change to force full re-extract on next ingest.
- `ObligationsIndexStale` adopts the shared `fusion.FingerprintHash` so the formulas cannot diverge; regression covered by `TestIngest_ExtractorEpochInvalidatesOldFingerprints`.

**SDD reconciler v0: plan grammar v2, wave gate, CUE shadow (ADR-018)**
- Plan grammar v2 (`internal/verify`, additive): `[~]` in-progress checkbox, author-owned stable `id:` keys, typed dispositions on `[-]` (`disposition: refused|deferred` + `reason:` + `trigger:`), `skip:` justification for `skip_test`.
- Wave gate `fathom verify --require-all` (`verify.GateRequireAll`) fails on open/in-progress tasks so a wave advances only when every task is claimed-and-verified or explicitly skipped.
- `fathom gen-plan-cue` emits a generated-only `#Plan` document and proves lossless round-trip against the Markdown parse; hand-authored plan CUE is deprecated.
- `fathom stop-hook` checks tasks newly claimed `[x]` this session against verification at session end; advisory by default, `--block` refuses session end with the failing citation.

**Obligations v2: first-fleet-compile hardening (ADR-016)**
- Prompt v1 to v2 and corpus narrowed to binding docs only (`CLAUDE.md`, `ADR.md`, `docs/plans/README.md`), after the first fleet compile turned `docs/design/**` essays into noisy `**`-scoped aspirational rules.
- Digest gate now includes `PromptVersion` after a prompt bump previously failed to recompile 27/32 sections still on v1 claims; covered by `TestCompile_PromptBumpDefeatsDigestGate`.
- Per-section failure resilience: a malformed/truncated model response now fails only its own section (tracked in `compile_runs.sections_failed`) instead of aborting an 88-section fleet corpus; `MaxTokens` raised 1200 to 2400.
- Claims whose subject left the corpus selector are now superseded on next compile rather than served forever.
- First full fleet compile under v2: 31/32 sections, 72 claims (9 rejected by compile-time verification), $0.047.

**Semantic layers v1: obligations tracer bullet (ADR-016)**
- Governed-claims substrate adds four additive tables: `semantic_claims` (content-addressed, tiered, status-tracked), `claim_links` (cites/scopes), `compile_runs` (token/USD ledger), append-only `claim_events` refute channel.
- Obligations compiler pipeline: governing docs to markdown sections, digest gate (unchanged sections cost $0), DLP redaction, injected LLM (strict JSON), compile-time anchor and scope verification, transactional store with supersede; rules word-budgeted at ≤28 words so a 6-rule payload stays ≤150 tokens.
- Citation sweep (post-ingest, token-free) re-hashes cited lines: drift flips status to `stale`, unresolvable citations to `contradicted`, restored docs heal to `fresh`; wired into `IngestService`.
- `obligations` MCP tool on the 15-tool companion surface serves rules ordered by scope specificity, capped at 6, with zero-output-on-clean; `compile_layer` tool and `fathom compile` CLI added, `dry_run` defaults true.
- PostToolUse injection via `fathom edit-hook` delivers ≤150-token obligations after every Edit/Write, silent on clean or error; precision eval harness gates the layer at precision ≥0.90 / recall ≥0.75, opt-in via `FATHOM_EVAL_LLM=1`.

**Fleet onboarding ergonomics (O7)**
- `ingest` tool onboards new services via optional `path` (absolute corpus root), ingesting a repo the index has never seen through the live daemon, no single-writer lock dance.
- Session-hook's "add this repo" guidance now names the exact call; capability forwarded through the auto-refresh wrapper (the O7 lesson) with a through-the-wrapper test.
- Subdirectory-rooted projects resolve their service: both hooks try the project dir's basename, then the enclosing git repo's basename, so `<repo>/deploy` no longer resolves to service "deploy".
- Edit-hook resolves file paths against whichever root matched; `reingest-fleet.sh` gains nested-path support plus three fleet repos (growing-gardens-bi, car-assistant, crossplane-platform).

**Done-means-proven verify gate (H1.2, O6)**
- Checkbox semantics: `[x]` in a plan is a claim; `verify.Gate` fails any claimed task whose contract fails (`--strict` also gates Unknown); `[ ]` open and `[-]` skipped never gate.
- `fathom verify` CLI is HTTP-first against the daemon, `--store` for CI; gate always computed locally so a stale daemon can't launder a claim. Exit 0/1/2, fail-open pre-push, fail-closed strict CI.
- Wired into lefthook pre-push and `scripts/ci.sh` here and in agentic-kit; first gated run caught 3 over-claims by the plan's own author (recall gap filed as dogfood O6).
- Gate honesty fix: an empty index now reads "cannot verify", never "all claims false" -- `fathom verify` probes the index first (`list_services` or symbol count) and exits 2 with a fix-it message when the service isn't indexed.

**Born-portable stores (ADR-017)**
- Repo-relative anchors from every connector, including LocalConnector; absolute refs from legacy stores still read.
- `fathom remap` repoints a service root in one metadata update.
- Watermark SPI (git HEAD or content-addressed `manifest:` hash) makes freshness domain-neutral, so non-git corpora (docs folders, findings exports) auto-refresh and sweep honestly.
- Ingest self-heal purges pre-ADR-017 absolute-anchored content wholesale so no symbol duplicates across anchor styles; contract test `TestStorePortability_RoundTrip` covers move corpus + remap.

**Adoption metrics fixed after rename**
- `fathom-adoption.sh` measured nothing for this repo after the rename because transcript dirs are path-keyed; fix adds name-era aliasing, per-ISO-week rollup, and a daemon call-log summary section.
- Measured baseline: 0% fathom share (0 vs 193 grep calls), the number the forced channels exist to move.

**Rename sextant to fathom (ADR-015)**
- Module path becomes `github.com/geoffmilleraz/fathom`, binary `cmd/fathom` moves to `bin/fathom`, MCP serverInfo/resource URIs/metrics prefixes, docs and scripts all renamed.
- Back-compat kept where the old name is data in the field: `.sextantignore` still honoured as legacy fallback behind `.fathomignore`, adoption metrics count both `mcp__sextant__`/`mcp__fathom__` prefixes, session-hook treats the two service/dir names as one identity, `reingest-fleet.sh` finds the repo under either directory name.
- Historical records (changelog, USAGE_JOURNAL.md, dogfood-observations.md) intentionally keep "sextant"; operator steps in `docs/rename-migration.md`.
- Motivated by fleet usage mining: sextant works when called but is dramatically under-used, agents reach for shell grep 10x+ more even where the index exists, because "index first" lived only in passive docs.

**Path-scoped tool surfaces on shared daemon (ADR-014)**
- `POST /rpc/{surface}` serves an immutable per-request view of the registry: `companion` is a curated 14-tool coding-agent hot loop; `coding`/`investigation`/`shared`/`platform` mirror the group profiles; bare `/rpc` remains full. Scoped endpoints cut tools/list schema payload ~80% for non-deferring clients (motivated by review of 86 tools ≈ 8.6k schema tokens).
- Surfaces hard-gate `tools/call`, closing the profile-bypass hole where a tool hidden from `tools/list` was still callable, including `batch` sub-calls; off-surface calls now error naming the surface and pointing at `POST /rpc`. Canned prompts scoped the same way.
- `set_profile` rejected over HTTP since the profile is process-global on the shared daemon; stdio keeps `set_profile` and gains `companion` as a valid value. `capabilities` and the manifest resource report `active_surface` alongside `active_profile`.
- Tool-call telemetry wired into the daemon via `serve-http --call-log` (default `workspace/logs/mcp-calls.jsonl`), recording request `surface`, individually logged `batch` sub-calls (`via:"batch"`), and atomic `O_APPEND` writes so a daemon kill can't merge lines.
- `session-hook` (Claude Code SessionStart) pings the daemon, maps project dir to service, and injects an active "resolve via sextant BEFORE Grep/Read" reminder, or an honest signal when the daemon is down or repo isn't indexed.

**Query staleness compaction, find_symbol fallback, schema slim**
- Query-side staleness is constant-size now (USAGE_JOURNAL.md ask #4): `staleness` block carries at most 3 example rows plus `stale_sources`, `total_sources`, `fresh_sources`, and a new `oldest_ingested_at` low-water mark; full list stays in `staleness_report`. Regression: `TestQuery_StalenessReport_CompactWhenManyStale`.
- `find_symbol` no longer punishes not knowing the exact identifier (USAGE_JOURNAL.md ask #3): matching degrades exact/key-suffix -> case-insensitive -> substring (capped at 20, `truncated` flag), with up to 5 `did_you_mean` suggestions on a full miss; response reports `matched_via`. Regressions: `internal/mcp/findsymbol_test.go`.
- `verify_plan`'s tool schema slimmed ~27% (~150 tokens), from ~554 tokens (~6% of the whole registry); `min_tier`'s valid values moved from prose into a JSON-schema `enum`. Behavior unchanged; full verdict reference in `docs/mcp-tools.md`.

## [0.2.0] — 2026-06-16

Enhancements driven by mining throughline's real Claude Code sessions (841 sextant calls,
1666 shell greps, a heavy manual freshness loop).

### Added
- **`references` MCP tool** — evidence-tiered, fleet-wide occurrence search (substring /
  whole-word / regex). Returns text matches at the `pre-scan` tier plus symbol definitions
  cross-referenced from the index at their real tier. Turns the ~658 repo-by-repo greps a
  session does into one structured call.
- **`verify_plan` accepts a Markdown checklist** (`plan_markdown`), and `gen-status-cue`
  accepts a `.md` plan file: each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a
  verifiable task. A `/verify` slash command runs the loop — surfacing `[x]`-but-not-verified
  items before "done" is trusted. (throughline's sessions were hand-rolling this.)

### Changed
- **`serve-mcp --auto-refresh` (default on)** — a query for a service whose git HEAD has
  drifted from the indexed commit transparently re-ingests (hash-gated, debounced per
  commit) before answering. Removes the manual staleness→ingest loop (49 ingests / 19
  staleness checks per 3 sessions). A second `serve-mcp` on the same store now refuses with
  actionable guidance instead of DuckDB's raw single-writer lock error.

## [0.1.0] — 2026-06-15

First tagged milestone: a multi-language code-intelligence + evidence engine with an MCP
server, a language-agnostic cross-repo graph, and an evidence-backed verification layer.

### Added
- **Multi-language extraction (~25 languages/tools).** Tree-sitter via CGO (ADR-013) for
  general languages — Python, JS/TS, Java, Ruby, Rust, C#, C, C++, Scala, PHP, Lua — and
  pure-Go parsers for config/IaC/tooling: Terraform/HCL, Kubernetes, Helm, Dockerfile,
  docker-compose, GitHub Actions, Protocol Buffers, GraphQL, CUE, Markdown, SQL, TOML,
  JSON. 93 `SourceKind`s. Interim intra-file call edges for the tree-sitter languages.
  Calibration corpus + harness.
- **Language-agnostic cross-repo blast radius.** `internal/platformlink` builds the
  cross-repo graph from stored multi-language observations instead of a Go-only filesystem
  walk; `pkg/platform.BuildFromStore`; the `platform-callers` CLI defaults to the store
  path (`--repos` is a labelled legacy Go-only mode). Cross-repo edges now resolve for
  every language whose extractor populates call edges.
- **Evidence-backed definition-of-done.** `internal/verify` engine + the `verify_plan` MCP
  tool: a per-task verdict computed from code evidence (the claimed symbol exists at a
  sufficient tier ∧ a test statically reaches it ∧ a required call-edge is wired ∧ no
  contradiction ∧ declared specs are covered), each with cited evidence. The
  `gen-status-cue` CLI renders a deterministic, one-directional `STATUS.cue` from a
  human-owned plan file (`schema/plan` CUE schema). **Static by construction**:
  `runtime_pass` is always `unknown` — a static check never claims a test passes.
- **Use-case adoption docs** under `docs/use-cases/` — a getting-started guide plus six
  how-tos (coding-context, cross-repo blast radius, spec verification, incident
  investigation, change review, fleet health) for wiring sextant into Claude Code, with a
  one-doc-per-use-case maintenance convention.
- **Build-identity / staleness signal.** `sextant version` reports the embedded VCS
  revision, commit time, and dirty flag (Go `-buildvcs`, no ldflags); `serve-mcp` logs it
  at startup, so a long-running server that has outlived commits is self-evident — compare
  against `git rev-parse HEAD`. (O3.)
- Earlier foundation in this release: typed edge-graph `traverse` + commit `history`
  ingestion; `spec_refs` and `brief` tools; git-HEAD auto-staleness; line numbers carried
  through to MCP results; `.sextantignore` repo-specific excludes; the Claude Code
  integration playbook and assessment.

### Fixed
- **`SymbolNode.Package` persists through the store.** It was silently dropped in the
  `SymbolNode → Observation` mapping, degrading `SymbolKey` reconstruction to `"svc::.Name"`
  and breaking precise cross-repo / call-graph matching off stored evidence. Added a
  `package` column with an idempotent migration.
- **The call graph resolves cross-package selector calls.** A callee recorded as
  `"pkg.Name"` (e.g. an external `*_test` package calling the symbol under test) never
  matched the node key `"svc::pkg.Name"`, so `test_impact` / `find_callers` /
  `rename_impact` under-counted in the common Go external-test-package pattern. Fixed with
  a precise `package.Name` index (a receiver-method call is never mistaken for a package
  selector).
- **`outline` honors its repo-relative contract.** `symbolnav.Outline` matched the stored
  absolute `FilePath` against the `file` arg by exact string, so a repo-relative path —
  the documented contract — returned `{"symbols":[]}`. Now resolves a relative query by
  path-segment suffix (absolute still exact). Found via fleet dogfooding (throughline).
  (O1.)

### Changed
- **The `post-commit` hook rebuilds `bin/sextant` on Go changes** before re-indexing, so
  the on-disk binary tracks HEAD. A running `serve-mcp` still needs a restart to load a new
  binary. (O3.)
