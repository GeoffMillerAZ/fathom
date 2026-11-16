# WR-1/2/4/5/12 existing admission, stamp and firing contract

Status: Draft for independent exact-commit review under W10-1. Freeze only.
Source base: `83226aece9b0f421381373f8dafc4a912d3888e5`. Owns this document only. One plan:
`docs/plans/wiring-receipts.md`, WR-1, WR-2, WR-4, WR-5, WR-12, all still unchecked.
No new production interface, test, code edit, package, active reservation, plan
claim, paid run or gate result. No approval is issued by this author.

## Authority and exact missing-port witnesses

W10-1 settles existing behavior, including its limitations. W2-3 leaves approval
with the foreman after an independent exact-commit LAND. W3-2 requires implementing
lanes' real RED, not a named test invented here. W3-4 does not migrate existing
maps, ordinary string errors or nil outputs. Intent: I9/I12 fail-open hooks, I38
read honesty, I61 retained-data boundary; ADR-029's harness seam and LAB-7's explicit
separate bench readers remain. No intent condition is measured or amended here.
The proof/receipt study in `docs/plans/wiring-receipts.md:7-26` remains a plan,
not evidence that current code provides every planned seam.

| Row | What exists at the base | What does not exist, with exact witness |
|---|---|---|
| WR-1 (`wiring-receipts.md:29`) | contextbench ArmResultFromClaudeJSON -> Score/ScoreWithPrecision; scenariobench's private instrumentTruthError/hooksInstrumentError/primerInstrumentError | ReceiptDefect has no Go declaration or caller anywhere in cmd/internal/schema/pkg. report.go:204-239 decodes the final JSON then scores it without any stream receipt input. ArmResult has no instrument_defect field. |
| WR-2 (`:30`) | WR-3's generated healthy/fault fixture corpus and independent truth labels | VoidWithoutPositiveControl has no Go declaration/caller in those roots. No contextbench broken-control admission step exists; fixtures alone do not enforce pair admission. |
| WR-4 (`:36`) | cmdSessionHook writes sessionWindowRow and sessionStamp, optionally reduced identityResult | computeWireStamp has no Go declaration/caller. sessionWindowRow (`discoveries.go:42`) has only schema_version/session_id/cwd/service/started; sessionStamp (`lifecyclehooks.go:30`) has no wire/idx. No hash of version/ExtractorEpoch/settings/surface is computed there. |
| WR-5 (`:37`) | stampBeatMeta (`beats.go:114`) copies metadata and adds v2 version/project/harness/hook_inv stamps | It never reads a session stamp, adds a wire hash, or adds a session id by itself. firingMeta optionally supplies session; emitBeat's nil SessionStart fired metadata has none. Caller-supplied wire/idx keys are merely unvalidated strings, not computed evidence. |
| WR-12 (`:52`) | hooks_firing and rule_outcome_writer are separate doctor rows (`doctor.go:296,305`); generated WR-3 corpus | checkWiringFiring has no declaration/caller. runDoctorChecks has no wiring_firing row and no per-session join/reason-code output across skip/beat/call/outcome inputs. Existing rows cannot be relabelled as WR-12. |

These are source gaps for the foreman, not permission to invent signatures. A
consumer can use the existing seams below where accessible. A consumer requiring
ReceiptDefect/VoidWithoutPositiveControl/computeWireStamp/checkWiringFiring is
blocked until its owning implementation and exact source handback exist. Private
cmd/fathom or scenariobench helpers cannot be imported by a new leaf package.
W10-1 authorizes this truthful freeze; it does not complete these five plan rows.

## Existing bench parse, validation and scoring order

WR-1/C1: `cmd/contextbenchscore/main.go:192-221` reads final result bytes then calls
ArmResultFromClaudeJSON or its precision variant. `internal/contextbench/report.go:192-239`
uses json.Unmarshal into claudeEnvelope (unknown fields ignored), computes four
usage counters' sum, copies final result/error/cost/timing, resolves primary model
by greatest output token count (lexicographic tie), then scores. Malformed/type-
wrong JSON returns zero ArmResult and a wrapping ordinary fmt error naming task
and arm. Valid empty object or JSON null is not refused: it scores empty output,
retains zeros and caller arm/task. Success is scorer pass AND !env.IsError.
Even matching output with is_error=true remains a scored failure, with token/cost
accounting retained. No MCP/tool/action/positive-control or preflight field is
consulted. RawResult is retained in ArmResult: this is NOT the allowlisted shared
ScoredCell proposed in CTR-13. Nil slices and omitempty behavior stay as declared.

WR-1/C2: `internal/contextbench/scorer.go:34-102` Score is pure, no context or I/O.
GroundTruth and AnswerAnyOf tokens are case-sensitive, empty tokens never match;
if a token ends in a digit, a following digit prevents a match. AnyOf needs one
matched token/group; ExactMatch and AllOf need no misses AND at least one match;
unknown ScoringRule returns Pass=false. One alternative per group, first matching
in declared order, contributes; matched/missing lists sort. No receipt can alter
this scorer today. Precision is its existing ScoreWithPrecision adapter, not a
new scoring engine (`precision.go:53`); the normal adapter leaves Scoring empty.

WR-1/C3: `internal/contextbench/cell.go:70-117` CellKey hashes labeled key=value newline fields
with SHA-256 to lowercase hex, in declaration-independent fixed order (corpus,
task, arm, model, harness, scoring, prompt subject, arm surface). It includes
ArmSurface for EVERY arm and neither defaults empty Scoring nor escapes embedded
newlines. Caller policy supplies a blank surface for an unaffected baseline;
CellKey itself does not enforce that convention. It does NOT call Validate.
Validate's ordered required fields: corpus_sha256, task_id, arm, model_id,
harness_version, scoring, prompt_subject. It requires neither arm surface nor a
receipt, and an empty Scoring still hashes literally but fails Validate. Ordinary fmt errors,
not typed refusals. Borrow acceptance (`:196`) bypasses nonborrowed results, rejects borrowed results
with empty CellKey or unequal wanted key; it is not proof of wiring. WR does not change CellKey, criterion pins or old saved cell semantics.

