# 43 - Portable receipts: evidence a third party can check, replay and file without fathom's state

Status: accepted 2026-10-10 for waves 1 and 2 (ADR-068, the owner-architect's ruling on D13 under the owner's
delegation). Waves 3 to 5 are decided in shape and gated: the record on EVX-G3, the recipe and replay on EVX-M3.
Revision 2 applies the adversarial review from the same day. Each review item is either applied in the text below or
rejected with a one-line note in section 10.2. The owner note is N12 of 2026-10-10. The owner thought this may be a
big near-term feature and asked for a verdict, not a build. Plan: `docs/plans/portable-receipts.md` (EVX-). It lands
beside vision 42 (ADR-067), whose repository-facts waves it rides as a filler.

This design builds on:

- receipts (vision 28, ADR-034)
- verbs (vision 29, ADR-035 decision 4)
- the review key (vision 30 section 3.2, ADR-040 decision 6, FL-17 and FL-18)
- change and compliance receipts (ADR-051, goal G25)
- test certifications (ADR-043)
- kit overlaps (ADR-063)

It amends none of them. It reopens two checked boxes whose proof never existed: VB-19 and CX-17 (section 2).

## 1. The question

The owner wrote on 2026-10-10:

> For systems that require evidence, can Fathom be a helpful tool in gathering that evidence in a way that can be
> done with like any third-party tool? Like maybe it's a standardized output or whatever that can help reconstruct
> the mapping or queries or whatever to be able to reproduce the query in Fathom and find the results again, while
> also having different output formats and types that can be exported in case the tool requires the evidence to be
> in a actual text format. [...] without having to rely on it being in the state, because it may not have the
> reader of the report, may not have access to the database or whatever. [...] it would also need to be able to
> output like hashing or whatever is used to ensure that the evidence is best linked [...] I'm sure you can think
> of a way better way to do this [...] I think this is actually a pretty big feature that could be prioritized in
> the near future, but I'm not sure what you think.

The note makes five asks:

1. A standard record of the query, so the query can be run again in fathom and the results found again.
2. Several export formats, plain text among them.
3. Usable by a reader with no access to fathom's store, daemon or state.
4. Hashing, or something better, so the evidence stays linked to its source and tampering shows.
5. A priority call: is this a big near-term feature?

Part of ask 1 already ships. `fathom export scip` (VB-32) writes the symbol and typed-edge mapping as a standard
SCIP index, which any SCIP reader can load. This design does not rebuild that.

## 2. Before designing: what applies

- *Non-goals* (`docs/intent/scope/non-goals.md`): no hosted backend, no dispatch loop, no incident command center.
  The incident-maps plan refuses an incident-tool export (IM-X4). Under ADR-040 and CX-8, fathom never posts,
  approves or blocks on its own authority, and CI decides. F14 keeps a forge app and any posting service in futures.
- *Principles*:
  - P1: governed evidence is the moat.
  - P3: never silent.
  - P4: no net tool growth.
  - P10: a CLI verb first.
  - P14: narrative is a lens.
  - P15: fathom writes only what it owns.
  - P17: state is computed.
  - P20: a named consumer before a build. A general format needs a second consumer.
  - P22: measured, not argued.
  - P23: a request pays for its answer.
- *Invariants*:
  - I3: anchors are root-relative.
  - I38: a count over incomplete reads is a named lower bound.
  - I44: a missing input reads unproven.
  - I56: fathom never downloads on its own authority.
  - I57: a cross-boundary link below the typed tier never gates.
  - I62: a judgment is never a fact and never enters a proof surface.
- *Decisions*:
  - ADR-011: org-agnostic.
  - ADR-034 and vision 28 section 3.6: "a receipt a stranger can check without fathom".
  - ADR-035 decision 4: every answer carries a tier and an anchor.
  - ADR-040 decision 6: one review key over every input, "without a second bespoke hashing API".
  - ADR-043 decision 5: fathom never verifies a test-kit signature.
  - ADR-049 decision 2, the table freeze: no new table, column, cache or background job.
  - ADR-051 U7 and U8.
  - ADR-063: no second result format. Under O4, the test-kit envelope is the source. Under O9, the words
    evidence, claim and pack are shared with agentic-kit and audited under K10.
  - ADR-065: a secret-bearing source is never stored or served.
- *Owner decisions*:
  - QK1: no Rekor and no public registry.
  - K5: no committed index snapshots.
  - The budget hold of 2026-09-29. Every reading in this design costs $0.
- *Goals*: G25 waits on two open readings. CX-M1 is a risk receipt on every PR in 2 repositories for 30 days. CX-M2
  is an evidence pack that re-verifies byte-identically.
