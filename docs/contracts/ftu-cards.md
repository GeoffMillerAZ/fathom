# FTU-0/1: feature-purpose reference cards

Status: approved by rule W2-3 after independent LAND review; no implementation, reading, or row completion claimed.
Authority: `docs/design/feature-tuning.md` FT-D1..D5;
`docs/plans/feature-tuning.md` FTU-0/1; ADR-082 decisions 1..6;
ADR-110 decisions 1..9 (proposed, owner-directed work); wave rulings W2-3,
W3-2/W3-4 and T2-3. WPF-3 still blocks FTU-0 completion.

## Binding context and decisions

The intent router, `docs/intent/scope.md` compatibility surface and
`scope/non-goals.md`, principles P7/P18/P22/P24, invariants I7/I12/I35/I38/I53/I62,
and futures F50/F64/F65 govern. Freshness is disclosed, missing evidence earns no
conclusion, proof is deterministic, and existing primer/edit/hook budgets remain.
I62 is not generally enforced yet; its stated boundary still binds this contract.
No future promotion or revisit trigger is established by this documentary work.
No compatibility surface changes here. Do not build a live map, second freshness
algorithm, sampler, table, scheduler, surface inventory, knob writer or runner.

| Decision | Choice and governing evidence |
|---|---|
| D1: reference, never ownership | Seed `Lab.ID` and quality/harness names are references. No `surface`, `features`, new lab id, or new closure list. FT-D1; `internal/labs/load.go` `Lab`, `LoadLabs`; `closure.go` `CheckLabClosure`. |
| D2: five small file records | Closed `#TuningCard`, exactly five committed feature mappings, non-recursive loader. FTU-0/1, FT-D1. Existing `LoadLabs` ignores directories, so `tuning/` cannot acquire surfaces. |
| D3: reuse the actual seams | `LoadLabs(dir string, knownSurfaces []string) ([]Lab, error)` and `CheckLabClosure(dir string, shipped []string) error` remain unchanged. Reuse `QualityReading` and `KillCriterion`; add tuning-local typed knobs without changing `LabKnob` or any existing API (W3-4/C1). LAB-1/2/3; FT-D1. |
| D4: distinguish policy from proof | Cards specify bars and point to FTU-2 preregistration; loading never registers a bar or measures an outcome. FT-D1, FTU-2, correction 1/BX-40; ADR-082 decisions 4..6. |
| D5: file-local schema grant | New `tuning.go` embeds `tuning.cue` and validates it independently; production schema dispatch is an integrating-helper request. T2-3, `schema/AGENTS.md`. |
| D6: existing report style | Add a file-only report builder/renderer; request serial shared wiring after WPF-3, ahead of FTU-3/5. FTU plan's serial order; `instrument_adoption_labs.go` `adoptionLabsSection`/`renderAdoptionLabs`. |

`internal/transcript/events.go` already supplies
`DecodeTranscriptEvents(path, salt string, fileIndex int) Result`, preserving physical
line/block order and sanitized events. LAB-7 remains open; its frozen LAB/WR handoffs
are pending under T2-2. This contract neither replaces that decoder nor assumes it
has the pending phase/receipt semantics. FTU-0/1 decode no transcripts and consume
no WPF receipts. Later readings wait for the explicit frozen handoffs.

## Serial lane ownership

Every path below is NEW. Read its ancestor instructions before implementation.
Each lane writes its real failing tests first, records RED before implementation,
then runs the bounded proof with `-race` under the lane environment and test lease
when heavy. W3-2: critic checks every assertion below; a weaker test is FIX.
No implementation lane edits a shared file as an incidental prerequisite.

| Group | Exact owned paths | Handed / completion boundary |
|---|---|---|
| A+B: independent schema, loader and report functions | `schema/labs/tuning.cue`, `schema/labs/tuning.go`, `schema/labs/testdata/tuning_valid.cue`, `schema/labs/testdata/tuning_invalid.cue`, `internal/labs/tuning_cards.go`, `internal/labs/tuning_cards_test.go`, `cmd/fathom/instrument_adoption_tuning.go`, `cmd/fathom/instrument_adoption_tuning_test.go` | T2-3 grants the independent validator; current LAB-1/2/3 APIs supply seeds. All schema assertions live in the already-owned `internal/labs/tuning_cards_test.go`. No `schema.go`/`schema.cue` edits. Function-level report proof until integrating-helper wiring. |
| C: committed cards and reconciliation | `calibration/labs.d/tuning/maps.yaml`, `calibration/labs.d/tuning/semantic-layers.yaml`, `calibration/labs.d/tuning/freshness.yaml`, `calibration/labs.d/tuning/resource-envelope.yaml`, `calibration/labs.d/tuning/edit-obligations.yaml`, `cmd/fathom/instrument_adoption_tuning_cards_test.go`, `docs/bench/readings/feature-tuning/ftu-1-card-reconciliation.md` | A+B's loader; no dependency on implemented FTU-2. Exactly five mappings and existing seed/closure proofs. |

