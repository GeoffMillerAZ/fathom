# MCP adoption setup and server hardening — 2026-09-21

## Scope and findings

Current executable follow-up: [September22 installed refresh](2026-09-22-mcp-installed-refresh.md).
The following host trials remain dated evidence, not trials of every later binary.

Owner requested working app-kit adoption tooling in both Codex and Claude Code,
after a new Conjure session showed only the legacy TailwindPlus server. This is a
bounded setup/server maintenance task, not a restart of the deferred G1–G6 campaign.
Authority includes app-kit MCP build/registration and directly related guidance;
no consumer business/UI changes, deployment, purchased-source editing or publication.

Live inspection found Codex 0.155.1 missing `appkit`. Claude Code 2.1.278 had the
correct user registration but its executable was a September 13 dirty build.
Neither app-kit nor Conjure had a project MCP override. Codex global guidance lacked
the kit route; Claude's global route still recommended blanket palette replacement.
Older ADR-0011/0015 also incorrectly described `.mcp.json` as Codex configuration.
The host-convention/stale-guidance revisit condition fired; those facts are corrected.

## Acceptance and implementation

- One unified local stdio server; no new service, auto-installer, telemetry, arbitrary
  execution tool or network-facing listener. Private corpora remain optional.
- Correct host-specific registration, preserve unrelated settings, private recovery
  copies, identical short global routers, and current setup docs exposed through MCP.
- Current executable, truthful build identity, first-512-byte adoption/trust guidance,
  read-only annotations, finite search/list limits and exact-source preview paging.
- Synthetic protocol refusals must leave the session usable. Real-binary opt-in
  checks bind intent resource/tool bytes, catalogs and purchased source to local files.
- Separate fresh host calls must succeed or be reported as unproven. Configuration
  and source lookup do not establish UI quality, fleet adoption or a released pin.

The pinned Go SDK advances from 1.6.1 to 1.8.0 after a separate module-file candidate
passed the complete MCP package. The candidate adds only `golang.org/x/time v0.15.0`
as an indirect dependency. The [upstream release](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0)
addresses bounded decoding and transport lifecycle failures; this is the ADR-0011
protocol-upgrade review, not adoption of sampling, remote OAuth or new execution.
Active-package tests also pass. No security escape-hatch flags are enabled.

## Compatibility / operator migration

Rebuild `appkit-mcp` and reconnect existing hosts. Do not compile on normal startup.
Commercial search accepts 1–20 rows; listing accepts 1–200 (zero retains each
existing default). Browse follows `has_more`/`next_offset` with unchanged filters
and index/session; it is not a revision-bound snapshot. A 205-item synthetic corpus
proves complete two-page enumeration without duplicates. Invalid axes and negative,
overflow-risk or out-of-range limits now fail explicitly.
An omitted/empty search query is valid structural discovery; it does not need an
embedding request. Nonempty semantic queries retain their explicitly annotated
network boundary. These are search contracts, not full-text similarity guarantees.
`get_component_preview` defaults to 32 KiB, capped at 64 KiB per page. Clients must
follow `has_more` / `next_offset` with the same variant and `expected_sha256` before
treating a large preview as a complete HTML document. Small previews stay complete;
pages preserve original UTF-8 bytes. This intentionally changes oversized responses.
Initialize version describes the actual build, including dirty/development status;
record the executable SHA separately, since a VCS revision cannot identify dirty bytes.

The paired global reminder points to consumer-owned contracts and the shared
[agent setup](../design/agent-setup.md), rather than duplicating architectural policy.
The original global files and old binary are retained privately, not committed.

## Verification record

Synthetic package and candidate-SDK tests passed, including explicit protocol
2025-03-26, 2025-06-18, 2025-11-25 and 2026-07-28 negotiation, annotations, schema
refusals/recovery, 205-item browse paging and UTF-8/hash-bound preview reconstruction.
The installed stdio probe discovers **22 tools**, compares intent tool/resource
bytes with disk, checks capability/quality revisions, and compares retrieved
TailwindPlus and MatDash source bytes with the owner's local SQLite indexes.
Archive discovery agrees with the private catalog's **17 archives**. This checks
retrieval integrity, not license completeness, renderer quality or adoption.

The first Claude Haiku trial passed seven calls. The first Codex Sol trial exited
zero but failed search/schema validation: internal filter-only support still had
a required `query` field. That trial is **failed**, not connection proof. The
omittable-query repair has a retained red/green regression. Repeating the unchanged
prompt against binary `896bd7c6d5262aea660e19e9085d4c54e865063c065c0362a728ae46d36f1296`
passed all seven calls in both hosts (Codex 45.5 s; Claude 25.6 s). Claude reported
$0.0527876 for that trial; it is not a general cost or token-efficiency benchmark.
Independent review then identified the semantic filter-only edge case above;
its regression fails before repair (two embedder calls), and passes after repair
(zero calls). The reviewed MCP package and race checks pass. Both hosts again
pass **7/7 calls** on the reviewed binary below (Codex 47.3 s; Claude 22.1 s).
The licensed installed-binary probe passes on that same hash, with clean shutdown.

| Reviewed identity | Value |
|---|---|
| Executable SHA-256 | `e91d165b9aa9332c646e237fda883b0e9d4843d06875148be351ea6e7fd6d5a2` |
| Intent SHA-256 | `1368cfc1c96d18c8f5347a9a1db1743baaa44cad48966cd1a12d6f40e2d0df66` |
| Capability revision | `6807cf22b45098a82a82f2297eead7151683d13e2fc3f5f3281b59f7a43a876d` |
| Quality revision | `4eaf958d46e61cc514e253501f749cdabbcf9f64d0e469052edf57a1a301ea9c` |

This executable truthfully identifies itself as a dirty development build from
the pre-landing commit. A clean post-commit rebuild changes its VCS identity and
hash; its separate `landed-live-probe.log` must pass before replacing the installed
binary. The model-host receipts above remain bound to the reviewed build.

The trials use each host's actual registered command/env in an **invocation-local
isolated projection**, without unrelated MCPs or built-in execution tools. They
prove real Codex/Claude protocol calls, not automatic loading of global instructions
or every project override. Effective user registration was inspected separately;
Claude's normal `mcp get appkit` reports Connected. Codex configuration comparison
proves only appkit was added; other settings and Claude registrations are preserved.
Existing sessions must restart/reconnect to load the replacement binary/guidance.

`task check` passes with canonical `TMPDIR=/private/tmp`: vet, all Go packages,
CUE, browser-controller fixture suites and generated-token cleanliness. Earlier
failures are retained: copied Go evidence needed an ignored module traversal
boundary; a native source-snapshot test rejected macOS's `/var` symlink alias.
No test, native symlink refusal or quality gate was skipped or weakened. The final
MCP-only edge repair is separately retested. Strict Fathom plan verification proves
**40/40 claimed tasks**, with 55 total; this is not the broader uplift percentage.
Scoped root-intent budgets and affected relative documentation links pass; this is not
a claim that every inherited intent/changelog debt was pruned.

Private originals: `/private/tmp/appkit-mcp-adoption.3oFbMP`. Selected receipts,
logs and host traces are retained in ignored
`workspace/artifacts/mcp-adoption/2026-09-21/`; configuration backups stay outside
the repository, durably copied to the private directory
`/Users/geoff/.local/state/app-kit/mcp-adoption/2026-09-21.EeUTBf/`.
Do not commit raw host traces, configuration backups or licensed
source. Final post-review probe status belongs with the exact executable identity;
do not extrapolate a prior binary's receipt to a new build.