- *Corrected premises*, each read in the code on 2026-10-10:
  - `receipt verify` needs the fathom binary but no daemon and no store. Its go/types oracle is
    `internal/contextbench/publicloc`. The honest phrase is "without fathom's state".
  - The toolchain rung can reach the network today. `loadGoTruth` runs go/packages with `os.Environ()`
    (`publicloc/gotruth.go:107-118`). Without `GOPROXY=off`, a missing module is downloaded, which breaks I56 on
    the live `receipt verify` path. Row EVX-6 fixes this now.
  - `receipt verify` extracts the commit into `os.MkdirTemp("")` (`receiptverify.go:263`). That is `$TMPDIR`, not
    fathom's scratch directory.
  - VB-19 is checked and claims "per-fact anchors (file, symbol fingerprint, content hash)". The wire carries only
    `resolution` and `index_commit`.
  - CX-17 is checked and claims "recomputes at the pack's commit". In fact it recomputes from the current store
    and the working-tree `verify.DirFiles(repo)` (`cmd/fathom/trace.go:296`).
  - Both VB-19 and CX-17 were reopened at landing (2026-10-10). Done means proven, so this is not an owner call.
  - The git watermark is the bare HEAD SHA (`internal/connector/local.go:254`). ADR-041 decision 4 re-extracts an
    edited file in place, so an answer can come from uncommitted content while it reports HEAD.
  - The index stores fingerprints, not bytes. `FingerprintHash` is the sha256 of the raw working-tree bytes,
    salted with the extractor epoch (`internal/fusion/extractor_epoch.go:662`). A git blob holds the clean bytes.
    With autocrlf, eol, ident, working-tree-encoding or LFS, the two differ even on a clean tree.
  - Review Kit's bridge takes seven source-binding inputs, not five: `SourceRevision`, `SourceWatermark`,
    `IndexedWatermark`, `ProducerBuildVersion`, `InputDigest`, `SourceFingerprints` and `SourceVersion`
    (`review-kit internal/fathombridge/bridge.go:45-67`). Its `SourceFingerprints` are `FingerprintHash` values
    (FT-6).
  - Review Kit's `SpanRef` is shaped to be swapped for agentic-kit's `evidence-span` (`review-kit
    internal/core/core.go:289`, ADR-0096 D8).
  - Fathom's own inbound test-evidence reader expects the subject name `gitCommit` with a digest key `sha1`
    (`internal/testevidence/envelopes.go:104-113`).
  - MCP serves no per-request `_meta` (`internal/mcp/protocol.go:14`). `handleToolsCallMeta` unmarshals only
    `name` and `arguments` (`dispatch.go:359-364`). An agent in Claude Code or Codex cannot set `_meta`.
  - `SymbolKey` is `service::pkg.Name` (`internal/core/keys.go:33`), so every symbol key names the repository.
  - There are seven SARIF callers, not six. The missed one is `review_diff`'s `receipt_sarif`
    (`internal/analysis/review/changeriskreceipt.go:41`, `internal/mcp/review_tool.go:229`). Two risk converters
    duplicate each other: `receiptSARIF` (`cmd/fathom/reviewreceipt.go:197`) and `changeRiskReceiptSARIF`
    (`changeriskreceipt.go:68`).
- *Corrections to the review itself* (read 2026-10-10):
  - The live board (`docs/languages.md`, generated 2026-10-10) reads TS, JS, Go and C# at L2 (call precision)
    excellent. The review's figures (.52, .19, .82, .78) come from `EXCELLENCE.md`'s older table.
  - The GitHub Actions dialect is dispatched (d3840987).
  - A Jenkinsfile dialect exists (`internal/fusion/jenkinsrun.go`, board L2 at target), so `early-testers.md:263`
    is stale.
  - The review's priority conclusion still holds on P20 and on the consumer reading alone (D13).
- *Fired triggers*: none.
  - ADR-051's "an adopter or regulator rejects the evidence pack format" has not fired.
  - ADR-035's "re-derivation reproduces under 95 percent" has not fired (VB-21 read 50 of 50).
  - ADR-063's "a kit builds a result format of its own" has not fired.

## 3. What exists, and what it lacks

