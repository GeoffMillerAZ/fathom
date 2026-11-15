# LIB-1 / #255: first tagged door release contract

Status: DRAFT W2-4 backlog. Contract only, no release/tag/upload authorization.
Base: 468dc705813382d584926887b451cbea145c76ac. Owns this document only.
Current pinned buildinfo.Release and nearest local reachable tag are v0.26.0.
This observation is not a claim about all remote releases. No tag created.

## Intent before design and settled decisions

Read intent router, scope compatibility/non-goals, autonomy, ADR-092, realignment
LIB-1, library-roadmap LIB-11/12 and LP-1, W1-3/W2-1/W3-3/4. The library is a
public compatibility surface. Governed claims require actual source/asset proof,
not a nonempty reading. No new download on product authority, no own-license
decision (OQ4 remains open), no public API implementation in this lane.
LL-M1 revisit fired and W2-1 answers it: door 80, outside consumer 81; raw counts
published, no door headroom. No claims that notice/green-main/CI triggers passed.

D1 settled by LIB-1/LP-1/#255: CUT-1, closure, notice and one green main wave plus
release ritual precede tag. Only architect chooses version/roll/tag.
Alternative tag now and fix assets later contradicts these rows.
D2 settled by W1-3: build locally -> validate notice/archive -> attest -> upload;
failed validation uploads nothing. Current release workflow invokes GoReleaser
release before attestation, so it must be repaired by #254 before acceptance.
D3 settled by W1-3: notice generated from pinned grammar lock/source/cache;
full license/copyright and Apache NOTICE where present; absent license fails,
no release-time fetch or handwritten fallback. Own license unchanged.
D4 settled by W2-1/W3-3: publish door/consumer raw counts and re-read on actual
candidate, any exported door change uses architect API hold diff <=80 lines.
D5 settled by root release/change rules: version record/buildinfo and exact-tree
gate/CI before tag, never force-push or claim a receipt presence command is proof.

## Concrete recommendations requiring ruling

U1 recommend v0.27.0, a minor release from pinned v0.26.0, for first public door.
Alternative v0.26.1 if architect establishes no compatibility change in the release
delta. Version is a concrete proposal, not a tag or implied architect approval.
U2 recommend release integrator owns exact version/roll/workflow/archive validation
paths below, #255 stays a reading-only lane. Alternative split archive validator
from integrator release. No broad release-directory grant.
U3 recommend tag only final rolled, clean main SHA whose mandatory local gate and
Linux CI both passed after #254 fixes. Alternative defer release entirely. “Green
some earlier tree” never authorizes this tag. Capture exact SHA in approval record.

## Release candidate and exact owned-path handoffs

Current #255 owns only:
docs/bench/readings/realignment/lib-1-tag.md and
docs/bench/readings/library-roadmap/lp-1-release.md.
It reads release evidence and writes honest verdicts, never versions/tag/workflow.

Proposed exclusive release-integrator grant, serial after #254 notice handback:
- .goreleaser.yaml
- .github/workflows/release.yml
- internal/buildinfo/buildinfo.go
- CHANGELOG.md
- docs/changelog/unreleased/ (only the exact fragment list frozen for this roll)
- scripts/check-release-archives.py (new)
- scripts/check-release-archives_test.py (new)
- scripts/validate-release-archives.sh (new mandatory controller orchestration)
- scripts/validate-release-archives_test.sh (new)
- docs/contracts/release-archives.json (new synthetic checker fixture only)

#254 retains its exact notice/source-generation files and tests, then transfers
workflow/config once at its committed handback. Integrator must not rewrite its
license policy. Exact fragment list is generated from release candidate BEFORE
roll and attached to approval; no wildcard deletion permission follows from this
contract. Existing release-impact policy and breaking migration requirements apply.
No tag command executes on a contract branch. Public assets derive from final
tagged main tree, not this draft base or a local dirty copy.

## Archive validation envelope and deterministic checker

