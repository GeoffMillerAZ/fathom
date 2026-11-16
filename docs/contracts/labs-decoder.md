# LAB-7 existing transcript decoder contract

Status: Draft for independent exact-commit review under W10-1. Existing behavior only.
Source base: `83226aece9b0f421381373f8dafc4a912d3888e5`. Owns this document only. Plan: `docs/plans/feature-labs.md:73`,
LAB-7, which remains unchecked. No implementation, new API, migration, dependency,
package reservation, test execution, measurement or approval is implied.

## Authority and boundary

W10-1 freezes code as it stands. W2-3 requires an independent critic and foreman
approval. W3-2 puts any future wiring RED in its implementing lane. W3-4's new-API
style is not permission to rewrite existing strings, JSON names, nil slices or errors.
The intent router, I38 (`docs/intent/invariants.md:68`), I61 (`:89`), ADR-029 decision 6
(`ADR.md:1459`) and ADR-082 (`ADR.md:6141`) govern coverage, privacy and Claude-only
measurement. No new trigger result or changed intent is claimed. W10 requests a
freeze, not the Codex adapter contemplated by WPF-6 or ADR-102 (still proposed).
No store, background reader, provider, scoring engine or second decoder is added.

## Exact callable interfaces and transient/result types

Package import: `github.com/geoffmilleraz/fathom/internal/transcript` (internal,
not a pkg/lang door export). `LineMax = 64 << 20` bytes. There are no receivers,
context parameters, typed refusal errors or named enum types in this package.
I/O currently has no cancellation argument. Do not add one as part of this freeze.
The signatures below include private helpers so adapters do not invent a seam.
Sources: `internal/transcript/decode.go:18-260`, `internal/transcript/events.go:19-213`.

```go
func DecodeLine(raw []byte) (Line, []Block, error)
func ResultOf(l *Line, b *Block, single bool) (stdout, stderr string, tr ToolResult)
func StructuredResult(l *Line, single bool) (tr ToolResult, ok bool)
func ResultText(raw json.RawMessage) string
func ResultCount(blocks []Block) int
func WalkFile(path string, visit func(int, *Line, []Block)) Result
func fileLabel(path string, index int) string
func walkFile(path string, index int, visit func(int, *Line, []Block)) Result
func WalkLines(r io.Reader, visit func(int, []byte)) Result
func DecodeTranscriptEvents(path, salt string, fileIndex int) Result
func isSubagent(path string) bool
func inputShape(v any) string
func metadataHash(salt, value string) string
```

These declarations are verbatim. `Event`, `Issue`, `Result` have no JSON tags:
encoding/json uses their Go field names and serializes nil slices as `null`.
No nonnil-slice or lower-snake-case promise is retrofitted. `Line`/`Block`/`ToolResult`
are transient, text-bearing local readout inputs, not safe retained event records.

```go
type Event struct {
	Kind      string
	Line      int
	Cwd       string
	GitBranch string
	Timestamp string
	ToolUseID string
	Name      string
	Shape     string
	IsError   bool
	Bytes     int
	HookEvent string
}

type Issue struct {
	File string
	Line int
	Kind string
}

type Result struct {
	Events          []Event
	Issues          []Issue
	MalformedLines  int
	OversizeLines   int
	UnreadableFiles int
	Duplicates      int
}

type Line struct {
	Type       string `json:"type"`
	Subtype    string `json:"subtype"`
	UUID       string `json:"uuid"`
	SessionID  string `json:"sessionId"`
	Timestamp  string `json:"timestamp"`
	Cwd        string `json:"cwd"`
	GitBranch  string `json:"gitBranch"`
	HookEvent  string `json:"hook_event"`
	Output     string `json:"output"`
	Attachment *struct {
		Type      string `json:"type"`
		HookEvent string `json:"hookEvent"`
		ToolUseID string `json:"toolUseID"`
	} `json:"attachment"`
	Message *struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type Block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
	HookEvent string          `json:"hookEventName"`
}

type ToolResult struct {
	Stdout              *string `json:"stdout"`
	Stderr              string  `json:"stderr"`
	Interrupted         bool    `json:"interrupted"`
	PersistedOutputPath string  `json:"persistedOutputPath"`
}
```

## Decode, ordering and failure laws