| Shape | Carries | Anchor | Checkable without fathom's state |
|---|---|---|---|
| `core.Receipt` and `fathom receipt verify` | callers, references, symbol location, certification reach (Go) | `index_commit`, a file and line for a location | yes, with the fathom binary and git (VB-20, VB-21 50 of 50) |
| `reviewkey.Key` (`fathom review-key`, `pkg/client.ReviewKey`) | one token over six component digests (manifest, closure, rules, plans, tooling, index), `Complete` and `Unknown`, and `ChangedSincePrior` through `Diff` | per-component digests | only with fathom |
| `FingerprintHash` | the stored per-file fingerprint, epoch-salted. These are the values Review Kit's `SourceFingerprints` carry | per file | yes, given the formula and the epoch |
| `fathom export scip` (VB-32) | symbols and typed edges as a SCIP index with the resolution tier, round-tripped against scip-go | file and range | yes, with any SCIP reader. This is the standard "mapping" export |
| `fathom certify manifest` | the certifications artifact. `--check` exits 1 when the committed file differs | the committed file | yes, by diff |
| MCP reply envelope | tier mix, `index_commit`, refresh, staleness, elision, and a `budget` partial that depends on load | the watermark | no |
| Change risk receipt (`review --receipt`, `review_diff`) | fired factors, band, authority line | changed paths, every factor at the first file, no line | no |
| SARIF (one emitter, seven callers, two duplicate risk converters) | verify refusals, obligations, rules replay, config and intent audit, risk receipt (CLI and `review_diff`) | path and start line | readable, but nothing to check it against |
| `fathom review --json` | dimensions, coverage honesty, input digests (`ReviewInputs`, `ProducerInfo`) | input digests | only by reimplementing fathom's input selection |
| Trace pack (`trace export`, `trace verify`) | requirement to code, test, certificate, run and decision | `COMMIT`, `digests.json`, `MANIFEST.sha256` | the file digests with coreutils, not the cells |
| Test evidence ingest | inbound DSSE and in-toto test-kit envelopes | run digest, commit subject | n/a (fathom emits none) |

What is missing:

- Nothing on the main path produces a receipt. A caller assembles one by hand.
- No fact carries a byte-level anchor.
- No record names its query together with the parameters and inputs that shaped the answer. `reviewkey` does this
  for review inputs only.

## 4. The frame: three questions a stranger asks of evidence

1. *Is the cited text really there?* An anchor git can check answers this: the blob at that path in that commit,
   and the digest of the exact lines cited.
2. *Has the record changed since it was made?* A digest the reader already trusts answers this, or a signature
   from someone the reader trusts.
3. *Is the claim true?* Only deriving it again answers this. Use a language toolchain where one exists, or replay
   the query in fathom at the recorded inputs.

Hashing answers only the second question, and only against a digest that reached the reader by a trusted path. A
signed false claim is still false. A faithful replay of a wrong edge reproduces the error, because replay proves
determinism, not truth.

So the better way than "hash everything" is to answer each question with the cheapest independent instrument:

- For question 1, git's own object ids. Git is already a Merkle tree over the source, so fathom needs no tree of
  its own for this.
- For question 2, a canonical digest plus the operator's own signer.
- For question 3, a re-derivation ladder. Every fact states the highest rung it reaches.

## 5. The decisions

### D1. Generalize what exists, not a new subsystem: adopted

N12 grows four existing pieces:

- ADR-034's receipt carries anchors and, later, a record.
- `reviewkey` carries the recipe and the replay explanation (D5, D6).
- `FingerprintHash` names each file's indexed state.
- The one SARIF emitter gains provenance.

The verbs follow the existing families. `fathom export receipt` sits beside `fathom export scip`, so there is no
second "export" verb. Checking, replaying and rendering stay under `fathom receipt`. There is no new top-level
verb, no MCP tool and nothing stored.

### D2. The record is an in-toto Statement v1 with one fathom predicate, deferred to wave 3 by P20: adopted

No standard records "the query that produced this". ADR-063 forbids a second result format, so the fathom-owned
part is a predicate inside an existing envelope, the way test-kit does it. A general predicate also needs a second
consumer (P20). It therefore waits until EVX-G3 records two consumers, or one the owner did not write. Until then a
receipt is the existing `core.Receipt` JSON with additive anchors (D3).

The shape is fixed now so it lands right:

- The list is named `facts`, not `claims`. "Claim" is an O9 word.
- The commit subject is `{name: "gitCommit", digest: {gitCommit: <sha>}}`. Fathom's inbound reader gains an
  explicit `gitCommit` key and reads its own record (a round-trip test), so one shape serves both directions.
- A CUE schema (`schema/receipt/`) ships with valid and invalid pairs.
- The bytes are RFC 8785 (JCS), and the predicate holds integers only.
- The producer identity uses agentic-kit's `evidence-producer` shape: kind deterministic analyzer, origin self.

The predicate URI is an owner call (Q-EVX-1). It must name no host or organization tied to a private repository
(ADR-011, QK1). `urn:fathom:` is out: it is an unregistered namespace whose name other products share.

### D3. The evidence unit is a fact with git-native anchors: adopted

An anchor carries:

- the commit and the object format (`sha1` or `sha256`)
- the repo-relative path and a 1-based inclusive line range
- the git blob id
- the excerpt sha256, using Review Kit's `SpanRef.ContentHash` formula
- a bounded excerpt
- the file's `fathom_fingerprint`, which is the `FingerprintHash` value Review Kit already binds