One manifest for the four existing target archives. Proposed JSON schema:
~~~json
{
  "schema": "fathom.release-archives/v1",
  "version": "v0.27.0",
  "commit": "<full final-main sha>",
  "tree": "<full final-main tree>",
  "archives": [
    {"os":"darwin","arch":"arm64","path":"<operator local archive>","sha256":"<64 hex>"},
    {"os":"darwin","arch":"amd64","path":"<operator local archive>","sha256":"<64 hex>"},
    {"os":"linux","arch":"arm64","path":"<operator local archive>","sha256":"<64 hex>"},
    {"os":"linux","arch":"amd64","path":"<operator local archive>","sha256":"<64 hex>"}
  ],
  "notice_sha256": "<generated pinned-notice digest>"
}
~~~

Angle-bracket values are required invocation inputs, not fixture passing values.
No fabricated live commit/digest in a draft. Closed fields: OS darwin/linux,
Arch arm64/amd64; implementation uses named string Enum members OS_DARWIN/OS_LINUX,
ARCH_ARM64/ARCH_AMD64. Explicit lower snake_case keys, unknown fields fail.
No future fields or path upload; local paths are operator-selected and redacted
from public readings. Result arrays always [], not null. Checker performs I/O
only, no provider/download/process launch. Typed ManifestRefusal/ArchiveLimitError
exceptions carry reason/observed/limit, not string sentinel errors.

Exact proposed interface:
python3 scripts/check-release-archives.py --manifest MANIFEST --out RESULT

RESULT schema fathom.release-validation/v1 has version/commit/tree and
checks [{archive_sha256,check,passed,reason}]. Refusal exits 2, checked violation 1,
all checks pass 0; no stdout secret/source data. Inputs read before any publication.
All four OS/Arch pairs appear exactly once, archives regular files, matching SHA.
Read tar metadata safely without extraction: reject absolute/traversal paths,
symlink/hardlink members, duplicate member names and unbounded members/archive.
Freeze bounds before run: <=256 MiB compressed/archive, <=1 GiB uncompressed/archive,
<=64 entries. A cap+1 fails whole archive with typed error, never partial pass.

Required exactly one fathom and one fathom-lang target binary, LICENSE.md, README.md,
and the generated third-party grammar notice (exact known basename THIRD-PARTY-NOTICES.txt). No placeholder/empty notice. The mandatory controller #254 check below regenerates expected notice from this
commit and its pinned source cache, checking every grammar entry/name/upstream/
pin/SPDX/copyright/full license/NOTICE. Metadata-only checker cannot replace it. Existing own LICENSE
bytes must match source and remain unchanged. Missing license/NOTICE where required
fails even if archive exists. Reject undocumented extra executable/script content.

Binary validation is file-format metadata, not cross-architecture execution:
ELF/Mach-O target OS/arch must match manifest; inspect Go build metadata for exact
source VCS revision, clean build and version record. Controller native smoke invokes the exact commands below on matching host; cross-target inability to execute is
not a build failure or fake runtime proof. Do not invent a fathom-lang version
verb: use existing extract/dialects/resources/contract known fixtures and metadata.
Attest exact archive and checksums bytes AFTER validation; upload those same bytes,
no rebuild between check/attest/upload. Published checksums and attestation subjects
must match local measured digests and tagged source. No receipt-only substitute.

## Named failing checker tests and release acceptance

Proposed scripts/check-release-archives_test.py unittest cases:
TestReleaseArchiveContract.test_four_targets_both_binaries
test_missing_binary, test_wrong_arch, test_foreign_commit, test_dirty_build,
test_checksum_mismatch, test_missing_notice, test_notice_pin_mismatch,
test_missing_pinned_license, test_missing_required_apache_notice,
test_project_license_unchanged, test_tar_escape_and_links, test_duplicate_member,
test_cap_plus_one, test_unknown_manifest_field, test_empty_arrays_json.
Each negative seeds its named defect and asserts nonzero typed failure with exact
archive/check named; positive uses synthetic metadata fixtures and exact expected
check list. Fixture paths are inline generated tar bytes under workspace tmp;
no real binaries/provider archives committed. W3-2 real failing tests belong to
the implementing checker lane, critic compares each assertion.
Command: python3 -m unittest discover -s scripts -p check-release-archives_test.py.
No test has executed here and this file does not yet exist.