The owner-directed correction settles MERGE A+B: one bounded implementation lane
owns eight paths, then C runs as a separate serial lane with seven paths: exactly 15 implementation paths.
No additional ownership request is needed. T2-3 governs independent schema placement;
W3-2 governs full schema assertions in the existing owned test file.
This contract lane owns only this file and its ignored scratch/handback. It changes
no plan checkbox or changelog fragment.

## Exact CUE and Go card shapes

`schema/labs/tuning.cue` uses package `labs`, self-contained private prefixed
helpers (so it compiles alone without `schema.cue`). Define `#TuningText` as a string
containing non-whitespace, `#TuningSHA` as 64 lowercase hex characters,
`#TuningCommit` as 40 or 64 lowercase hex characters, `#TuningLabID` with the current
`#LabID` pattern. `#TuningPath` is a nonempty repository-relative slash path: no
absolute path, drive prefix, backslash, empty, `.` or `..` segment. Go also checks
these conditions. A knob location may append `:<symbol-or-setting>` to such a path;
validate the path before the first colon. No machine path or network URI is a source.

All fields below are required. Every object is closed, including list elements;
all strings are `#TuningText` except the tighter shapes/enums stated here. Lists
marked `+` have at least one element; other lists may be empty but are required.
There are no defaulted CUE values, optional numeric effects or arbitrary attributes.

```text
#TuningCard: {
  feature: maps | semantic-layers | freshness | resource-envelope | edit-obligations
  labs: []#TuningLabID
  quality: +[]{instrument: text, reading: path}
  firing: {mode: seed | n/a, reason: text}
  purpose_metrics: +[]text
  population: text
  eligibility_source: text
  comparison: {without: text, with: text}
  knobs: +[]{name: text, kind: text | cap | threshold | membership | default,
            location: text}
  inputs: +[]{path: path, commit: commit, sha256: sha, purpose: text}
  reading: path
  bar: {policy: text, source: text, preregistration: path}
  kill: +[]{criterion: text, reading: path}
  wpf: {preflight: [W0, W1, W2, W3, W6], requirement: text}
  default_state: unchanged_candidate_off
}
```

Here `text`, `path`, `commit`, `sha` stand for the helpers above. Enum members are
quoted CUE strings. `quality.reading`, `reading`, `kill.reading`, and
`bar.preregistration` name repository paths; a planned reading/prereg path need not
exist to load a card. `source` is a cited policy section/row, not a claimed measurement.
`bar.preregistration` is exactly `docs/bench/prereg/feature-tuning.md` in all five
committed cards; no loader probes it. CUE conditional constraints enforce: maps
has exactly the two distinct brief seed ids in either order; semantic-layers/edit-obligations
have exactly one `lab-obligations` id; freshness/resource-envelope have empty `labs`
and `mode: "n/a"`. Cardinality and membership are enforced together in CUE, so
no repeated lab id can pass schema validation.
The other three use `mode: "seed"`; that means consult each seed's actual firing
mode (session brief is itself n/a), never synthesize a new firing question.
Go validates exact lab sets without dependence on caller list order.

In `schema/labs/tuning.go` expose only:

```go
// TuningSource embeds tuning.cue, independently compilable.
var TuningSource string // go:embed tuning.cue
func ValidateTuningCard(data []byte) error

type TuningValidationError struct { Diagnostic string `json:"diagnostic"` }
func (e *TuningValidationError) Error() string
```

`data` is a single JSON object (not a file path or YAML); compile `TuningSource`,
lookup `#TuningCard`, unify JSON and `Validate(cue.Concrete(true))`. Invalid JSON,
schema or incomplete data returns `*TuningValidationError`, identifying the bad
field. No context, I/O, caches, mutable registry, dependency or existing schema edit.
YAML conversion belongs to the loader, using the already imported CUE YAML decoder.
Malformed or multi-document YAML is `TuningCardSchema`, wrapping a
`TuningValidationError` with a document diagnostic; later schema failures wrap the
validator error unchanged. Field diagnostics apply when a field can be identified.

In `internal/labs/tuning_cards.go`, exact exported types and entrypoint:

```go
type TuningFeature string
type TuningFiringMode string
type TuningDefaultState string
type TuningKnobKind string

type TuningKnob struct {
    Name string `json:"name"`
    Kind TuningKnobKind `json:"kind"`
    Location string `json:"location"`
}

type TuningFiring struct {
    Mode TuningFiringMode `json:"mode"`
    Reason string `json:"reason"`
}
type TuningComparison struct {
    Without string `json:"without"`
    With string `json:"with"`
}
type TuningInput struct {
    Path string `json:"path"`
    Commit string `json:"commit"`
    SHA256 string `json:"sha256"`
    Purpose string `json:"purpose"`
}
type TuningBar struct {
    Policy string `json:"policy"`
    Source string `json:"source"`
    Preregistration string `json:"preregistration"`
}
type TuningPreflightRung string
type TuningWPFRequirement struct {
    Preflight []TuningPreflightRung `json:"preflight"`
    Requirement string `json:"requirement"`
}
type FeatureTuningCard struct {
    Feature TuningFeature `json:"feature"`
    Labs []string `json:"labs"`
    Quality []QualityReading `json:"quality"`
    Firing TuningFiring `json:"firing"`
    PurposeMetrics []string `json:"purpose_metrics"`
    Population string `json:"population"`
    EligibilitySource string `json:"eligibility_source"`
    Comparison TuningComparison `json:"comparison"`
    Knobs []TuningKnob `json:"knobs"`
    Inputs []TuningInput `json:"inputs"`
    Reading string `json:"reading"`
    Bar TuningBar `json:"bar"`
    Kill []KillCriterion `json:"kill"`
    WPF TuningWPFRequirement `json:"wpf"`
    DefaultState TuningDefaultState `json:"default_state"`
}
func LoadFeatureTuningCards(dir string, seedCards []Lab) ([]FeatureTuningCard, error)

type TuningCardErrorKind string
type TuningCardError struct {
    Kind TuningCardErrorKind `json:"kind"`
    Path string `json:"path"` // basename, not caller's absolute directory
    Reference string `json:"reference"` // field or unresolved id/name
    Cause error `json:"-"`
}
func (e *TuningCardError) Error() string
func (e *TuningCardError) Unwrap() error
```

Export these constants with their named type (no public bare-string closed fields):

| Type | Names and exact values |
|---|---|
| `TuningFeature` | `TuningMaps="maps"`, `TuningSemanticLayers="semantic-layers"`, `TuningFreshness="freshness"`, `TuningResourceEnvelope="resource-envelope"`, `TuningEditObligations="edit-obligations"` |
| `TuningFiringMode` | `TuningFiringSeed="seed"`, `TuningFiringNA="n/a"` |
| `TuningKnobKind` | `TuningKnobText="text"`, `TuningKnobCap="cap"`, `TuningKnobThreshold="threshold"`, `TuningKnobMembership="membership"`, `TuningKnobDefault="default"` |
| `TuningDefaultState` | `TuningDefaultUnchangedCandidateOff="unchanged_candidate_off"` |
| `TuningPreflightRung` | `TuningW0="W0"`, `TuningW1="W1"`, `TuningW2="W2"`, `TuningW3="W3"`, `TuningW6="W6"` |
| `TuningCardErrorKind` | `TuningCardRead="read"`, `TuningCardSchema="schema"`, `TuningCardFilename="filename"`, `TuningCardDuplicate="duplicate"`, `TuningCardUnknownLab="unknown_lab"`, `TuningCardUnknownInstrument="unknown_instrument"` |

W3-4/C1 requires `TuningKnob.Kind` to use the closed five-value Go vocabulary
as well as CUE validation. Define it only in the new `tuning_cards.go`; the existing
`LabKnob` API (`internal/labs/load.go:LabKnob`, whose `Kind` is `string`) stays
unchanged. `QualityReading.Instrument` is an open caller-derived reference, not a
closed vocabulary; reuse it and `KillCriterion` unchanged. No new registry/API for
future use.

Loader rules, in order: read directory entries by name; inspect regular `.yaml`
files only, ignoring directories and unrelated extensions; refuse symlink `.yaml`
entries with `TuningCardRead` (never follow out of the card directory). Convert one
YAML document to JSON, call `ValidateTuningCard`, decode, check duplicate feature, check filename equals
`string(Feature)+".yaml"`, check duplicate list references, then cross-check
all lab/instrument references. Known labs are exactly caller-provided validated
`Lab.ID`s. Known instruments are the union of those cards' `Harness` entries and
`Quality[].Instrument` entries, excluding empty/`none`; no static second catalog.
Thus the current seeds admit `replay`, `scenario`, `hook-path`, etc.; admitting a name
proves a reference exists, not that its measurement has run. Caller obtains seeds
through `LoadLabs`, never synthesizes them from tuning files or `features` labels.

Sort returned cards by feature, lab ids lexically, quality by instrument then reading,
metrics lexically, knobs by location then name then kind, inputs by path then commit
then SHA256 then purpose, kill by reading then criterion. Keep WPF's exact rung order.
Schema validation takes precedence over every Go-only duplicate, filename or
reference check (W3-2; the fixed `#TuningCard` shape above). In particular
duplicate WPF rungs violate the exact five-element tuple and return `TuningCardSchema`,
not `TuningCardDuplicate`; missing, extra or reordered rungs do likewise. Repeated
lab ids also violate the feature-specific exact lab set (two distinct brief seeds,
one obligations seed or empty set), so return `TuningCardSchema`. Assert both
exceptions without weakening the schema to reach a later error.
For otherwise schema-valid lists return `TuningCardDuplicate`: quality repeated
(instrument,reading), inputs repeated paths (even with different pins/purpose),
and exact duplicate purpose metrics, knobs or kill elements. These are all remaining
list checks; WPF and labs have the schema exceptions just stated. Duplicate feature
across schema-valid files is checked before filename, so a second maps card under
another filename reports `TuningCardDuplicate` rather than `TuningCardFilename`.
Invalid schema in that second file still reports `TuningCardSchema` first.
All exported slices are nonnil, including
empty results and every error return (`[]FeatureTuningCard{}`, never partial cards).
Missing directory is `TuningCardRead`, not a successful empty inventory. An existing
empty directory returns `[]`, and cannot qualify reconciliation. Preserve wrapped
filesystem cause for `errors.Is/As`; the report prints a relative category/field,
not an absolute cause path. No arbitrary new size cap/limit is introduced.