The anchor has four states:

- *Uncommitted*. The `FingerprintHash` of `git cat-file --filters <commit>:<path>` differs from the stored
  fingerprint. The anchor and the receipt read `clean: false`.
- *Filtered*. The `--filters` bytes differ from the blob bytes (autocrlf, eol, ident, working-tree-encoding, LFS).
  The anchor is marked `filtered` and gets no git rung.
- *Withheld*. The file is secret-bearing or suppressed. It gets no excerpt and no excerpt digest: a short file's
  digest can be confirmed by guessing (ADR-065).
- *Inclusion proof* (optional, `--inclusion`). The anchor carries the commit object and the tree objects along the
  path. A reader with no clone can then check blob-at-path with `git hash-object --stdin -t <type>` outside any
  repository.

The anchor maps one to one onto `SpanRef` and onto `evidence-span`'s location fields. Review Kit's planned swap
(ADR-0096 D8) therefore needs no change in fathom. Fathom fills no `Author` or `Grade`.

### D4. Every fact says who can re-derive it: adopted

Each fact carries `rederive.level`, taken from a closed table per kind (section 6.7), never from the producer's
say-so:

- `git`
- `toolchain:<name>` (today `go/types` and `go test`)
- `fathom@<version>`

Three rules apply to every fact:

- A judgment never enters a receipt (I62).
- A cross-boundary link below the typed tier never renders as an error (I57).
- An absence fact carries its read set in agentic-kit's `evidence-signature.CoverageManifest` shape, and with
  reads missing it renders as a lower bound (I38).

Git proves the cited text is present. It does not prove that the text is a definition.

Toolchain rungs beyond Go are deferred (EVX-D5), but not for want of oracles. The language program already measures
against scip-typescript, scip-python, scip-go, scip-java and Roslyn (`docs/design/language-breadth.md:131-217`), and
VB-19 has an `external(<indexer>@<version>)` tier. The real cost is that each rung needs its language toolchain and
a dependency install, which fathom must not fetch (I56).

### D5. The recipe is a review key over the query's inputs: adopted, wave 4, gated on EVX-M3

A recipe is a `reviewkey.Key` over its six components plus a `params` component. The `params` component holds the
tool, every parameter with its default filled in, and `budget_bytes`. Symbolic refs and relative times are resolved
to fixed values.

The recipe adds the inputs reviewkey misses:

- the service ignore file `~/.fathom/ignore.d/<service>` (`serviceignore.go:227`), named by role
- store-only inputs by digest: ingested runs, imported findings, claim verdicts, and coverage and mutation profiles
- one watermark per service for a cross-repo answer (VB-30)

An input it cannot resolve goes to `Unknown`, so the recipe reads `Complete: false`. The result digest covers the
canonical structured result. It excludes the envelope's `suppressed`, `refresh` and `budget` fields
(`dispatch.go:453-460`), which depend on load and refresh state, not on the answer. There is no query language.

### D6. `fathom receipt replay` answers same, changed or cannot reproduce, each with why: adopted, wave 4, gated on EVX-M3

The exit codes are 0 for same, 1 for changed or invalid, and 2 for cannot reproduce. The cases:

- *Same commit*. Replay runs on the live index even when the epoch or fathom version moved. A moved input reads
  "changed, explained" and names it. `--scratch` cannot rebuild an old epoch.
- *Another commit*. Replay needs `--scratch`, which runs `git archive` into `os.MkdirTemp` (the same `$TMPDIR`
  place `receipt verify` uses) and never into a worktree.
- *Explained*. The explanation is `reviewkey.Diff`.
- *Index moved*. Replay reads `index_commit` from each reply. If RT-99 moved the index mid-replay, replay retries
  once, then reads cannot reproduce with the reason "index moved".

### D7. Tamper evidence is layered, and the operator's signer does the signing: adopted

Integrity has three layers: the file's sha256, a sha256 per fact, and git object ids. For authenticity, the
operator signs with their own tool:

- `cosign attest-blob` makes a DSSE in-toto envelope.
- `cosign sign-blob` makes a detached signature, not DSSE.
- `ssh-keygen -Y sign` makes an SSHSIG.

Each works with a key or a KMS key and no transparency log, so it suits private repositories. The public key must
reach the reader by a separate trusted path. Fathom holds no key, verifies no signature, makes no network call and
writes no wall-clock time unless `SOURCE_DATE_EPOCH` or `--observed-at` sets one.

### D8. Formats are one-way projections, and each carries the source's digest: adopted

The projections are:

- SARIF, for findings-shaped facts only
- Markdown and plain text
- a POSIX `checks` script

