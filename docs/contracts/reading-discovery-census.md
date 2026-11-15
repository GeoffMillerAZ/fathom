# Fleet roster, discovery comparison and file-presence census

Draft for #281 RT-M11 and #291 UND-M3. Base
 d9eef4794afdbc200d1b8697ce121d9d688fedaf. Wave-1 UND-M3 has no predecessor,
and gates UND-1 and T-IaC's fleet clause. RT-M11 gates discovery's default flip.
Shared inputs are the complete roster and immutable source/index snapshots.
No fleet queried and no result supplied by this draft.

## Decisions

| ID | Recommended choice / authority | Alternative |
|---|---|---|
| D1 | Freeze every registered service before looking at results, preserve unreadable/missing/stale entries. RT-M11 says every registered service, UND-M3 says indexed repository; I30/I38, clientWiringRegistry precedent. | A convenience list of available local roots. Rejected: changes both denominators. |
| D2 | Isolated snapshots with identical config except respect_gitignore=false/true and exact dry-run switch pair. RT-68/70, RT-M11, ADR-047 amended d5. | Toggle live repository config or assume omitted flag forces false. Rejected: LocalConnector loads per-corpus config. |
| D3 | Compare observation identities, not dry-run file-count ratio. RT-M11's authored 1% observations bar. | Call --dry-run --stats sufficient proof. Rejected: it never extracts observations. |
| D4 | UND-M3 is file presence, one repository per dialect/tool, with unknown residue. realignment UND-M3, I38/I65, ADR-067. | Treat source fixtures or generic YAML filenames as CloudFormation fleet presence. Rejected. |

Initial CTR-10 (26ffbfeda...) supplies facts/hazards, not fleet census inputs.
No API change, new door package, CLI integration or measurement threshold.
Decision-port/provider none. No documented future needs a new abstraction here.

## Existing shapes and exact read sites

* pkg/client/client.go: (*Client).Services(ctx context.Context) ([]string,error).
  cmd/fathom/observatory_apply.go clientWiringRegistry calls this and
  (*Client).ServiceMeta to obtain registered roots. internal/store/platform_store.go
  PlatformStore.ServiceIDs() []string is sorted outer-store inventory; Get(id)
  returns the per-service store. QueryAll silently omits errors and MUST NOT be
  used for completeness. Reconcile Services output with this physical inventory
  and per-store ListServiceIDs(ctx); report orphans/duplicates without inventing
  services for the headline denominator.
* internal/store/store.go ServiceMeta {ServiceID,Path,IndexedCommit,IndexedAt};
  WatermarkOf(ServiceMeta) is commit@indexed_at (derived.go). FileStat
  {Path,Size,MtimeNS,Ino,ContentHash,Lang,Excluded}; Excluded is empty/oversize/
  unreadable. SQLiteStore.FileStats(ctx,serviceID) ([]FileStat,bool,error) applies
  scope filters and returns ok=false for absent/empty raw rows. False does not
  distinguish a fully empty corpus from an old index with no stats.
* Exact file list at an immutable, consistent SQLite snapshot, read-only transaction:

```sql
SELECT path, size, mtime_ns, ino, content_hash, lang, excluded
FROM file_stats WHERE service_id = ? ORDER BY path;
```

  This is fileStatsRaw's existing query in internal/store/derived.go. Bind the
  registered service ID, never interpolate. Public scoped FileStats is a second
  view, not a replacement for raw-snapshot accounting of suppressions.
  Supplementary existing ListSourceRefs union (sqlite_observations.go):

```sql
SELECT source_ref FROM observations WHERE service_id = ?
UNION
SELECT source_ref FROM fingerprints WHERE service_id = ? ORDER BY source_ref;
```

  This finds evidence-bearing paths, but cannot supply all discovered files;
  fingerprint/observation absence is not file absence. File-list schema/query
  mismatch is unknown, never fallback to a source-tree fixture.
* cmd/fathom/ingest.go cmdIngest supports --service --path --store --mode local
  --dry-run --stats --json and --respect-gitignore. The last flag sets
  LocalConnector.RespectGitignore=true; false leaves corpus configuration active.
