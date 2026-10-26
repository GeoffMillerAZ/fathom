# A bounded concern site at the file-output boundary

GT12A / MC, 2026-10-24. Implementation admission under [0042](../intent/decisions/0042-concern-sites-require-independent-disclosure-pins.md).
Maintainer owns this bounded synthetic source-review receiver. It extends W1, not
its budget or the two-slot ceiling. The full GT12 objective remains open.

## MC-0: an explicit optional build

The ordinary `go build ./cmd/command-guard` artifact excludes the model/HTTP engine.
It rejects any classifier selection, permission set, policy pin or classifier object
as incomplete/unsupported before source reads or scanner dispatch. No silent ignore.
`go build -tags command_classifier ./cmd/command-guard` includes the exact receiver
below. Fixed help text identifies capability; executable hashes identify the tested
bytes. The qualifier remains one artifact and qualifies the supplied guard's actual
behavior. Build the tagged guard for the fixture profile and the ordinary guard for
core-only use. Run tagged race/vet checks in addition to the unchanged full gate.

The initial combined artifact failed MC-A10: median ratio 1.674609 on the declared
corpus, with byte parity. Package initialization and help-only timings implicate
startup/linked dependencies, not a selected model call. Preserve failed evidence and
measure the same corpus after separating builds. Do not claim the tagged artifact
meets the ordinary build's performance bar. No extra process, runtime plugin,
dependency or new shared abstraction is introduced.

## MC-1: one real receiver, one semantic question

Insert the optional site after whole-selected-input regex and optional Gitleaks
checks, before file output projection. Reuse `decision/llm.NewLLM` and the existing
OpenAI-compatible `LLMClient`. Ask one choice question over the
complete selected UTF-8 text: `concern`, `no-concern`, `unknown`. A protected criterion
supplies the contextual review instructions. Exact hash/path/control/size facts stay
deterministic. One aggregate call avoids per-file request multiplication.

State is a JSON-framed ordered input list containing index, original byte length and
text, without filenames. Include omitted-from-view content. Preserve original text
and multiplicity, not a summary or prompt that silently truncates. Treat state as
untrusted data, never instructions. Structured formatting cannot make its claims true.

Build the base decision request with fixed question ID `sensitivity`, then derive
the actual ID as `sensitivity.` plus `decision.RequestKey(baseRequest)`. Require that
sole derived answer key, without fallback to the fixed ID. This deterministic
context witness rejects accidental replay across changed state, criterion or options.
It is not peer/model authentication, semantic freshness or residency proof; endpoint,
raw model assertion and deployment qualification remain separate. Do not publish the
derived key or source/context hashes. Allow its bytes within existing wire budgets.

Concern => blocked; unknown/refusal/malformed/missing/late/failed result => incomplete.
Both withhold all source rows. No-concern adds no hold only after every mandatory
check passes. No model can waive a floor, approve an effect or prove sensitive-data
absence. No hidden reasoning or self-reported confidence is displayed or persisted.
Existing LLM probabilities are uncalibrated; this profile does not use them as a
confidence threshold. Stronger judgment or human review requires a separately
selected eligible deployment/authority, not an automatic fallback or retry.

## MC-2: independent disclosure scope

Scope adds optional `requiredClassifierIds`, exactly absent/empty or
`["semantic-concern"]`, plus `permittedClassifierSHA256`: up to 32 unique exact
lowercase SHA-256 input pins, and `classifierPolicySHA256`, the exact raw policy-file
digest. These pins are absent/empty when no classifier is selected. Policy's optional
`classifier` must match required scope. Hash and decode the same bounded policy-file
snapshot once, retaining its digest privately in memory. Scope's policy pin must
match before source reads or any scanner/model call. Do not hash a later file read
or trust a caller-provided private-state field. This binds every policy byte,
including criterion, endpoint and requested/response model selections, independently
of the adjacent criterion self-hash. Formatting changes require a new policy pin.

Check every requested file digest against the protected disclosure set in `validate`,
before source reads or external scanners. Captured bytes must then match that request
digest before model serialization. Additional permitted hashes are permissions,
not evidence that those inputs were read or reviewed. MC-A1's zero-request claim
means zero classifier HTTP requests and zero optional scanner subprocesses.