A JUnit projection of `fathom verify` is built only if EVX-V1 names a CI sink that reads JUnit (Jenkins). That is a
cheaper consumer than the record. HTML and OSCAL each wait for a named consumer.

### D9. The first producers are the ones with a consumer: adopted

In order:

1. All seven SARIF callers, with the duplicate risk converters merged into one in wave 1.
2. `fathom export receipt` for the four Go kinds in wave 2.
3. `fathom review` and the change risk receipt as records in wave 3. Review Kit binds them.
4. Any agent-surface tool through the recipe in wave 4.
5. The trace pack in wave 5.

### D10. Nothing stored, no new table, no MCP tool, no new parameter, no wire change: adopted

`fathom export receipt <tool> --args <json>` runs the tool through an in-process server over the store. The
precedent is `fathom instrument --adoption`. Dispatch captures the recipe where the defaults are known. No reply
changes, and no `_meta` is read.

### D11. Privacy by construction: adopted

A receipt names no host, URL, organization, absolute path or user name. The service is redacted by default, because
`SymbolKey` and `params.service` both name the repository. `--keep-service` opts in. A redacted receipt replays only
when the caller passes `--service S`.

The rest:

- Repository identity is optional: the root commit ids when the clone has them, or a URI the operator configures.
- Excerpts are bounded to 40 lines by default. `--no-excerpts` keeps digests only.
- One allowlist constructor builds every exported receipt. This reuses the pattern of `internal/research/bundle`.
- A privacy-scan row checks the golden set.

### D12. Call it a receipt, not "evidence", and call its list `facts`: adopted

"Receipt" has been fathom's word since ADR-034. "Evidence", "claim" and "pack" fall under O9 and K10. The anchor
aligns with `SpanRef` and with its planned `evidence-span` successor (D3).

### D13. Priority: two small waves now, then gated: accepted by the owner-architect (2026-10-10)

Waves 1 and 2 cost about 5.5 worker-days and $0. They run as fillers between language batches, and language
excellence stays the main line. The record (wave 3) waits for a second consumer (P20, EVX-G3). Recipe and replay
(wave 4) and the projections and trace pack (wave 5) wait for EVX-M3. This is a medium feature in slices, not a big
near-term bet.

Three reasons:

- No consumer outside the owner's own code is named.
- Kubernetes policy engines (Kyverno, policy-controller) check attestations on image digests, not git blobs.
- Observability export is refused (IM-X4).

## 6. Design