* cmd/fathom/ingest_dryrun.go runIngestDryRunStats(ctx,connector.Connector,
  core.ServiceRef,io.Writer,bool) int calls Discover only, never Fetch or store.
  ingestDryRunStats fields are service,path,dry_run,files,bytes,oversize_files,
  oversize_bytes,read_cap_bytes,default_ignored_dirs,skipped_dirs,ignored_dirs,
  git,gitignored,git_unlisted,stat_failed,largest. Files/bytes are not observations.
* internal/connector/local.go LocalConnector.Discover(ctx,core.ServiceRef)
  ([]core.SourceRef,error) loads include/ignore policy; defaultignore.go
  loadConnectorConfig reads .fathom/config.toml. gitdiscover.go uses git
  ls-files -co --exclude-standard, keeps skipDir and counts exclusions.
* internal/langclass/classmap.go LanguageForPath(path string) string is basename/
  extension only (.tf/.tfvars => terraform; YAML => yaml). ClassifyDialect in
  classify_dialect.go can consume pinned DialectRule and bounded head bytes.
  extractor.LoadDialects() ([]Dialect,error) and dialects/<id>/dialect.json,
  doc.json own content-claimed formats. CloudFormation requires actual matching
  content/ingested dialect evidence, not a YAML extension. Pin these files.

Read-only consumer inventory: Client.Sources uses language classes; scoped FileStats
feeds summaries, not a fleet census grader. runIngestDryRunStats feeds only stats.
No existing command here computes the full RT-M11 observation comparison. No
imaginary --observations or --dialects fleet query is authorized.

## Roster and snapshot manifest

Before either outcome: capture raw Services result, outer-store and inner service
inventories, mapping to opaque repository IDs, registered root, indexed commit and
indexed_at, current source commit/dirty/untracked manifest, classifier/config hash,
suppression policy, binary hash/commit, clock and exact snapshot hashes. Preserve
all members when a root/store is inaccessible. Public report has opaque aliases,
no private root, host or repository names. Private mapping stays under workspace/.

A store snapshot must be a consistent SQLite backup including committed WAL state,
not a raw database copy beside an active writer. Record transaction/snapshot identity
and schema; source snapshots include ignored and untracked files when present in
the registered corpus. A clean git clone is not equivalent to that corpus.
Reconcile roster at capture start/end; changed membership invalidates the freeze
until recorded as a new preregistration, never silently prunes services.

No raw roster/snapshot is provided in this author lane. The manifest schema is
frozen; actual IDs/hashes/counts are explicit missing inputs. It cannot be advertised
as a complete measurement preregistration until those cells are populated before
outcome access. Draft approval does not authorize broad local-root discovery.

## #281 RT-M11 exact comparison

For each registered service create two identical isolated corpus snapshots including
config, tracked/untracked/ignored content and nested-repo metadata. Freeze all
ignore/include/default/skipDir policies. In each copy set ONLY connector
respect_gitignore to the assigned boolean; report that config diff and new hashes.
If config is unparseable, root non-git, nested repo unlistable or a file unreadable,
retain the service as incomplete. No live configuration change.

Frozen command pair (flags before positional args):

```sh
fathom ingest --mode local --service "$SERVICE_ALIAS" --path "$CONTROL_ROOT" --store "$SCRATCH_STORE" --dry-run --stats --json
fathom ingest --mode local --service "$SERVICE_ALIAS" --path "$TREATMENT_ROOT" --store "$SCRATCH_STORE" --dry-run --stats --respect-gitignore --json
```

The scratch store is not opened in this path. Paths above are supplied local
manifest values; no command runs here. Capture exit/stdout/stderr and gitignored,
git_unlisted, stat_failed and every exclusion category. Require git=true in the
assigned git mode; verify control's false setting is not overridden by another
corpus config. Include/reinclude and built-in exclusions are unchanged.

Full bar additionally needs observation sets at the same code/parser configuration
for both modes, with identical source bytes. Existing Discover+Fetch+extract
pipeline can provide them in isolated stores only under a later granted capture;
dry-run stats alone cannot. Pin extractors, all options, parse failures, ingest
completion and observation tuple multiset. Compare stable existing identity fields
(service_id,concern_id,source_ref,source_kind,symbol,package,line) and canonical
observation payload to distinguish changed/dropped facts without generated IDs.
Do not count display row IDs as observations or duplicate the same tuple silently.
If a tuple is not unique, retain multiset multiplicity.