- LAB-7/L1 (`events.go:39-163`): one path per invocation, physical line order then
  source block order, never timestamp sort or cross-file merge. Caller supplies
  deterministic file ordinal and `bundle.LoadOrCreateSalt` salt. There is no
  validation of negative ordinals or refusal on empty salt: empty salt or empty
  metadata yields empty Cwd/GitBranch. Otherwise metadata is HMAC-SHA256 truncated
  to eight bytes / 16 lowercase hex digits (`events.go:204`). Timestamp stays an
  uninterpreted string. No session hash is manufactured by the decoder.
- LAB-7/L2 (`decode.go:67-95`): DecodeLine trims whitespace. Blank gives zero Line,
  nil blocks, nil error. Non-object first byte returns ordinary error text
  `expected Claude transcript object`; a failed envelope unmarshal returns zero
  Line and its json error. Absent message, absent/null/string content yields no
  blocks, including string content whose internals are not further validated.
  Object/non-array content returns decoded Line and ordinary error
  `expected Claude content array or string`; bad array returns decoded Line and
  json error. Unknown envelope/block fields are ignored. Empty object is valid.
  Block errors can therefore preserve envelope timing for sizing while events
  classify the whole physical line as malformed.
- LAB-7/L3 (`decode.go:186-260`): WalkLines reads with a 1 MiB buffered reader,
  bounded accumulation at LineMax including a newline byte if present. Blank lines
  consume physical numbers but invoke no visitor. Final unterminated lines work.
  Oversize consumes/skips the entire line then continues, increments OversizeLines,
  emits oversize_line. Non-EOF read errors preserve a partial visited line and add
  unreadable_file at next physical line, incrementing UnreadableFiles. No reader
  close in WalkLines; WalkFile owns os.Open/deferred Close, ignores Close errors.
  Open failure gives one unreadable_file Issue at line 0. Nil visitors are allowed.
  WalkLines has no file identity and does not decode JSON or count malformed lines.
- LAB-7/L4 (`decode.go:178-212`, `events.go:151-163`): WalkFile labels issues
  session:<ordinal> or subagent:<ordinal>; its public wrapper always uses ordinal 0.
  A subagent path has agent-*.jsonl under subagents or subagents/workflows/<run>.
  No raw path or OS error is retained. Issue kinds are bare strings:
  malformed_line, oversize_line, unreadable_file, invalid_tool_input,
  invalid_skill_input. Decode errors count once per bad envelope line; input
  errors count unique physical lines, not issues. Issues sort stably by line.
  Result is the coverage/error channel: DecodeTranscriptEvents/WalkFile/WalkLines
  return no Go error. Empty/fully bad/unreadable inputs can return nil Events;
  Issues can be nil. Empty results are not evidence of a healthy transcript.
- LAB-7/L5 (`events.go:55-150`): each decoded nonblank line emits line, preceded
  by synthetic subagent_start on first valid line in a subagent file. Attachment
  hook_additional_context, explicit system subagent_start/stop, then system
  hook_response additional context, then block events follow. A readable subagent
  file with a valid line ends with synthetic subagent_stop using last metadata;
  unreadable tail suppresses it. Explicit boundaries can coexist with synthetic
  ones. Hooks are not deduplicated. Unknown block types emit nothing beyond line.
- LAB-7/L6 (`events.go:90-149`): nonempty use ids and result ids deduplicate in
  separate maps per file. Empty ids never alias. Duplicate count increments for
  each suppressed occurrence, but line metadata events remain. A repeated invalid
  use is skipped before validation. A Skill use emits tool_use then skill,
  same id/shape; safe skill name matches ^[a-z0-9][a-z0-9:_-]{0,63}$, else skill.
  Wrong typed skill input adds invalid_skill_input; missing/invalid JSON adds
  invalid_tool_input and shape invalid. Task/Agent emits tool_use then
  subagent_start; only nonempty ids track a matching result's subagent_stop.
- LAB-7/L7 (`events.go:175-213`): shape vocabulary is null, boolean, number,
  string, array[...] (source item order), object{...} (sorted member shapes),
  invalid on bad input. Private inputShape has an unknown fallback not produced
  by ordinary successful JSON decode. Object keys and scalar values are dropped.
  Tool names, tool ids, hook event names and timestamps remain raw identifiers;
  they are not a general redaction/sanitization boundary for arbitrary bad inputs.
  Never persist transient prompt, input/output or attachment text through events.
