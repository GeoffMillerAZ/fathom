# Static-content starter: composition and acceptance

Work in progress for `core.static-content` under ADR-0069. The Gallery composition
and focused model/HTTP regressions are implemented. This initial assessment retains
failed browser acquisitions; [later validation and browser work](2026-09-16-static-content-browser.md)
has its own artifacts. Full quality gates, static-host delivery and adoption remain open.

## Task and design boundary

A developer reads guidance, follows its outline, discloses answers, visits a real
reading destination and recovers from missing/unavailable content. The scene is
desk-based kit selection on a laptop or narrow reference window, in either ambient
light condition; retain the Gallery's explicit light/dark choice and restrained
semantic roles rather than imposing one scheme. Native browser behavior suffices.

The existing Gallery shell supplies navigation, theme links and focus conventions.
The complete composition reference is Tailwind Plus Syntax's documentation layout:
bounded prose, distinct heading hierarchy, an outline and onward reading. Inspected
private source: archive
`syntax@015a49d79eda5796986933f460094310d8f4ced5e8a579551f99d0bd9e11b8e1`,
`tailwind-plus-syntax/syntax-ts/src/components/DocsLayout.tsx` and
`src/app/docs/installation/page.md`. The archive SHA-256 matched its identity.

This is an original kit composition informed by those structural choices, not a
Syntax port or visual-equivalence claim. No purchased text, source, imagery or fonts
are copied. It reuses `appkit/ui/docs` and existing semantic presentation; app-owned
plain content and destination records remain private to the Gallery package.
No public rendering API, CMS, search engine, automatic route discovery or framework
is introduced. ADR-0041's private FAQ adapter remains an independent source choice.

## Acceptance fixed before browser acquisition

[Fixture contract](fixtures/static-content/README.md) separates exact model/HTTP
refusals, script-free browser tasks and local budget observations. The CUE contract
sets asset/response/document/load ceilings before measurements. Invalid typed input
must refuse atomically. Synthetic state controls must never imply a real backend.

The static-content starter gap triggers ADR-0069's existing revisit condition; this
implements that admitted task without expanding the five-area scope. Full static
delivery, complete accessibility/performance and consumer integration remain separate
requirements. The inventory may link scoped evidence only after it exists.

## Implementation and narrow checks

`/content` provides the guide, native outline and independent FAQ disclosures.
`/content/choosing-a-profile` is a real reading destination. The native GET state
form distinguishes ready/empty (200) from synthetic unavailable (503); an unknown
destination returns 404 with a themed recovery link. Theme links retain the selected
page/state. Content/destination validation rejects invalid inputs before output;
fragment targets accept native focus without entering the Tab order.

The shared `docs.RenderToCPanel` now supplies 44px minimum targets, long-word wrapping
and explicit link focus utilities. These are shared markup/class changes: rebuild
consumer CSS after upgrading and scan `appkit/ui/docs/*.go`. The boxed ToC and rail
remain unchanged; no consumer is automatically upgraded. The catalog's `content.docs`
record links to the Gallery composition and names its limited delivery claim.

The new `TestRenderToCPanel_TargetAndFocusClasses` failed against unchanged renderer
SHA-256 `e58bfc5b366ff20c99ddba936c4cbc4b882c0b6ac65f0fde857313b52d7a5298`:
both controls lacked minimum target/wrapping classes, and links lacked explicit focus
classes. The test hash was
`bfeaf273515dfa683240fbba9199bf8f6d99d0700ef29767165f033b6a30409a`.
After repair, focused ToC and Gallery static-content tests passed. This is a source
regression, not a measured pre-repair browser comparison or accessibility certification.

The first private preparation at `/Volumes/m2ext/appkit-evidence/appkit-static-content-SRE6n9`
failed before CSS/build/browser work: the preparer's Go-list template contained an
incorrectly escaped newline. Its executed source, command, stderr and failure receipt
remain intact. The corrected preparer changes only that source-list formatting;
it does not change any task oracle or numeric budget.

The local preparation at `/Volumes/m2ext/appkit-evidence/appkit-static-content-Dh0hru` passed full
Gallery/documentation-package tests and two identical builds. A subsequent explicit
fragment-focus styling repair has its own passing preparation at
`/Volumes/m2ext/appkit-evidence/appkit-static-content-bk2nf9`: 166 retained local source inputs,
full package tests, unchanged-source checks and two matching builds. It uses existing
module/build caches with offline Go resolution, not two cold or standalone builds.
No working-tree generated CSS or existing preview was replaced.