Classify each source path as outside/inside gitignored directories from the
snapshotted git ignore result, keeping tracked files and explicit re-includes
separate. For each service denominator = control observations outside gitignored
directories, numerator = those missing in treatment in that same stratum. Publish
inside/outside/reinclude/unknown losses and gains separately. Unknown ignore state
cannot remove observations from the denominator. Ratio≤.01 passes; >.01 means
that service's default must not flip. Zero denominator reports unmeasured, not 0%
PASS. Do not average away a failing service. Every registered service requires a
verdict; one unavailable one prevents a fleet-wide flip claim. No flip executed.

## #291 UND-M3 exact census

Use the frozen indexed-service roster and index snapshots, not the new RT-M11
control outputs. A repository counts once per confirmed dialect/tool regardless
of file count, with companion file counts and exact evidence-path hashes. Dedup
same indexed root/worktree using I30 canonical identity, disclose folded aliases
and preserve differing snapshots as a conflict rather than unioning them unseen.

Per service run the exact file_stats query above, record schema, ok/coverage and
all exclusions. Classify registered basename/extension cases with the pinned
LanguageForPath. For content-claimed dialects use existing stored dialect-specific
observations only when bound to that same snapshot/source hash, or retained source
bytes evaluated by the pinned dialect's actual detect rules. Both are file-presence
evidence; neither is a runtime tool-use claim. No source bytes => unresolved
ambiguous files remain unknown. A Terraform file confirms Terraform presence.
Generic YAML/JSON without a CloudFormation detect witness does not confirm CFN.

Tool presence uses the pinned tool-format dialect manifest IDs and their match/
detect rules (e.g. tool config format, not an installed binary or a filename
substring). Publish the enumerated classifier/rule inventory and one evidence row
per match. Unclassified files are counted, never assigned an invented tool. No
independent keyword dictionary. Unsupported/oversize/unreadable/suppressed files
remain in coverage and may prevent a total absence conclusion.

T-IaC fleet clause is YES with at least one fully bound Terraform or CFN presence;
NO requires complete roster/file/detect coverage with no such presence; otherwise
UNKNOWN. OQ15's outside-adopter question is unchanged. UND-M3 has no success-share
threshold. Reading is PASS only for a completed census with disclosed unknown
residue and honest lower bounds; any unavailable service/file inventory prevents
a complete fleet total and is INCOMPLETE. A confirmed positive can be reported
beside INCOMPLETE coverage, without claiming the whole row completed.

## Prospective scratch-checker tests

No observed RED/GREEN or measurement here. Implementing lanes write the following
before running the reading, and critic checks each assertion (W3-2). They own their
issue reading plus workspace scratch only. No production files are reserved.

| Name | Exact assertions |
|---|---|
| TestFleetRosterAndSnapshotCoverage | retain absent/unreadable service; stale/indexed-vs-source mismatch visible; Services vs physical orphan mismatch visible; partial WAL copy rejected; membership changed during capture invalidates freeze; aliases same root folded with count; conflicting snapshots unknown; no public private path/name. |
| TestRTM11IsolatedSwitchPair | omitted flag with true corpus config not control; explicit false/true pair works; same include/default ignores required; non-git/git_unlisted/stat_failed retained; no live config touched; files/bytes not observation counts. |
| TestRTM11ObservationLoss | multiset duplicates retained; stable tuple/payload diff counts lost observation; inside-ignore excluded only with witness; reinclude retained; 1/100 equality passes, 2/100 fails; zero denominator unmeasured; gains do not cancel losses; one failed service prevents fleet flip; missing extraction receipt unknown. |
| TestUNDM3DialectPresence | ten .tf files count one repository/terraform; generic YAML alone never CFN; matching bound CFN detect confirms; stale/content-hash mismatch unknown; same-source duplicate observations do not increase repository count; unknown and excluded paths counted separately. |
| TestUNDM3CensusCompletion | confirmed presence makes fleet clause YES with partial census disclosed; absence requires complete coverage; missing old file_stats cannot become empty census; source fixture cannot stand for a service; tool dialect presence never called runtime use; OQ15 untouched. |

Readings only: docs/bench/readings/tenancy/rt-m11-2026-11.md and
 docs/bench/readings/realignment/und-m3-census.md. Keep exact source/manifest/checker
hashes and every exception in each report. No boxes, implementation or default flip.

