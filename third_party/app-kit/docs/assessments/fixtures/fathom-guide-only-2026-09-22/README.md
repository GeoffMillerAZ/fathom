# Fathom guide integration: separated review packet

Applied to active Fathom's working tree, without consumer commit/push/restart.
[Evidence and remaining gates](../../2026-09-22-fathom-guide-only.md).
Do not apply the old mixed `theme-guide-and-analytics.patch` together with these.

Base: Fathom `fa8c8ed854ff319227638c4ef79a15d3c00e50b8`.
Required exact app-kit mirror: `f7ab0fbd3496e9dc3da8c97b6c4290f09a71ec50`.
The mirror is deliberately not duplicated in this packet; produce its exact Git
archive and verify every file plus `.vendor-commit`. Never patch the mirror body.
No licensed corpora are included by that archive or these patches.

| Patch | Scope | SHA-256 |
|---|---|---|
| [theme-guide.patch](theme-guide.patch) | Seven guide/command/CUE/CSS builder files; original tests unchanged | `da638ef423859fa4af85df73783d2374d9e7d7a2339580e6a25d2eb0d0f401fb` |
| [module-prerequisite.patch](module-prerequisite.patch) | Only indirect uniseg v0.4.7 and two checksums in root manifests | `eb0ad07d65ed95b7788770d58afc684b7821b201d3c60e8837cc23e45692c55d` |
| [display-truth.patch](display-truth.patch) | Registry lower bounds and independently unavailable call-log/ledger display; no service JSON change | `bf8e6c1dc938df8de9377642f032ad26fb8dd6d3f8a9419494fc576590861b84` |
| [css-content-read.patch](css-content-read.patch) | Follow-on to theme-guide.patch: authoritative content-read guard and 12 synthetic refusal/control cases | `0c1a747411523a2630aefddca0df63ed74ebb0c5b2b3b089c782a30a8ad0bbe1` |
| [adoption-handoff.patch](adoption-handoff.patch) | Selected metadata/artifact pins, generated CSS, consumer adoption/intent/command/changelog notes | `265aa70ff0e5f963691966e11cd05a8536c9d7eb3f9d70c84c9633fbf5f34f5f` |

The owner has granted expanded authority for all three patches. The full mirror also
changes its transitive module graph and large-number chart label spacing; these
effects are disclosed in the assessment. Do not adopt only the guide and suppress
its failing source-validity tests: that produces misleading synthetic specimens.

Read-only check, from a fresh exact Fathom baseline:

```sh
git apply --check /path/to/packet/theme-guide.patch \
  /path/to/packet/module-prerequisite.patch /path/to/packet/display-truth.patch
```

This was executed successfully. It does not install the exact mirror, refresh
adoption metadata, verify generated CSS or establish browser acceptance.

The final two patches follow the original three; do not apply this packet again to
the now-integrated checkout. Recheck HEAD/edits before any independent replay.
The CSS follow-on was applied after the original guide patch in an isolated baseline
and both scripts match the tested candidate exactly. The final source passes real
build/check, readonly race/vet/build and selected static adoption checks.

The retained [browser owner](browser/owner.mjs) takes six absolute arguments:
copied binary, baseline CUE, Playwright entry, Chrome executable, prospective contract,
and a new output directory. Run with the contract-pinned Node executable; the owner
checks identities and performs the bounded two-context synthetic workflow and cleanup.
The [contract](browser/prospective-contract.json) belongs to this exact candidate,
not arbitrary future source. Browser passes are scoped; large charts, zoom and broader
quality gates remain open. Existing agent routing, service JSON and agentic-kit are
preserved. No consumer commit, push, daemon restart or deployment is authorized.