WR-1/C4: the scenariobench precedent is real but private, post-session, and has
its own transcript format (`internal/scenariobench/stream.go:179-225,299-351`).
ParseSessionStream does os.ReadFile (wrapping error with path); parseSessionStreamBytes
uses a 16 MiB scanner, ignores blank/nonobject/malformed lines and currently
ignores sc.Err. It reports no LAB-7 coverage counters. Init events accumulate
server statuses, MCPTools counts the latest init's mcp__ tools; nil MCPServers
means absent init and empty map means explicit empty list (omitempty serializes
both absent). Hook responses count nonempty HookEvent even when the hook errored;
HooksFiredByName retains optional matcher detail. SessionStartContext is set while
empty, so a later nonempty response may replace an earlier empty one. No stream
migration to internal/transcript is authorized by this contract.

WR-1/C5: `sessions.go:635-685` runOneSession calls h.RunSession first. A Go harness
error returns immediately. Only if res.Error is empty does it evaluate, in order:

1. instrumentTruthError: wantServers order, first missing -> `instrument: mcp server <name> absent`, first non-connected status -> `instrument: mcp server <name> <status>`; empty requirements pass regardless of map.
2. hooksInstrumentError: unwired bypass; wired needs SessionStart > 0, else `instrument: no SessionStart hook fired`.
3. primerInstrumentError: unwired or SessionStart==0 bypass; substring `not an indexed service` refuses, otherwise missing `is the indexed service` refuses. This is a literal text heuristic, not matching an expected service id.
4. postToolUseInstrumentError: unwired or no FilesEdited bypass; otherwise needs PostToolUse > 0. It does not require obligations_injected or prove per-name matching.

First nonempty string is set to BOTH Error and InstrumentError. Existing res.Error
bypasses these four instrument checks. Leak/escape/baseline scans still run after
the instrument block; none is upgraded into a new receipt field. RescoreSessionsFromRaw
uses the same helper ordering on its own path (`rescore.go:340-364`).

WR-1/C6: `sessions.go:792-837` CellLost is pure, returns ordinary string, no typed
disposition: empty sessions -> no sessions ran; then whole-slice priority passes
for any answer-key leak, any sandbox escape, any baseline ran fathom, any
InstrumentError; then every-session Error; then no turns. Exactly one optional
Harness implementing ExpectsNoTurns() bool can permit zero turns, after earlier
failures. Otherwise empty string means not lost, not oracle success. A zero-use
arm with actual turns/outcome is not rejected merely for no product calls.
This historical rule is distinct from WPF's before-agent pair admission, and no
code here converts it to a new primary scoring definition.

## Session windows, stamps and beat order

WR-4/C1: `cmd/fathom/sessionhook.go:40-237` channel kill exits before any work or
record; event branch goes to cmdSessionHookEvent before the normal fired count.
Normal path sets rpcOrigin and noteBeatHarness BEFORE emitBeat(session-hook,fired,nil),
which happens before flag validation, stdin session decoding and guard admission.
Unknown harness is fail-open; stdin read is bounded at 64 KiB; absent/bad stdin
means no session id. resetHookDedupe precedes beginHookGuard; a silent guard exits
before window/stamp writes. After sessionContext, spent deadline suppresses extra
identity/brief work. With session id, optional indexedServiceFor lookup (2 s
client) precedes recordSessionWindow with wall-clock RFC3339 start. That window
is written before primer/attention composition and writeSessionStamp. Stamp is
schema 1, source, post-cut brief counts, attention flag, wall-clock UTC At, Cwd,
optional identity. Existing code constructs reduced identity at :142 (Schema,
Service, Symbols, CountErr, Langs, Path), not FreshnessNote/IndexedCommit. Payload
cost beat follows stamp; output encode can fail silently after records exist.
A stamp therefore proves a write attempt, not successful output delivery or
completion of preflight. There is no global transaction or once-only wire hash.

WR-4/C2: `discoveries.go:56-139` FATHOM_SESSIONS_LOG override, else root git top /
statedir logs/sessions.jsonl. Empty session id is no-op; mkdir 0755, open append
0644, cross-process exclusive flock, JSON newline; every failure and close/unlock
error swallowed. Repeated starts append repeated rows. readSessionWindows reads
whole file, missing/unreadable -> nil, bad rows skipped without coverage count.
sessionStartFor returns last parsable RFC3339 row matching id (not maximum time),
zero time/false if none. Does not validate schema, service, root, uniqueness or
prove stamp existence. Nil rows stay nil, never synthesized as healthy.

WR-4/C3: `lifecyclehooks.go:47-83` sessionStampDir resolves FATHOM_SESSION_STAMPS
or per-user session-stamps. Empty resolved dir/id -> silent no-op; mkdir 0700,
json.Marshal, os.WriteFile path SessionID+.json at 0600 (overwrite, not atomic
rename and not flocked), errors swallowed. No session-id path validation or
schema/source enum validation is added here. readSessionStamp empty id, absent,
unreadable or malformed JSON -> zero value, no Go error. Unknown fields ignored;
valid unknown-schema or wrong embedded id is not refused. Freshness fields exist
on identityResult but no wire/idx is derived or required by stamp helpers.