## Required five-card content

The following are specifications for group C, not newly observed measurements.
No numeric effect/n/split/interval/window is invented or borrowed from seed helped
bars. FTU-2 owns those registrations and independent labels; preserve existing bars
below by citation. Every card carries all components, even currently unknown ones.

| File / seed refs / instruments | Purpose, population and eligibility | Comparison, $0 inputs and bar / kill |
|---|---|---|
| `maps.yaml`; `lab-session-brief`, `lab-subagent-brief`; `replay`, `hook-path` | Repeated derivation per all later-session read/search actions; censored tokens to first oracle-correct edit; useful question coverage and resolved/attempted anchors; answer-time fresh/stale/unresolved/unknown anchors. Registered project-window sessions and all independent questions, unknowns included. Eligibility: FTU-2 independent labels, prior map frozen at session boundary and anchor truth at later revision; repeated file overlap alone is insufficient. | Without map assistance versus with stored maps; brief variant also compares cold start and maps without brief. Immutable public fixture/map-input sources and frozen replay inputs at $0; live edit-token benefit stays unmeasured without actual telemetry. FT-D2 bar: every asserted anchor resolves, no correctness regression, registered reduction in repeated exploration and lower token estimate, existing budgets. False fact, no reduction or breach kills brief variant. |
| `semantic-layers.yaml`; `lab-obligations`; `scenario`, `hook-path` | Scoped precision, recall, unknown residue, validated eligible use and authoring cost separately. Independent source/scope pairs and operator authoring exercise at matched coverage; FTU-2 labels and existing compiler scope traps define eligibility, never a model-authored oracle. | Without selected pack (raw fallback) versus same inputs with selected pack, retaining other plugin capabilities. Frozen compiler responses/scope fixtures, citation sweep inputs and operator timing; no paid compilation. FT-D3 bar: existing precision 0.90, no unsupported anchor or scope escape, no increased cost at matched coverage. Scope escape, precision miss or extra cost without coverage gain kills candidate. |
| `freshness.yaml`; no labs; `hook-path` | Index age at answer, independently checkable HEAD contradictions (disclosed and undisclosed), admitted-change-to-serving latency. All answer assertions and fixed fixture changes; unknown HEAD truth, coalesced/rejected/unfinished refreshes remain named. Eligibility: independently checkable HEAD truth, not index age or agent use. | Existing refresh/disclosure behavior versus proposed setting on same fixed change sequence; no daemon-free correctness claim. Public fixture revisions, frozen request/accept/complete traces with injected clock; reuse queue and semantic sweep, no sampler. FT-D4 bar: zero undisclosed contradictions and FTU-2 latency bound inside resource budgets. Hidden stale answers or backlog growth at matched load kills variant. Firing n/a: queue/answer correctness has no agent-choice firing class. |
| `resource-envelope.yaml`; no labs; `hook-path` | Host footprint, correlated CPU seconds per completed ingest and per-harness/channel hook p95, with input size/graph stage. Complete comparable workload/tenant windows; eligibility: existing RT clauses and attributable job/window evidence. Missing CPU correlation is unknown, never process CPU assigned to an ingest. | Existing settings versus proposed setting at matched workload; resource/day logs and tenant harness inputs at $0. FT-D4 bar: original RT-M1/4/8/15 clauses/windows and descriptor-derived hook budget; no new fleet-wide latency bar. Exit rung, breach or unpriced regression kills candidate. Seven UTC days remain seven UTC days; fixture PASS never replaces sustained reading. OS gauges remain separately named. Firing n/a: resources are quality, not choice firing. |
| `edit-obligations.yaml`; `lab-obligations`; `replay`, `hook-path` | Correct scoped rule at eligible edits, deadline/budget correctness and oracle-correct acted-on lift. All preregistered edits including clean edits and non-use; eligibility: AD-20/21 evidence through LAB-11 and FTU-2 independent target/scope labels, never exposure or a relabelled clean edit. | Plugin minus obligations versus with obligations; candidate text/order compares matched delivery holdout through BEN-4. Frozen-response replay for target/scope/deadline/caps, forward holdout for lift; replay alone cannot keep a knob. FT-D5 bar: no wrong scope/cap regression, positive acted-on lift at FTU-2 uncertainty bound. Wrong scope, breach, or confirmed null/harm in forward window kills variant; keep clean-file silence. |