### 6.1 The record (wave 3)

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {"name": "gitCommit", "digest": {"gitCommit": "9f2c..."}},
    {"name": "internal/verify/sarif.go", "digest": {"gitBlob": "4b1e..."}}
  ],
  "predicateType": "<Q-EVX-1>",
  "predicate": {
    "schema": 1,
    "repository": {"object_format": "sha1", "root_commits": ["e0a1..."]},
    "commit": "9f2c...", "clean": true,
    "producer": {"name": "fathom", "kind": "deterministic", "origin": "self", "version": "v0.10.0", "build": "fb7d9f3f...", "modified": false},
    "recipe": {"key": "sha256:...", "complete": true, "components": {"index": "...", "params": "...", "rules": "...", "tooling": "..."}},
    "queries": [{"id": "q1", "source": "receipt:callers", "params": {"service": "@", "symbol": "renderSARIF", "depth": 1}, "result_digest": "sha256:..."}],
    "facts": [{
      "id": "sha256:...", "query": "q1", "kind": "callers", "text": "RenderSARIF calls renderSARIF",
      "tier": "verified", "resolution": "typed",
      "anchors": [{"path": "internal/verify/sarif.go", "start_line": 99, "end_line": 100, "blob": "4b1e...",
                   "excerpt_sha256": "c03a...", "excerpt": "func RenderSARIF(...", "fathom_fingerprint": "7d10..."}],
      "rederive": {"level": "toolchain:go/types", "check": "fathom receipt verify <file>"}
    }],
    "limits": []
  }
}
```

Field rules:

- The subjects use the in-toto digest-set names `gitCommit` and `gitBlob`.
- A fact id is the sha256 of the fact's canonical bytes without its `id`.
- Everything is sorted.
- `"service": "@"` marks a redacted service.

### 6.2 The anchor (waves 1 and 2)

- *Excerpt bytes*: from the first byte of `start_line` through the last byte of `end_line`, including that line's
  newline when it has one. There is no normalization. A test vector shared with Review Kit pins the bytes,
  including a file with no final newline.
- *States*: `committed`, `uncommitted`, `filtered` or `withheld`, as defined in D3. `--require-clean` exits 1 on any
  anchor that is not `committed`.
- *Non-git corpora* (a `manifest:` watermark): the anchor carries the file's sha256, and the git rung becomes "the
  sha256 of the file you hold".
- *Line moves*: the excerpt digest is SARIF's `partialFingerprints["fathomExcerpt/v1"]`. Replay `--at <rev>` uses it
  to relocate an anchor, which then reads `same, moved`.

### 6.3 The recipe (wave 4)

The recipe is captured inside dispatch, never reconstructed by the caller (D5, D10). It stores config inputs by
role, never by path. A truncated answer's result digest covers the elision marker, and `budget_bytes` is a
parameter. A replay at a different budget therefore reads changed and names the budget.

### 6.4 Replay (wave 4)

| Outcome | Exit | When | What it prints |
|---|---|---|---|
| same | 0 | the result digest is equal | the facts confirmed. With `--at`, anchors relocated by digest read `moved` |
| changed, explained | 1 | the digest differs and `reviewkey.Diff` names a moved input | the inputs that moved, and the facts added or removed |
| changed, unexplained | 1 | the digest differs and every input is equal | a determinism defect, named, with the `fathom observe` line to file it |
| invalid | 1 | a fact id, a blob or an excerpt digest does not match | the first failing item, checked before any replay |
| cannot reproduce | 2 | the inputs cannot be rebuilt | one reason from a closed list (below) |

The closed list of cannot-reproduce reasons:

- the commit is absent, or the clone is shallow
- the tool is retired (ADR-053)
- a parameter is unknown to this version
- the language is not extracted by this build
- a config entry is unavailable
- a store input is absent
- the index was dirty at export
- the answer was partial at export (RT-57 `settleCall`)
- the index moved during replay
- `--scratch` was not given
- the daemon is down, or admission was refused

### 6.5 Tamper evidence, layer by layer

| Layer | What it shows | Who can check it |
|---|---|---|
| sha256 of the file | the file is the one whose digest you hold | anyone, with `sha256sum` |
| fact ids (wave 3) | a fact lifted into a comment or a SARIF result belongs to the record | anyone with an RFC 8785 library |
| blob ids and excerpt digests | the cited bytes exist at that path in that commit | anyone with git and coreutils |
| inclusion proof (opt-in) | the blob sits at that path under that commit, with no clone | anyone with `git hash-object` |
| signature (optional) | who vouched for the file | the signer's own verifier (D7) |
| replay | the fact is what fathom answers at those inputs | anyone running fathom |
| toolchain re-derivation | the fact holds independent of fathom's index | anyone with the toolchain. Today that runs through the fathom binary, offline (EVX-6) |

A Merkle tree over facts is not earned yet (EVX-D3).

### 6.6 Projections

| Format | Carries | For |
|---|---|---|
| receipt JSON (wave 2), then `statement` (wave 3) | everything | machines, auditors, replay |
| `sarif` | findings-shaped facts only, each with `partialFingerprints`. The run carries `automationDetails.id` and `properties.fathom` (commit, clean, producer, epoch, source digest). It carries `versionControlProvenance` only when a repository URI is configured | code scanning, reviewdog, Jenkins Warnings NG, editors |
| `md`, `text` | every fact with its `path#Lstart-Lend @ commit` anchor, excerpt, rederive level and re-check line, and the source's sha256 | PR comments a CI job posts, tickets, write-ups |
| `checks` | the git rung for every anchor, using only git, `head`, `tail` and a sha256 tool. It exits non-zero on the first failure, by name | a stranger with no fathom |
| `junit` of `fathom verify` (conditional) | plan rows as test cases | only if EVX-V1 names a JUnit-reading sink |

Query results that are not findings never go into SARIF. Two lines per anchor of the `checks` script:

```sh
git rev-parse "$commit:$path"     # must print the anchor's blob id
git cat-file blob "$blob" | head -n "$end" | tail -n "+$start" | sha256sum   # must print excerpt_sha256
```

### 6.7 Who can re-derive what

| Fact kind | git alone | a toolchain | fathom at version V |
|---|---|---|---|
| the cited text exists (every anchor that is not `filtered`) | yes | | |
| a lexical search hit | yes, `git grep -P` for patterns RE2 and PCRE read alike | | replay |
| a symbol's definition site | the text only, not that it is the definition | go/parser (Go only) | replay |
| callers and references, typed tier | | go/types (Go only, `receipt verify`) | replay |
| callers and references, heuristic tier or non-Go | | deferred (EVX-D5) | replay |
| certification reach and the test's result | | go/types for reach, `go test -run` for the result | replay |
| blast radius, rename impact, a proven zero | | | replay, with the read set |
| governing obligations | the rule's own text, by anchor | | replay |
| a plan row proven | each anchor | `go test` for a test contract | replay (`fathom verify`) |
| conservation planes, the risk band, review dimensions | | | replay |
| a decision-port judgment | never in a receipt | | |

### 6.8 Consumers