WR-5/C1: `beats.go:114-144` stampBeatMeta returns a fresh nonnil string map.
Default schema_version is exactly "2", fathom_version is buildinfo.Version();
project_dir only if Getwd succeeds, harness only if beatHarness nonempty,
hook_inv only if currentHookInvocation nonempty. Caller entries overwrite EVERY
default, including empty strings. Nil input works and is not mutated. No I/O
context or error is exposed (Getwd failure simply omits project_dir). No implicit
session join: `firingMeta(meta,sessionID,file)` mutates/returns the input map
(allocates if nil), sets only nonempty session/file, does not clear prior keys.
firingMetaArm adds arm=off only when true, also does not clear preexisting arm.
These maps have no typed enum/refusal contract. Existing unversioned rows remain
version-unknown, never backfilled (`beats.go:140-143`). No wire-version is inferred.

WR-5/C2: `beats.go:40-60` emitBeat calls noteAdviceSkip, resolves central log via
FATHOM_HOOKS_LOG or per-user hooks.jsonl, mkdir 0755, best-effort rotate, open
append/create/write 0644, heartbeat.NewLog(time.Now), Beat(stampBeatMeta(meta)),
ignores every error. 64 MiB rotation threshold, one prior .1 only, so long history
is lost. heartbeat.Beat/Load types and JSON remain the vendored kit's existing
ones (`heartbeat.go:49-130`): channel cannot be empty, NewLog requires nonnil
writer/clock, timestamp UTC, mutex per Log, not shared across hook processes.
Load acceptPartial drops invalid JSON rows, scanner limit 1 MiB still returns
scan error. No cross-process write transaction or immutable stamp is promised.

## Firing outputs and honest limits

WR-12/C1: `doctor.go:1543-1611` checkHooksFiring returns doctorCheck, NOT a per-session
receipt. For Claude it resolves call-log candidates locally first, asks daemon
only when needed/daemonUp. No candidates -> skipped. First readable log with no
origin edit-hook -> fail (does not continue to a later readable positive log);
any edit-hook -> pass with edit-hook/verify-cli counts. No readable log -> fail.
No recent window or session-id filter, skip-beat join, stamp hash, exact product
commit comparison or requirement to have invoked a tool is present in this row.
Configuration alone is not firing. These limitations stay explicit.

WR-12/C2: non-Claude requires descriptorHooksVerifiable (known registry descriptor,
HooksWired, !Unverified); otherwise skipped. checkHarnessHooksFiring uses
loadHarnessHookEvidence on current central beat log, not .1, matching only
Meta[harness] to descriptor ID, ANY beat kind/channel and all retained time/repos.
Present unreadable log -> skipped, missing/zero matching -> fail, positive -> pass
with sorted channels/counts/newest. heartbeat.Load accepts partial invalid rows.
It is harness-wide historical evidence, not session/repo conformance.
`doctor_harness_firing.go:47-104,182-205` defines these exact limits.

WR-12/C3: `doctor_outcomewriter.go:105-191` checkRuleOutcomeWriterFromFiles is a
separate rule_outcome_writer row. Active hook guard -> skipped (no 32 MiB beat
read). Outside hook: read newest 1 MiB ledger tail within 72 h; any in-window row
short-circuits to pass; unreadable ledger -> skipped. Otherwise reads bounded
beat log current/.1 (32 MiB total) and checks edit-hook/answered, at/after window,
emitted=true, doubt!=true, phase post/post-bash. Positive integer claims -> fail
if no ledger row; missing claims key -> warn/unknown; no qualifying positive or
missing claims -> pass as nothing to write. Partial span named if available.
This does not pair a specific claim or session to its ledger outcome. The
readBeatsSince helper (`doctor_retention.go:208`) suppresses stat/tail read errors
and can return nil/zero coverage: do not invent a stronger completeness receipt.

WR-12/C4: doctorStatus is named string with pass, unknown, timeout, fail, skipped,
warn; doctorCheck JSON is name/status/reason/remedy, no typed reason_code.
doctorExitCode (`doctor.go:503`) fails only confirmed fail, so exit 0 does not
prove every check. WR-12's future new reason codes and unknown logic are not
silently added to this shape or parsed from free Reason.

WR-12/C5: supporting readers remain their own APIs. readCallLog delegates to
classifyCallLogRead and exposes read=true only with nil read error; malformed-row
count stays separate (`instrument_adoption.go:450`). readBeatLog/readBeatLogTail
(`research_report.go:338,364`) return nil,false on empty path/open/stat/seek/load
error, accept partial invalid JSON rows, and do not expose error classes. Tail
reads last maxBytes, including a partial first row dropped by heartbeat.Load.
Existing WR-9 era consumers (`instrument_adoption.go:113`) group filtered call
rows by exact (FathomVersion,SchemaVersion), never beat string stamps or wire;
missing version becomes unknown (predates version stamps when schema=0), no
backfill. They preserve counts and deterministic timestamps/order, not evidence
of session conformance. Stamp schema is a string in beat Meta but int in call
entries/session rows. No cross-surface coercion is added.

## Exact interface and type snapshots

Declarations below preserve names, receiver syntax, types and JSON tags.
They are existing code, not proposed replacements. Types referenced outside
these snippets remain the types in their cited source package (e.g. Arm, Worktree,
BoxCheck, identityResult and harness.Descriptor), never local copies in a new
consumer. No context is added to pure operations; the private doctor helpers
retain their http.Client/deadline plumbing and existing I/O signatures.

`internal/contextbench/report.go`:

```go
func (e claudeEnvelope) resolvedModelID() (primary string, aux []string)
func PrimaryModelID(outputTokens map[string]int64) (primary string, aux []string)
func ArmResultFromClaudeJSON(task Task, arm string, raw []byte) (ArmResult, error)
func ArmResultFromClaudeJSONWithPrecision(task Task, arm string, raw []byte) (ArmResult, error)
func armResultFromClaudeJSON(task Task, arm string, raw []byte, scoring string) (ArmResult, error)
```

`internal/contextbench/scorer.go`:

```go
func Score(task Task, output string) ScoreResult
func tokenPresent(output, want string) bool
func endsInDigit(s string) bool
```

`internal/contextbench/cell.go`:

```go
func (in CellInputs) canonical() string
func CellKey(in CellInputs) string
func (in CellInputs) Validate() error
func AcceptBorrowed(want string, r ArmResult) error
func RefuseMismatchedBorrows(results []ArmResult, base CellInputs, surfaces map[string]string) (kept []ArmResult, refused []string)
```

`internal/scenariobench/stream.go`:

```go
func ParseSessionStream(path string) (SessionResult, error)
func parseSessionStreamBytes(transcript []byte) SessionResult
func (ps *parsedStream) handleInit(ev streamEvent)
func (ps *parsedStream) handleHookResponse(ev streamEvent)
```

`internal/scenariobench/sessions.go`:

```go
func runOneSession(h Harness, req SessionRequest, wantServers []string, excludePaths []string, cellDir string, subjectsDir string) (SessionResult, error)
func postToolUseInstrumentError(wired bool, res SessionResult) string
func primerInstrumentError(wired bool, res SessionResult) string
func instrumentTruthError(wantServers []string, res SessionResult) string
func hooksInstrumentError(wired bool, res SessionResult) string
func CellLost(sessions []SessionResult, harness ...Harness) string
```

`cmd/fathom/beats.go`:

```go
func beatLogPath() string
func emitBeat(channel, kind string, meta map[string]string)
func firingMeta(meta map[string]string, sessionID, file string) map[string]string
func firingMetaArm(meta map[string]string, sessionID, file string, armOff bool) map[string]string
func stampBeatMeta(meta map[string]string) map[string]string
func rotateBeatLogIfNeeded(path string)
```

`cmd/fathom/lifecyclehooks.go`:

```go
func sessionStampDir() string
func writeSessionStamp(st sessionStamp)
func readSessionStamp(sessionID string) sessionStamp
```

`cmd/fathom/discoveries.go`:

```go
func sessionsLogPathFor(root string) string
func recordSessionWindow(root string, row sessionWindowRow)
func readSessionWindows(root string) []sessionWindowRow
func sessionStartFor(root, sessionID string) (time.Time, bool)
```

`cmd/fathom/sessionhook.go`:

```go
func cmdSessionHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int
```

`cmd/fathom/doctor.go`:

```go
func runDoctorChecks(root, explicitSvc, harness, baseURL string) []doctorCheck
func doctorExitCode(checks []doctorCheck) int
func resolveCallLogCandidates(hc *http.Client, baseURL string, daemonUp bool, root string) []string
func checkHooksFiring(hc *http.Client, baseURL string, daemonUp bool, root, harness string) doctorCheck
```

`cmd/fathom/doctor_harness_firing.go`:

```go
func loadHarnessHookEvidence(id string) harnessHookEvidence
func (e harnessHookEvidence) sortedChannels() string
func descriptorHooksVerifiable(harnessLabel string) (harness.Descriptor, bool)
func checkHarnessHooksFiring(d harness.Descriptor) doctorCheck
```

`cmd/fathom/doctor_outcomewriter.go`:

```go
func readRuleOutcomeLedgerTail(path string, since time.Time) ruleOutcomeLedgerReading
func checkRuleOutcomeWriterFromFiles(now time.Time) doctorCheck
func checkRuleOutcomeWriter(ledger ruleOutcomeLedgerReading, beats []heartbeat.Beat, coveredFrom, now time.Time) doctorCheck
```

`cmd/fathom/doctor_retention.go`:

```go
func readBeatsSince(path string, since time.Time, maxBytes int64) (beats []heartbeat.Beat, coveredFrom time.Time)
```

`cmd/fathom/instrument_adoption.go`:

```go
func adoptionEraRows(entries []adoptionCallEntry) []AdoptionEraRow
func readCallLog(path string) (entries []adoptionCallEntry, skippedMalformed int, read bool)
func classifyCallLogRead(path string) (entries []adoptionCallEntry, skippedMalformed int, err error)
```

`cmd/fathom/research_report.go`:

```go
func readBeatLog(path string) (beats []heartbeat.Beat, read bool)
func readBeatLogTail(path string, maxBytes int64) (beats []heartbeat.Beat, read bool)
```

`third_party/agentic-kit/foundations/telemetry/heartbeat/heartbeat.go`:

```go
func NewLog(w io.Writer, now func() time.Time) (*Log, error)
func (l *Log) Beat(channel, kind string, meta map[string]string) error
func Load(r io.Reader, acceptPartial bool) ([]Beat, error)
```

`internal/contextbench/task.go` (all fields/tags, standalone comment lines elided):

```go
type ScoringRule string

type TaskClass string

type Task struct {
	ID          string      `json:"id"`
	Category    string      `json:"category,omitempty"`
	Class       TaskClass   `json:"class,omitempty"`
	Prompt      string      `json:"prompt"`
	GroundTruth []string    `json:"ground_truth"`
	ScoringRule ScoringRule `json:"scoring_rule"`
	Source      string      `json:"source,omitempty"`
	Notes       string      `json:"notes,omitempty"`

	TruthAtHead []TruthAnchor `json:"truth_at_head,omitempty"`

	AnswerAnyOf [][]string `json:"answer_any_of,omitempty"`

	Requires []string `json:"requires,omitempty"`

	Exercises []string `json:"exercises,omitempty"`

	MaxPredicted int `json:"max_predicted,omitempty"`

	MinTrials int `json:"min_trials,omitempty"`
}
```

`internal/contextbench/scorer.go` (all fields/tags, standalone comment lines elided):

```go
type ScoreResult struct {
	TaskID  string   `json:"task_id"`
	Pass    bool     `json:"pass"`
	Matched []string `json:"matched,omitempty"`
	Missing []string `json:"missing,omitempty"`
}
```