`quality[].reading` points at the existing source test/reading paths used in that
card, not fictional measurements. `reading` and every tuning `kill[].reading` are
respectively `docs/bench/readings/feature-tuning/ftu-m1-maps.md`,
`ftu-m2-layers.md`, `ftu-m3-freshness.md`, `ftu-m4-resources.md`,
`ftu-m5-obligations.md` under that same directory. A proposed tuning kill does not
replace the seed's existing K1/K4 kill. Keep seed helped-arm meanings intact.

Exact knob locations (one record per knob, descriptive names from FT-D2..D5):

Maps use these exact source anchors and roles (FT-D2; FTU-1). Each location record names one anchor from this table; roles distinguish current controls from
candidate choices. No invented configuration setting is implied.

| Map knob / role | Existing source anchors and current semantics |
|---|---|
| Pointer versus expansion | `cmd/fathom/maphooks.go:mapShowHint` emits the explicit `map show --budget 1200` recovery; `cmd/fathom/maphooks.go:renderMapPointer` emits the bounded pointer, not inline expansion. Candidate delivery choice stays within these seams. |
| Entry/delivery cap | `cmd/fathom/maphooks.go:mapPointerLineMax` (280 bytes), `cmd/fathom/maphooks.go:mapPointerMaxLines` (2 maps) and `cmd/fathom/maphooks.go:mapPointerFreshCap` (60 checked entries) bound pointer delivery/checking; `cmd/fathom/maphooks.go:maxBriefMaps` caps brief selection at 3. These are distinct limits, not an invented expansion-entry setting. |
| Task overlap selection | `cmd/fathom/maphooks.go:briefMaps` (currently line 652) ranks named maps then unique recent-file overlap, retaining stable recent-use order; `cmd/fathom/maphooks.go:mapPointerLine` ranks matched anchored entries for a prompt/edit touch. |
| Recent-use horizon | `cmd/fathom/maphooks.go:loadAliveMapsCut` sorts alive candidates by `LastUsed`, id tie-break; `cmd/fathom/maphooks.go:threadRecentFiles` derives recent files from current worktree/open-note pointers. There is no configurable recent-use horizon here. `internal/explmap/lifetime.go:mapLifetime` reports idle time without expiring maps; a candidate horizon is a reading proposal, not an existing setting. |
| Composition depth | `internal/explmap/model.go:MaxDepth` (currently line 30) sets 4; `internal/explmap/resolve.go:ResolveMap` enforces it. Tune only within this existing limit. |
| Primer share | `cmd/fathom/maphooks.go:joinSubagentPrimer` preserves the map id within the supplied budget; `cmd/fathom/sessionhook.go:composePrimer` includes `briefMapLines` inside the thread brief and whole-primer cap (`sessionPrimerBudget`). Preserve I12. |
| Injection on/off selection | `cmd/fathom/maphooks.go:threadsHoldMaps` gates map channels; `cmd/fathom/maphooks.go:briefMapLines` selects alive maps for the primer and `cmd/fathom/maphooks.go:withMapPointer` / `cmd/fathom/maphooks.go:withPromptMapPointer` deliver edit/prompt pointers. These are existing call seams, not a new on/off config key. |

`cmd/fathom/brief.go` is the fleet narrative, not map injection.
`cmd/fathom/mapshow.go` renders requested expansion; it does not select the brief's
overlap. Neither is an injection/overlap knob anchor. Pointer constants currently
start at `maphooks.go:385`; use named symbols, not fragile line-only identities.

- Layers: pack/corpus selection at `internal/semantic/packs.go`; section/digest
  boundaries at `internal/semantic/compile.go`; payload order/caps/raw fallback at
  `cmd/fathom/edithook.go`. No tier/citation weakening.
- Freshness: coalescing/start waits at `internal/refresh/refresh.go`, store backoff at
  `cmd/fathom/servehttp_store.go`, disclosure/map anchor checks at
  `internal/explmap/fresh.go`. No second freshness writer.
- Resources: queue settings at `internal/refresh/refresh.go`, host budget/admission at
  `internal/tenancy/budget.go`, payload cap at `cmd/fathom/edithook.go`.
- Edit obligations: order/scope at `internal/semantic/packs.go` and
  `cmd/fathom/edithook.go`; edit payload budget at `cmd/fathom/edithook.go`.

Group C pins each immutable `inputs` entry to an existing tracked public fixture or
source file with its actual Git commit and SHA256 of that commit's file bytes.
Examples available now: `internal/connector/testdata/fauxtown/orders-api/main.go`,
`internal/semantic/compile_test.go`, `internal/refresh/refresh_test.go`,
`internal/tenancy/daysummary_test.go`, `cmd/fathom/resources_test.go`.
They pin the $0 fixture machinery, not independent outcome labels or private logs.
Card prose must name additional measurement inputs still missing (FTU-2 labels,
transcripts, observed traces, timed authoring, forward holdouts, CPU attribution).
Never commit private transcripts/logs, fabricate digests of absent inputs, or claim
source-test pins freeze future labels. Later readings pin their actual admitted
inputs separately. Card loading is structural/reference validation; it performs
no Git invocation, digest audit, fixture execution or metric collection.