## Decision models

None applies. Deterministic source presence and snapshot joins; provider none.

## Executable scratch handoff (critic correction)

Frozen PROPOSED interface, not implemented tooling or run evidence. W3-2 leaves
real failing tests to the implementing reader. All earlier assertions remain
binding. No product/telemetry/API grant. Synthetic test success cannot close
missing raw inputs, policies or dispatch prerequisites. READY is for document
review only. Resource-windows is final FIX round2; another unresolved finding is
HOLD. The other seven reading drafts are FIX round1 and have one round remaining.

Exact layout: workspace/tmp/reading-discovery-census/checker.py (normalize/grade),
contracts.py (schema validation/declaration guard), test_checker.py (unittest
ContractTests), manifest.json, normalized.json, result.json and the three schemas
below. Fixture directory is fixtures/<exact-test-name>/ with input.json and
expected.json for EVERY assertion case in the earlier table. Implementing reader
creates these workspace-only files. Copy each JSON code block to its named file.

contracts.py implements only the closed JSON-Schema subset below (objects/
required/additionalProperties, arrays/items, enum/const, anyOf/null, minimum,
string pattern, date-time). Standard library only, no network schema fetch or
dependency. Check RFC3339 timestamps, unique artifact IDs, SHA256 and foreign
references, non-nil arrays, nonfinite numbers rejected. Quantity decimals use
canonical base10 strings; null upper bound is unbounded, never zero.
contracts.py guard parses the three Python files with ast without importing them,
requires checker normalize/grade entrypoints and ContractTests(unittest.TestCase)
with EACH exact method selected below declared once; checks the three schema
hashes pinned by the contract commit and fixtures for every case. Declaration
presence is never a substitute for running each assertion.

checker.py normalize --manifest PATH --output PATH validates manifest, reads ONLY
listed artifacts under this scratch root (reject absolute paths, traversal and
symlink escape), verifies raw hashes/lengths and uses only the named adapters.
SourceRef positions are half-open original byte offsets, record_index zero-based.
Every normalized row retains native lineage. Preserve identical distinct rows,
stack/frame/parent order and copy-overlap identity. Never synthesize missing facts.
Unreadable/absent native data appears as coverage gaps/nulls. No polling, capture,
network, provider, ingest, Compile/Sweep or live configuration mutation.

checker.py grade --manifest PATH --normalized PATH --output PATH revalidates
schemas, hashes, lineages, then recomputes all earlier criterion assertions from
native evidence, independent labels and bound policies. Declared PASS or approval
fields cannot supply evidence. Emit canonical UTF-8 JSON, sorted object keys;
criteria sorted by row_id,criterion_id,group_key and quantities by name. Every
authored clause and group appears, including missing required groups as INCOMPLETE.
Coverage includes all expected/read/malformed/torn/skipped rows and gaps.
complete=true needs independent accounting, not artifact presence. Pending policies
remain pending_policy_ids. Null/absent facts never become zero or empty success.
Unused group_key dimensions are empty strings. All result arrays are non-nil.

dispatch_state is BLOCKED for this draft interface; foreman approval plus real
input/prerequisite handoff is separate. Grade exit0 means valid computation,
including a FAIL/KILL/INCOMPLETE result, not measurement PASS. guard/validate/
normalize/grade exit1 means assertion/schema/identity/limit/refusal error; exit2
operational inability to read/write requested paths. Where output is writable,
errors have typed grader_errors kind and SourceRefs. TestCase exits nonzero on
any missing/weakened assertion. No new numerical bar or policy from this interface.

D-HANDOFF (W3-2, I38, L-R/parallel-lanes §3): recommend this exact standard-library
scratch layout, closed schemas and all tests; alternative ad-hoc reader protocol
is rejected because it leaves the handed contract unspecified. Proposed commands
below run from the clone root AFTER the implementing lane creates these files.
None ran in the author lane.