Release proof is separate:
1. Capture approved version/final SHA and exact CUT-1/LIB-11/LIB-12 evidence.
2. Roll changelog/version with exact fragment manifest, then mandatory local gate
   and Linux CI on final clean main tree, with no later source mutation.
3. Explicit architect authorization names that version and SHA. Tag only then.
4. Local build/archive validation -> attest -> upload exact measured bytes.
5. #255 reads tag source + assets/digests/attestations and records LIB-1 verdict.
6. Outside-module/proxy reading LL-M7 follows the tag; LP-1 remains open until
   its additional consumer obligations hold. Tag alone cannot claim LP-1.

No release step runs in this task. Source evidence: .goreleaser.yaml includes
fathom-lang in four target builds but archives currently list only LICENSE/README;
release workflow currently uploads before attest. Existing buildinfo tests hold
version-to-changelog consistency. Draft proof only git diff --check and source
inspection. No tag/upload/gate/CI/proxy/notice/runtime or library-row claim.
Decision models: none. Archive identity/license completeness is deterministic.

## Corrected #254 handoff and generated provenance (F1)

Read #254's source-only correction 5e779cd54 after f9b744758 in its isolated lane:
noticeName is THIRD-PARTY-NOTICES.txt. Reuse its GenerateNotice(lockBytes, cache)
and CheckReleaseArchiveNotice(archive, expected), not a second licence generator.
That lane is not yet source-populated/proven for real 55 grammar archives; no PASS
is inferred here. Freeze integrator's parent to its approved landed handback before
release integration. Author does not edit or execute the other lane.

Mandatory full-validation controller:
bash scripts/validate-release-archives.sh --manifest MANIFEST --lock LOCK
--source-cache CACHE --notice-bin NOTICEBIN --out RESULT (one invocation line).

Exact LOCK is candidate commit's
internal/extractor/tsengine/grammars/grammars.lock.json. CACHE is operator-populated
local cache of <tarball_sha256>.tar.gz, matching every row's existing source digest.
NOTICEBIN is built from the approved #254 source at candidate commit, with local
binary SHA recorded. No release-time fetch. The controller verifies candidate
clean source identity before its work, runs the existing CLI exactly:
NOTICEBIN generate LOCK CACHE workspace/tmp/release-check/THIRD-PARTY-NOTICES.txt
NOTICEBIN check LOCK CACHE ARCHIVE1 ARCHIVE2 ARCHIVE3 ARCHIVE4
Then and ONLY then runs the no-process Python metadata checker, supplying the
generated notice file and provenance JSON. Grammar CLI internally regenerates
the expected notice from pinned sources for check. A supplied expected digest or
arbitrary notice file never bypasses it. Generation/check failure exits before
metadata PASS, attestation or upload. Controller owns subprocesses, not Python.

Provenance JSON keys exactly schema="fathom.notice-provenance/v1", commit, tree,
generator_commit, generator_sha256, lock_sha256, source_archives (sorted entries
grammar/pin/tarball_sha256/actual_sha256), notice_sha256. Generated by controller
from actual bytes/invocation, no caller-authored success boolean. All cache entries
required by this lock are present and matching, no stale pin/cached text substitute.
Store raw grammar generation/check stdout/exit and byte digests beside RESULT.
Python --notice FILE --notice-provenance FILE --manifest MANIFEST --out RESULT
checks byte consistency and archive metadata only; full validation receipt is
created solely by mandatory controller after real #254 checks. Never label an
isolated Python metadata pass “notice complete”. RESULT records separate
notice-generation/notice-archive-check/metadata checks and actual command exits.
No-process means Python does not execute binaries. Native smoke is separate
controller work after these checks; it cannot turn a failed notice into success.

