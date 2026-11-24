# Consumer helpers for the understanding library: span lookups, row export, outline and agent cookbook

Status: Accepted for build 2026-11-23 (architect ruling, owner lifted the ADR-116 pause for LP-8). Decision: [ADR-116](../../ADR.md#adr-116-the-understanding-library-gets-helpers-and-a-row-export-not-a-query-language-2026-11-23). Plan: `docs/plans/library-roadmap.md`, program LP-8, rows HLP-1 to HLP-4 (HLP-15 follows HLP-3). Register and ratings: [library roadmap section 7](library-roadmap.md#consumer-helpers-adr-116). Builds on [library-builder-kit.md](library-builder-kit.md) (LK-D6) and I69.

Codes in this document: CH-D decision, CH-R risk. Section 2 names the contract for review, section 5 the risks. Spec-driven: each helper lands as its own bundle with its reading, plan row, change note fragment, usage text and compatibility notes.

Reading order: section 1 bounds the build, section 2 is the contract, section 6 is the decision-model verdict.

## 1. What this builds, and what it does not

Four helpers that every consumer of the library otherwise writes for itself. Each is a pure function over a
`lang.Result`, or a thin output mode of `fathom-lang`. Intent that binds (ADR-116, ADR-092, I69, LK-D6):

- CH-D1 No store, daemon, index, network, cache file, value read or new process. No change to any dialect, any
  extractor or the read path. A finding about a dialect goes to the reading and the register, not into this build.
- CH-D2 Existing output is frozen. The default `extract` JSON, every existing binary golden and every existing API
  snapshot stay byte-identical. A changed existing golden is a defect in this build.
- CH-D3 Disclosure over guessing. Where a Result cannot decide an answer (a symbol with no end line, an incomplete
  read), the helper says unknown. It never returns a confident answer it cannot back.
- CH-D4 Nothing a helper returns may contain file text the Result does not already carry. The one exception is
  `Slice`, which returns the caller's own bytes to the caller and exists only in the Go API.
- CH-D5 One new public package, `pkg/lang/view`, documented as provisional: fields and functions may change in a
  minor release until the LP-8 readings pass and one release has shipped. It imports `pkg/lang` and the standard
  library only. It joins the API snapshot test (`testdata/api/view.txt`) and the closure test. The door's package
  budget must not grow: if it would, stop and escalate.
- CH-D6 New binary surface is additive: verbs `at`, `outline` and `schema`, and the flag `extract -rows`. Each new
  JSON shape carries `schema_version`, is pinned by a binary golden, and is marked provisional in the usage text
  and the compatibility notes (`docs/intent/scope.md` and its member file).
- CH-D7 Names in section 2 are the contract for review. A name may change with a one-line reason in the LAND file.
  Semantics, disclosure rules and bars may not change without an architect ruling.

## 2. The helpers

### HLP-1 Span lookups (`pkg/lang/view`, verb `at`)

Facts about the input: a `lang.Symbol` has `Line` and `EndLine`, and `EndLine` is 0 when the extractor does not
compute it. There are no columns and no parent field. Nesting is span containment.

Go API:

- `func NewIndex(r lang.Result) *Index` builds the lookup once. Deterministic, no retained reference to caller
  slices that a caller could mutate into a wrong answer (copy what is kept).
- `func (ix *Index) Enclosing(line int) (lang.Symbol, Status)`
- `func (ix *Index) EnclosingRange(start, end int) (lang.Symbol, Status)` (ESCALATE-26 R2, additive)
- `func (ix *Index) Touched(start, end int) (syms []lang.Symbol, complete bool)`
- `func Slice(src []byte, s lang.Symbol) ([]byte, bool)`
- `type Status string` with the closed set `found`, `none`, `unknown`.

Semantics:

- A symbol has a known span when `EndLine >= Line > 0`. Its span is the lines `Line` to `EndLine` inclusive.
- `EnclosingRange(start, end)` (ESCALATE-26 R2): among symbols with a known span containing every line of
  `start..end`, the innermost is the one with the latest `Line`, then the earliest `EndLine`, then the largest
  `Key` (ESCALATE-27 R4: the innermost is the last symbol in the one listing order, `Line`, `EndLine` descending,
  `Key`, so among equal spans the first listed is the outer one, as HLP-2 nests them; one containment rule,
  shared by HLP-1 and HLP-2). `Enclosing(line)` is `EnclosingRange(line, line)`.
  - `found` (ESCALATE-27 R3): the read is not `Incomplete`, a known span contains the whole range, and no
    unknown-span symbol has `Line < 1` or a `Line` at or after the candidate's `Line` and at or before `start`.
    An unknown-span symbol that starts before the candidate's `Line` can only be outer and does not block `found`.
    One that starts after `start`, inside the range or beyond it, cannot hold the whole range and does not block
    it either.
  - `none`: the read is not `Incomplete`, no known span contains the range, and no unknown-span symbol starts at
    or before `start`, an unknown-span symbol with `Line < 1` counting as starting at or before `start` (R3).
  - Otherwise `unknown`, and the candidate (possibly the zero Symbol) is returned as a hint.
  - Symbols that start inside the range are not the enclosing symbol. `Touched` reports them, and the `at`
    output carries both beside each other.
- `Touched(start, end)`: every symbol whose known span intersects the range, plus every unknown-span symbol whose
  `Line` lies in the range. Sorted by `Line`, then `EndLine` descending, then `Key`. `complete` is false when the
  read is `Incomplete` or an unknown-span symbol starts before `start`.
- A `Skipped` result has no symbols: `Enclosing` is `unknown`, `Touched` is empty with `complete` false.
- `start < 1`, `end < start`: status `none` or an empty set with `complete` true is wrong. Return `unknown` and
  `complete` false. Never panic.
- `Slice` returns the bytes of lines `Line` to `EndLine` with their line endings, and false when the span is
  unknown or lies outside `src`. It reads nothing but `src`.

Budget (ESCALATE-26 R1): `pkg/lang/view` is opt-in, like `pkg/lang/cue`. The door's closure budget (CH-D5,
`pkg/lang` at 80) does not grow. The closure test asserts the forbidden set and that the closure of `view` is the
door's plus `view` and nothing else, so a consumer of `view` links one package more than a consumer of `pkg/lang`
alone. `view` does not import `pkg/lang/deps`.