| First browser input (`bk2nf9`) | SHA-256 |
|---|---|
| Both Gallery binaries | `04c42c8c785b12b1bc00505526efb054d7437733ddeca3eb4a538d5a0672794b` |
| Private generated CSS (51,715 bytes) | `f77f7eb665a10432a0f391975004ac9cdb95fdd1874b279c24dd1b8b4fef6941` |
| Concrete local budget contract | `87229eb068a7916b442132922f0436dcd1e90979253db5a91b9119f89a2e0507` |

Both preparations predate later authored readiness-note updates; neither binary
automatically inherits those metadata changes.
The first later source-MCP document check failed on recurring filesystem `Operation
not permitted` reads, not a verified document-content mismatch. After ordinary access
returned, the same `TestIntentDocumentsThroughMCP` passed. The earlier failure remains
an environment failure, not a retroactive pass. Readiness CUE validation, quality-plan
and catalog package tests, 120 local documentation links and source diff checks pass.
The pre-typeahead browser harness passes syntax checks and 14 pure refusal canaries before
acquisition. Its independent review removed forced-focus false proof, tightened
global request accounting and preserved graceful-exit/actual-listener-refusal checks.

## First browser acquisition: retained failure

`/Volumes/m2ext/appkit-evidence/appkit-static-content-bk2nf9/static-content-LtN3gx/receipt.json`
has SHA-256 `e7da2964a7c41d14d547336b1921772e0f25749abf4aa8726cfa95cdccadc499`.
The frozen harness is
`bc3b2eb38777ac4e6b9c70eefaf062a693b9f84bf70d6e6b656f2b008bc8aa59`;
binary, CSS and contract match the table above. Chrome is 152.0.7977.84.

The first 390px/light reading, article and native-back observations pass their
scoped oracles. The subsequent keyboard select sequence submits `state=ready`
instead of selecting `empty`, so the required empty-state recovery link is absent
and the run fails. This observation does not establish an application defect or
general correctness. Five captures and the failed request/HTML are retained.
Browser/Gallery exit 0 with no signal; port 63637 returns `ECONNREFUSED`.

## Second acquisition: premature submission, not a browser diagnosis

`/Volumes/m2ext/appkit-evidence/appkit-static-content-bk2nf9/static-content-Y6uGcp/receipt.json`
has SHA-256 `d01c981de83b3460d27324effea6fc4d2c0720e21898d037216f167b8886a6a8`.
It uses the same binary, CSS and budget contract. The revised native sequence
`Space → Home → ArrowDown → Enter` fails with `wanted empty, selected ready,
focused false`. Reading, article and native-back observations pass again.

The request ledger includes `/content?theme=light&state=ready` and CSS before the
assertion labelled `preSubmit`. Enter therefore allowed a form submission during
selection; the assertion observes the reloaded page, not a proven pre-submit state.
The earlier interpretation that no state GET occurred, or that this establishes a
headless native-select limitation, is incorrect. No application defect is established.
Six captures and the failed receipt remain retained. Both processes exit 0 with no
signal; port 64169 returns `ECONNREFUSED`.

The bounded trace at `appkit-static-content-bk2nf9/select-diagnostic-V6ihmM` retains
`select-diagnostic-receipt.json` SHA-256
`2726d2d8dc44179ac351c2e407bdbc1b10933c51d215ad8131e7b157ad6bd2dd`.
Value `ready`, index 0 and SELECT focus persist through Space/Home/ArrowDown.
Enter navigates to `state=ready` and moves focus to BODY. Both processes exit 0
without a signal; port 49219 refuses connections. This isolates premature submission;
why that native sequence did not select a different option remains unresolved.
The diagnostic's page-local event array is empty after reload and supplies no further
input proof. It is not performance evidence or an acceptance run.

The [follow-up](2026-09-16-static-content-browser.md) tests native full-label typeahead
without selection-time Enter and checks actual value before explicit button submission.
No JavaScript value mutation is allowed. Neither failed run passes a complete browser,
performance or visual gate; required tasks, input methods and budgets stay fixed.