`internal/contextbench/report.go` (all fields/tags, standalone comment lines elided):

```go
type ArmResult struct {
	TaskID string `json:"task_id"`
	Arm    string `json:"arm"`
	Trial   int  `json:"trial,omitempty"`
	Success bool `json:"success"`
	IsError bool `json:"is_error,omitempty"`

	Matched []string `json:"matched,omitempty"`
	Missing []string `json:"missing,omitempty"`

	InputTokens         int64 `json:"input_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	TotalTokens         int64 `json:"total_tokens"`

	WallTimeMS int64  `json:"wall_time_ms"`
	NumTurns   int    `json:"num_turns,omitempty"`
	RawResult  string `json:"raw_result,omitempty"`


	ModelID string `json:"model_id,omitempty"`
	AuxModels []string `json:"aux_models,omitempty"`
	CostUSD float64 `json:"cost_usd,omitempty"`
	Scoring   string  `json:"scoring,omitempty"`
	Precision float64 `json:"precision,omitempty"`
	Recall    float64 `json:"recall,omitempty"`
	Predicted int     `json:"predicted,omitempty"`
	TokensToFirstGoldHit int64 `json:"tokens_to_first_gold_hit,omitempty"`
	ToolCalls            int   `json:"tool_calls,omitempty"`

	CellKey     string `json:"cell_key,omitempty"`
	ArmSurface  string `json:"arm_surface,omitempty"`
	Borrowed    bool   `json:"borrowed,omitempty"`
	SourceRunID string `json:"source_run_id,omitempty"`

	PayloadBytes        int64  `json:"payload_bytes,omitempty"`
	ToolSearchTurns     int    `json:"toolsearch_turns,omitempty"`
	TurnsExclToolSearch int    `json:"turns_excl_toolsearch,omitempty"`
	GoldSourceTool      string `json:"gold_source_tool,omitempty"`

	TrustedWithoutRecheck *TrustedWithoutRecheckStats `json:"trusted_without_recheck,omitempty"`

	PathLeak       bool     `json:"path_leak,omitempty"`
	PathLeakPaths  []string `json:"path_leak_paths,omitempty"`
	OutOfRootReads int      `json:"out_of_root_reads,omitempty"`
	OutOfRootPaths []string `json:"out_of_root_paths,omitempty"`
}