The pins authorize this profile's public/synthetic test/review data only. They are
operator-selected facts, not an engine inference that a file is public. Keep both
scope and policy outside candidate write authority through dispatch. The policy pin
detects drift; it does not authenticate either file or replace protection. A source
label or a model saying "public" cannot
replace these pins. This profile does not accept private-input qualification claims.
Private local-only use still needs measured process/model residency, egress/logging,
disclosure policy and a criterion/model/prompt/cohort-bound acceptance decision.

## MC-3: versioned controls and strict transport

Policy `classifier` has closed fields:

| Field | Contract |
|---|---|
| `profile` | `loopback-concern/1` |
| `endpoint` | explicit `http://127.0.0.1:PORT/v1` or `http://[::1]:PORT/v1`; numeric canonical port 1..65535, no userinfo/query/fragment/encoded path |
| `model`, `expectedResponseModel` | nonempty printable ASCII, at most 128 bytes each; first requests, second checks the response assertion |
| `criterion`, `criterionSHA256` | non-whitespace valid text, at most 4096 bytes, exact SHA-256 of those bytes |
| `timeoutMillis` | 1..30000 for the entire classifier stage, including preparation and validation |
| `maxInputBytes` | 1..16384 aggregate original selected bytes |
| `maxResponseBytes` | 1..16384, with at most one overflow-probe byte |
| `maxTokens` | requested output limit 1..1024; not proof of billed or server-compute cost |

Only POST `/v1/chat/completions`, one connection/request attempt, no streaming,
tools, credentials, environment-derived configuration, DNS, proxy, redirects,
keepalive reuse or hosted fallback. Use a command-owned HTTP client/transport with
fixed numeric dialing, disabled proxy/compression, an 8 KiB response-header ceiling,
128 KiB encoded-request ceiling and the configured reply/deadline limits. Cap the
encoded request before dialing; do not truncate it. A size or deadline hold is not
a favorable classification. No model/server download, discovery or installation.

Read/close a bounded full HTTP reply before the existing client decodes it. Reject
invalid Unicode, duplicate JSON keys, trailing data, non-200 status, compressed data,
wrong response model, non-assistant/text response, refusal/tool call, multiple choices
or non-stop finish. Provider metadata/extensions may remain opaque and bounded;
they cannot alter these critical fields. Validate inner decision JSON strictly:
exact `answers.<derived-ID>`, exact choice and full finite probability map for the
three options, no unknown fields/questions and total probability within 0.001 of 1.
Null is not numeric zero. The selected choice must have the unique highest declared
probability; a mismatch or tie is a deterministic ambiguity/inconsistency hold.
Bound both JSON layers to 24 nesting levels.
Probabilities remain uncalibrated and are not used to establish confidence/authority.
Check the existing decision port result and completed HTTP witness together.

A loopback hop establishes a client destination only. A service can relay remotely,
change models or log content. Neither a model name nor its response authenticates
the process. No local-inference, no-egress, calibration or zero-cost claim follows.
The reused LLM port leaves generation sampling at the deployment default; its zero
temperature value means default, not explicit zero. Pin and measure actual native
generation settings during deployment qualification. Do not rewrite provider
payloads or claim semantic determinism from a nominal sampling setting.

An aggregate input with zero source bytes is deterministically incomplete/input_limit
without an HTTP call. Empty evidence is not a model no-concern result.

## MC-4: compatibility, receipts and truthful counters

Absent classifier preserves existing inspection JSON/text/compact bytes and checks. Add optional
`externalClassifier` to file receipts and every file format. Existing scanner fields
keep their meanings. Do not invoke the classifier for a mandatory regex/scanner hold.
Git requests with selected classifier controls return incomplete/unsupported before
any Git/model process; no silent ignored criterion. No exported five-root API changes.

The closed classifier receipt has fixed profile, `status`, `code`, `expectedFiles`,
`selectedFiles`, `selectedBytes`, `attemptedRequests`, `completedDecisions`,
`requestBytes`, `responseBytes`. No paths, source hashes, endpoint/model names, raw
answer/diagnostic or arbitrary echoed IDs. Counts mean:

- Expected files are the selected logical inputs; classifier-selected files/bytes
  count the full locally captured input considered for dispatch, not server reading.
  Selected bytes can reach the operation's 64 MiB ceiling on an input-limit hold.
- Attempts count transport dispatch attempts, including failed connections, at most
  one. Encoded request bytes are a bounded local serialization count, not confirmed
  network delivery. Full typed decisions within budget count completion, at most one.
- Response bytes are bounded bytes observed, including at most one cap-probe byte.
  They are neither billable tokens nor proof the model consumed every source byte.

`not-invoked/not_invoked` has zero attempts/completions/request/reply counts.
`complete/no_concern`, `concern/concern` and `unknown/unknown` require one actual
attempt and one timely complete typed response. Unknown still holds outer output.
`incomplete` has a fixed reason (`disclosure_denied`, `input_limit`, `request_limit`,
`response_limit`, `deadline`, `connection_failed`, `http_error`, `response_invalid`,
`model_mismatch`, `refused`, `unsupported_response`, `execution_incomplete`) and no
completed decision. Complete coverage requires expected=selected files and <=16 KiB.
Outer codes are `classifier_concern` (blocked) or `classifier_incomplete` (held).
Invalid controls keep existing incomplete/invalid semantics and zero dispatch.

| Stage | Classifier selected/attempt/completion/request/reply counts |
|---|---|
| Mandatory regex/scanner hold | all zero; expected files may already be declared |
| Input-limit hold | selected files/bytes reflect full captured corpus; all other counts zero |
| Encoded request-limit hold | full selected counts; attempts/completions/reply zero; requestBytes is the 131073-byte cap probe |
| Connect or HTTP failure | full selected counts, one attempt, zero completions; requestBytes counts the bounded serialized request, reply counts only bounded observed bytes |
| Valid timely typed response | full selected counts, one attempt/completion, bounded actual request/reply byte counts |

An early control/pin rejection emits no classifier receipt, like other invalid
controls. It performs no source read or external work. Status counters never claim
network delivery, server consumption, calibrated quality or billing accuracy.

Do not drop receipts or source checks to fit output caps. Tiny caps/writer failures
retain existing all-or-no-report/source-prefix failure semantics. Existing Gitleaks
reuse remains independent; no classifier answer cache or authorization reuse.

## Acceptance and qualification

| ID | Required evidence |
|---|---|
| MC-A1 | Missing/extra required site, invalid control, unpermitted digest or changed input produces no request and no source output. |
| MC-A2 | Late regex/scanner holds make zero model calls. Selected text outside the view is included, exact and ordered, without filenames. |
| MC-A3 | Actual HTTP peer no-concern preserves verified rows; late concern/unknown/refusal/malformed/omission withholds every row. These peers are fixtures, not AI quality evidence. |
| MC-A4 | Redirect/proxy/DNS/URL/header/credential/compression/extra-choice/tool/finish/model-mismatch cases cannot disclose to another client destination or pass a result. |
| MC-A5 | Input/encoded request/header/reply/deadline/attempt bounds, full body closure, missing completion witness and canceled contexts hold. No sleeps or live providers in tests. |
| MC-A6 | Strict outer/inner wire rejects duplicate keys, bad Unicode, unknown/static/stale question IDs (including changed state/criterion), invalid values/probabilities and partial output; no answer/error/source/key leak in receipts. |
| MC-A7 | Old default inspection output byte parity and fixed NQ native core remain green. Ordinary build holds every selected-control category before source/scanner I/O. Both CUE versions vet actual new ordinary/text/compact reports plus positive/negative controls: absent/default, required-site/allowlist/policy-pin presence, malformed/duplicate pins, endpoint syntax/range and UTF-8 byte limits, especially 4096-byte criterion. Go checks cross-file digest/coverage relations. |
| MC-A8 | Git selected-classifier use holds before Git/model dispatch; formats/tiny caps preserve every receipt and mandatory check. |
| MC-A9 | Independent security and guide-only fixture trial, source/binary pins, race/vet/full gate, strict plan proof and normal push gates. |
| MC-A10 | Nine alternating native ordinary-build default-mode pairs against the saved pre-classifier guard use the same small public corpus, controls and host. Inspection wire bytes must agree in all three formats; median elapsed overhead above 25% requires tuning or parking. Preserve the initial combined-artifact failure. Tagged artifact startup remains a separately disclosed cost. Fixture-peer timings never establish model speed/cost. |