All cards' `wpf.requirement` explicitly requires both arms' W0-W3/W6 scripted
preflight before any agent action, baseline permitted tools/startup/channel evidence,
profile identity and receipt join in the later reading. Daemon-free exceptions retain
their existing scope. W4/W5 are outcomes, never exclusions; probe calls are not agent
use. Post-agent diagnostics cannot retroactively void valid cells. Preserve all
assigned-arm outcomes and zero-call lines even when a component metric is missing.
Any use-conditioned secondary is labelled exactly
"biased toward the product: selects on tool use" and supports no headline or ruling.
No WPF receipt type, validity function, profile digest API or eligibility grader is
specified here; those belong to the WPF/LAB/WR contracts.

## State and report seam

Specified means policy text plus citation exists. Registered means FTU-2's supplied,
validated preregistration freezes effect/n/split/uncertainty/window before data.
Unmeasured means the actual metric evidence is absent or insufficient. These are
separate dimensions: a valid card is specified AND can be wholly unmeasured. A
path string, a file's existence, seed effect size, fixture PASS, tool firing, or a
ledger PASS for the owning lab is never proof of a registered tuning bar/outcome.
The current loader does not certify registration even if the target path exists.

All five defaults mean existing product behavior unchanged; tuning candidate off.
This does not claim the existing feature itself is off. LAB-36/TUN-1/TUN-3 own
keep/revert/confirmation, TUN-2 owns defaults. No setters or outcome evaluator here.
Later readings emit PASS/FAIL/INCONCLUSIVE with component measures via their existing
owners; FTU-0/1 emit no result ledger row and add no second verdict vocabulary/API.

In the NEW `cmd/fathom/instrument_adoption_tuning.go`:

```go
type AdoptionFeatureTuning struct {
    CardsDir string `json:"cards_dir"`
    Cards []labs.FeatureTuningCard `json:"cards"`
    CardError string `json:"card_error"`
}
func adoptionFeatureTuningSection(root string, seedCards []labs.Lab,
    seedErr error) AdoptionFeatureTuning
func renderAdoptionFeatureTuning(w io.Writer, section *AdoptionFeatureTuning)
```

Builder calls `LoadFeatureTuningCards` at `calibration/labs.d/tuning` only when
`seedErr == nil`. Otherwise it returns empty nonnil cards and a named seed-error
category. Missing/unreadable tuning inputs get a named error, never zero healthy
cards. Sanitize error output to relative paths/kind/reference; no private absolute
filesystem cause. Renderer returns no error (matching existing report style), writes
plain deterministic text with no ANSI, sorted feature names, purpose/components,
policy and source, population/eligibility, comparisons, knobs, immutable input pins,
kill and WPF requirements, preregistration pointer, default state and explicit lines
`bar registration: not established (FTU-2 handoff required)` and
`outcomes: unmeasured (no tuning reading evaluated)` for every loaded card. For
errors print `CARDS UNREADABLE` with the category; do not print registration or
outcome success. JSON carries complete cards and errors, no invented measurements.

Integrating-helper wiring requests, not files owned by A+B or C:

1. After WPF-3's call-site integration, add
   `FeatureTuning AdoptionFeatureTuning` with `json:"feature_tuning"` to existing
   `AdoptionLabs`. From `adoptionLabsSection`, pass its actual loaded seed cards
   and load error to `adoptionFeatureTuningSection`; skipped inventory/seed reads
   must pass an error too. Call `renderAdoptionFeatureTuning` from
   `renderAdoptionLabs`. Preserve WPF conformance output and all existing fields.
   Do not reload seeds from tuning data or run closure against tuning cards.
2. Route the production labs validation path's tuning records through
   `schema/labs.ValidateTuningCard`, preserving existing `Source` and `#Lab` dispatch.
   Identify the actual validation entry during integration rather than inventing
   a shared dispatcher name. The new loader already calls the independent validator.
3. Integrator adds its shared-call-site tests in its own explicitly owned turn;
   A+B's function tests cannot prove this wiring. WPF-3, LAB-3/LAB-5 dependencies,
   production reachability and verify still gate FTU-0. No box is checked here.

App-kit review: `docs/app-kit.md`, `appkit.adoption.json`, MCP guidance → intent →
`cli-kit`; capability search `cli output`/profile `cli` retrieved `cli.output`
(catalog `3ee2c1f`, capability `6ffa60ed`). `list_quality_work` for that capability
returned no authored items, hence nothing to retrieve/reuse. `contribution-loop`
review: no candidate; domain-specific card policy stays local. Its status-table/
NDJSON shape does not fit the existing nested text/JSON report, so preserve that
style. No new CLI, visual reference, dependency, runtime, theme or pin change.
This scoped divergence is recorded here because the lane cannot edit adoption docs.

## Implementing-lane RED and exact tests

A+B places ALL schema and loader assertions below in its already-owned
`internal/labs/tuning_cards_test.go`, importing the independent validator and
`TuningSource`. No separate schema test grant or shared schema edit is required.
The fixtures remain at the four T2-3 schema paths above. W3-2 requires the full
mutation assertions, not a reduced fixture smoke test.