- *Review Kit* (the owner's kit). One record carries all seven bridge inputs under one digest, and its anchors are
  `SpanRef`s. A fathom-origin item that comes back through Review Kit and `findings import` is refused as a second
  witness.
- *Forges and CI*.
  - The SARIF sinks are code scanning, reviewdog, Jenkins Warnings NG and the check-run payload. GitHub code
    scanning on a private repository needs a paid licence.
  - JUnit is a conditional projection (D8).
  - Fathom posts nothing.
- *Compliance*. The trace pack carries the test-kit envelopes it cites, because certificate states come from
  ingested envelopes. `trace verify` replays at the pack's commit through a scratch ingest of the pack. OSCAL waits.
- *Incident reviews*. A person carries the file, and IM-X4 holds.
- *Not consumers*:
  - Kubernetes admission policy. It checks image digests, not git blobs.
  - Observability backends. Export there is refused by IM-X4.

## 7. Sequence

| Wave | Content | Exit test |
|---|---|---|
| 0 | EVX-V1: the CI sink the owner's CI actually has, and the record consumers (at least one the owner did not write, or two) | the reading names each consumer, who wrote it, and who agreed |
| 1 | the anchor core. SARIF provenance in all seven callers, with the risk converters merged. The excerpt fingerprint. Dead code removed. `receipt verify` made offline (EVX-6) | every caller validates against the vendored SARIF 2.1.0 schema with no network. Output is byte-identical twice. A fingerprint survives a 40-line shift. A missing module exits 2 with no download. Kill: EVX-K1 |
| 2 | `fathom export receipt` for the four Go kinds with anchors (the existing JSON, additive). `receipt verify` checks anchors with git first. The `checks` script. `--inclusion`. Service redaction and the privacy scan. The "without fathom" page | a dirty-tree fixture marks `uncommitted`. An autocrlf fixture marks `filtered`. `--require-clean` exits 1. A secret file has no digest. The script passes with no fathom on PATH and fails by name on a tamper. Median receipt at most 256 KB, export at most 2 s warm. EVX-M1 |
| 3 (gate EVX-G3) | JCS, the CUE schema, the Statement with `facts` and producer identity, the inbound `gitCommit` round trip, `--format statement` for the Go kinds, the risk receipt and `fathom review` (seven bridge inputs), `receipt verify` on records, `findings import` refusing a self-witness | RFC 8785 vectors pass. A golden record is byte-identical across runs, time zones and locales. CUE pairs vet. EVX-M3 starts |
| 4 (needs EVX-M3 KEEP) | the recipe on reviewkey, the result digest, `receipt replay` with `--at` and `--scratch`, `export receipt <tool>` in-process | EVX-M2 |
| 5 | `render` md, text and sarif from a record. The trace pack as a record carrying its envelopes. The signing page. `junit` of verify if EVX-V1 named a JUnit sink | each render is a pure function of the record's bytes. `trace verify` passes with the store at another commit |

## 8. Cost

- *Build*: about 2.5 worker-days for wave 1, 3 for wave 2, 4 for wave 3, 4 for wave 4 and 3 for wave 5. That is
  about 16.5 in all, with 5.5 of it now. No provider spend, and every reading is $0.
- *Runtime*: an export costs the tool's answer plus one `git cat-file --batch` pass and one `--filters` pass over
  the cited files. A replay at the live index costs one call. A replay at another commit costs a cold ingest (72 s
  on fathom's own tree), and only with `--scratch`.
- *Store*: nothing. No table, column, cache or job.

## 9. The decision port

There is no site for it. Every field is computed or read. A model-written summary in `md` is refused: generation is
the port's weak side (ADR-056), narrative is a lens (P14), and a judgment never enters a proof surface (I62). With
provider none, everything above holds. This is the Jev check: considered, with no site and no future.

## 10. Rejected

### 10.1 Rejected alternatives

| Alternative | Why rejected |
|---|---|
| A fathom bundle format | ADR-063 forbids a second result format |
| SLSA VSA as the predicate | it attests to an artifact, not to facts about code |
| W3C PROV, CycloneDX or SPDX annotations, OpenVEX | none of them fits a code fact |
| A public transparency log or keyless signing by default | QK1 and I56 |
| Fathom holding keys or verifying signatures | the signer's tools own both |
| A query language or mapping DSL | vision 29's do-not list. SCIP already exports the mapping |
| Exporting the store or a snapshot | K5 |
| A Merkle tree over facts now | not earned (EVX-D3) |
| Query results in SARIF | code scanning would show them as alerts |
| An `export_evidence` MCP tool or a tool parameter | P4 |
| Wall-clock timestamps | they break byte identity |
| An incident-tool export, forge posting or a blocking conclusion | IM-X4, ADR-040, CX-8 |
| HTML now | it needs a named consumer and the app-kit doctrine |
| A Kubernetes admission consumer for the DSSE envelope | policy engines check image digests |

### 10.2 Dropped from revision 1, or review items not taken

| Item | Why |
|---|---|
| The recipe requested through MCP `_meta` | dropped. Fathom serves no per-request `_meta`, and agents cannot set it. The CLI path (D10) replaces it |
| `fathom receipt export` | dropped as a second export verb. It is now `fathom export receipt` |
| The predicate list named `claims` | dropped as an O9 collision. It is now `facts` |
| `urn:fathom:receipt:v1` | dropped. The namespace is unregistered and other products share the name |
| Subject `{name: "commit"}` | dropped. It clashed with fathom's own inbound reader |
| The record in wave 2 | dropped for P20. A general predicate waits for a second consumer (EVX-G3) |
| The recipe as a parallel set of inputs | dropped. It now reuses `reviewkey` (ADR-040 decision 6) |
| "`jq -cS` matches the canonical form" | dropped as an unpinned claim. Use an RFC 8785 library |
| Rejecting `evidence-span` alignment (old D12) | withdrawn. Review Kit's own plan swaps `SpanRef` for it |
| A git rung for filtered paths, and an excerpt digest for secret files | dropped. The rung would always fail, and the digest is guessable |
| Replay `--scratch` for an epoch or version change at the same commit | dropped. A scratch ingest cannot rebuild an old epoch |
| "Replay uses fathom's scratch directory, as `receipt verify` does" | dropped as false. Both use `os.MkdirTemp` |
| "A heuristic-tier result is never `error` (I57)" | narrowed. I57 covers cross-boundary links below typed, not every heuristic result |
| The review's precision figures, "GHA undispatched" and "no Jenkinsfile dialect" | not taken. They are stale against the 2026-10-10 board, d3840987 and `jenkinsrun.go`. The priority conclusion stands on other grounds |
| "JUnit of verify is a real Jenkins consumer" | taken only as conditional. No Jenkins sink is named yet (EVX-V1) |
| The git inclusion proof by default | taken as opt-in (`--inclusion`). Tree objects can add KB per anchor, under the 256 KB kill bar |
| `GOPROXY=off` inside `loadGoTruth` for every caller | scoped to the `receipt verify` path. The bench callers are out of this design's scope |

## 11. What we do not claim

- That a hashed or signed receipt is true.
- That the toolchain rung is fathom-free today.
- That replay across versions reproduces an answer. It explains the difference.
- That replay proves a fact true. It proves determinism.
- That any fact outside Go has a toolchain rung.
- That a proven zero is complete beyond its read set.
- That a third party consumes receipts today.
- That GitHub code scanning is free on private repositories.
- That a receipt is an audit opinion or maps to any control.

## 12. Kill criteria in one place

| Reading | Bars (fixed before the run) | Kills |
|---|---|---|
| EVX-K1 (wave 1) | CX-M1 reads PASS within 60 days of wave 1 landing: a sink named in EVX-V1 consumed fathom SARIF or check output on every PR in 2 repositories for 30 days | further SARIF work (wave 5's SARIF render is refused), and the SARIF sink stops counting toward EVX-G3 |
| EVX-M1 (wave 2) | three receipts: fathom, one agentic-kit or app-kit package, and one public Go repository at a pinned commit. A fresh session with only git, coreutils, POSIX sh and the docs page, no fathom on PATH, no daemon. 100% of genuine anchors check. Every tampered copy fails, naming the item (one excerpt byte, one blob-id digit, one path per receipt). One `--inclusion` receipt checks with no clone. One fix round | the "without fathom" claim and the `checks` projection. Anchors and SARIF stay |
| Size (wave 2 onward) | the median receipt for one `fathom review` on this repository is at most 256 KB, and export takes at most 2 s on a warm index | excerpts become opt-in |
| EVX-M3 (wave 3) | within 60 days of wave 3, a consumer the owner did not write, or two distinct consumers, each read a record and act on a named fact | waves 4 and 5 move to futures. The record stays as `receipt verify`'s input |
| EVX-M2 (wave 4) | pre-registered frame: 50 calls drawn from the daemon call log with seed 20261010, at least 5 per agent tool, exported then replayed at equal commit, fathom version and extractor epoch. At least 48 of 50 read `same`, and zero read `changed, unexplained`. Fixtures: an epoch bump names the epoch, an unreachable commit exits 2, a shifted anchor reads `same, moved`. One fix round | replay for each failing tool, which then renders "replay unreliable for <tool>", plus an observation per defect |
| A kit ships a code-evidence predicate, or Review Kit asks for its own | n/a | fathom adopts it and retires its own (ADR-063's revisit) |