## MC-N: native client-boundary fixture profile

Extend the existing qualification driver with `files-loopback-fixture/1`: core's
18 cases plus the 12 fixed cases below. This profile starts its own bounded,
ephemeral IPv4 loopback HTTP peer inside the driver, with no external endpoint,
scanner, model, credentials or caller-provided payload/argv. Its receipt explicitly
marks `syntheticPeerOnly: true`. The peer performs no inference and logs no content.
This qualifies the native client/output boundary, not Ollama or judgment quality.

| Case ID | Required observation |
|---|---|
| `classifier-no-concern` | one complete typed no-concern request; verified source rows preserved |
| `classifier-concern` | one request; blocked/classifier_concern; no source rows |
| `classifier-unknown` | one typed unknown; incomplete/classifier_incomplete; no rows |
| `classifier-model-mismatch` | one request; model assertion mismatch holds |
| `classifier-refusal` | one request; refusal holds |
| `classifier-duplicate-answer` | one request; duplicate inner answer key holds |
| `classifier-truncated` | one request; non-stop finish holds |
| `classifier-unpinned-input` | incomplete/invalid; zero peer requests |
| `classifier-policy-drift` | change policy bytes after scope pin; incomplete/invalid; zero requests |
| `classifier-regex-hold` | mandatory rule hold; zero requests; classifier not invoked |
| `classifier-redirect` | one first-peer request; zero redirected-peer requests; HTTP hold |
| `classifier-response-limit` | one request; exact bounded overflow-probe count; response-limit hold |

Freeze all cases and expected outcomes in the roster descriptor. Preserve existing
core/scanner descriptor identities and wire shapes. The peer independently checks
complete ordered input framing, criterion and options, requested model, schema,
method/path and absence of credentials. A malformed request fails qualification even
when the guard's final status happens to match a negative-case expectation. Independently
count peer requests, compare the receipt's local byte/call counts, and withhold all
raw request/reply bodies from published qualification receipts. Close every listener
and use finite read/write/header bounds. No CLI endpoint/skip/response override.

The additive native workflow observes this fixed fixture profile alongside core
on Linux, Windows and macOS. Its witnesses cover actual reported OS/architecture
and IPv4 fixture behavior only. IPv6 native runtime behavior and actual model/runtime
quality/residency remain separately unqualified. Retain failed runs and byte pins.

Admission implements the receiver and real bounded HTTP dispatch. It does not close
GT12 or qualify any installed Ollama/model. Native deployment selection, resource fit,
measured residency/egress/logging, independently labeled cohort/calibration, false
favorable/hold/abstention and cost/latency bars remain required before promotion.
Do not manufacture a calibrated safety claim from protocol fixtures. Stop this
profile on any unpinned disclosure, false completion, lost hard hold or budget escape.

## Sources

Checked 2026-10-24. Primary references, not measured deployment proof:

- [Ollama API compatibility](https://docs.ollama.com/api/openai-compatibility) supports a subset of Chat Completions and can route cloud models through a local server. Reuse the existing compatible client, not a second engine.
- [Ollama structured outputs](https://docs.ollama.com/capabilities/structured-outputs) documents `response_format` support and a cloud limitation. Validate the actual selected deployment; schema adherence is not semantic truth.
- [Ollama FAQ](https://docs.ollama.com/faq#how-do-i-disable-ollamas-cloud-features) documents cloud-disable configuration and restart/logging behavior. Do not change it incidentally or treat its existence as full residency proof.

The existing generic decision response reports its configured model. This profile
checks the bounded raw HTTP model assertion separately rather than pretending the
port supplies a verified resolved model. Calibration utilities remain available for
the later independently measured site; no new shared abstraction is justified yet.
