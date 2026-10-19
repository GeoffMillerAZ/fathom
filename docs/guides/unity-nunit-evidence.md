# Native Unity NUnit execution evidence

This is a local producer assertion and integrity contract. It does not verify a signature or defend against a malicious local writer. Fathom never starts Unity. Existing test-kit vocabulary/statuses stay authoritative; this bounded native adapter adds source provenance and exact C# identity, not a new certification system. ADR-043's non-Go trigger has fired. I8 remains informational, I43 remains unchanged: a dirty/source-hash-bound native runtime observation is not a live clean-commit certificate.

The example below is a schema sketch with placeholder hashes and deliberately cannot validate. All keys are closed. No credentials, arbitrary provider or remote endpoint is accepted.

The receipt alone is not the attested result. ADR-043/I43 and deferred F76 require a producer-generated standard #TestResult envelope, with `key=sha256:<source digest>`, full native case identity/status/duration, actual commit/dirty facts, `env.runnerClass=unity`, Editor toolchain digest, and artifacts `unity-nunit-xml`/`unity-source-inputs`. `attestation.path/sha256` bind that record. XML/receipt only stays UNKNOWN and out of execution tables. A local unsigned standard envelope is accepted as the producer assertion already supported by Fathom; no signature is invented. The native importer will cross-check all case outcomes and identities and cannot be bypassed by routing an unbound C# envelope through generic Go ingestion.

The source-input artifact names the canonical bytes defined below, not a differently encoded JSON file. It can be retained as source-inputs.bin. The envelope does not refer to the final receipt hash, avoiding a cycle. `trace.traceId` equals run_id so native run identity is part of the standard content-addressed record. Use the regular active receipt path `<root>/.artifacts/unity-tests/active/<project>/<mode>.json`, replaced atomically with started before each next attempt; archive completed receipts separately. The importer retains that active path and completed hash, so newer incomplete attempts invalidate an older pass without an attempt scheduler or new witness subsystem. Flake policy for the no-retry producer is `{maxRetries:0,quarantineAfter:3,budget:"0%"}`; no retry/flake lineage is inferred. Case id is `<native assembly without .dll>::<native fullname>`, case name is the full native fullname. Skipped and Inconclusive map to standard skip with raw distinctions retained by XML; an all-nonpassing run never becomes pass. A schema sketch is not real evidence.

## Producer ordering

Write the receipt as `status:started` before launching the recorded Editor command. Capture `source.inputs`/inventories before launch; include the actual project Assets (all regular files including .meta), Packages and ProjectSettings trees, relevant generated project/assembly-reference/config inputs outside those trees, plus exact producer and Editor bytes. After process exit, parse the actual native XML, capture the same trees/inputs again and only publish `complete` if unchanged. Preserve refused/failed attempts rather than refreshing old hashes. Use a unique immutable `-testResults` path per run. Never stamp historical XML: it can only be parsed as unattested data.

Complete requires every shown field except `symbols`; `attestation` is mandatory and references the existing test-kit #TestResult record, whose bytes must agree with XML and source facts; started may be incomplete and is always refused for execution admission. `mode` is EditMode or PlayMode. `target`, filter and command are exact facts, not inferred from XML. `command[0]` resolves to editor.path; required argv pairs are -projectPath, -buildTarget, -testPlatform and -testResults, plus -runTests. Nonempty filter requires an identical -testFilter pair. XML.path resolves to the command's -testResults path, without following an unrelated replaced symlink. Safe argv only; a producer must never put secrets in receipt commands.

`exit_code:0` is required for an all-passed XML. A complete failed run may have the native nonzero test-failure exit; license/compiler failure with old passing XML is refused. XML start/end must fit the producer interval, allowing only its native whole-second timestamp precision. The receipt cannot prove its producer wrote started on time; that is the trusted runner's responsibility and a fixture/integration obligation.

## Source digest algorithm

Inputs are `{path,sha256}` rows. Internal paths are forward-slash repository relative, external SDK/reference input paths may be absolute. Inventories stay inside root, walk all regular files recursively, and name the exact sorted membership in `paths`. No traversal, backslashes, NUL/control bytes, invalid UTF8, duplicate paths, symlink files/directories or nonregular members. Every inventory member must have a hash. All current input hashes and inventory membership are checked again on admission and when native runtime evidence is read.

Canonical source bytes are UTF8:

1. Literal `fathom.unity.inputs/v1\n`.
2. For each distinct inventory root, sorted by path: `tree\0` + root + `\n`.
3. For each distinct input sorted by its path: `file\0` + path + `\0` + lowercase sha256 + `\n`.

