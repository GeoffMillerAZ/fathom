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