```sh
set -eu
cd "workspace/tmp/reading-discovery-census"
test -f checker.py
test -f contracts.py
test -f test_checker.py
test -f manifest.schema.json
test -f normalized.schema.json
test -f result.schema.json
python3 contracts.py guard
python3 -m unittest test_checker.ContractTests.TestFleetRosterAndSnapshotCoverage
python3 -m unittest test_checker.ContractTests.TestRTM11IsolatedSwitchPair
python3 -m unittest test_checker.ContractTests.TestRTM11ObservationLoss
python3 -m unittest test_checker.ContractTests.TestUNDM3DialectPresence
python3 -m unittest test_checker.ContractTests.TestUNDM3CensusCompletion
python3 contracts.py validate --kind manifest --input manifest.json
python3 checker.py normalize --manifest manifest.json --output normalized.json
python3 contracts.py validate --kind normalized --input normalized.json
python3 checker.py grade --manifest manifest.json --normalized normalized.json --output result.json
python3 contracts.py validate --kind result --input result.json
```

### Native adapters

| Adapter ID | Frozen normalization contract |
|---|---|
| native_roster | Client.Services/ServiceMeta, PlatformStore.ServiceIDs/Get and each Store.ListServiceIDs; independent reconciliation, no QueryAll. |
| native_sqlite | Read-only consistent WAL-aware snapshot; exact FileStats and supplemental source-ref SQL above, bound service ID. |
| native_dry_run | Actual ingestDryRunStats from explicit isolated false/true configs and command pair above. Counts not observations. |
| observation_export | BLOCKED: missing exact producer/capture contract separately required below, no invented export CLI. |
| dialect_detection | Pinned LanguageForPath + actual dialect detect rules/same-hash observations/content; ambiguity/suppression retained. |

All input roles must appear in manifest.inputs. Missing roles use a named absent
Artifact with null hash/size and named coverage, never a placeholder evidence file.
Adapter IDs are dispatch keys of checker.py, not product commands. Independent
labels/derived classifications are checked against native source refs, not trusted
booleans. Prerequisite and policy authority must resolve to immutable receipts;
pending fields cannot be promoted by the reader. Unsupported native conversion
or missing capture contract keeps the corresponding consumer BLOCKED.

### Explicit separate observation capture contract, still missing

observation_export is deliberately unavailable. RT-M11 dispatch stays BLOCKED until
a separate critic-approved handoff fixes exact producer binary/entry point, code
ownership, extractor/options, and store/query/export invocation for BOTH isolated
arms. It must produce observations in the schema below, backed by complete native
payload artifacts, source/config hashes, ignore witnesses, multiplicity, ingest
completion and all parser failures, with immutable capture receipt. Validate that
received handoff/export using contracts.py and checker.py above. No current
dry-run/Services/FileStats route is claimed to produce observations, and no export
CLI is invented. UND-M3 can be assessed separately with its own complete approvals/
inputs; it cannot clear RT-M11. TestRTM11ObservationLoss must additionally reject
missing/unapproved capture contract, one-arm export, stale config, missing native
payload and incomplete parse/ingest coverage. These are retained missing contracts,
not an inferred permission to implement a new producer.