type claudeEnvelope struct {
	IsError    bool   `json:"is_error"`
	NumTurns   int    `json:"num_turns"`
	Result     string `json:"result"`
	DurationMS int64  `json:"duration_ms"`
	Usage      struct {
		InputTokens              int64 `json:"input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
	} `json:"usage"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	ModelUsage map[string]modelUsage `json:"modelUsage"`
}

type modelUsage struct {
	OutputTokens int64 `json:"outputTokens"`
}
```

`internal/contextbench/cell.go` (all fields/tags, standalone comment lines elided):

```go
type CellInputs struct {
	CorpusSHA256 string
	TaskID       string
	Arm          string
	ModelID string
	HarnessVersion string
	Scoring string
	PromptSubject string
	ArmSurface string
}
```

`internal/scenariobench/types.go` (all fields/tags, standalone comment lines elided):

```go
type SessionResult struct {
	Index             int            `json:"index"`
	Role              string         `json:"role"` // single | planner | worker
	RowID             string         `json:"row_id,omitempty"`
	Turns             int            `json:"turns"`
	ToolSearchTurns   int            `json:"toolsearch_turns"`
	InputTokens       int64          `json:"input_tokens"`
	OutputTokens      int64          `json:"output_tokens"`
	CacheReadTokens   int64          `json:"cache_read_tokens"`
	CacheCreation     int64          `json:"cache_creation_tokens"`
	TotalTokens       int64          `json:"total_tokens"`
	CostUSD           float64        `json:"cost_usd"`
	FirstEditTurn     int            `json:"first_edit_turn"` // 0 = no edit
	TokensToFirstEdit int64          `json:"tokens_to_first_edit"`
	FilesRead         []string       `json:"files_read"`   // sorted, unique, repo-relative
	FilesEdited       []string       `json:"files_edited"` // sorted, unique
	BytesRead         int64          `json:"bytes_read"`
	BoxChecks         []BoxCheck     `json:"box_checks,omitempty"`
	ToolCalls         map[string]int `json:"tool_calls"`
	MCPServers map[string]string `json:"mcp_servers,omitempty"`
	MCPTools     int           `json:"mcp_tools,omitempty"`
	PayloadBytes int64         `json:"payload_bytes"`
	WallTime     time.Duration `json:"wall_time_ns"`
	RawPath      string        `json:"raw_path,omitempty"`
	HooksFired map[string]int `json:"hooks_fired,omitempty"`
	HooksFiredByName map[string]int `json:"hooks_fired_by_name,omitempty"`
	BudgetHit bool `json:"budget_hit,omitempty"`
	FinalAnswer string `json:"final_answer,omitempty"`
	AnswerFrom string `json:"answer_from,omitempty"`
	SessionStartContext string `json:"session_start_context,omitempty"`
	Error           string `json:"error,omitempty"`
	InstrumentError string `json:"instrument_error,omitempty"`
	AnswerKeyLeak bool `json:"answer_key_leak,omitempty"`
	BaselineRanFathom bool `json:"baseline_ran_fathom,omitempty"`
	SandboxEscape bool `json:"sandbox_escape,omitempty"`
}

type Harness interface {
	Name() string
	Version() string
	RunSession(req SessionRequest) (SessionResult, error)
}

type SessionRequest struct {
	Worktree     string
	Prompt       string
	SystemAppend string
	Arm          Arm
	MaxTurns     int
	Model        string
	RawDir       string
	Index        int
	Role         string
	RowID        string
	Env          map[string]string
	MCPConfigPath string
	MCPServerNames []string
	SettingsPath   string
	AllowedTools    []string
	DisallowedTools []string
}
```

`cmd/fathom/lifecyclehooks.go` (all fields/tags, standalone comment lines elided):

```go
type sessionStamp struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	Source        string `json:"source"` // startup|resume|clear|compact|fork
	BriefInjected bool   `json:"brief_injected"`
	BriefBytes    int    `json:"brief_bytes"`
	IntentLine bool   `json:"intent_line,omitempty"`
	At         string `json:"at"`
	Cwd      string          `json:"cwd,omitempty"`
	Identity *identityResult `json:"identity,omitempty"`
}
```

`cmd/fathom/discoveries.go` (all fields/tags, standalone comment lines elided):

```go
type sessionWindowRow struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	Service       string `json:"service,omitempty"`
	Started       string `json:"started"` // RFC3339
}
```

`cmd/fathom/identity.go` (all fields/tags, standalone comment lines elided):

```go
type identityResult struct {
	FreshnessNote string `json:"freshness_note,omitempty"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	Schema        int    `json:"identity_schema"`
	Service       string `json:"service"` // "" when nothing matched
	Root          string `json:"root,omitempty"`
	MatchedBy     string `json:"matched_by,omitempty"`
	Symbols       int    `json:"symbols,omitempty"`
	CountErr      string `json:"count_error,omitempty"`
	Langs         string `json:"languages,omitempty"`
	Path          string `json:"path,omitempty"` // the service's recorded corpus root
}
```

`cmd/fathom/doctor.go` (all fields/tags, standalone comment lines elided):

```go
type doctorStatus string

type doctorCheck struct {
	Name     string       `json:"name"`
	Status   doctorStatus `json:"status"`
	Reason   string       `json:"reason"`
	Remedy   string       `json:"remedy,omitempty"`
	probeErr error        // underlying probe error, never inferred from elapsed context alone
}
```

`cmd/fathom/instrument_adoption.go` (all fields/tags, standalone comment lines elided):

```go
type adoptionCallEntry struct {
	SchemaVersion  int    `json:"schema_version"`
	FathomVersion  string `json:"fathom_version"`
	TS             string `json:"ts"`
	Tool           string `json:"tool"`
	Service        string `json:"service,omitempty"`
	Group          string `json:"group,omitempty"`
	Surface        string `json:"surface,omitempty"`
	Via            string `json:"via,omitempty"`
	Origin         string `json:"origin,omitempty"`
	OffSurface     bool   `json:"off_surface,omitempty"`
	ClaimsAttached int    `json:"claims_attached,omitempty"`
	ResultCount *int   `json:"result_count,omitempty"`
	MatchedVia  string `json:"matched_via,omitempty"`
	VerdictOutcome map[string]int `json:"verdict_outcome,omitempty"`
	ResponseBytes int     `json:"response_bytes,omitempty"`
	ElidedCount   *int    `json:"elided_count,omitempty"`
	LatencyMS     float64 `json:"latency_ms"`
	Error         string  `json:"error,omitempty"`
}

type AdoptionEraRow struct {
	Version string `json:"version"`
	Schema  int    `json:"schema"`
	Calls   int    `json:"calls"`
	FirstTS string `json:"first_ts"`
	LastTS  string `json:"last_ts"`
}
```

`cmd/fathom/doctor_harness_firing.go` (all fields/tags, standalone comment lines elided):

```go
type harnessHookEvidence struct {
	channels map[string]int
	total    int
	last     time.Time
	path     string
	readErr  error
}
```

`cmd/fathom/doctor_outcomewriter.go` (all fields/tags, standalone comment lines elided):

```go
type ruleOutcomeLedgerReading struct {
	path     string
	exists   bool
	err      error
	newest   time.Time // the newest row's ts; zero when there is none
	inWindow int       // rows at or after the window's start
}
```

`third_party/agentic-kit/foundations/telemetry/heartbeat/heartbeat.go` (all fields/tags, standalone comment lines elided):

```go
type Beat struct {
	Channel string `json:"channel"`
	Kind string `json:"kind,omitempty"`
	At time.Time `json:"at"`
	Meta map[string]string `json:"meta,omitempty"`
}
```

ScoringRule constants (`task.go:47-51`): ExactMatch="exact-match",
AllOf="all-of", AnyOf="any-of". Cell scoring constants (`paired.go:405-408`, ScoringSubstring /
ScoringPrecision) remain plain string values "substring" / "precision", separate
from Task.ScoringRule. No new constants or enums are declared by this document.
SessionStamp.Source remains unchecked string (startup/resume/clear/compact/fork
are documented values). Doctor status values are listed above. No nonnil promise
is made for existing slices/maps or result wrappers.

## Real consumers and named conformance assertions

All current direct production uses of the frozen primary seams follow. This is
an exact static inventory at 83226aece9b0f421381373f8dafc4a912d3888e5, including source rescoring, dry and external
paths; definitions, comment-only references, fixtures and tests are excluded.
The shared stamp test covers every emitBeat call through its single stamp site;
this contract does not assert every old beat already carries session or wire.

- `cmd/contextbenchscore/main.go`: 212:ArmResultFromClaudeJSON; 214:ArmResultFromClaudeJSONWithPrecision.
- `cmd/fathom/beats.go`: 60:stampBeatMeta.
- `cmd/fathom/claimcheck.go`: 155:sessionStartFor.
- `cmd/fathom/discoveries.go`: 128:readSessionWindows; 583:sessionStartFor; 659:sessionStartFor.
- `cmd/fathom/doctor.go`: 296:checkRuleOutcomeWriterFromFiles; 306:checkHooksFiring; 1550:checkHarnessHooksFiring.
- `cmd/fathom/doctor_harness_firing.go`: 184:loadHarnessHookEvidence.
- `cmd/fathom/doctor_outcomewriter.go`: 112:checkRuleOutcomeWriter; 115:checkRuleOutcomeWriter.
- `cmd/fathom/edithook_teststorun.go`: 191:sessionStartFor.
- `cmd/fathom/gatepreflight.go`: 324:sessionStartFor.
- `cmd/fathom/hookdedupe.go`: 165:readSessionStamp.
- `cmd/fathom/routes.go`: 175:readSessionWindows; 267:readSessionWindows; 280:readSessionWindows.
- `cmd/fathom/sessionhook.go`: 145:recordSessionWindow; 187:writeSessionStamp.
- `cmd/fathom/sessionhook_events.go`: 363:readSessionStamp.
- `cmd/fathom/sessionledger.go`: 265:readSessionStamp.
- `cmd/fathom/threadpromote.go`: 179:readSessionWindows.
- `cmd/scenariobench/main.go`: 822:CellLost.
- `cmd/scenariobench/rescore.go`: 75:CellLost.
- `internal/scenariobench/dryrun.go`: 81:CellLost.
- `internal/scenariobench/external_corpus.go`: 179:CellLost.
- `internal/scenariobench/rescore.go`: 349:instrumentTruthError; 351:hooksInstrumentError; 354:primerInstrumentError; 357:postToolUseInstrumentError.
- `internal/scenariobench/sessions.go`: 198:runOneSession; 227:runOneSession; 289:runOneSession; 641:instrumentTruthError; 643:hooksInstrumentError; 646:primerInstrumentError; 649:postToolUseInstrumentError.

| Consumer family / existing tests | Assertions frozen by those source witnesses |
|---|---|
| contextbench scorer and CLI: internal/contextbench/scorer_test.go TestScore_CaseSensitiveSubstring, TestScore_EmptyOutputAlwaysFails, TestScore_MatchedMissingAreSortedAndDeterministic; scorer_boundary_test.go:7 TestScore_LineCitationTokenNeverMatchesLongerLine | case-sensitive tokens, empty failure, Matched sorting only, line digit boundary; Missing sorting and determinism are source-only laws; no receipt arg added |
| JSON scorer: report_test.go:8,50,66 TestArmResultFromClaudeJSON, TestArmResultFromClaudeJSON_ClaudeReportedError, TestArmResultFromClaudeJSON_MalformedInput | four usage components sum; reported error sets IsError and fails (the fixture has empty usage and nonmatching output); malformed JSON returns Go error |
| cell borrowers: cell_test.go:25,127,156 TestCellKeyDependsOnEveryInvariant, TestAcceptBorrowedRefusesForeignKey, TestRefuseMismatchedBorrowsDropsTheCell | all applicable key inputs change hash, foreign key refuses, mismatch dropped rather than reused; no wiring assertion implied |
| scenario live and rescore helpers: sessions_test.go:352,441,690,788 TestInstrumentTruthError, TestHooksInstrumentError, TestPrimerInstrumentError, TestPostToolUseInstrumentError | expected-server absent/nonconnected strings and unwired bypass; SessionStart positive count; indexed-service text heuristic; edit needs PostToolUse, no edit bypass |
| scenario cell callers: sessions_test.go:381,417,467,502,527,540,732,819 TestRunSessions_InstrumentTruthGate_LosesTheCell, TestRunSessions_InstrumentTruthGate_AllConnectedIsNotLost, TestRunSessions_WiredArmNoSessionStartHook_LosesTheCell, TestRunSessions_WiredArmSessionStartHookFired_NotLost, TestCellLost_AnySessionInstrumentErrorLosesCell, TestCellLost_NoInstrumentErrorUnaffected, TestRunSessions_WiredArmNotIndexedPrimer_LosesTheCell, TestRunSessions_WiredArmEditNoPostToolUseHook_LosesTheCell | instrument loss is cell-wide even if another session succeeded; connected/fired fixture not lost; missing hook/bad primer/edit-hook defects set instrument error |
| WR-3 stream consumers: internal/scenariobench/wiringfaults_test.go:10 TestWiringFaultStreamsInstrumentTruth | each frozen stream parses; only hook-unregistered produces missing SessionStart, only mcp-not-connected produces server-absent for the tested helpers. Does NOT test WR-12 diagnosis of every fault |
| session window readers/writer: cmd/fathom/discoveries_test.go:109 TestSessionWindows_RecordAndLookup | two recorded windows, lookup recorded id/time, missing id false. No once-only or hash assertion |
| stamp readers/dedupe/subagent/ledger: cmd/fathom/intentbaseline_test.go:48 TestSessionStampCarriesIntentLine | true IntentLine survives stamp read; default false retained, no fabricated cohort |
| all beat writers: cmd/fathom/beats_test.go:156 TestBeatVersionStamp; :59 TestEmitBeat_RotatesBeforeAppending | schema/version/project/caller fields, no input mutation, caller overrides; rotate current to .1 before append. No assertion wire/session auto-added |
| doctor hooks_firing: cmd/fathom/doctor_test.go:1012 TestCheckHooksFiring; doctor_indirect_test.go:111 TestDoctorConfiguredIsNotFiring | Codex reads stamped beat evidence, empty log fails naming harness, unknown no-codec skips, dead unlocatable daemon path skips, configured source with no log fails; configuration not execution |
| doctor rule_outcome_writer: cmd/fathom/doctor_outcomewriter_test.go:47 TestDoctorNamesSilentRuleOutcomeWriter | stale/absent ledger with claims fails, live row passes, no claims passes, old missing-claims build warns; row registered in doctor |
| identity reuse: cmd/fathom/identity_test.go:21 TestIdentityEndpointAndSubagentReuse | same-dir parent stamp with identity answers subagent without RPC; changed-directory resolution; absent-stamp resolution and preservation of all identity pointer fields are source-only laws |
| ledger cohort: cmd/fathom/thread_test.go:240 TestStampJoinLabelsLedgerRows | Source clear / brief true / 512 bytes copied to ledger; missing stamp remains source empty and brief false |
| dedupe stamp prerequisite: cmd/fathom/hookdedupe_test.go:358 TestDedupeUnprimedSessionFailsOpen | absent stamp cannot authorize suppressing first delivery; keep SessionID/At prerequisite in loadHookDedupe |
| existing era reader: cmd/fathom/instrument_adoption_eras_test.go:24,94,130 TestInstrumentAdoptionEras, TestInstrumentAdoptionErasSortAndSchema, TestInstrumentAdoptionErasEmpty | missing stamps unknown; scoped era totals sum to report; schema separates era, timestamp order handles fractions/offsets; empty input yields empty rows |
| independent seeded truth: cmd/fathom/wiringfaults_test.go:44,73 TestWiringFaultCorpus, TestWiringFaultTwins; wiringfaults_records_test.go:15 TestWiringFaultRecordShapes | product writers regenerate byte-identical corpus, frozen fault list/twins differ only declared inputs, record stamps/types preserved. No diagnosis output grades its own labels |

WR-3 limitations are binding fixture facts (`testdata/wiring-faults/manifest.json`):
channel-off has DetectableFromRecords=false because no hook record can distinguish
the kill switch; mcp-not-connected's explicit init capture is bench-only; stale
binary and stale-hook-binary are distinct (daemon can remain current). The eight
fault labels plus two healthy cases must not be hand-relabelled to claim WR-12
100 percent from insufficient inputs. Source laws not asserted by those existing tests (for example matching output
AND is_error=true, JSON null, caller-supplied empty stamp collisions, malformed
window rows and every failure-order tie) remain source-only freezes, with no
claimed existing test coverage. A future modifying lane supplies the actual RED
for the behavior it touches. No positive-control gate function currently
consumes this manifest in contextbench. Its future wiring must prove the unknown
case rather than infer an exact fault from silence.

## Existing approved bench contracts and dependent handback

Approved CTR-13 (`docs/contracts/ctr-13.md`, independent source approval e299b73ee83ec0d3db8f66c7f8f885bb00609777)
and bench-consolidation require one shared benchledger scorer/projection and
PairPreflight/PreflightReceipt/PreflightDisposition from that contract. They are
specified future shared types, not present implementations in this source base:
there is no Go AdmitPair or DeclaredConformanceGate declaration here. Reuse the
approved exact types when their implementing source arrives, never duplicate
their scoring, disposition or receipt engine in a WR adapter. Do not confuse them
with current contextbench.ArmResult or scenariobench.SessionResult.
Records/backfill contracts do not provide alternate receipt/admission ports and
this freeze neither assigns reserved ids nor changes their frozen manifests.

`docs/plans/wiring-proof.md:24-37` consumes WR in WPF-2 (doctor) and WPF-4
(pair admission), plus WPF-6's LAB-7 adapter. New wiring still needs actual
implementing-lane RED, same shared preflight types, production source handback
and independent review. Preserve correction 1/BX-40: healthy preflight with zero
later actions/calls keeps actual assigned-arm outcome and spent tokens; W4/W5
never primary admission inputs. That is the governing future wiring obligation,
not a claim current WR supports before-agent admission. No ENV/WPF/FTU work is
implemented or dispatched by this document.

Foreman gap handback (grouped by actual rows): WR-1/2 lack exported admission
ports; WR-4/5 lack computed wire/idx and propagation; WR-12 lacks session firing
join. Governing W10 permits freezing these limits, not redesign. A review of this
draft must distinguish truthful freeze readiness from dependent implementation
readiness and rule only on this exact immutable commit.

## Static proof and guarded commands (runtime unrun)

Author evidence: signature/field/tag extraction matches source at 83226aece9b0f421381373f8dafc4a912d3888e5;
all named existing tests resolve, all missing-port declaration searches are empty;
direct-use inventory and git diff --check reviewed. No runtime tests, real RED,
measurement, plan verify, fullgate or approval performed. Future proof commands
are direct package tests only, after lane-env and live lease/gate check:

```sh
test ! -e ../out/GATE-RUNNING || exit 2
python3 ../workspace/tmp/package-test-lease.py go test -race ./internal/contextbench ./internal/scenariobench -run '^(TestScore|TestArmResultFromClaudeJSON|TestCellKeyDependsOnEveryInvariant|TestAcceptBorrowed|TestRefuseMismatchedBorrows|TestInstrumentTruthError|TestHooksInstrumentError|TestPrimerInstrumentError|TestPostToolUseInstrumentError|TestCellLost|TestRunSessions_(InstrumentTruthGate|WiredArm)|TestWiringFaultStreamsInstrumentTruth)' -count=1
python3 ../workspace/tmp/package-test-lease.py go test -race ./cmd/fathom -run '^(TestBeatVersionStamp|TestEmitBeat_RotatesBeforeAppending|TestSessionWindows_RecordAndLookup|TestSessionStampCarriesIntentLine|TestCheckHooksFiring|TestDoctorConfiguredIsNotFiring|TestDoctorNamesSilentRuleOutcomeWriter|TestWiringFault)' -count=1
```

No -update fixture mutation and no fullgate; max three SH package tests globally,
no nested EX. Builds/ingest/timing remain EX and outside the author scope. Any new
WR/WPF tests must be actual failing tests of owning production paths, not weaker
aliases of this source inventory. The foreman alone owns independent critics,
approval and next normal land bundle after ACK-4, with gate PASS verbatim (W10-3).

## Decision models

None. This document freezes deterministic existing inputs/outputs and explicitly
missing wiring. No judge_wiring site, sampler, provider or budget is added.