Add scripts/check-release-archives_test.py cases test_arbitrary_notice_matching_digest
and test_stale_notice_matching_digest through the controller fixture: regenerate
expected from small valid pinned source cache, substitute arbitrary/stale archive
notice AND supply its matching digest. Full controller must still fail through
#254's regenerated expected bytes, with no attest/upload stub invoked. Preserve
#254 TestReleaseArchiveRequiresGrammarNotice as authority, not copy/paste a second
licence policy or expected literal. The cache/lock/generator identity must all bind
candidate commit, not a reading's claimed text.

## Canonical tar namespace and extracted-byte witness (F2)

Mandatory #254 CheckReleaseArchiveNotice runs first with its repaired semantics.
For every member: only regular file or directory; for a directory remove exactly
one trailing slash before checking, for regular file remove none. Reject empty,
absolute, ".", "..", any raw spelling unequal to POSIX clean path, leading ../,
empty components/repeated slashes, ./ prefix at any depth/repetition, traversal
components, backslash/colon Windows spellings, component trailing dot/space,
symlink/hardlink/special entry. Reject notice case variants on case-insensitive
targets. Exactly one root THIRD-PARTY-NOTICES.txt regular member, byte-identical
to regenerated expected. Never validate a stripped alias while extracting raw
spellings. Duplicate canonical member names fail across all types. Python may
check these again, but cannot weaken or skip the required #254 result.

Named new negative cases test_repeated_dot_notice_alias, test_repeated_slash_alias,
test_directory_file_alias, test_notice_case_alias, test_trailing_dot_space_alias,
test_backslash_or_colon_alias, test_hardlink_and_symlink_notice cover both a valid
notice followed by alias and alias followed by valid notice. USTAR/PAX/GNU tar
fixtures retained as #254's TestReleaseArchiveRejectsMemberAliases/
TestReleaseArchiveNoticeExtractionWitness already does. New
test_extracted_notice_equals_validated_bytes runs ONLY synthetic accepted fixtures
through native tar in an isolated workspace directory and asserts extracted root
notice equals expected bytes; a rejected repeated-dot overwrite fixture demonstrates
the old hazard. Controller test owns that subprocess, metadata Python never extracts.
No OS claim from a host that was not exercised. #254 native Darwin witness does
not imply Linux/Windows extraction parity.

## Frozen controller native smoke handoff

After safe validation, extract accepted native archive under workspace tmp and
invoke only matching OS/arch binaries. Exact source fixtures at candidate commit:
cmd/fathom-lang/testdata/golden/src/{a.go,main.tf} and
pkg/lang/iac/testdata/registry.opentofu.org_example_widget.v1.iaccat.zip.
Operator variables BINDIR, ROOT, CATALOG are resolved inside isolated workspace.
Commands:
~~~sh
"$BINDIR/fathom" version
"$BINDIR/fathom-lang" dialects
"$BINDIR/fathom-lang" extract -root "$ROOT" a.go
"$BINDIR/fathom-lang" resources -root "$ROOT" main.tf
"$BINDIR/fathom-lang" contract -catalog "$CATALOG" -version 1.0.0 widget_thing
~~~
Assert exits 0, version matches approved release + source identity, outputs parse
as their existing JSON goldens and default resource values absent. Compare source
fixture/golden digests at same candidate, not author-generated expected stdout.
No invented version verb for fathom-lang. Cross-target checks remain static
ELF/Mach-O/build metadata; no fake runtime PASS. No network/provider/model/daemon
required. Commands above are future controller handoff, none executed by author.

Run future checker/shell tests:
python3 -m unittest discover -s scripts -p check-release-archives_test.py
bash scripts/validate-release-archives_test.sh
go test -race ./cmd/grammarnotice -run
'^(TestReleaseArchiveRequiresGrammarNotice|TestReleaseArchiveRejectsMemberAliases|TestReleaseArchiveNoticeExtractionWitness)$'
-count=1 (last command one shell line).
Capture actual RED before implementation per W3-2 and compare assertions independently.
Only U1 version and U2 exact integrator/fragment grants plus U3 final tag authority
remain decisions. Notice basename/canonical semantics are source facts, not choices.