### manifest.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/manifest-v1"},"protocol":{"const":"reading-discovery-census"},"source_base":{"type":"string"},"preregistered_at":{"$ref":"#/$defs/Timestamp"},"window":{"type":"object","properties":{"from":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]},"to":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]}},"required":["from","to"],"additionalProperties":false},"artifacts":{"type":"array","items":{"$ref":"#/$defs/Artifact"}},"inputs":{"type":"array","items":{"$ref":"#/$defs/Input"}},"policies":{"type":"array","items":{"$ref":"#/$defs/Policy"}},"prerequisites":{"type":"array","items":{"$ref":"#/$defs/Prerequisite"}}},
  "required": ["schema","protocol","source_base","preregistered_at","window","artifacts","inputs","policies","prerequisites"],
  "additionalProperties": false,
  "$defs": {"Artifact":{"type":"object","properties":{"artifact_id":{"type":"string"},"relative_path":{"type":"string"},"sha256":{"anyOf":[{"$ref":"#/$defs/Hash"},{"type":"null"}]},"size_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"state":{"$ref":"#/$defs/ArtifactState"},"error":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["artifact_id","relative_path","sha256","size_bytes","state","error"],"additionalProperties":false},"ArtifactState":{"type":"string","enum":["available","absent","unreadable"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Input":{"type":"object","properties":{"role":{"type":"string","enum":["services_result","physical_store_inventory","inner_service_inventory","source_snapshot","sqlite_snapshot","service_meta","file_stats","source_refs","control_config","treatment_config","control_dry_run","treatment_dry_run","observation_control","observation_treatment","ignore_classification","dialect_rules","content_witness","capture_contract"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"adapter":{"type":"string","enum":["native_roster","native_sqlite","native_dry_run","observation_export","dialect_detection"]}},"required":["role","artifact_ids","adapter"],"additionalProperties":false},"Policy":{"type":"object","properties":{"policy_id":{"type":"string"},"status":{"type":"string","enum":["approved","pending","not_applicable"]},"authority_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"parameters_artifact_id":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["policy_id","status","authority_refs","parameters_artifact_id"],"additionalProperties":false},"Prerequisite":{"type":"object","properties":{"id":{"type":"string","enum":["RT-70","observation_capture_contract","roster_snapshot"]},"state":{"type":"string","enum":["proven","missing","unproven"]},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["id","state","evidence_refs"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### normalized.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/normalized-v1"},"protocol":{"const":"reading-discovery-census"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"records":{"type":"object","properties":{"services":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"service_alias":{"type":"string"},"repository_key":{"type":"string"},"registered":{"type":"boolean"},"indexed_commit":{"anyOf":[{"type":"string"},{"type":"null"}]},"indexed_at":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"source_commit":{"anyOf":[{"type":"string"},{"type":"null"}]},"source_dirty":{"anyOf":[{"type":"boolean"},{"type":"null"}]},"snapshot_hash":{"anyOf":[{"$ref":"#/$defs/Hash"},{"type":"null"}]},"consistent_wal":{"anyOf":[{"type":"boolean"},{"type":"null"}]}},"required":["source","service_alias","repository_key","registered","indexed_commit","indexed_at","source_commit","source_dirty","snapshot_hash","consistent_wal"],"additionalProperties":false}},"files":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"service_alias":{"type":"string"},"path":{"type":"string"},"size":{"type":"integer","minimum":0},"mtime_ns":{"type":"integer","minimum":0},"ino":{"type":"integer","minimum":0},"content_hash":{"type":"string"},"lang":{"type":"string"},"excluded":{"type":"string"},"scope_hidden":{"type":"boolean"}},"required":["source","service_alias","path","size","mtime_ns","ino","content_hash","lang","excluded","scope_hidden"],"additionalProperties":false}},"observations":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"service_alias":{"type":"string"},"arm":{"type":"string","enum":["control","treatment"]},"concern_id":{"type":"string"},"source_ref":{"type":"string"},"source_kind":{"type":"string"},"symbol":{"type":"string"},"package":{"type":"string"},"line":{"type":"integer","minimum":0},"canonical_payload_sha256":{"$ref":"#/$defs/Hash"},"multiplicity":{"type":"integer","minimum":0},"ignore_state":{"type":"string","enum":["inside","outside","reinclude","unknown"]},"ignore_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["source","service_alias","arm","concern_id","source_ref","source_kind","symbol","package","line","canonical_payload_sha256","multiplicity","ignore_state","ignore_refs"],"additionalProperties":false}},"dry_runs":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"service_alias":{"type":"string"},"arm":{"type":"string","enum":["control","treatment"]},"respect_gitignore":{"type":"boolean"},"git":{"type":"boolean"},"files":{"type":"integer","minimum":0},"bytes":{"type":"integer","minimum":0},"git_unlisted":{"type":"integer","minimum":0},"stat_failed":{"type":"integer","minimum":0},"config_diff_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["source","service_alias","arm","respect_gitignore","git","files","bytes","git_unlisted","stat_failed","config_diff_refs"],"additionalProperties":false}},"matches":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"service_alias":{"type":"string"},"path":{"type":"string"},"classification":{"type":"string","enum":["dialect","tool","unknown"]},"id":{"type":"string"},"rule_hash":{"$ref":"#/$defs/Hash"},"content_hash":{"anyOf":[{"$ref":"#/$defs/Hash"},{"type":"null"}]},"witness_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["source","service_alias","path","classification","id","rule_hash","content_hash","witness_refs"],"additionalProperties":false}},"labels":{"type":"array","items":{"$ref":"#/$defs/Label"}}},"required":["services","files","observations","dry_runs","matches","labels"],"additionalProperties":false}},
  "required": ["schema","protocol","manifest_sha256","coverage","records"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["services_result","physical_store_inventory","inner_service_inventory","source_snapshot","sqlite_snapshot","service_meta","file_stats","source_refs","control_config","treatment_config","control_dry_run","treatment_dry_run","observation_control","observation_treatment","ignore_classification","dialect_rules","content_witness","capture_contract"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Label":{"type":"object","properties":{"label_id":{"type":"string"},"sources":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"labeler_role":{"type":"string","enum":["native_oracle","independent_person","architect"]},"labelled_at":{"$ref":"#/$defs/Timestamp"},"outcome_blind":{"type":"boolean"},"value":{"type":"string"}},"required":["label_id","sources","labeler_role","labelled_at","outcome_blind","value"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### result.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/result-v1"},"protocol":{"const":"reading-discovery-census"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"normalized_sha256":{"$ref":"#/$defs/Hash"},"state":{"$ref":"#/$defs/EvidenceState"},"dispatch_state":{"const":"BLOCKED"},"criteria":{"type":"array","items":{"$ref":"#/$defs/Criterion"}},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"unknown_inputs":{"type":"array","items":{"$ref":"#/$defs/Unknown"}},"pending_policy_ids":{"type":"array","items":{"type":"string"}},"grader_errors":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string","enum":["schema","identity","limit","refusal","decode","coverage"]},"source_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"message":{"type":"string"}},"required":["kind","source_refs","message"],"additionalProperties":false}}},
  "required": ["schema","protocol","manifest_sha256","normalized_sha256","state","dispatch_state","criteria","coverage","unknown_inputs","pending_policy_ids","grader_errors"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["services_result","physical_store_inventory","inner_service_inventory","source_snapshot","sqlite_snapshot","service_meta","file_stats","source_refs","control_config","treatment_config","control_dry_run","treatment_dry_run","observation_control","observation_treatment","ignore_classification","dialect_rules","content_witness","capture_contract"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Criterion":{"type":"object","properties":{"row_id":{"type":"string","enum":["RT-M11","UND-M3"]},"criterion_id":{"type":"string","enum":["registered_roster","snapshot_consistency","outside_ignore_observation_loss","indexed_file_census","dialect_presence","tool_presence","t_iac_fleet_presence"]},"group_key":{"type":"object","properties":{"service_alias":{"type":"string"},"population_id":{"type":"string"},"dialect_id":{"type":"string"},"utc_day":{"type":"string"},"channel":{"type":"string"}},"required":["service_alias","population_id","dialect_id","utc_day","channel"],"additionalProperties":false},"state":{"$ref":"#/$defs/EvidenceState"},"quantities":{"type":"array","items":{"$ref":"#/$defs/Quantity"}},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"unknowns":{"type":"array","items":{"$ref":"#/$defs/Unknown"}}},"required":["row_id","criterion_id","group_key","state","quantities","evidence_refs","unknowns"],"additionalProperties":false},"EvidenceState":{"type":"string","enum":["PASS","FAIL","KILL","INCOMPLETE","INCONCLUSIVE","HOLD"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Quantity":{"type":"object","properties":{"name":{"type":"string"},"unit":{"type":"string"},"value":{"anyOf":[{"type":"string"},{"type":"null"}]},"lower_bound":{"anyOf":[{"type":"string"},{"type":"null"}]},"upper_bound":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["name","unit","value","lower_bound","upper_bound"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"},"Unknown":{"type":"object","properties":{"input_role":{"type":"string","enum":["services_result","physical_store_inventory","inner_service_inventory","source_snapshot","sqlite_snapshot","service_meta","file_stats","source_refs","control_config","treatment_config","control_dry_run","treatment_dry_run","observation_control","observation_treatment","ignore_classification","dialect_rules","content_witness","capture_contract"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"}},"required":["input_role","artifact_ids","reason"],"additionalProperties":false}}
}
```

### Pinned scratch schema digests

SHA256 of each JSON code block plus one terminal newline. contracts.py guard
compares exact file bytes to these constants, not a newly generated policy.

- manifest.schema.json: a19806e9dc528fb85138aeb741a1d726bb0bc4f0654085bad87fae87121c8704
- normalized.schema.json: 6a4eb260b70a7583163c6166828658fd56f72e3580b7f22f73bf968a3b8c09af
- result.schema.json: 9fbdba98bd3370a0335679ff484f248bfab89d0253be81df18a8a223b0ecdca7