Here `\0` and `\n` mean actual NUL and newline bytes, not literal backslash characters. SHA256 of those bytes is `source.before_sha256`; complete must carry an identical `source.after_sha256`. Inventory membership is redundant with hashed inputs and verified by actual walk. The completed receipt retains the before manifest. Root/project/producer/Editor/command/XML identities are additional validated receipt fields, not silently folded into this digest.

The root and project must match the target service/project. Producer.path and Editor.path are individually hash-checked. Reference/config closure is producer-declared rather than independently discovered by Fathom. Maxima will be explicit bounded constants; no unbounded XML/entity/tree or external fetch.

## Typed method identity and variants

Optional `symbols` references an actual SCIP artifact plus its source receipt, both hash-bound. Its root/source validation must succeed using the existing native source-receipt contract. Only exact, unique typed C# declarations matching native assembly + classname/methodname and source location can join; the indexed declaration must carry the current extractor-epoch fingerprint of the same raw hash-bound source bytes, so a stale syntax row cannot borrow a fresh SCIP definition. Raw, prior-epoch and stale fingerprints are refused; the source comparison reads at most 16MiB per C# file. Nested/generic/overloaded/duplicate or absent identities stay refused/unjoined rather than guessed. No stripping argument strings to do a bare-name join. Parameterized cases preserve their full native name/id and each outcome, grouped under one proven method identity only after that binding.

Omitted/unusable symbols can still yield an explicitly unattested/unjoined parse, but cannot yield a native runtime PASS. It is acceptable for the producer to omit symbols when there is no current real index; do not repair an old index's source receipt to manufacture freshness. Parser-only fixtures must never enter test_runs.

Any nonempty native testFilter stays unjoined/UNKNOWN: selected cohort counts do not prove the unfiltered method family, and this version does not interpret filter grammar. The full filtered observation may still be admitted.

For a requested method, the newest complete matching native run is evaluated as a cohort: any failed variant => FAIL; all expected variants passed => PASS; skipped/inconclusive/explicit/missing variants or stale/absent/mixed receipt/source => UNKNOWN. Full raw outcomes/labels remain available. An older pass never overrides a newer unknown/incomplete result. Use a canonical symbol key or the full namespace/class/method for runtime lookup. A bare method is accepted only when unique; class-only suffixes and bare/namespace/overload ambiguity cannot select an arbitrary family. These observations do not invent framework dispatch/call edges, runtime coverage of reached functions, or human/game quality.

## CLI and storage

Extend existing `tests ingest` with explicit `--unity-nunit XML --unity-receipt JSON`, mutually exclusive with --results. No new MCP tool. Native provenance must survive store roundtrip and be revalidated by runtime reads. The additive store/read seam retains run identity and the active receipt hash, rechecks the receipt, XML, envelope, source and native SCIP sidecar on each runtime evidence load, then uses the complete method cohort. Native rows are explicitly excluded from the live-certificate ladder. Existing Go/envelope joins remain unchanged. This adapter is opt-in and does not change default test ingestion or execute a command.

## Decision models

None applies. XML identities/outcomes, content hashes, inventories, input bounds and cohort aggregation are deterministic. Provider none; no source/model egress.


## Implemented bounded refusals and limits

This candidate accepts bare standard JSON #TestResult only for native admission; existing generic DSSE handling remains unchanged. Receipt/standard JSON max32MiB; native XML max16MiB, 100,000 leaves, depth64; source closure max100,000 inputs /8GiB; inventory max100,000 entries per root; optional SCIP max256MiB and its existing native receipt limits. Unknown native producer/version and Editor argv outside the safe allowlist refuse. Only EditMode/PlayMode and the four explicitly selected desktop/Web targets are supported.

Native C# binding additionally requires the matching literal AssemblyName and Compile membership in a hash-bound generated .csproj directly under the selected project. This is declared project ownership with a real typed definition, not independent Unity compilation proof; retain the assembly oracle separately. Overloads, nested/generic identities, missing/duplicate declarations, ambiguous method cohorts and Retry/Repeat syntax remain unjoined/UNKNOWN. No retry lineage is fabricated. The parser preserves the observed Unity root state `Failed(Child)` while requiring failed native leaves; arbitrary compound outcomes refuse. It refuses malformed counts, duplicate identities and case-nested test structure; unsupported suite-only setup failures stay refused rather than guessed.

A source-bound standard record without symbols may be stored as an unjoined observation. It cannot yield runtime_pass. XML/receipt without the standard record never enters test_runs. A newer started receipt invalidates the previous result without inventing another execution or updating old hashes. Rechecking uses local reads at the runtime evidence load boundary; there is no filesystem watcher or per-call atomic snapshot guarantee.


## Invoke

The external runner must write started before invoking Unity, produce native XML
and the standard envelope from that same attempt, and atomically publish the
completed active receipt. Fathom does not provide or invoke that runner:

```sh
fathom tests ingest --manifest /checkout --service workshop --store /scratch/evidence.sqlite \
  --unity-nunit /checkout/artifacts/runs/run-unique/results.xml \
  --unity-receipt /checkout/.artifacts/unity-tests/active/game/Workshop/EditMode.json
```

Completed archive receipts cannot replace the active receipt for admission. The
same project/mode active path must publish started before *every* next attempt,
including a differently filtered attempt, to invalidate earlier results. Never
rewrite archived XML, envelope, source manifest, symbols or receipts to make a
historical run appear fresh. A missing symbol artifact is an honest unjoined
observation; produce a new actual index before a new execution for typed runtime
identity.

The `producer` name/version supported by this contract is
`unity-local-runner`/`1`. This names a reviewed local producer assertion; it is
not an authenticated identity or allowlist of hash-trusted executables. The
operator owns the runner's before-launch ordering and input-closure declaration.
The importer validates its declared files, argv, intervals and standard result.
No signature, hosted execution, malicious-host defense or model call is claimed.

## Minimal shape

All complete receipt keys below are required except `symbols`. Paths and hashes
are placeholders; source inputs must include all inventory files, generated
project/reference closure, producer and actual Editor executable. External
reference paths may be absolute. The three inventories are exactly the selected
project's Assets, Packages and ProjectSettings, including .meta files.

```json
{
  "schema": "fathom.unity.nunit/v1", "status": "complete", "run_id": "run-unique",
  "root": "/checkout", "project": "game/Workshop", "mode": "EditMode",
  "target": "WebGL", "filter": "", "started_at": "2026-10-19T12:00:00Z",
  "finished_at": "2026-10-19T12:00:03Z",
  "producer": {"name":"unity-local-runner","version":"1","path":"Tools/runner.py","sha256":"<sha256>"},
  "editor": {"path":"/sdk/Unity","version":"6000.3.15f1","sha256":"<sha256>"},
  "command": ["/sdk/Unity","-batchmode","-projectPath","/checkout/game/Workshop",
    "-buildTarget","WebGL","-runTests","-testPlatform","EditMode",
    "-testResults","/checkout/artifacts/runs/run-unique/results.xml"],
  "exit_code": 0,
  "xml": {"path":"artifacts/runs/run-unique/results.xml","sha256":"<sha256>"},
  "source": {"before_sha256":"<canonical source sha256>","after_sha256":"<same sha256>",
    "inputs":[{"path":"<input>","sha256":"<sha256>"}],
    "inventories":[{"root":"game/Workshop/Assets","paths":["<all files>"]},
      {"root":"game/Workshop/Packages","paths":["<all files>"]},
      {"root":"game/Workshop/ProjectSettings","paths":["<all files>"]}]},
  "attestation": {"path":"artifacts/runs/run-unique/test-result.json","sha256":"<sha256>"},
  "symbols": {"scip_path":"artifacts/csharp.scip","scip_sha256":"<sha256>",
    "source_receipt_sha256":"<sha256 of csharp.scip.receipt.json>"}
}
```

For the envelope, top-level verdict is `fail` if any native case failed, `pass`
only if every selected leaf passed, otherwise `error`. Native Skipped and
Inconclusive map to case `skip`; full distinctions, messages and stack traces
remain in raw XML. Case ids are `<assembly without .dll>::<native fullname>`,
case names retain the entire native fullname including parameters, attempts is
1, and native seconds map exactly to Go-style durations. `timing.startedAt`
represents native start time; env contains actual commit/dirty/race facts,
runnerClass `unity` and `toolchainDigest=sha256:<Editor digest>`.
`trace.traceId=run_id`; artifact refs are `unity-nunit-xml` and
`unity-source-inputs` with their actual digests. Flake policy is
`{maxRetries:0,quarantineAfter:3,budget:"0%"}`. The native reader validates the
standard mirrored #TestResult schema as well as these cross-field constraints.
A suite-only setup failure with no failed leaf is currently refused, not guessed.


Current project calibration is native macOS ARM64. The generic parser/unit/store
fixtures do not establish another Editor host, target, package version or Unity
platform module. The four target values describe explicit runner selection,
not execution proof for Windows/Linux players. Keep host/platform evidence
separate from C# syntax, typed binding and runtime result facts.


## Store compatibility

Native joins are stored in additive `test_cases.native_symbol_key`; legacy
`test_symbol_key` stays empty for those cases. Older readers cannot accidentally
interpret one native variant as an attested Go-style result without receipt
validation. Current readers project the protected key only alongside native
binding metadata. Old read-only stores without the new optional columns retain
their legacy Go evidence reads; transitional native rows without protected keys
are suppressed. A new opt-in/disposable store is still the recommended adoption
path, with the stable default and its configuration preserved.