Binary: `fathom-lang at [-root DIR] [-deadline D] [-max-memory SIZE] [-in-process] PATH:LINE[-END]...` (`LINE` and `END` at least 1, `END` not below `LINE`, else a usage error, exit 2) reads each
file once under the existing supervised boundary and prints
`{"schema_version":1,"queries":[{"path","start","end","status","enclosing":{symbol}|null,"touched":[symbols],"complete":bool,"error":""}]}`.
Several queries on one path share one read. Exit codes as `extract`. No source text is printed.

Proof (row HLP-1):
- Property test, fixed seed: for every golden Result in the repository's pinned goldens and for generated symbol
  sets (nested, overlapping, equal spans, unknown spans, incomplete), `Enclosing` and `Touched` equal a brute-force
  reference written independently in the test file.
- Table tests for every clause above, including the three ways `found` degrades to `unknown`.
- Binary golden for `at`. API snapshot added.
- Reading `docs/bench/readings/library-roadmap/hlp-1-span-lookups.md`, opening with its verdict line. It reports,
  per dialect on the outline corpus of HLP-2, the share of symbols with a known span and the share of lines where
  `Enclosing` is `found`, `none` and `unknown`. No bar on those shares: they tell a consumer where span lookups
  work. PASS means the tests above pass and the shares are published.

### HLP-3 Row export and schema (`extract -rows`, verb `schema`, `view.Rows`)

Purpose: let jq, SQLite or DuckDB query facts with no glue code. This is the answer to the query-language
question (ADR-116).

- `fathom-lang extract -rows ...` prints JSON Lines instead of the nested document. Same reads, same flags, same
  exit codes. Without `-rows` the output is unchanged (CH-D2).
- Row kinds, one JSON object per line, each with `"row"`:
  - `header`, first line: `rows_version` (1), `schema_version` of the nested shape it mirrors.
  - `file`: `path`, `error`, `dialect`, `language`, `skipped`, `level`, `incomplete`, and what is needed to rebuild
    the nested document exactly (see lossless below).
  - `symbol`, `reference`, `unresolved`, `fact`: `path`, `i` (zero-based position in its array), then every field
    of the matching Go struct under its existing JSON name.
  - `note`: `path`, `i`, `note`.
- Order: files in the order the nested document lists them. Within a file: the `file` row, then symbols,
  references, unresolved, facts, notes, each in array order. Byte-equal output for equal input.
- Lossless: a function in the test tree rebuilds the nested document from the rows, and the result equals the
  nested output byte for byte after canonical JSON encoding, for every file in the binary goldens and the pinned
  corpus sample of HLP-2. If the nested output distinguishes a null array from an empty one, the `file` row
  carries that distinction.
- Rows carry nothing the nested document withholds (CH-D4): the export is a re-shaping of the same values.
- Go API: `func Rows(path string, errText string, r lang.Result) []Row` and a stable JSON encoding of `Row`, so a
  Go consumer produces the same lines the binary prints. The binary uses this function.