A+B's schema tests in `internal/labs/tuning_cards_test.go`:

- `TestFeatureTuningCardSchemaValid`: compile embedded `TuningSource` alone; load
  `tuning_valid.cue` with one fully concrete case per feature, including empty labs
  for freshness/resources; marshal each object to JSON and require validator nil.
  Assert no fixture needs `schema.cue`, FTU-2 file, WPF receipt or new dependency.
- `TestFeatureTuningCardSchemaInvalid`: interpret every fully commented negative
  in `tuning_invalid.cue` as the current schema-test mutation convention does.
  Independently remove every required top/nested field; add unknown fields at
  every object level (including list elements); test whitespace-only text, every
  invalid enum, empty required lists, non-hex/wrong-length hashes/commits,
  absolute/backslash/traversal paths, wrong seed set/mode for each feature and W4,
  W5, missing, reordered or duplicate preflight rung. Each returns
  `*TuningValidationError` and diagnostic names the mutated field. Malformed JSON
  returns that type. Include repeated lab ids for each feature, and wrong lab
  cardinality; schema rejects these before Go reference checks. Valid controls must
  pass before each mutation.

A+B's loader tests in the same `internal/labs/tuning_cards_test.go`:

- `TestFeatureTuningCardLoad`: write valid YAML in reverse feature order; load with
  validated current-shaped seed fixtures. Assert every field decodes exactly,
  result/list sorting matches the rules above, and input seed data is unchanged.
  Empty labs for freshness/resources and seed n/a session firing remain intact.
- `TestFeatureTuningCardReferences`: remove a referenced seed; then separately
  replace a quality instrument with an unknown name. Assert respective typed
  `unknown_lab`/`unknown_instrument`, basename and exact unresolved reference,
  nonnil empty result. Add an instrument only to seed `Quality`, then only to seed
  `Harness`; both are accepted without a new static list. `none` returns
  `TuningCardUnknownInstrument`; empty instrument text returns `TuningCardSchema`
  before reference validation. Assert their exact field/reference and wrapped types.
- `TestFeatureTuningCardRefusals`: isolated wrong filename returns `TuningCardFilename`;
  a schema-valid duplicate feature in a later differently named file returns
  `TuningCardDuplicate` before filename checking. Independently test schema-valid duplicate list cases: quality pair, input path with different pins/purpose, exact
  metric, knob and kill; each is `TuningCardDuplicate` with the offending field.
  Duplicate lab ids and duplicate WPF rungs (both replacement and appended rung)
  return `TuningCardSchema` with `labs`/`wpf.preflight` diagnostic, never duplicate.
  Combine invalid schema with wrong filename/duplicate feature/unknown reference:
  each returns `TuningCardSchema` before those Go checks. Other invalid fields and
  multi-document/broken YAML return `TuningCardSchema`; missing directory is
  `TuningCardRead`. Assert each kind via `errors.As` to `*TuningCardError`, basename
  and field/reference; schema cases unwrap to `*TuningValidationError`.
  Every refusal returns nonnil empty results and no partial success.
  A dangling symlink YAML is `read`; missing-directory errors preserve
  `errors.Is(err, os.ErrNotExist)`. Never rely on chmod failures under privileged
  runners. Unrelated extensions/subdirs are ignored.
  Empty existing directory is successful nonnil empty, not five-card reconciliation.
- `TestFeatureTuningCardJSON`: recursively assert every exported slice is nonnil
  in loaded results, empty results and error results; marshal empty labs as `[]`,
  never `null`; all introduced field keys are exactly the lower_snake_case tags.
  Assert `TuningKnob.Kind` has named type `TuningKnobKind`, each of its five typed
  constants round-trips with the exact string above, and all other kind strings
  fail schema validation. This asserts the Go field type, not merely JSON validity;
  the existing `LabKnob.Kind` remains unchanged.

A+B's report tests in `cmd/fathom/instrument_adoption_tuning_test.go`:

- `TestFeatureTuningCardAdoption`: call new builder on scratch root with supplied
  validated seeds and valid tuning files. Assert exact cards directory, sorted
  complete cards and empty error. Existing seed inventory and surfaces unchanged.
  Missing tuning directory, corrupt card, unknown reference and supplied seed
  error each give nonempty named error, no partial cards and nonnil slices; a seed
  error prevents a malformed tuning file from masking the seed failure.
- `TestFeatureTuningCardUnregistered`: with NO FTU-2/prereg file, valid cards load;
  renderer prints specified policy, registration-not-established and unmeasured
  lines for every feature plus unchanged-candidate-off; JSON has no result/effect/
  verdict/default-qualified fields. Adding an arbitrary prereg file or seed ledger
  PASS does not change output. This test does not assert FTU-2 exists or works.