- LAB-7/L8 (`decode.go:99-148`): ResultCount counts all tool_result blocks.
  StructuredResult succeeds only when single=true, line toolUseResult starts
  literally with {, unmarshals and has nonnil stdout. It can retain partially
  decoded tr when ok=false. ResultOf uses structured streams only on success
  and !IsError; otherwise ResultText(content), empty stderr, zero ToolResult.
  ResultText concatenates only text blocks without separators or returns a JSON
  string, else empty. Event Bytes is len(stdout)+len(stderr), bytes not runes.
  PersistedOutputPath is never opened by this decoder. Pointer helper callers
  supply nonnil Line/Block; no defensive nil-pointer result is promised.

## All current production consumers and their obligations

Inventory from all production imports of internal/transcript at the pinned base.
Tests also call the package directly. The independent ledger, hooks, contextbench
and scenariobench readers remain separate as LAB-7 explicitly says.

| Consumer / exact source seam | Required invariant and existing conformance witness |
|---|---|
| labs corpus: `cmd/fathom/labscmd.go:217,257,295`, readLabsCorpus -> DecodeTranscriptEvents -> labs.ExtractDecisionPoints | sorted path ordinal, shared salt, stem session identity; propagate all counters/issues, per-digest unreadable failure and directory failures. `labscmd_test.go:64` TestCmdLabsCorpusWritesSessionsAndPrintsAggregates and `:96` TestCmdLabsCorpusJSONIsDeterministic assert two points, hashed filename/no raw data, identical repeated JSON. `:226,257,391` unreadable directory/transcript/digest-only tests require named coverage. |
| decision points: `internal/labs/points.go:72-144` ExtractDecisionPoints(res transcript.Result, salt, sessionID string) ([]Point, Counts) | copy coverage; empty salt/session refuses points with unhashed_session, increments UnreadableFiles; paired Skill/Task/Agent replaces preceding use; named subagent start only, bare boundaries ignored. Join results by nonempty id, 0-based call ordinal, preceding 5/following 3, hash session with bundle.HashService, ToolUseID json:"-". `points_test.go:29,59,82,95,102,116,149,160,168,182,196,210` named tests below. |
| replay: `cmd/fathom/readoutcmd_replay.go:448-511,604,636,1277` (*replayAgg).readFile -> WalkLines -> (*replayAgg).line -> DecodeLine, ResultCount, StructuredResult, ResultText; aliases replayTranscriptLine/Block/ToolResult | raw tool-marker prefilter, object-valued content ignored for historical parse count, own pending/window/dedupe logic. Open failure counts no file; opened unreadable file counts one; partial read keeps calls and finalizes windows. Never replace with projected Event. `readoutcmd_transcript_round2_test.go:51,94` tests below assert exact parity. |
| replay sizing: `cmd/fathom/readoutcmd_sizing.go:245,268-302` sizingLine alias, (*replaySizing).line -> DecodeLine and ResultCount | preserve valid envelope on block error and assistant turns/object content, existing prefilter. `readoutcmd_transcript_round2_test.go:74` TestReplaySizingObjectContentStillCountsTurn asserts five turns including invalid-block/object messages and post=3. `readoutcmd_sizing_test.go:116` existing sizing/dedupe/spawn/compaction aggregate coverage. |
| readout report: `cmd/fathom/readoutreport.go:482-505,570` readoutReportFile -> WalkFile; readoutReportLine -> DecodeLine; roResultOf -> ResultOf | local text remains transient to re-render rules, per-consumer joins/dedupe; WalkFile receipt currently discarded, not silently upgraded to complete coverage. `readoutreport_test.go:72` TestReadoutReportJoinsBytesToReferenceAndRecovery holds bytes/reference/recovery join. |
| baseline report: `cmd/fathom/readoutreport_baseline.go:268-311` blReadFile -> WalkLines, blLine -> DecodeLine | own marker filter and three-call follower logic, keep only calls with result time; WalkLines receipt currently discarded. `readoutreport_baseline_test.go:14,31,69` assert touched-within-three, since bound and id-preferring join. |

## Named assertions and guarded future proof

All named tests here already exist. They are source witnesses, not claims of a
run in this draft. Any new WPF-6/FTU-3 wiring owns its real failing test, production
call site and source handback. Those rows keep their own plans/contracts; no new
Codex format is accepted merely because DecodeLine ignores unknown fields.

| Existing test (file) | Frozen assertions |
|---|---|
| TestTranscriptEventsAndOrder, TestTranscriptDedupeByToolUseID (`internal/transcript/events_test.go:32,67`) | exact ordered kinds, metadata, Skill/Agent pairing, separate use/result dedupe count 2 |
| TestTranscriptInputShapeKeepsNoKeysOrValues, TestTranscriptResultRepresentations, TestTranscriptHookAttachments, TestTranscriptSubagentFileBoundaries (`events_test.go:80,128,139,150`) | recursive shape grammar, byte counts 7/5/3/2, attachment id/event, synthetic start/line/stop |
| TestTranscriptMalformedLineCountedAndNamed, TestTranscriptUnreadableFileCountedAndNamed, TestTranscriptOversizeLineCountedAndContinues (`events_test.go:92,104,162`) | three malformed numbered issues with later use retained; directory unreadable count 1; oversized line skipped then line 2 retained |
| TestTranscriptEventsContainNoText, TestMetadataHashNeedsASalt (`events_test.go:112,175`); TestTranscriptMetadataHashesAndNoText, TestTranscriptIssueFileLabels (`privacy_test.go:25,62`) | seeded text absent, empty salt no hash; fixed HMACs, deterministic same salt/different other salt; class/ordinal only for normal/subagent/workflow/missing files |
| TestTranscriptInvalidToolInputsAreIssues, TestTranscriptSkillNamesRestricted, TestTranscriptBlankCRLFLinesAreNoOp, TestTranscriptInvalidEnvelopeHasNoPartialMetadata (`privacy_test.go:85,109,126,140`) | malformed input/skill class and count, 64-char skill grammar, blank/CRLF physical numbering, envelope type failure zeroes Line |
| TestExtractDecisionPointsCutsCallsInOrder, TestExtractDecisionPointsCountsASkillOnce, TestExtractDecisionPointsKeepsFewerThanThreeFollowers, TestExtractDecisionPointsIgnoresBareBoundaries, TestExtractDecisionPointsJoinsResults (`internal/labs/points_test.go:29,59,82,95,102`) | ordered decisions, no double Skill, short tail, bare boundary ignored, result present/error/bytes |
| TestExtractDecisionPointsCarriesDecoderCounts, TestExtractDecisionPointsCarriesOversizeLines, TestExtractDecisionPointsPairsATaskWithItsSubagentStart, TestExtractDecisionPointsWindowBounds, TestExtractDecisionPointsPrecedingWindowUpperBound, TestPointsDoNotSerializeTheToolUseID, TestExtractDecisionPointsRefusesAnUnhashedSession (`points_test.go:116,160,149,168,182,196,210`) | every coverage counter survives, one Task choice, five/three bounds, id not serialized, salt/session absence counted |
| TestCmdReadoutReplayTranscriptCompatibilityCounts, TestReplayTranscriptReadErrorKeepsPartialCounts (`cmd/fathom/readoutcmd_transcript_round2_test.go:51,94`) | 1 file/session, 2 unparsed, 3 Bash calls/15 bytes, repeat 1/5 bytes, compaction 1 pre/reread with 3 incomplete post turns; partial read additionally unpaired=1; missing file 0 vs opened unreadable 1 |

Source/static proof for this draft: exact declaration extraction, local consumer
inventory, named-test existence, pinned source citation check, `git diff --check`.
No runtime result, gate PASS, plan verification, benchmark or RED is claimed.
The foreman owns fullgate and next-bundle sequencing. For an implementing/review
lane only, after sourcing lane-env, rechecking the live lease and GATE-RUNNING:

```sh
test ! -e ../out/GATE-RUNNING || exit 2
python3 ../workspace/tmp/package-test-lease.py go test -race ./internal/transcript ./internal/labs -run '^(TestTranscript|TestMetadataHashNeedsASalt|TestExtractDecisionPoints|TestDecisionPoints|TestPointsDoNotSerialize)' -count=1
python3 ../workspace/tmp/package-test-lease.py go test -race ./cmd/fathom -run '^(TestCmdLabsCorpus|TestCmdReadoutReplayTranscriptCompatibilityCounts|TestReplaySizingObjectContentStillCountsTurn|TestReplayTranscriptReadErrorKeepsPartialCounts|TestCmdReadoutReplaySizesDedupeSpawnsAndCompactions|TestReadoutReport|TestTestRunRowCarriesToolUseIDAndBaselinePrefersIt)' -count=1
```

These are direct targeted package tests only (shared lease, max three), not nested
EX acquisition. Builds, ingest, timing and fullgate remain outside this author lane.

## Decision models

None. This freezes deterministic local decoding and existing data retention. It
creates no model site and authorizes no provider or benchmark spend.