- `fathom-lang schema [extract|rows|at|outline]` prints the JSON Schema (draft 2020-12) for that shape, embedded
  in the binary so a consumer gets the schema of the version it runs. Sources live beside the command and are
  also published under `docs/schema/fathom-lang/`. A test fails when the two copies differ.
- Drift guard: a test walks the JSON tags of `lang.Result`, `lang.Symbol`, `lang.Reference`, `lang.Unresolved` and
  `lang.Fact` and fails if a field is missing from the schemas or the rows.
- Validation uses the JSON Schema support of the CUE module already pinned in `go.mod`. No new module dependency.
  If that cannot check the draft used, stop and escalate. Follow `schema/AGENTS.md` for valid and invalid pairs.

Proof (row HLP-3): the lossless test, the schema accepting every golden output and rejecting each invalid
fixture (a missing required field, a wrong type, an unknown row kind), the drift guard, binary goldens for
`-rows` and `schema`, and one executed SQL example (an in-memory SQLite database loaded from the rows through the
driver already in the module, from a test outside the door's closure) that answers "symbols per canonical kind per
file" and "references with no in-file target". Reading `hlp-3-row-export.md` with its verdict line and the row
counts per kind on the corpus sample.

### HLP-2 File outline (`view.Outline`, verb `outline`)

Purpose: give an agent the shape of a file in place of the file.

Go API:

- `func Outline(path string, r lang.Result, o OutlineOptions) OutlineDoc`
- `type OutlineOptions struct { MaxNodes int }` (0 means no limit)
- `type OutlineDoc struct { Path, Dialect, Language, Level, Skipped string; Incomplete bool; Nodes []OutlineNode; Symbols, References, Unresolved, Facts, Notes, Omitted int }`
- `type OutlineNode struct { Key, Name, Kind, Canonical, Signature string; Line, EndLine int; SpanKnown bool; Children []OutlineNode }`
- `func (d OutlineDoc) Text() string`

Semantics:

- Nodes are the Result's symbols, ordered by `Line`, then `EndLine` descending, then `Key`. A node is a child of
  the innermost known-span symbol that strictly contains its span (same rule as HLP-1). Unknown-span symbols are
  top-level nodes with `SpanKnown` false. Equal spans nest by `Key` order, first as parent.
- Every node string comes from the Result unchanged, in `Nodes` and in JSON. The outline adds no text from the
  file (CH-D4). `Text` alone writes a run of whitespace that holds a newline or CR as one space (see as built).
- `MaxNodes`: when the outline has more nodes, drop the deepest level first, then the next, until it fits, and
  set `Omitted` to the number dropped. If the top level alone exceeds the limit, keep the first `MaxNodes` in
  order. Truncation is never silent: `Text` ends with a line stating how many were omitted.
- `Text` form, stable and line-oriented:
  `# <path> dialect=<d> level=<l>[ incomplete][ skipped=<reason>]`, then one line per node,
  `<two spaces per depth><line>-<end or ?> <canonical or kind> <name>[  <signature>]`, then
  `# symbols=<n> references=<n> unresolved=<n> facts=<n> notes=<n>[ omitted=<n>]`.
- An `Incomplete` or `Skipped` read still produces a document, with the flag in the header, so a consumer is told
  the outline may be partial.

Binary: `fathom-lang outline [-root DIR] [-format text|json] [-max-nodes N] [-deadline D] [-max-memory SIZE]
[-in-process] PATH...` prints the text form, or `{"schema_version":1,"files":[{"path","error","outline":{...}}]}`.

As built: the JSON field names are the snake case of the Go fields (`end_line`, `span_known`), `nodes` and `children`
are arrays and never null, and `outline` is null for a file that could not be read. The text form prints
`# <path> error=<text>` for that file, which the spec did not name. The nesting calls `before` and `covers` of
`pkg/lang/view/contain.go`, the one containment rule of HLP-1.

As built (ESCALATE-28 R5): Go Signatures carry raw newlines and tabs (80 fields in 48 development corpus Go files),
so one line per node and every string unchanged cannot both hold in text. `Text` writes each whitespace run that
holds a newline or CR as one space and drops such a run at either end of a string. The nodes and the JSON keep the
strings unchanged, so CH-D4 holds. A test holds text lines equal to nodes plus two over the goldens and the corpus.

As built (ESCALATE-28 R6): Keys are not unique in real reads (one-line TypeScript overloads share Key and span), so
`before` in `contain.go` is made total after Key by Kind, Canonical, Name, Signature, Package and Tier, and
`moreInner` is its exact reverse. R4 stays one rule, and no input order changes an outline, `Enclosing` or `Touched`.
A test holds this for duplicate Keys under shuffled input. The `at` goldens of HLP-1 are byte-identical.

Reading outcome (ESCALATE-28 R7): the HLP-2 bar was fixed before the reading and does not move. It reads FAIL for
Python and TypeScript and PASS for Go and JavaScript, so row HLP-2 stays open and HLP-2b is proven. No pooled-bytes
bar, no outline without signatures and no new bar is adopted. The reading adds, as description with no bar, the
ratio by file size bucket and the fixed bytes of the header and footer lines. HLP-4 needs HLP-1, HLP-2b and HLP-3.
HLP-16, an outline size bar by file size preregistered and read on a corpus not used here, is registered as deferred.

Proof (row HLP-2):
- Byte-equal output for equal input, and for symbols supplied in shuffled order.
- A test that every string in an outline is a field of the Result it came from.
- Goldens for both formats, API snapshot.
- Reading `hlp-2-file-outline.md`. Corpus, fixed now: the development pools pinned by COV-0 for Python,
  JavaScript and TypeScript and the three Go subjects of COV-M3, every file with at least one symbol, read with
  the reader at the bundle's commit. Measure text-form outline bytes divided by file bytes per file. Report the
  median and the 90th percentile per dialect and the count of files. Bar, fixed now: median at or under 20
  percent for each of the four dialects. A dialect over the bar is a FAIL for that dialect and is reported as one.
  The reading states that this measures size, not whether an agent does better with an outline.

### HLP-4 Agent cookbook

Ruling that amends the landed row: HLP-4 no longer waits on LBK-11. It needs HLP-1, HLP-2b and HLP-3 (ESCALATE-28
R7: HLP-2b is the outline verb, and HLP-2 stays open on its size bar). When LBK-11 lands, its cookbook links this chapter.

- `docs/use-cases/library-agent-cookbook.md`: the common agent jobs, each as a short recipe with the exact
  command, the output shape, and what unknown means for that job.
  1. Shape of a file before reading it (`outline`).
  2. Which function a stack-trace line or a diff hunk belongs to (`at`).
  3. Which symbols a diff touches (`at` with ranges taken from `git diff -U0` hunk headers, shown as a script).
  4. What a file depends on, and what is unknown (`deps`).
  5. Does a change break a declared boundary (`boundaries`).
  6. Ad hoc questions over a tree (`extract -walk -rows` loaded into SQLite, three example queries).
- `examples/lang/agenttools/tools.json`: tool definitions for those jobs in the common function-calling form
  (name, description, JSON Schema for input, the command template). No server and no MCP (ADR-116).
- A test builds the binary and executes every recipe and every tool definition against a small fixture tree, and
  fails when one stops running or its output no longer validates against its schema. SQL recipes run through the
  driver already in the module. No recipe depends on a tool the test cannot run.
- Follow `docs/AGENTS.md` for use-case guides, including the `## Maintained` line.

Proof (row HLP-4): that test in the gate, and reading `hlp-4-agent-cookbook.md` listing each recipe and its test.

## 3. Order and independence

HLP-1 and HLP-3 are independent and may be built side by side. HLP-2 needs HLP-1's containment rule (share the
code, do not copy it). HLP-4 needs all three. LP-8 closes when the four rows verify and its own reading is
written. Each row lands as its own bundle with its reading, plan row bound to a contract the verifier can check,
change note fragment (new verbs, flag and package are observable), usage text and compatibility notes.

## 4. Also recorded in the first bundle

- Register entry HLP-15, row export for dependency edges (`deps -rows`: one row per edge and per unknown, the
  shape that makes cross-file questions one SQL join). Rating 4 / 2 / 2. Selected, scheduled after HLP-3 is
  proven so it copies a proven row contract. Add it to the rated table and as an open row after LP-8, outside
  this build.
- Register finding slot: dialects whose symbols lack an end line, to be filled by the HLP-1 reading.

## 5. Risks

- CH-R1 A helper answers confidently on a partial read. Guard: CH-D3 and the degradation tests.
- CH-R2 The row export becomes a second compatibility surface that drifts. Guard: generated from the same structs,
  the drift test, the lossless test, provisional marking.
- CH-R3 The outline bar is met on size while being useless to an agent. Stated in the reading. The measurement of
  agent benefit belongs to the bench program and is not claimed here.
- CH-R4 Scope creep into an index or a server. Refused by ADR-116. Anything that needs state across files is out.

## 6. Decision models

None applies. Every helper is computed from a read: which symbol contains a line, how rows are shaped and how an
outline nests are decided by the data. Provider `none` is the whole feature, and not calling a helper is the
opt-out.