- `TestFeatureTuningCardReport`: text and JSON preserve every card's purpose,
  population/eligibility, comparisons, knobs, input pins, bar/kill/WPF/default;
  text has no ANSI or absolute temp path. Error text says `CARDS UNREADABLE`, never
  reports a passing outcome or healthy zero; JSON cards are `[]`, not `null`.
  These are function tests, explicitly not shared report reachability proof.

C's `cmd/fathom/instrument_adoption_tuning_cards_test.go`:

- `TestCommittedFeatureTuningCardsLoad`: use `shippedLabSurfaces`, `LoadLabs`, then
  new loader; assert exact five filename/feature mappings and exact lab sets in
  the table, required component prose/knob locations/input pins, existing instrument
  names, every bar's governing citation and FTU-2 pointer, all WPF/default/firing
  requirements, and absence of `surface` ownership fields. Independently hash each
  input via `git show <commit>:<path>` and compare SHA256 (immutable, not worktree
  bytes). Assert pinned paths exist at the named commit. Do not require planned
  readings/preregistration or future independent labels to exist. Validate all
  named existing source/knob paths AND each map anchor's symbol identity and semantic role
  from the map table above. Parse Go declarations/bodies (standard `go/parser` / `go/ast`):
  require `briefMaps` to rank `Named` then `Overlap` and cap by `maxBriefMaps`;
  `mapPointerLine` to count `touch.touches` matches and bound `mapPointerMaxLines`;
  pointer constants to be 280/2/60; `loadAliveMapsCut` to sort `LastUsed` with id tie-break;
  `threadRecentFiles` to consume worktree/open-note paths; `mapLifetime` not to expire
  by idle duration; `MaxDepth` to be 4 and `ResolveMap` to enforce it;
  `composePrimer` to call `briefMapLines` under `threadsHoldMaps` and retain the cap;
  `joinSubagentPrimer` to preserve the id under its budget; and the named hint,
  pointer renderer and edit/prompt wrappers to call the delivery seams stated above.
  Assert each committed knob location matches its prescribed anchor/role; a location
  pointing overlap at `mapshow.go` or injection at `brief.go` must fail even though
  both files exist. No fabricated config key. Preserve metrics
  as unmeasured until actual readings, not a numeric zero.
- `TestCommittedSeedCardsLoad`: NEW cmd-package test wrapper, not a duplicate of
  the existing internal-package test: call real shipped inventory and `LoadLabs`,
  assert the three reused seed ids exist, their exact current surface lists and
  helped arms remain unchanged and tuning adds no seed. Existing internal test
  runs separately. Use expected current seed metadata, not a hardcoded whole
  shipped-surface inventory.
- Run existing `TestCommittedLabsCloseOverShippedSurfaces` unchanged: its real
  inventory must close, and its added synthetic surface must still fail closure.
  In the new committed-card test, also assert closure succeeds before/after tuning
  load and original seeds are equal, giving both closure checks without a new owner.

Proof commands for implementing lanes (never this documentary lane's claimed proof):

```sh
# A+B (all schema assertions in internal/labs/tuning_cards_test.go):
go test -race ./internal/labs ./cmd/fathom -run '^TestFeatureTuningCard'
# C (cmd wrapper makes all three names real in this package):
go test -race ./cmd/fathom -run '^TestCommitted(SeedCardsLoad|LabsCloseOverShippedSurfaces|FeatureTuningCardsLoad)$'
go test -race ./internal/labs -run '^TestCommittedSeedCardsLoad$'
```

For each new test, RED is the actual named compile failure or failed assertion on
its lane's parent (missing symbol/schema/cards), followed by GREEN after the change;
record the exact output. Do not fake RED by adding a skipped test or use an already
passing closure test as new-feature RED. The contract author runs no implementation
proof and claims none. The foreman owns the full gate.

C's reconciliation report lists exactly five mappings, exact filenames, reused ids,
existing instruments, pinned input audit, required-field load evidence and both
closure results. End with `FTU-1 verdict: PASS` only if all that row's checks pass;
otherwise `FTU-1 verdict: FAIL` and concrete gaps. Explain that this is reconciliation,
not feature benefit or FTU-2 registration. Completion still requires the integrating
helper's dependency/wiring proof and `fathom verify` before any assigned plan box.

## Pending integration gates

1. WPF-3 and LAB/WR frozen handoffs: production call-site and receipt integration
   remain with the integrating helper after those contracts land (T2-2/T2-3 and
   FTU-0's dependencies). No row completion or provisional receipt API here.
2. FTU-2 registration format is not handed to this lane. Registration remains
   explicitly unestablished until that owner supplies the validated format and a
   separately owned integration (FT-D1/D4, FTU-2, ADR-082 decisions 4..6).
   Never infer registration from path existence. The merged grant needs no decision
   or additional ownership request.

## Decision models

No new judged site applies. Card validation, reference closure, report state and
immutable-input audits are deterministic with provider `none`; no calibration band
applies. Existing experimental profiles stay off as opt-out. FTU-2's independent
oracles and later readings, including their kill conditions, measure benefit; no
model supplies missing labels, telemetry, registration, scope or receipt facts.
