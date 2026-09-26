package qualityplan

schema_version:                1
enhancement_standard_revision: "converted-core-v1"

standards: {
	"typed-config": {
		name:        "Typed configuration"
		requirement: "Use profile-appropriate typed inputs, useful defaults, explicit app-owned slots and atomic validation before output."
		gate_refs: ["structure", "security", "integration"]
	}
	"semantic-theme": {
		name:        "Semantic theming"
		requirement: "Use semantic roles, keep identity separate from scheme and preserve truthful state meaning across supported presentation modes."
		gate_refs: ["visual", "data", "interaction"]
	}
	"native-interaction": {
		name:        "Native interaction"
		requirement: "Prefer native controls, visible focus, stable identity and explicit ownership across keyboard, narrow, zoom and lifecycle paths."
		gate_refs: ["structure", "interaction", "lifecycle"]
	}
	"honest-states": {
		name:        "Honest states"
		requirement: "Declare applicable empty, loading, saving, success, error, permission, stale, partial and conflict states with useful recovery."
		gate_refs: ["data", "interaction", "integration"]
	}
	"bounded-performance": {
		name:        "Bounded performance"
		requirement: "Name the workload and budgets for assets, useful view, update, memory and cancellation before adding heavier machinery."
		gate_refs: ["performance", "lifecycle"]
	}
	"safe-composition": {
		name:        "Safe composition"
		requirement: "Escape untrusted content, review trusted slots and keep routes, authorization, data and dependency closure explicit and app-owned."
		gate_refs: ["security", "structure", "integration"]
	}
	"reproducible-delivery": {
		name:        "Reproducible delivery"
		requirement: "Version inputs, rules, patches and outputs; support exact replay and consumer builds without hidden sibling-source dependencies."
		gate_refs: ["reproducibility", "integration"]
	}
}

backlog_policy: {
	mode:          "just-in-time"
	default_state: "unselected-not-run"
	selection_requires: [
		"a named consumer task and target profile",
		"an exact original or first-party capability",
		"a bounded batch and frozen acceptance contract",
		"an owner and explicit stopping condition",
	]
	owner_batch_requires: [
		"named capability or source IDs rather than an all-catalog sweep",
		"a case, token or time budget with retained partial results",
	]
	unselected_claim: "Commercial variants absent from this inventory remain on demand with every gate not run; absence is neither rejection nor a pass."
}

items: [
	{
		id:                            "core.shell-settings"
		kind:                          "starter-area"
		owner:                         "app-kit maintainer"
		task:                          "Navigate the application shell, inspect identity and settings, edit and save, then recover from validation, permission and persistence failure."
		profile:                       "go-web"
		priority:                      "core"
		core:                          true
		state:                         "in-progress"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["shell.layout", "shell.appearance", "shell.account-workflow", "shell.settings-sections", "interaction.forms"]
		source_refs: [
			{path: "docs/design/application-shell.md", note: "Shell task and app-owned authority contract."},
			{path: "appkit/ui/dashboard/shell.go", note: "Shared shell renderer."},
			{path: "appkit/ui/form/settings.go", note: "Experimental settings-section renderer."},
			{path: "docs/assessments/2026-09-23-radio-feedback.md", note: "Scoped radio feedback and native recovery evidence; full starter gates remain open."},
			{path: "docs/assessments/2026-09-23-button-destinations.md", note: "Scoped link refusal and disabled-action evidence; no route authority or full starter acceptance."},
			{path: "docs/assessments/2026-09-23-select-state.md", note: "Missing nonempty select values and duplicate-empty placeholders no longer silently substitute choices. Scoped controlled-option reset/validation/submission evidence; native-key choice failure, disabled-choice policy and full starter gates remain separate."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed shell and settings models exist, but the complete workflow contract is not accepted.", next_step: "Review every exposed option and refusal against the complete shell task.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Typed section refusal and scoped adoption evidence."}]}
			"semantic-theme": {status: "partial", reason: "Selected shell routes have two-scheme evidence, not a complete identity and accessibility matrix.", next_step: "Run the complete shell task across selected identities, schemes and contrast states.", evidence: [{path: "docs/assessments/2026-09-13-shell-browser.md", note: "Scoped shell theme and viewport observations."}]}
			"native-interaction": {status: "partial", reason: "Navigation, forms and recovery are exercised in scoped synthetic journeys.", next_step: "Close remaining long-label, zoom, focus-return and lifecycle cases for the composed shell.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Keyboard, narrow and script-disabled scoped checks."}]}
			"honest-states": {status: "partial", reason: "Synthetic save, validation, conflict and permission paths exist without full account-workflow acceptance.", next_step: "Inventory and prove every applicable shell state without inventing an account backend.", evidence: [{path: "docs/design/application-shell.md", note: "Required shell states and ownership."}]}
			"bounded-performance": {status: "partial", reason: "The pinned local Profile/Settings and load-error/retry baseline meets its asset and navigation budgets; useful-paint, input, memory, warm-cache and degraded-condition proof remain open.", next_step: "Extend the selected shell workload to useful-view, input/update, memory and degraded-condition budgets without inheriting this local result.", evidence: [{path: "docs/assessments/2026-09-16-shell-performance.md", note: "Thirty source-grounded v3 observations; earlier harness failures remain retained."}]}
			"safe-composition": {status: "partial", reason: "Trusted settings slots and app-owned actions are explicit, while full composed security review remains open.", next_step: "Review every trusted slot, route, CSRF and authorization boundary in the selected consumer task.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "RAC retains credential POST, CSRF and persistence authority."}]}
			"reproducible-delivery": {status: "partial", reason: "Two offline consumer builds and selected-vendor checks exist without a released clean pin.", next_step: "Repeat from a versioned candidate without a sibling checkout and retain exact migration evidence.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Scoped RAC vendoring and build evidence."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Scoped duplicate builds do not establish released source closure.", next_step: "Run a clean versioned consumer build with exact source and artifact pins.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Two identical offline trimmed builds."}]}
			structure: {status: "partial", reason: "Heading, label, order and atomic-refusal checks cover selected sections.", next_step: "Check the full composed shell structure and all entrypoints.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Settings-section structural checks."}]}
			visual: {status: "partial", reason: "Selected views were inspected at bounded widths and schemes.", next_step: "Complete identity, long-content, zoom and assistive visual review.", evidence: [{path: "docs/assessments/2026-09-13-shell-browser.md", note: "Bounded shell captures and visual review."}]}
			interaction: {status: "partial", reason: "Synthetic navigation and settings tasks pass but do not cover the full acceptance matrix.", next_step: "Run every required keyboard, pointer, narrow and recovery task.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Scoped browser journeys."}]}
			lifecycle: {status: "partial", reason: "Save interruption and recovery are scoped, not full shell lifecycle proof.", next_step: "Exercise cancellation, stale responses and repeated mount or route transitions.", evidence: [{path: "docs/assessments/2026-09-15-shell-settings-input.md", note: "Settings interruption observations."}]}
			security: {status: "partial", reason: "The consumer retains CSRF, credential and permission enforcement; the kit does not authenticate it.", next_step: "Review the exact selected action and authorization boundaries with negative requests.", evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Consumer-owned credential and CSRF boundary."}]}
			data: {status: "partial", reason: "Synthetic form values and failures are checked without production data semantics.", next_step: "Verify field meaning, missing values and persistence agreement in each adopter.", evidence: [{path: "docs/design/application-shell.md", note: "Synthetic state contract."}]}
			performance: {status: "partial", reason: "Thirty local desktop/light and narrow/dark observations meet unchanged payload/navigation ceilings, including native no-script load recovery; this is not the full performance gate.", next_step: "Measure useful-paint, input/update, memory, warm-cache and degraded-condition behavior for the selected consumer task.", evidence: [{path: "docs/assessments/2026-09-16-shell-performance.md", note: "Exact binary/CSS/script identities, 87 response records, task facts and owned-process cleanup; no production or native-host performance claim."}]}
			integration: {status: "partial", reason: "G2's finite shell behavior/adoption acceptance is met in RAC and Fathom; complete starter certification and versioned delivery remain separate.", next_step: "Finish the declared core standards and versioned delivery without rerunning or enlarging the completed G2 task.", evidence: [{path: "docs/assessments/2026-09-16-shell-acceptance-matrix.md", note: "Clause-bound Gallery/RAC/Fathom acceptance; fresh Gallery 287 and RAC 288 checks, retained pre-CSS failures and negative controls; no consumer edits."}, {path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "RAC scoped presentation adoption."}, {path: "docs/assessments/2026-09-16-fathom-shell-acceptance.md", note: "Actual read-only recovery after unchanged 30s TTL."}]}
		}
		consumers: [{
			app: "resume-as-code"
			revision: {kind: "artifact", sha256: "d4dd332f8e486a6a9aa8adb72bd547e5f17a897c3733c9bd6210ee05e1dcba6c"}
			integration_status: "verified-scoped"
			evidence: [{path: "docs/assessments/2026-09-16-settings-section-recipe.md", note: "Owner-authorized presentation adoption with preserved app behavior."}]
		}]
	},
	{
		id:                            "core.analytics"
		kind:                          "starter-area"
		owner:                         "app-kit maintainer"
		task:                          "Filter and compare metrics, explain the result, inspect rows and detail, and export the same truthful dataset."
		profile:                       "go-web"
		priority:                      "core"
		core:                          true
		state:                         "in-progress"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["analytics.comparison", "analytics.bars", "analytics.stat", "data.table"]
		source_refs: [
			{path: "docs/design/analytics-journey.md", note: "Complete analytics task and truth contract."},
			{path: "appkit/ui/analytics/analytics.go", note: "Typed shared dataset contract."},
			{path: "docs/assessments/2026-09-23-table-link-safety.md", note: "Managed data.table destination refusal and native target/focus task; app-owned actions and full table/core acceptance remain separate. No quality status promotion."},
			{path: "docs/assessments/2026-09-23-table-action-column.md", note: "Shared action-column inference and mixed-row alignment; caller state, custom actions and explicit empty-state reservation preserved. Scoped proof, not full table/core acceptance."},
			{path: "docs/adr/0077-incubate-rectangular-analytics-export.md", note: "Experimental single-dataset table CSV; shared presentation cells and explicit empty-selection context limit. Existing exports unchanged; no quality promotion."},
			{path: "docs/adr/0074-incubate-signed-analytics-comparisons.md", note: "Experimental signed single-dataset renderer; existing renderer contracts and all partial gates remain unchanged."},
			{path: "docs/adr/0075-incubate-app-owned-analytics-detail-links.md", note: "Optional native row destinations, with app-owned routing and unchanged CSV; no permanent promotion."},
			{path: "docs/assessments/2026-09-23-analytics-detail-links.md", note: "Scoped native link/detail/Back/export and long-label evidence; narrow missing-state density, full quality and independent adoption remain open."},
			{path: "docs/adr/0076-preserve-readable-missing-state-columns.md", note: "Independent missing-row width repair; rebuild CSS and review snapshots and selected pins without changing historical link-only evidence."},
			{path: "docs/assessments/2026-09-23-analytics-state-width.md", note: "Four frozen browser cells prove readable missing reasons and native keyboard reach/return, retaining baseline density failures; full quality and consumer gates remain partial."},
			{path: "docs/assessments/2026-09-22-signed-analytics.md", note: "Historical four-cell signed acquisition passes numerical/native/contrast checks but fails the unchanged HTML payload target; original failures retained."},
			{path: "docs/assessments/2026-09-22-gallery-nav-payload.md", note: "Gallery-only style deduplication passes all four unchanged signed payload/browser cases and 384 checks across 12 paired shell cases; no full gate promotion or consumer change."},
			{path: "docs/assessments/2026-09-22-adoption-boundary-hardening.md", note: "Explicit-zone validation and pre-output refusal controls; retained host-dependent Local failure, not browser or full typed-config acceptance."},
			{path: "docs/assessments/2026-09-21-core-rendering-hardening.md", note: "Scoped text/dimension admission regressions; full standards and consumer gates remain partial."},
			{path: "docs/assessments/2026-09-22-bar-resolution-and-guide-discovery.md", note: "Positive subpixel mark reproduction and table-only resolution fallback; no changed dataset, CSV, low-level primitive or quality promotion."},
			{path: "docs/assessments/2026-09-22-analytics-scroll-settling.md", note: "Exact baseline/candidate native table right-edge and return-left tasks pass with bounded observation and no-input controls; historical immediate-sample failures remain retained, with no full accessibility or G3 promotion."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed dataset validation exists without full journey acceptance.", next_step: "Review remaining configuration and boundary refusals across chart, table, detail and CSV.", evidence: [{path: "docs/assessments/2026-09-15-analytics-renderer-specimen.md", note: "Renderer API and CSV integration evidence."}]}
			"semantic-theme": {status: "partial", reason: "Roles and missingness semantics have scoped review, not complete accessibility acceptance.", next_step: "Verify every metric role and state in both themes without color-only meaning.", evidence: [{path: "docs/assessments/2026-09-15-analytics-role-meaning.md", note: "Declared role-meaning repair."}]}
			"native-interaction": {status: "partial", reason: "Filter, detail, return and export have scoped keyboard journeys.", next_step: "Complete focus, long-label, narrow, zoom and alternative-table acceptance.", evidence: [{path: "docs/assessments/2026-09-15-analytics-zoom-and-adoption.md", note: "Normal and actual-zoom synthetic tasks."}]}
			"honest-states": {status: "partial", reason: "Eight fixed complete/missing/stale/partial comparison journeys prove explanations, chart/table values, Gateway detail/back and all CSV cells in normal and actual200% Chrome; broader consumer acceptance remains open.", next_step: "Adapt the state oracles to selected consumers and cover remaining service, viewport, theme and accessibility requirements.", evidence: [{path: "docs/design/analytics-journey.md", note: "Required state and agreement contract."}, {path: "docs/assessments/2026-09-17-analytics-truth-state-journeys.md", note: "Source-bound eight-journey proof; earlier subset CSV oracle retained; no full consumer or G3 acceptance."}]}
			"bounded-performance": {status: "partial", reason: "The 200-row kit boundary, RAC's selected render/payload budgets and 24 Gallery styled-report readiness phases pass; full consumer, memory and cancellation performance remain open.", next_step: "Measure selected-consumer useful-view/update plus memory, warm-cache and cancellation budgets; reuse the exact Gallery receipt only within its declared scope.", evidence: [{path: "docs/assessments/2026-09-15-analytics-maximum-rows.md", note: "Bounded maximum-row evidence."}, {path: "docs/assessments/2026-09-22-analytics-adoption-integration.md", note: "Selected real consumer renderer/export with synthetic data; not browser timing."}, {path: "docs/assessments/2026-09-22-analytics-report-readiness.md", note: "Complete HTML/CSS, fixed data/style/geometry and native filter/detail/return with unchanged 500/2500ms budgets; not LCP/INP, human speed or real-device acceptance."}]}
			"safe-composition": {status: "partial", reason: "The recipe's composed HTML and CSV boundaries have hostile-input and HTTP checks; consumer routing and authorization remain app-owned.", next_step: "Verify the selected consumer's link/export destinations and authorization with its own synthetic dataset.", evidence: [{path: "docs/assessments/2026-09-15-analytics-renderer-specimen.md", note: "Scoped escaping and composition evidence."}, {path: "docs/assessments/2026-09-16-analytics-composition-safety.md", note: "Hostile HTML/SVG/table text, independent CSV semantics, exact headers and GET/HEAD parity; no browser or consumer certification."}]}
			"reproducible-delivery": {status: "partial", reason: "Exact fixtures and receipts exist without a complete versioned consumer upgrade.", next_step: "Replay the full journey from a pinned candidate in a selected consumer.", evidence: [{path: "docs/assessments/2026-09-15-analytics-zoom-and-adoption.md", note: "Bounded retained browser evidence."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Fixtures and receipts are pinned but not a complete consumer build closure.", next_step: "Replay from a versioned source and selected consumer data fixture.", evidence: [{path: "docs/assessments/2026-09-15-analytics-renderer-specimen.md", note: "Exact fixture and renderer evidence."}]}
			structure: {status: "partial", reason: "A fixed four-state comparison composition now verifies overview, Gateway detail/back and both exports in ordinary and actual200% Chrome.", next_step: "Complete remaining service and consumer structural/accessibility coverage without generalizing this fixed composition.", evidence: [{path: "docs/assessments/2026-09-17-analytics-truth-state-journeys.md", note: "Eight state journeys, exact table/SVG label-value semantics and all33 CSV cells; selected state/theme pairs only."}]}
			visual: {status: "partial", reason: "Selected normal and zoom views were reviewed.", next_step: "Complete accessibility, density and consumer-theme review.", evidence: [{path: "docs/assessments/2026-09-15-analytics-zoom-and-adoption.md", note: "Scoped view and zoom observations."}]}
			interaction: {status: "partial", reason: "Synthetic filter, detail, return and export tasks pass in bounded lanes.", next_step: "Run the full keyboard and alternative-output journey in selected consumers.", evidence: [{path: "docs/assessments/2026-09-15-analytics-zoom-and-adoption.md", note: "Scoped keyboard task evidence."}]}
			lifecycle: {status: "partial", reason: "Bounded browser ownership and cleanup are checked, not live update cancellation.", next_step: "Exercise stale updates, cancellation and repeated journey transitions.", evidence: [{path: "docs/assessments/2026-09-15-analytics-zoom-and-adoption.md", note: "Browser ownership and cleanup evidence."}]}
			security: {status: "partial", reason: "Gallery recipe source/HTTP tests cover hostile text, source-link schemes and CSV download boundaries, not consumer authorization or a full security review.", next_step: "Review selected consumer export/link destinations and authorization; verify dependency and private-source boundaries for its delivered artifact.", evidence: [{path: "docs/assessments/2026-09-16-analytics-composition-safety.md", note: "Bounded hostile-data handler tests with exact retained source and package results."}]}
			data: {status: "partial", reason: "Gallery agreement and RAC's actual channel-yield calculation/renderer/export pass fixed synthetic expectations; full consumer acceptance remains open.", next_step: "Adapt the same canaries to the second selected app's own units, missingness and source rules; retain live-read and authorization limits.", evidence: [{path: "docs/assessments/2026-09-15-analytics-renderer-specimen.md", note: "Expected values and CSV agreement."}, {path: "docs/assessments/2026-09-22-analytics-adoption-integration.md", note: "RAC all-row and five-state native export proof; app-owned calculations, not an analytics.RenderBars adoption or live snapshot guarantee."}]}
			performance: {status: "partial", reason: "Kit row and RAC widget/export budgets pass; Gallery's 24 complete-asset readiness phases stay below 500ms ordinary and 2500ms degraded. Full consumer/performance acceptance remains open.", next_step: "Record selected-consumer useful-view/update, memory and cancellation evidence; do not generalize local emulation or infer a speedup from three trials.", evidence: [{path: "docs/assessments/2026-09-15-analytics-maximum-rows.md", note: "Bounded row workload."}, {path: "docs/assessments/2026-09-22-analytics-adoption-integration.md", note: "RAC 20-record widget/CSV byte, time and allocation budgets; CSS/network/browser excluded."}, {path: "docs/assessments/2026-09-22-analytics-report-readiness.md", note: "Maxima 103.4ms ordinary/595.0ms degraded and 81186 decoded bytes; failed callback acquisition retained, unchanged app/CSS, scoped native task and owned cleanup verified."}]}
			integration: {status: "partial", reason: "RAC's channel-yield export is integrated and verified in its dirty working tree, with a returned contribution record; two complete consumer journeys and reviewed versioned propagation remain open.", next_step: "When available, verify the second selected consumer and reviewed delivery; retain RAC-ADOPT-04 and its exact source/test identities without claiming a released upgrade.", evidence: [{path: "docs/design/analytics-journey.md", note: "Consumer acceptance boundary."}, {path: "docs/assessments/2026-09-22-analytics-adoption-integration.md", note: "Authorized originating-app integration, retained source/patch/browser evidence and reusable checks; no consumer commit or dependency refresh."}]}
		}
		consumers: [{
			app: "resume-as-code"
			revision: {kind: "artifact", sha256: "c727501e305a091802070e18761d6c8c0ceea8b9efe627be63edb79a86c1e831"}
			integration_status: "verified-scoped"
			evidence: [{path: "docs/assessments/2026-09-22-analytics-adoption-integration.md", note: "Revision identifies the executed synthetic browser-03.test binary, not a shipped app. Actual RAC ops.ChannelYield, kit viz and native CSV agree in the selected five-state task. Working-tree only; RAC-ADOPT-04 returns reusable checks. No full journey, release, analytics.RenderBars adoption or second-app propagation."}]
			receipt: {path: "docs/assessments/2026-09-22-analytics-adoption-integration.md", sha256: "4bf0ef771214a9d9f40d788630ee8c56b8512754afc4e943e6e1539133856069"}
			review: {by: "Codex integration and independent scoped evidence review", ref: "docs/assessments/2026-09-22-analytics-adoption-integration.md"}
		}]
	},
	{
		id:                            "core.static-content"
		kind:                          "starter-area"
		owner:                         "app-kit maintainer"
		task:                          "Read and navigate static content, disclose FAQ or help detail, and follow truthful destinations without requiring JavaScript."
		profile:                       "go-web"
		priority:                      "core"
		core:                          true
		state:                         "in-progress"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["content.docs", "interaction.chrome", "interaction.callouts"]
		source_refs: [
			{path: "docs/design/faq-adapter.md", note: "Native disclosure and source-reference choices."},
			{path: "appkit/ui/docs/docs.go", note: "Static documentation presentation package."},
			{path: "docs/assessments/2026-09-21-core-rendering-hardening.md", note: "Canonical fragment encoding with 24 baseline and 24 candidate native-keyboard checks; not a reproduced browser failure or full accessibility gate."},
			{path: "apps/gallery/static_content.go", note: "Gallery-owned complete reading model, native routes and synthetic recovery states."},
			{path: "docs/design/static-content-delivery.md", note: "Separate experimental plain-file reference: build-only Go, explicit CSS, theme variants and base-path-bound 404 recovery; not a new catalog profile claim."},
			{path: "docs/assessments/2026-09-22-static-cold-delivery.md", note: "Offline source/dependency closure prepared; inventory-order preflight failed before any cold build. Exact next repair and controls retained; no delivery or quality promotion."},
			{path: "docs/assessments/2026-09-22-static-cold-replay.md", note: "Separate corrected pair: identical binaries and eleven files from two empty local Go cache roots with file-only dependencies and selected sandbox denials; no consumer, independent-host or full-quality claim."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "The Gallery model validates bounded plain text, reserved identities and destination keys, and the compiled browser task retained exact state/query facts; selected-consumer policy acceptance remains open.", next_step: "Review the same typed composition against a selected consumer's content and destination policy.", evidence: [{path: "docs/assessments/2026-09-14-faq-typed-adapter.md", note: "Separate private typed FAQ evidence."}, {path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Exact compiled Gallery task and retained state/query facts."}]}
			"semantic-theme": {status: "partial", reason: "The local browser task preserved light and dark theme context through ready, article, state and recovery journeys at two widths.", next_step: "Complete zoom, contrast, accessibility and selected-consumer theme review.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Four-lane local theme and state evidence."}]}
			"native-interaction": {status: "partial", reason: "Script-free native details, fragment focus, history, state selection and guide recovery passed in the bounded Gallery journey.", next_step: "Complete broader keyboard destination, zoom, assistive-technology and consumer checks.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Native local browser journey; catalog and quality destinations were typed but not followed."}]}
			"honest-states": {status: "partial", reason: "Gallery distinguishes ready/empty/unavailable/unknown. The separate plain-file reference also proves actual deep-path 404 and usable recovery under a non-root host prefix.", next_step: "Verify equivalent truthful states and host configuration in a selected consumer.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "52 Gallery observations across four lanes."}, {path: "docs/assessments/2026-09-16-static-export.md", note: "90 static-file observations; unknown paths return 404, direct named error files only prove appearance."}]}
			"bounded-performance": {status: "partial", reason: "The exact local unthrottled Chrome task stayed within its predeclared byte and navigation-timing ceilings; this is not a broader performance gate.", next_step: "Add applicable cold, warm, degraded-network and selected-consumer performance evidence.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Local maxima: 72,984 decoded bytes, 24.5 ms response end, 33.6 ms DCL and 56 ms load."}]}
			"safe-composition": {status: "partial", reason: "Plain content and escaping boundaries are explicit for selected renderers.", next_step: "Review rich-content escape hatches, destinations and dependency closure.", evidence: [{path: "docs/assessments/2026-09-14-faq-typed-adapter.md", note: "Plain-text and source-boundary evidence."}]}
			"reproducible-delivery": {status: "partial", reason: "Gallery and the bounded exporter have matching local builds. Two independent empty-cache local exporter builds now match binary/CSS/all eleven site files with source/shared-cache/network denial and frozen file-only dependencies; consumer hosting and independent-host proof remain open.", next_step: "Prove an authorized consumer's exact source/CSS build and static-host MIME, CSP, cache and 404 policy; review a fresh closure when source or toolchain changes.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Exact Gallery repeat build."}, {path: "docs/assessments/2026-09-16-static-export.md", note: "Original exact-file generic-host browser evidence; no fresh browser claim."}, {path: "docs/assessments/2026-09-22-static-cold-replay.md", note: "Two local Go1.26.3 darwin/arm64 CGO-disabled cold builds and selected tests; not hermetic or consumer acceptance."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Two frozen-input local builds with independently empty caches produce identical exporter binaries and all eleven files. Canonical inventory regressions, seven exact test functions per run, selected sandbox denials and clean owned-process exits pass; earlier failed preflight and output-checker receipts remain unchanged.", next_step: "Prove selected-consumer content/hosting policy and any required independent-host/toolchain profile; do not repeat the same local pair without an input change or new hypothesis.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Gallery repeat-build/runtime identities."}, {path: "docs/assessments/2026-09-16-static-export.md", note: "Original static-file browser identities; not rerun."}, {path: "docs/assessments/2026-09-22-static-cold-delivery.md", note: "Original failed inventory preflight remains failed."}, {path: "docs/assessments/2026-09-22-static-cold-replay.md", note: "Scoped local cold pair and corrected independent output check pass; consumer/full quality remain open."}]}
			structure: {status: "partial", reason: "The bounded browser task verified one main landmark, heading and outline targets, native disclosures, typed destinations and script-free documents.", next_step: "Complete zoom, accessibility-tree and selected-consumer structural review.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Four-lane structure and exact document evidence."}]}
			visual: {status: "partial", reason: "Independent scoped review found no readability, wrapping, clipping, theme or focus blocker in selected ready, state, recovery and focus captures.", next_step: "Complete actual zoom, contrast, accessibility and selected-consumer visual review.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Selected narrow/wide light/dark state and focus captures; not all-capture certification."}]}
			interaction: {status: "partial", reason: "The bounded journey exercised native outline focus, independent FAQ disclosure, article navigation, browser back, state selection, theme changes and recovery without JavaScript.", next_step: "Complete broader keyboard, assistive-technology and selected-consumer destination checks.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Guide journey only; catalog and quality destinations were not followed."}]}
			lifecycle: {status: "partial", reason: "Gallery state/history checks and the separate static reference's native Back/Forward, article/theme navigation and deep-404 recovery pass with exact response accounting and owned-process cleanup.", next_step: "Verify a selected consumer's navigation and static-host ownership.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "52 Gallery document observations."}, {path: "docs/assessments/2026-09-16-static-export.md", note: "90 static-file observations, 180 exact responses and clean Chrome/server shutdown."}]}
			security: {status: "partial", reason: "Plain content boundaries are reviewed; rich content and destination policy remain app-owned.", next_step: "Run hostile content and unsafe-destination canaries for the selected composition.", evidence: [{path: "docs/design/faq-adapter.md", note: "Content and route ownership boundary."}]}
			data: {status: "partial", reason: "The local browser journey matched synthetic ready, empty, unavailable and unknown copy, HTTP status, theme context and recovery without stale ready content.", next_step: "Apply the same truth checks to selected consumer-owned content and data policy.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Visible synthetic state and recovery facts."}]}
			performance: {status: "partial", reason: "Gallery and the smaller static-file reference pass local unthrottled byte/navigation screens; no cold, warm, degraded-network, memory or consumer performance acceptance is implied.", next_step: "Run the remaining performance conditions for the selected delivery and consumer environments.", evidence: [{path: "docs/assessments/2026-09-16-static-content-browser.md", note: "Gallery local numeric screen."}, {path: "docs/assessments/2026-09-16-static-export.md", note: "Static maximum 25959 decoded document+CSS bytes; 17.1/22.2/38.1ms response/DCL/load; no visitor JS or fonts."}]}
			integration: {status: "partial", reason: "The recipe is first-party but lacks a selected consumer upgrade.", next_step: "Integrate one complete content task and retain its versioned delivery evidence.", evidence: [{path: "docs/design/just-in-time-conversion.md", note: "Static-content starter boundary."}]}
		}
		consumers: []
	},
	{
		id:                            "core.client-heavy-boundary"
		kind:                          "starter-area"
		owner:                         "app-kit maintainer"
		task:                          "Edit locally without losing draft, selection or history during outer updates, and dispose or remount the client subtree cleanly."
		profile:                       "go-web"
		priority:                      "core"
		core:                          true
		state:                         "in-progress"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["interaction.forms", "interaction.overlays"]
		source_refs: [
			{path: "docs/design/client-editor-island.md", note: "One-renderer ownership and lifecycle contract."},
			{path: "docs/assessments/fixtures/client-editor/protocol.ts", note: "First-party mount, update, snapshot and dispose protocol."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "The private specimen has a typed boundary but no public or accepted kit API.", next_step: "Review configuration, instance identity and invalid-update refusals for the selected client task.", evidence: [{path: "docs/design/client-editor-island.md", note: "Typed ownership boundary."}]}
			"semantic-theme": {status: "partial", reason: "Selected theme and zoom lanes exist without general client-subtree theme acceptance.", next_step: "Verify the selected renderer under every supported theme and forced-state mode.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-zoom.md", note: "Scoped actual-zoom theme evidence."}]}
			"native-interaction": {status: "partial", reason: "Typing, keyboard formatting, undo, update and disposal are scoped to one private specimen.", next_step: "Close IME, clipboard, assistive technology and complete focus lifecycle gaps.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-island.md", note: "Scoped editor interaction and lifecycle evidence."}]}
			"honest-states": {status: "partial", reason: "Draft preservation, busy, failure and cancellation are synthetic and bounded.", next_step: "Map every selected app state and recovery without implying persistence.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-datastar.md", note: "Outer update, failure and cancellation tasks."}]}
			"bounded-performance": {status: "partial", reason: "A declared large-document workload exists without consumer budget acceptance.", next_step: "Set and verify payload, input, update, memory and teardown budgets for the selected app.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-zoom.md", note: "Large-document and lifecycle workload."}]}
			"safe-composition": {status: "partial", reason: "One DOM owner and explicit disposal are proven while rich-content safety remains incomplete.", next_step: "Verify HTML safety, CSP, hostile content and app-owned persistence boundaries.", evidence: [{path: "docs/design/client-editor-island.md", note: "Composition and ownership contract."}]}
			"reproducible-delivery": {status: "partial", reason: "Two fixture builds match, but the renderer remains a private source-native specimen.", next_step: "Replay the exact selected renderer closure and consumer integration from versioned inputs.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-island.md", note: "Private build and retained evidence."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Private builds are retained without a promoted or consumer-delivered API.", next_step: "Rebuild the selected exact renderer closure from a versioned candidate.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-island.md", note: "Scoped build evidence."}]}
			structure: {status: "partial", reason: "Mount and ownership structure is checked for one specimen.", next_step: "Validate repeated instances, labels and subtree ownership in the selected app.", evidence: [{path: "docs/design/client-editor-island.md", note: "Mount and ownership contract."}]}
			visual: {status: "partial", reason: "Selected normal and actual-zoom lanes pass scoped visual checks.", next_step: "Complete consumer theme, long-content and accessibility review.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-zoom.md", note: "Scoped zoom and visual evidence."}]}
			interaction: {status: "partial", reason: "Core editing tasks are proven without IME, clipboard or assistive-technology coverage.", next_step: "Run the complete selected editor input matrix.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-island.md", note: "Scoped editing tasks."}]}
			lifecycle: {status: "partial", reason: "Update, dispose, remount, cancellation and outer morph are bounded synthetic tasks.", next_step: "Replay lifecycle and failure paths inside the selected consumer.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-datastar.md", note: "Actual outer-morph lifecycle evidence."}]}
			security: {status: "not-run", next_step: "Review rich-content safety, CSP, origin and persistence boundaries for the selected consumer."}
			data: {status: "partial", reason: "Draft snapshots and revision semantics are synthetic, not consumer-domain truth.", next_step: "Bind selected app validation, save and conflict semantics without moving authority into the kit.", evidence: [{path: "docs/design/client-editor-island.md", note: "Draft and authority boundary."}]}
			performance: {status: "partial", reason: "Local large-document measurements do not establish a consumer budget.", next_step: "Measure complete payload, input, update, memory and teardown against the named workload.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-zoom.md", note: "Bounded scale workload."}]}
			integration: {status: "partial", reason: "The Go-hosted specimen proves a boundary, not real consumer adoption.", next_step: "Select one app task and retain its versioned integration and recovery evidence.", evidence: [{path: "docs/assessments/2026-09-16-client-editor-datastar.md", note: "First-party integration specimen."}]}
		}
		consumers: []
	},
	{
		id:                            "core.themes-style-guide"
		kind:                          "starter-area"
		owner:                         "app-kit maintainer"
		task:                          "Select identity and scheme, preview real application states, validate semantic changes and export a reviewed candidate without changing saved preferences."
		profile:                       "go-web"
		priority:                      "core"
		core:                          true
		state:                         "in-progress"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["recipe.styleguide.local", "foundation.tokens", "foundation.theme.geoff-synthwave", "foundation.theme.geoff-indigo", "shell.appearance"]
		source_refs: [
			{path: "docs/design/theme-workbench.md", note: "Named identity and workbench contract."},
			{path: "docs/design/app-owned-styleguide.md", note: "Local preview, reset, validation and export contract."},
			{path: "apps/styleguide/handler.go", note: "Experimental local style-guide handler."},
			{path: "docs/assessments/2026-09-22-fathom-guide-only.md", note: "Fathom's f7ab mirror, active source equivalence and scoped browser return; broader acceptance remains open."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "CUE tokens, bounded edit models and three additional light/dark families exist without complete consumer acceptance.", next_step: "Retain invalid-edit refusals and verify selected families in real consumer specimens.", evidence: [{path: "docs/assessments/2026-09-13-app-owned-theme-review.md", note: "Typed edit and export evidence."}, {path: "docs/assessments/2026-09-16-paired-themes.md", note: "Three CUE-authored paired identities and deterministic checks."}]}
			"semantic-theme": {status: "partial", reason: "Named identity, scheme and semantic roles are separated; new paired palettes pass selected contrast checks, not every real state.", next_step: "Complete real-state meaning and supported-effect review in selected consumers.", evidence: [{path: "docs/assessments/2026-09-13-personal-themes.md", note: "Current named-preset roles and limits."}, {path: "docs/assessments/2026-09-16-paired-themes.md", note: "Selected text, control, status and focus pairs plus bounded browser replay."}]}
			"native-interaction": {status: "partial", reason: "Preview, reset and export controls have scoped browser tasks.", next_step: "Complete keyboard, narrow, zoom, long-label and recovery checks against real app specimens.", evidence: [{path: "docs/assessments/2026-09-14-theme-lab-browser.md", note: "Scoped theme-lab browser tasks."}]}
			"honest-states": {status: "partial", reason: "Baseline, candidate, invalid, contrast-unknown, stale acknowledgement and reset are declared.", next_step: "Verify every state against selected real consumer specimens and failures.", evidence: [{path: "docs/design/app-owned-styleguide.md", note: "State and export contract."}]}
			"bounded-performance": {status: "partial", reason: "Handler and scoped transport budgets pass, but actual memory acquisitions fail RSS, sampling and evidence checks. CSS reuse is proven only for ordinary Gallery pages; browser cancellation and consumer work remain open.", next_step: "Four fixed idle/edit no-submit controls pass while the same-candidate Stop reference fails; isolate submission/loading without inferring causation or safe target classification. Preserve target refusal; review memory metrics prospectively before full cache/cancellation/consumer acceptance.", evidence: [{path: "docs/assessments/2026-09-16-theme-guide-request-cost.md", note: "Prospective in-process workload."}, {path: "docs/assessments/2026-09-16-theme-guide-browser-cost.md", note: "Historical body/ZIP and numeric evidence with explicit timing qualification."}, {path: "docs/assessments/2026-09-16-theme-guide-transport.md", note: "Complete-asset timing, 29-case independent verification, session-bound degraded coverage and clean owned-process exits; not full performance acceptance."}, {path: "docs/assessments/2026-09-17-gallery-css-revalidation.md", note: "Two top-level CSS revalidations; 120 candidate theme tasks still have no cache reuse. Neither memory nor preview cache acceptance."}, {path: "docs/assessments/2026-09-17-theme-guide-memory.md", note: "Three failed memory acquisitions and corrected context attribution; small JS growth does not excuse RSS or evidence failures. No budget relaxation."}, {path: "docs/assessments/2026-09-17-theme-guide-target-controls.md", note: "Escape fails; bounded cleanup passes. Four no-app controls are clean. The ordinary/Stop comparison rules out cancellation as necessary; one ordinary run fails at reset, another passes, and Stop runs fail before retry. Read-only unknown-target metadata is inconclusive. Four idle/edit controls pass while the same-candidate Stop reference fails; eleven cancellation cells and independent acceptance remain open."}]}
			"safe-composition": {status: "partial", reason: "The guide is local and bounded but full font, asset and authentication wrappers remain app-owned.", next_step: "Review selected sources, fonts, assets, authentication and export destinations per consumer.", evidence: [{path: "docs/design/app-owned-styleguide.md", note: "Local-only security and ownership boundary."}]}
			"reproducible-delivery": {status: "partial", reason: "Generated themes and reviewed exports have deterministic checks without complete released consumer propagation.", next_step: "Verify selected themes and guide from a versioned candidate in a clean consumer build.", evidence: [{path: "docs/assessments/2026-09-15-resume-theme-guide.md", note: "RAC guide build and browser evidence."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Generated theme and export checks are scoped to working-tree candidates.", next_step: "Replay from exact selected inputs and a versioned consumer pin.", evidence: [{path: "docs/assessments/2026-09-15-resume-theme-guide.md", note: "Scoped theme-guide build evidence."}]}
			structure: {status: "partial", reason: "Token schema and bounded guide structure are tested for current presets.", next_step: "Validate each admitted family and complete real-state specimen set.", evidence: [{path: "docs/assessments/2026-09-13-app-owned-theme-review.md", note: "Schema and bounded source evidence."}]}
			visual: {status: "partial", reason: "Current named identities and three new light/dark pairs have scoped visual and contrast observations, not complete specimen acceptance.", next_step: "Review all real states, supported effects, actual zoom and below-fold specimens.", evidence: [{path: "docs/assessments/2026-09-14-theme-lab-browser.md", note: "Scoped captures and inspection."}, {path: "docs/assessments/2026-09-16-paired-themes.md", note: "Twelve exact captures and limited root visual review; original RED provenance gap retained."}]}
			interaction: {status: "partial", reason: "Preview, reset, validation and export tasks are bounded synthetic workflows.", next_step: "Complete keyboard, narrow, zoom and failure recovery in selected consumers.", evidence: [{path: "docs/assessments/2026-09-15-resume-theme-guide.md", note: "RAC scoped guide tasks."}]}
			lifecycle: {status: "partial", reason: "Draft isolation, reset and cooperative callback cancellation are scoped; browser stop/retry and broader concurrent consumer lifecycle remain unproven.", next_step: "Investigate submission/loading after four clean no-submit controls and a failing same-candidate Stop reference; preserve unknown-target refusal. Verify the full cancellation matrix independently, then stale candidates and concurrent consumers.", evidence: [{path: "docs/assessments/2026-09-13-app-owned-theme-review.md", note: "Client-isolated preview and reset evidence."}, {path: "docs/assessments/2026-09-16-theme-guide-transport.md", note: "Request context and atomic failure-output regression; no arbitrary callback preemption."}, {path: "docs/assessments/2026-09-17-theme-guide-target-controls.md", note: "Escape fails; bounded cleanup passes. Four no-app controls are clean. The ordinary/Stop comparison rules out cancellation as necessary; one ordinary run fails at reset, another passes, and Stop runs fail before retry. Read-only unknown-target metadata is inconclusive. Four idle/edit controls pass while the same-candidate Stop reference fails; eleven cancellation cells and independent acceptance remain open."}]}
			security: {status: "partial", reason: "Local loopback and bounded input rules exist; app authentication and asset rights remain external.", next_step: "Review each consumer wrapper, source access and export boundary.", evidence: [{path: "docs/design/app-owned-styleguide.md", note: "Explicit local and app-owned boundaries."}]}
			data: {status: "partial", reason: "Semantic token diffs and selected contrast pairs are checked without whole-screen meaning certification.", next_step: "Verify semantic state meaning and contrast pairs for every selected real specimen.", evidence: [{path: "docs/assessments/2026-09-13-personal-themes.md", note: "Role mapping and contrast limitations."}]}
			performance: {status: "failed", reason: "Actual theme-guide memory acquisitions fail the fixed renderer RSS and retained-growth ceilings, with unknown-target, sampling and evidence failures retained. Earlier handler/timing/body passes do not complete this gate.", next_step: "Four fixed idle/edit no-submit controls pass while the same-candidate Stop reference fails; isolate submission/loading without inferring causation or safe target classification. Preserve target refusal; review memory metrics prospectively before full cache/cancellation/consumer acceptance.", evidence: [{path: "docs/assessments/2026-09-16-theme-guide-request-cost.md", note: "Warm-process handler costs."}, {path: "docs/assessments/2026-09-16-theme-guide-browser-cost.md", note: "Historical body/ZIP accounting; INC-52 timing qualification preserved."}, {path: "docs/assessments/2026-09-16-theme-guide-transport.md", note: "Corrected flattened application-target canary and exact browser phase evidence; historical failures remain explicit."}, {path: "docs/assessments/2026-09-17-gallery-css-revalidation.md", note: "Exact-byte validators save repeated ordinary Gallery CSS transfer, not opaque preview transfer. Candidate and failed control retained separately."}, {path: "docs/assessments/2026-09-17-theme-guide-memory.md", note: "Three failed memory acquisitions and corrected context attribution; small JS growth does not excuse RSS or evidence failures. No budget relaxation."}, {path: "docs/assessments/2026-09-17-theme-guide-target-controls.md", note: "Escape fails; bounded cleanup passes. Four no-app controls are clean. The ordinary/Stop comparison rules out cancellation as necessary; one ordinary run fails at reset, another passes, and Stop runs fail before retry. Read-only unknown-target metadata is inconclusive. Four idle/edit controls pass while the same-candidate Stop reference fails; eleven cancellation cells and independent acceptance remain open."}]}
			integration: {status: "partial", reason: "RAC mounted the experimental guide and Fathom has a source-equivalent verified-scoped working-tree integration; released propagation and full two-consumer acceptance remain open.", next_step: "Version and release an exact consumer integration and complete the remaining per-consumer gates before promotion.", evidence: [{path: "docs/assessments/2026-09-15-resume-theme-guide.md", note: "Scoped RAC guide integration."}, {path: "docs/assessments/2026-09-22-fathom-guide-only.md", note: "Exact f7ab mirror, active source equivalence and scoped Fathom browser checks; large charts, zoom and broader acceptance remain open."}]}
		}
		consumers: [{
			app: "fathom"
			revision: {kind: "artifact", sha256: "52666a587cc44dfc146064cd58d5003d34d79d319592d8d16d6394c1ff454198"}
			integration_status: "verified-scoped"
			evidence: [{path: "docs/assessments/2026-09-22-fathom-guide-only.md", note: "Fathom guide binary built from the exact f7ab mirror and current source-equivalent working tree; scoped CSS, build and browser tasks pass without release or full G4 promotion."}]
		}]
	},
	{
		id:                            "candidate.command-palette"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; originating consumer Fathom (FATHOM-KIT-01, vision 35 observatory console)"
		task:                          "Open the command palette from anywhere with Mod+K, type to fetch server-rendered grouped results, move with ArrowUp/ArrowDown, follow a result with Enter, close with Escape back to the invoker, and complete the same search on a plain GET page without JavaScript. Critical states: idle, debounced loading, grouped results, no results, partial results, error or timeout, stale results falling back to the search page, and the no-JavaScript page in light/dark and at narrow width."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["interaction.command-palette", "interaction.overlays"]
		source_refs: [
			{path: "appkit/ui/palette/palette.go", note: "Experimental L3b renderers, typed configuration and the shared results fragment."},
			{path: "appkit/ui/palette/palette.js", note: "L2 script: shortcut, debounce, stale-request abort, listbox upgrade, selection and focus return."},
			{path: "apps/gallery/palette.go", note: "Gallery specimen and no-JavaScript search page with reserved synthetic state queries."},
			{path: "docs/assessments/2026-09-25-command-palette.md", note: "Scoped Gallery browser evidence, gate decisions and limits; Fathom integration not yet performed."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed Palette/Results with defaults and atomic refusals exist; no reviewer or consumer has exercised the public options.", next_step: "Review the option set against Fathom's integration and keep refusals atomic.", evidence: [{path: "appkit/ui/palette/palette_test.go", note: "29 configuration and 12 results refusals before output."}]}
			"semantic-theme": {status: "partial", reason: "Role utilities only; light/dark inspected in the Gallery without measured contrast ratios.", next_step: "Measure selected-option, muted and notice contrast in both schemes and in consumer themes.", evidence: [{path: "docs/assessments/2026-09-25-command-palette.md", note: "Light 1440px and dark 390px observations."}]}
			"native-interaction": {status: "partial", reason: "Real link trigger, native dialog/form, combobox/listbox with aria-activedescendant and focus return pass scoped Chromium checks.", next_step: "Verify screen readers, other engines, touch/IME input and zoom.", evidence: [{path: "docs/assessments/2026-09-25-command-palette.md", note: "Keyboard contract rows A and D."}]}
			"honest-states": {status: "partial", reason: "Idle, loading, results, empty, partial (including zero results while a source was not searched), error and timeout states are distinct in markup and announcement and reproducible with synthetic queries.", next_step: "Confirm each state with Fathom's real sources, including its stale index and unavailable-source cases.", evidence: [{path: "docs/assessments/2026-09-25-command-palette.md", note: "State rows B and E."}]}
			"bounded-performance": {status: "partial", reason: "Debounce, per-request timeout, stale-request abort, 512 KiB response and 200-result bounds exist; no workload or budget was measured.", next_step: "Name Fathom's search workload and measure first results, update latency and memory.", evidence: [{path: "appkit/ui/palette/palette_test.mjs", note: "Deterministic debounce, abort and timeout checks with doubles."}]}
			"safe-composition": {status: "partial", reason: "All text is escaped, destinations and URLs are validated, fragments are same-origin and marker-checked; routes and authorization stay app-owned.", next_step: "Review Fathom's search route authorization and caching (Vary) in its integration.", evidence: [{path: "appkit/ui/palette/palette_test.go", note: "Escaping, unsafe destination and cross-origin refusals."}]}
			"reproducible-delivery": {status: "partial", reason: "Golden markup, deterministic rendering and pinned-Tailwind Gallery CSS; no versioned consumer build or migration exercised.", next_step: "Integrate from a versioned kit revision in Fathom with its own CSS scan and retain the build.", evidence: [{path: "appkit/ui/palette/testdata/dialog.html.golden", note: "Golden dialog markup."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Goldens and deterministic rendering pass on a working tree over dc77e66 only.", next_step: "Replay from a versioned kit revision in a clean consumer build.", evidence: [{path: "appkit/ui/palette/testdata/results.html.golden", note: "Golden results fragment."}]}
			structure: {status: "partial", reason: "Parsed-DOM tests and the browser role upgrade cover the Gallery specimen only.", next_step: "Check Fathom's composed pages: one h1, landmarks and unique ids with the palette mounted.", evidence: [{path: "appkit/ui/palette/palette_test.go", note: "Ids, label association, hooks and heading levels."}, {path: "docs/assessments/2026-09-25-command-palette.md", note: "Browser role, group and option checks."}]}
			visual: {status: "partial", reason: "Light and dark, wide and 390px views were inspected without a reviewed baseline.", next_step: "Review captures in Fathom's themes, including zoom and long labels.", evidence: [{path: "docs/assessments/2026-09-25-command-palette.md", note: "Scenario A and C observations."}]}
			interaction: {status: "partial", reason: "Shortcut, arrows, Enter, Escape, stale Enter, dismissal and modified click pass in one Chromium build.", next_step: "Repeat with assistive technology, other engines and touch/IME input.", evidence: [{path: "docs/assessments/fixtures/command-palette/results.json", note: "Run of record."}]}
			lifecycle: {status: "partial", reason: "Debounce, abort of superseded and closed requests, timeout, reopen, re-mount after a markup swap, late triggers and init(root) are exercised with doubles and in one Chromium run; simultaneous instances and navigation-heavy sessions are not.", next_step: "Exercise two palettes and navigation-heavy sessions with Fathom's swaps.", evidence: [{path: "appkit/ui/palette/palette_test.mjs", note: "Lifecycle doubles."}, {path: "docs/assessments/fixtures/command-palette/results.json", note: "Scenario B request log."}]}
			security: {status: "partial", reason: "Escaping, href and URL refusal and same-origin insertion of exactly one fragment answering the requested query (full pages, including redirects to pages carrying the palette, are refused) are tested; CSP and consumer authorization are not.", next_step: "Review Fathom's route, CSP and caching with negative requests.", evidence: [{path: "appkit/ui/palette/palette_test.go", note: "Hostile text and destinations."}, {path: "appkit/ui/palette/palette_test.mjs", note: "Cross-origin and non-fragment refusals."}]}
			data: {status: "partial", reason: "Fixed synthetic queries produce expected groups, counts and distinct empty/partial/error states.", next_step: "Check that Fathom's results match its verbs and never present absent data as zero results.", evidence: [{path: "apps/gallery/palette_test.go", note: "Fixture expectations for results, empty, partial and error."}]}
			performance: {status: "not-run", next_step: "Run only after Fathom names a search workload and budgets."}
			integration: {status: "not-run", next_step: "Integrate in Fathom, remove any local fork, rerun its task journeys and record the contribution disposition."}
		}
		consumers: []
	},
	{
		id:                            "candidate.fathom-layered-graph"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; originating consumer Fathom (observatory console, vision 35, FATHOM-KIT-02)"
		task:                          "Show one code symbol's callers and callees to a bounded depth, or a layered dependency flow, as a deterministic picture with a table alternative that states every reduction."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["analytics.graph"]
		source_refs: [
			{path: "appkit/ui/viz/graph.go", note: "Experimental L3a LayoutGraph/LayoutEgo with the class-free Table alternative."},
			{path: "apps/gallery/graph.go", note: "Gallery /graph specimens with synthetic data, linked from /components and the catalog preview."},
			{path: "docs/ui-components.md", note: "viz tier row and graph contract."},
			{path: "docs/assessments/2026-09-25-layered-graph.md", note: "Scoped package and Chromium evidence; Fathom has selected, not integrated, the renderer."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed nodes, edges, kinds, styles, rank modes and bounds refuse invalid input before output with a compiled example; no independent review or consumer integration yet.", next_step: "Integrate with Fathom's own symbol IDs and routes and review the option surface before any promotion.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "25 layout and 6 ego refusals (control characters included), defaults and bounds."}, {path: "appkit/ui/viz/graph_example_test.go", note: "Compiled public example."}]}
			"semantic-theme": {status: "partial", reason: "Only var(--ak-color-*) roles; with the default kit tokens, text pairs clear 4.5:1 and meaning-bearing marks (edges, the focus border and the legend-named dashed aggregate border, now ink-muted) clear 3:1 against both adjacent fills in light and dark; dashed/width cues back color. Normal node borders stay decorative (line-strong, about 1.8:1) because text identifies those boxes. App-owned themes, forced colors and print are unchecked.", next_step: "Measure the same role pairs under the consumer's theme and check forced-colors rendering.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "TestGraph_RolePairsMeetContrastFloors, TestGraph_AggregateCueIsMeasuredRole and TestGraph_DashedMeaningIsNotColorOnly."}, {path: "docs/assessments/2026-09-25-layered-graph.md", note: "Light/dark 1280 and 390 px captures reviewed."}]}
			"native-interaction": {status: "partial", reason: "Keyboard users reach nodes and destinations through native table links that name each node uniquely; SVG links are pointer-only and removed from the tab order; narrow layout scrolls inside a named region. Table links, SVG node links and disclosures measure at least 44px high in the Gallery; the table's link size is app CSS, so a consumer must style it the same way.", next_step: "Verify in Fathom's composed page, including zoom and a screen-reader pass.", evidence: [{path: "docs/assessments/2026-09-25-layered-graph.md", note: "Tab order, disclosure, link activation, 44px target sizes, AX tree and region scroll in four Chromium cells."}]}
			"honest-states": {status: "partial", reason: "Empty graph, empty columns, collapsed columns, omitted ego nodes, cycle-closing and merged edges are stated in the picture, description and table; edges into a collapsed node keep one line and count per style and emphasis rather than taking the OR of their members; freshness, partial or stale data remain app-owned.", next_step: "Map Fathom's absent, stale and partial index states onto the notes and empty wording.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "Bounded, per-style aggregate lane, hidden-edge, ego omission and empty-state tests."}]}
			"bounded-performance": {status: "not-started", next_step: "Name Fathom's neighbourhood workload and measure layout time, SVG bytes and memory against a budget before claiming performance."}
			"safe-composition": {status: "partial", reason: "All caller text is escaped and node links accept only relative or http(s) references; routes and authorization stay app-owned.", next_step: "Review Fathom's symbol routes and authorization when it integrates the renderer.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "Hostile text and href refusal tests."}]}
			"reproducible-delivery": {status: "partial", reason: "Same input yields byte-identical SVG and table goldens; no versioned consumer upgrade or replay exists yet.", next_step: "Adopt through a reviewed kit pin in Fathom and replay its fixture.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "Golden and determinism tests."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Golden SVG/table bytes (including self-loop and same-column arc geometry) and a 25-run determinism check pass; no consumer build closure.", next_step: "Replay from a pinned kit revision inside Fathom.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "Goldens under appkit/ui/viz/testdata/graph_*."}]}
			structure: {status: "partial", reason: "role=img with title/desc over an aria-hidden drawing that parses as standalone XML, table captions and header scopes with each node named uniquely (label, then group, then ID), zero class attributes; checked in package tests and the Chromium AX tree.", next_step: "Repeat inside the consumer page structure.", evidence: [{path: "appkit/ui/viz/tier_test.go", note: "L3a no-class/no-literal-color specimens include LayoutGraph and LayoutEgo."}, {path: "docs/assessments/2026-09-25-layered-graph.md", note: "AX tree: five images with names and descriptions, no exposed children or links."}]}
			visual: {status: "partial", reason: "Twenty specimen captures in Chromium, light/dark, 1280 and 390 px, reviewed after two fixes; nine re-captures of the neighbourhood, bounded and given-columns specimens reviewed after the 2026-09-25 review fixes (44px linked boxes, per-style aggregate lines, measured aggregate border).", next_step: "Review under Fathom's theme and fonts, where width estimates may crowd labels.", evidence: [{path: "docs/assessments/2026-09-25-layered-graph.md", note: "Scoped visual review; captures retained outside the repository."}]}
			interaction: {status: "partial", reason: "Keyboard tab order skips SVG links, Space opens the table, Enter follows a table link, pointer follows an SVG node link, ArrowRight scrolls the narrow region; every graph table link, SVG node link and disclosure summary measures at least 44px high.", next_step: "Run the consumer's task journey, zoom and a screen reader.", evidence: [{path: "docs/assessments/fixtures/layered-graph/browser-check.js", note: "Executed harness bytes."}, {path: "docs/assessments/2026-09-25-layered-graph.md", note: "Results for four viewport/theme cells."}]}
			lifecycle: {status: "not-run", next_step: "Decide with Fathom whether live freshness re-renders the graph and test repeated renders if it does."}
			security: {status: "partial", reason: "Hostile labels, IDs, groups and options are escaped; javascript: and protocol-relative links are refused before output.", next_step: "Review consumer destinations and authorization.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "TestGraph_EscapesEveryCallerString and TestGraph_InvalidInputFailsBeforeOutput."}]}
			data: {status: "partial", reason: "SVG and table render one layout; fixture ranks, sides, collapse counts and notes match expected values.", next_step: "Check Fathom's real neighbourhood counts against its verbs.", evidence: [{path: "appkit/ui/viz/graph_test.go", note: "Rank, ego and bounding expectations."}, {path: "apps/gallery/graph_test.go", note: "Served specimens carry the same notes and tables."}]}
			performance: {status: "not-run", next_step: "Measure only after Fathom names a workload and budget."}
			integration: {status: "not-run", next_step: "Integrate in Fathom's code neighbourhood and blast-radius views, then record the contribution and its verification."}
		}
		consumers: [{
			app: "fathom"
			revision: {kind: "selection", sha256: "b9630e3d2b6ec4667362cfd23e8911025093d2af3af923fce66f2b351e676272"}
			integration_status: "selected"
			evidence: [{path: "docs/assessments/2026-09-25-layered-graph.md", note: "Revision is Fathom docs/design/observatory-console.md naming FATHOM-KIT-02 at f8b40609; selection only, no integration or adoption."}]
		}]
	},
	{
		id:                            "candidate.fathom-heatmap-truth-states"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; requested for Fathom's coverage matrix (FATHOM-KIT-03), whose selection is not yet in a published Fathom revision"
		task:                          "Read a project by layer coverage matrix in which a measured value, a measured zero and missing (not compiled), unknown and error cells stay distinct, each with its reason, and inspect the same cells as a table. Critical states: value, measured zero, missing, unknown, error, all-missing grid, empty grid, hostile text, and unchanged output for legacy callers."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["viz.heatmap"]
		source_refs: [
			{path: "appkit/ui/viz/heatmap.go", note: "First-party L3a renderer; truth states are opt-in through HeatmapOptions.States or TruthStates, and the original path is unchanged."},
			{path: "apps/gallery/heatmap.go", note: "Gallery /components/heatmap specimen: one synthetic fixture drives the chart, its table alternative, an all-missing grid and the legacy comparison."},
			{path: "docs/assessments/2026-09-25-heatmap-truth-states.md", note: "Scoped package and Chromium evidence, the unrecorded Fathom selection and retained limits."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed CellState, HeatmapCell and an opt-in Validate exist, but the renderer stays total instead of refusing and no independent review has accepted the options surface.", next_step: "Review the options with Fathom's integration and decide whether opt-in Validate is sufficient admission.", evidence: [{path: "appkit/ui/viz/heatmap_truth_test.go", note: "Validate refusals, unrecognized states, zero contradictions and non-finite values."}]}
			"semantic-theme": {status: "partial", reason: "Marks use ink-muted, danger and ink roles that clear their floors in both default schemes, but the value ramp's low end stays below 3:1 and forced-colors and print were not checked.", next_step: "Review the selected consumer theme, forced-colors and print output; decide whether a ramp floor option is needed.", evidence: [{path: "appkit/ui/viz/heatmap_truth_test.go", note: "Role contrast measured on surface and surface-raised, light and dark."}, {path: "docs/assessments/2026-09-25-heatmap-truth-states.md", note: "Light and dark captures at 1440 and 390px inspected."}]}
			"native-interaction": {status: "partial", reason: "The SVG is a non-interactive image; keyboard and assistive access depend on the app's table alternative, which is only demonstrated in the Gallery.", next_step: "Verify the table alternative and focus behavior in Fathom's composed page.", evidence: [{path: "docs/assessments/2026-09-25-heatmap-truth-states.md", note: "Tab order, focus-visible scroll regions and arrow-key scrolling at 390px dark."}]}
			"honest-states": {status: "partial", reason: "All five states, all-missing and empty grids are implemented and golden-tested in the kit; real consumer reasons and review are pending.", next_step: "Exercise Fathom's actual not-compiled, stale and failed reasons and have the result reviewed.", evidence: [{path: "appkit/ui/viz/heatmap_truth_test.go", note: "Mixed, all-missing, hostile and large-cell goldens; absent cells never fill or widen the range."}, {path: "apps/gallery/heatmap_test.go", note: "Chart titles and table cells agree cell by cell."}]}
			"bounded-performance": {status: "not-started", next_step: "Name Fathom's largest coverage matrix and measure render bytes and time against its 250 KB page budget."}
			"safe-composition": {status: "partial", reason: "Labels, reasons and unrecognized states are escaped and the output parses as XML; no independent security review was performed.", next_step: "Review the consumer's reason sources and confirm no trusted markup reaches the renderer.", evidence: [{path: "appkit/ui/viz/heatmap_truth_test.go", note: "Hostile-text golden and escaping assertions."}]}
			"reproducible-delivery": {status: "partial", reason: "Output is deterministic and legacy goldens were frozen from the unmodified renderer, but no versioned consumer delivery or replay exists.", next_step: "Deliver through Fathom's reviewed kit mirror and replay its build without a sibling checkout.", evidence: [{path: "appkit/ui/viz/testdata/heatmap_legacy.svg.golden", note: "Pre-change legacy output, still byte-identical."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Deterministic goldens only; no clean versioned consumer build.", next_step: "Replay from a versioned candidate in Fathom's build.", evidence: [{path: "appkit/ui/viz/testdata/heatmap_legacy.svg.golden", note: "Legacy bytes frozen before the change."}, {path: "appkit/ui/viz/heatmap_truth_test.go", note: "Determinism and truth-state goldens."}]}
			structure: {status: "partial", reason: "role=img, title, desc, per-cell titles and well-formed XML are asserted; the Gallery table has scoped headers; no accessibility-tree review.", next_step: "Review the composed page structure in Fathom.", evidence: [{path: "appkit/ui/viz/heatmap_truth_test.go", note: "Structure and XML well-formedness."}, {path: "apps/gallery/heatmap_test.go", note: "Table header scopes and caption."}]}
			visual: {status: "partial", reason: "Light and dark captures at 1440 and 390px were inspected in one Chromium lane; captures are not retained in the repository.", next_step: "Review Fathom's coverage matrix in both themes with its real labels.", evidence: [{path: "docs/assessments/2026-09-25-heatmap-truth-states.md", note: "Viewport, theme, fixture and observed results."}]}
			interaction: {status: "partial", reason: "Keyboard scroll regions and pointer hit-testing pass in the Gallery; the chart itself has no interaction.", next_step: "Repeat keyboard and pointer checks in Fathom's page.", evidence: [{path: "docs/assessments/2026-09-25-heatmap-truth-states.md", note: "Tab order, arrow scrolling and hit-testing."}]}
			lifecycle: {status: "not-run", next_step: "Check repeated renders and page updates once Fathom's live-freshness flow re-renders the matrix."}
			security: {status: "partial", reason: "Escaping of hostile caller text is tested; no independent review.", next_step: "Review reason and label sources in the consumer.", evidence: [{path: "appkit/ui/viz/heatmap_truth_test.go", note: "Hostile-text golden."}]}
			data: {status: "partial", reason: "State resolution, range exclusion and chart/table agreement pass on synthetic fixtures; no consumer data.", next_step: "Adapt the agreement check to Fathom's coverage data.", evidence: [{path: "apps/gallery/heatmap_test.go", note: "Cell-by-cell chart and table agreement."}, {path: "appkit/ui/viz/heatmap_truth_test.go", note: "Declared absence ignores numeric entries and the range."}]}
			performance: {status: "not-run", next_step: "Measure render bytes and time for Fathom's largest matrix."}
			integration: {status: "not-run", next_step: "Once Fathom publishes its selection, pin that revision here; then integrate in Fathom's coverage matrix under authority, remove any local workaround and retain the build and browser evidence."}
		}
		// No consumer is pinned: the FATHOM-KIT-03 selection exists only in an
		// unpublished Fathom draft, so its revision cannot be checked here.
		consumers: []
	},
	{
		id:                            "candidate.code-excerpt"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; originating consumer Fathom (FATHOM-KIT-04, observatory console refactor page)"
		task:                          "Read a bounded source excerpt with numbered lines whose changed, not-executed and highlighted states are labelled in text; follow a line permalink; scroll long lines by keyboard without wrapping, in light and dark and at narrow width; see an explicit gap, empty state or refusal instead of silent output."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["content.code-excerpt"]
		source_refs: [
			{path: "appkit/ui/code/excerpt.go", note: "Experimental first-party L3b renderer; no licensed source adapted."},
			{path: "apps/gallery/code_review.go", note: "Gallery /agentic/code specimen with synthetic source, every state, gap, reveal, empty and refused input."},
			{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Scoped unit, contrast and Chrome keyboard/narrow/theme evidence at 94e6815 and review repairs at 5bf90cd; not consumer integration."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed Excerpt, Line, LineState and StateLabels with atomic refusal of blank labels, non-increasing numbers, unknown states, unsafe links and oversized input; no independent review.", next_step: "Review the public fields with Fathom's integration before any promotion.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "Refusal, default, override and boundary cases."}]}
			"semantic-theme": {status: "partial", reason: "Role utilities only; ink, ink-muted, state rules and focus frame measured in both schemes of the default palette.", next_step: "Measure the other first-party palettes and forced-colors rendering.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "TestExcerptRoleContrast and TestExcerptUsesRoleUtilitiesOnly."}, {path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Light caution rule failure that moved not-executed to danger."}]}
			"native-interaction": {status: "partial", reason: "Native links, named tabindex region and :target frame pass scoped Chrome keyboard, pointer and narrow checks. Line-number permalinks measure 48x24px at a 24px pitch (WCAG 2.2 2.5.8); they were 48x20px at 20px before 5bf90cd, when only the caption link and summary were measured. They stay below the kit's 44px standalone floor, a recorded catalog divergence.", next_step: "Screen-reader, zoom and other-engine checks; decide whether long excerpts need fewer tab stops.", evidence: [{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Browser matrix T1-T3, T5-T7 and the review repairs."}, {path: "appkit/ui/code/excerpt_test.go", note: "TestExcerptLineRowsMeetMinimumTargetSize."}]}
			"honest-states": {status: "partial", reason: "Stated gaps, empty excerpt, refusals and revealed hidden characters; the app owns state meaning.", next_step: "Confirm Fathom's not-run and partial reading-set states map onto these without inventing coverage.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "Gap, empty and refusal tests."}]}
			"bounded-performance": {status: "not-started", next_step: "Name Fathom's refactor-page workload, then measure payload and render time against a budget; MaxLines only bounds input."}
			"safe-composition": {status: "partial", reason: "Escapes all text and refuses non-web links. Reveals, by Unicode class, controls, every format character (bidi, zero-width, tag), U+2028/U+2029, default-ignorable fillers, variation selectors and invalid bytes in lines, caption and link text; caption and link text are bdi-isolated. Before 5bf90cd a hand-picked list missed 153 format characters and the separators, and the caption was not revealed. Classification follows Go's Unicode 15.0 tables; no trusted slots.", next_step: "Review caller-supplied anchor prefix uniqueness and link authorization in Fathom.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "Escaping, line-separator, tag-character and caption reveal tests."}, {path: "appkit/ui/internal/linetext/linetext_test.go", note: "Class-by-class hidden-character enumeration."}]}
			"reproducible-delivery": {status: "partial", reason: "Deterministic golden output; no consumer upgrade or standalone consumer build yet.", next_step: "Deliver through Fathom's kit mirror and rebuild its scanned CSS.", evidence: [{path: "appkit/ui/code/testdata/excerpt_review.html.golden", note: "Reviewed golden HTML."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Golden HTML and repeated-render determinism in the kit only.", next_step: "Replay in the consumer's clean build.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "TestRenderExcerptGolden."}]}
			structure: {status: "partial", reason: "Package and Gallery structure tests; no independent review.", next_step: "Independent structure review with the consumer page.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "Anchors, labels, legend and list structure."}, {path: "apps/gallery/code_review_test.go", note: "Specimen builder and route."}]}
			visual: {status: "partial", reason: "1440 and 390 captures in light and dark were inspected but not retained.", next_step: "Retain reviewed captures for the other palettes and the consumer page.", evidence: [{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Visual inspection notes and targeted-frame repair."}]}
			interaction: {status: "partial", reason: "Scoped Chrome keyboard, pointer and narrow tasks pass on synthetic data. Every line-number permalink is measured since 5bf90cd (48x24px at a 24px pitch); the earlier pass measured only the caption link and summary.", next_step: "Screen reader, zoom and a second engine.", evidence: [{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Browser matrix results."}, {path: "docs/assessments/fixtures/code-review/browser-matrix.js", note: "Executed harness."}]}
			lifecycle: {status: "not-run", next_step: "Check repeated rendering inside Fathom's live-refresh page; no script is involved."}
			security: {status: "partial", reason: "Escaping, link refusal and class-based hidden-character reveal in lines, caption and link text, unit-tested and observed in Chrome.", next_step: "Review in the consumer's CSP and authorization context.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "TestExcerptEscapesAndRevealsHiddenCharacters, TestExcerptRevealsLineTerminatorsAndTagCharacters and TestExcerptRevealsCaptionLabelAndLinkText."}]}
			data: {status: "partial", reason: "Numbers, states and legend counts equal the input in fixtures; the app owns what a state means.", next_step: "Verify against Fathom's reading-set data.", evidence: [{path: "appkit/ui/code/excerpt_test.go", note: "Legend counts and line data."}]}
			performance: {status: "not-run", next_step: "Run only after the consumer supplies a workload and budget."}
			integration: {status: "not-run", next_step: "Integrate in Fathom's refactor page, rebuild its scanned CSS, rerun its task and record the contribution in its docs/app-kit.md."}
		}
		consumers: []
	},
	{
		id:                            "candidate.unified-diff"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; originating consumer Fathom (FATHOM-KIT-05, observatory console review surfaces)"
		task:                          "Review a proposed text change as unified hunks with old and new line numbers, text +/- markers and spoken line labels; expand and collapse unchanged context by keyboard; see stated omitted ranges, no-change, no-final-newline and diff-budget states in light and dark and at narrow width."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["work.unified-diff"]
		source_refs: [
			{path: "appkit/ui/hitl/unified.go", note: "Experimental first-party L3b renderer beside hitl.DiffView."},
			{path: "appkit/ui/hitl/linediff.go", note: "Deterministic Myers line diff and hunk grouping."},
			{path: "apps/gallery/code_review.go", note: "Gallery /agentic/code computed, precomputed and unchanged specimens."},
			{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Scoped unit, contrast and Chrome keyboard/narrow/theme evidence at 94e6815 and review repairs at 5bf90cd; not consumer integration."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed UnifiedDiff, Hunk and DiffLine; texts or hunks, never both; Replaced only with hunks; overlapping hunks, mismatched numbers and a no-newline mark before a side's last line refused atomically; caller hunks never mutated.", next_step: "Review the public fields with Fathom's integration before any promotion.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "Refusal and precomputed-hunk tests."}]}
			"semantic-theme": {status: "partial", reason: "Role utilities only; text, rules and focus measured in both schemes of the default palette.", next_step: "Measure the other first-party palettes and forced-colors rendering.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "TestUnifiedDiffRoleContrast."}]}
			"native-interaction": {status: "partial", reason: "Named tabindex region and native details pass scoped Chrome keyboard and narrow checks.", next_step: "Screen-reader, zoom and other-engine checks.", evidence: [{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Browser matrix T4-T7."}]}
			"honest-states": {status: "partial", reason: "No-change, no-final-newline and budget-fallback states are stated in text, the budget note also for precomputed hunks through UnifiedDiff.Replaced. Ranges skipped between precomputed hunks are stated per side and never called unchanged; before 5bf90cd they read 'Unchanged <old> X-Y' from old-side numbers only. Added/removed counts cover the supplied hunks only.", next_step: "Confirm Fathom's review data never implies an applied change.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "TestUnifiedDiffHonestEmptyAndBudgetStates, TestUnifiedDiffOmittedRangesNeverClaimUnchanged and TestUnifiedDiffPrecomputedNoNewlineAndBudget."}]}
			"bounded-performance": {status: "not-started", next_step: "Name a consumer workload and measure HTML size and render time; MaxDiffLines and the 1000-edit budget only bound input and search."}
			"safe-composition": {status: "partial", reason: "Escapes all text and names. Reveals, by Unicode class, controls, format and tag characters, U+2028/U+2029, default-ignorable fillers, variation selectors and invalid bytes in lines, label and side names, which are bdi-isolated. Before 5bf90cd a hand-picked list missed 153 format characters and the separators, and names were not revealed. Classification follows Go's Unicode 15.0 tables; no trusted slots.", next_step: "Review in the consumer's CSP and authorization context.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "TestUnifiedDiffEscapesAndRevealsHiddenCharacters."}]}
			"reproducible-delivery": {status: "partial", reason: "Deterministic diff and golden HTML; no consumer upgrade yet.", next_step: "Deliver through Fathom's kit mirror and rebuild its scanned CSS.", evidence: [{path: "appkit/ui/hitl/testdata/unified_config.html.golden", note: "Reviewed golden HTML."}]}
		}
		gates: {
			reproducibility: {status: "partial", reason: "Golden HTML and seeded determinism checks in the kit only.", next_step: "Replay in the consumer's clean build.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "TestRenderUnifiedDiffGolden."}, {path: "appkit/ui/hitl/linediff_test.go", note: "Seeded determinism."}]}
			structure: {status: "partial", reason: "Table-free list structure, labels and hunk headers tested; no independent review.", next_step: "Independent structure review with the consumer page.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "Markers, numbers and labels."}, {path: "apps/gallery/code_review_test.go", note: "Specimen builder and route."}]}
			visual: {status: "partial", reason: "1440 and 390 captures in light and dark were inspected but not retained.", next_step: "Retain reviewed captures for the other palettes and the consumer page.", evidence: [{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Visual inspection notes."}]}
			interaction: {status: "partial", reason: "Scoped Chrome keyboard disclosure and narrow tasks pass on synthetic data.", next_step: "Screen reader, zoom and a second engine.", evidence: [{path: "docs/assessments/2026-09-25-code-review-renderers.md", note: "Browser matrix results."}, {path: "docs/assessments/fixtures/code-review/browser-matrix.js", note: "Executed harness."}]}
			lifecycle: {status: "not-run", next_step: "Check repeated rendering and disclosure state inside the consumer page; no script is involved."}
			security: {status: "partial", reason: "Escaping and class-based hidden-character reveal in lines, label and side names, unit-tested and observed in Chrome.", next_step: "Review in the consumer's CSP and authorization context.", evidence: [{path: "appkit/ui/hitl/unified_test.go", note: "Escaping, line-separator, tag-character and name reveal tests."}]}
			data: {status: "partial", reason: "Scripts rebuild both texts and match an independent LCS on 400 seeded cases; hunk headers match git diff; every DiffText hunk set and a subset of it pass the precomputed validation on the same cases.", next_step: "Verify against Fathom's review data.", evidence: [{path: "appkit/ui/hitl/linediff_test.go", note: "TestDiffTextIsMinimalAndValid and TestHunksMatchUnifiedConventions."}]}
			performance: {status: "not-run", next_step: "Run only after the consumer supplies a workload and budget."}
			integration: {status: "not-run", next_step: "Integrate in Fathom's review surfaces, rebuild its scanned CSS, rerun its task and record the contribution in its docs/app-kit.md."}
		}
		consumers: []
	},
	{
		id:                            "candidate.fathom-live-freshness"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; originating consumer Fathom (FATHOM-KIT-06, vision 35 observatory console)"
		task:                          "Follow an app-owned SSE stream and mark server-rendered panels updated with one polite announcement and a refresh link, never re-rendering or moving content. Critical states: no script or off, connecting, live, updated, reconnecting with bounded backoff, paused while hidden, stopped and resumed, unavailable, replay from the render-time cursor and the gap wildcard."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["interaction.live-freshness"]
		source_refs: [
			{path: "appkit/ui/live/live.go", note: "Experimental L3b helpers and the documented global, hook, keyboard and connection contract."},
			{path: "appkit/ui/live/live.js", note: "Dependency-free L2 subscriber; the one global is AppKitLive."},
			{path: "apps/gallery/live_freshness.go", note: "Synthetic Gallery SSE fixture with bounded replay, subscribers and fault controls; not a reusable event bus."},
			{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Implemented and queued for Fathom integration and acceptance; scoped kit-side browser evidence only."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Region and Page validate IDs, source keys, labels, as-of text, URLs (including the port range a browser accepts), event names and an unpadded cursor of at most 256 bytes before output; the script refuses invalid options before connecting. Consumer configuration review is open.", next_step: "Review Fathom's source keys, event names and cursor against these refusals when it integrates.", evidence: [{path: "appkit/ui/live/live_test.go", note: "Atomic refusal table, escaping and golden output."}, {path: "appkit/ui/live/live_test.mjs", note: "Script option refusals open no stream."}]}
			"semantic-theme": {status: "partial", reason: "Role utilities only; light and dark text contrast measured in one browser; forced colors, zoom and other identities are unmeasured.", next_step: "Check forced colors, actual zoom and a consumer identity.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Canvas-converted light/dark contrast; an earlier invalid oklch instrument is retained as not evidence."}]}
			"native-interaction": {status: "partial", reason: "Native button and link; one stop/resume control keeps focus and is offered only when it can act; refresh reloads and focuses the region without scrolling and the region paints its focus-visible ring (a review found focus:outline-none had cancelled it; now checked by computed style); keyboard paths pass in one Chromium run.", next_step: "Verify screen-reader speech, other engines and a real tab switch.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Keyboard publish, refresh, stop and resume checks, including the painted region focus ring."}, {path: "appkit/ui/live/live_test.mjs", note: "Focus return, modified clicks and one-shot restore."}]}
			"honest-states": {status: "partial", reason: "Off, connecting, live, updated, reconnecting, paused, stopped, unavailable and unsupported states are declared, tested and shown as off without JavaScript; after a refused configuration or in an unsupported browser no resume control is offered and no resume is announced.", next_step: "Map Fathom's replay window and gap behavior onto the wildcard in its own stream.", evidence: [{path: "appkit/ui/live/live_test.mjs", note: "Twenty-seven state-machine tests with a fake stream and clock, two against a real local server."}, {path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Browser states including the no-JavaScript context."}]}
			"bounded-performance": {status: "partial", reason: "Attempts, delays, event size, sources per event, tracked regions, fixture subscribers and replay window are bounded and tested; the attempt budget resets only after 10 s open or an event that advances the cursor, so a server that accepts and closes is retried a bounded number of times, and the first retry is jittered. No payload, memory or latency budget was measured.", next_step: "Name Fathom's workload and measure script cost, reconnect churn and long-running memory.", evidence: [{path: "appkit/ui/live/live_test.mjs", note: "Bounded jittered backoff, give-up against an accept-then-close server (fake and real) and oversized-event drop."}]}
			"safe-composition": {status: "partial", reason: "Event data is never inserted; labels and as-of text are escaped; the script uses same-origin credentials; fixture controls accept loopback same-origin requests only. CSP-restricted hosts are untested.", next_step: "Test under Fathom's CSP and review its stream authorization.", evidence: [{path: "appkit/ui/live/live_test.go", note: "Hostile text and URL escaping; dependency-free single-global check."}, {path: "apps/gallery/live_freshness_test.go", note: "Cross-site, foreign-origin, remote-peer and malformed request refusals."}]}
			"reproducible-delivery": {status: "partial", reason: "Deterministic golden markup and an embedded script with a revalidating Handler; no versioned consumer delivery or vendoring.", next_step: "Deliver through a versioned kit revision to Fathom and verify its CSS scan.", evidence: [{path: "appkit/ui/live/live_test.go", note: "Golden region and page status; Handler GET/HEAD/ETag."}]}
		}
		gates: {
			reproducibility: {status: "not-run", next_step: "Rebuild Fathom from a versioned kit revision and compare artifacts."}
			structure: {status: "partial", reason: "Markup semantics, reserved-space layers and one-h1 composition are checked in Go and one browser.", next_step: "Repeat inside Fathom's composed panels.", evidence: [{path: "appkit/ui/live/live_test.go", note: "Structural and golden checks."}, {path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Browser structure checks."}]}
			visual: {status: "partial", reason: "Wide light and narrow dark captures were reviewed and contrast computed. Zero box movement is measured for marking at 390 and 1280 px and, after a review found the connection line grew at 320-340 px, for marking, stop, resume and reconnect at 320, 340 and 360 px; the script-free line already has the running height.", next_step: "Review zoom, forced colors and Fathom's panels.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Scoped visual and movement evidence."}]}
			interaction: {status: "partial", reason: "Keyboard publish, refresh, stop, resume and focus return with a painted focus ring pass in one Chromium run.", next_step: "Verify assistive technology and other engines.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "45 browser checks on the corrected build."}, {path: "appkit/ui/live/live_test.mjs", note: "Event contract and click handling."}]}
			lifecycle: {status: "partial", reason: "Dropped streams, refused connections, bounded give-up, stop/resume and synthetic hidden pause replay correctly; real tab switches and network loss are untested.", next_step: "Exercise real visibility changes, network loss and long sessions.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Lifecycle checks against the fixture."}, {path: "apps/gallery/live_freshness_test.go", note: "Fixture replay, gap, drop, outage and capacity."}]}
			security: {status: "partial", reason: "No event data reaches the DOM and hostile text is escaped; fixture mutations need loopback same-origin requests. No CSP or authorization review of a consumer stream.", next_step: "Review Fathom's /events authorization, origin and CSP.", evidence: [{path: "apps/gallery/live_freshness_test.go", note: "Refusal table."}, {path: "appkit/ui/live/live_test.mjs", note: "Hostile event data stays out of markup."}]}
			data: {status: "partial", reason: "Only regions following a named source are marked; replay uses the render-time cursor, also on an explicit start, and ids of any script round-trip as UTF-8; the as-of text stays visible when updated. Real event semantics are app-owned.", next_step: "Verify Fathom's event-to-source mapping against its pages.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Cursor, replay and wildcard results; the lost-event defect and its fix are retained."}]}
			performance: {status: "not-run", next_step: "Measure script cost, reconnect churn and memory for a named Fathom workload."}
			integration: {status: "not-run", next_step: "Integrate in Fathom's observatory console, remove any local fork and verify its task."}
		}
		consumers: [{
			app: "fathom"
			revision: {kind: "source", sha256: "b9630e3d2b6ec4667362cfd23e8911025093d2af3af923fce66f2b351e676272"}
			integration_status: "selected"
			evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Revision is Fathom's docs/design/observatory-console.md at f8b40609, which selects FATHOM-KIT-06; nothing is integrated."}]
		}]
	},
	{
		id:                            "candidate.fathom-page-heading"
		kind:                          "candidate"
		owner:                         "app-kit maintainer; originating consumer Fathom (FATHOM-KIT-07, vision 35 observatory console)"
		task:                          "Render a page's one h1 with an optional breadcrumb trail, eyebrow, meta line and up to four actions that wrap below the title at narrow width. Critical states: title only, full, long unbroken content, narrow actions below, unsafe action inert, invalid input refused, light and dark."
		profile:                       "go-web"
		priority:                      "high"
		core:                          false
		state:                         "queued"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["interaction.page-heading"]
		source_refs: [
			{path: "appkit/ui/chrome/heading.go", note: "Experimental L3b PageHeading and RenderPageHeading composing Breadcrumbs and form buttons unchanged."},
			{path: "apps/gallery/page_heading.go", note: "Gallery specimen with full, title-only and long-content variants."},
			{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Implemented and queued for Fathom integration and acceptance; scoped kit-side browser evidence only."},
		]
		standards: {
			"typed-config": {status: "partial", reason: "Typed fields with bounded meta and actions; blank or control-character text and invalid IDs fail before output.", next_step: "Review Fathom's page titles, meta and actions against the bounds.", evidence: [{path: "appkit/ui/chrome/heading_test.go", note: "Refusal table and goldens."}]}
			"semantic-theme": {status: "partial", reason: "Role utilities only; eyebrow and meta contrast measured in light and dark in one browser.", next_step: "Check forced colors, zoom and Fathom's identity.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Canvas-converted contrast."}]}
			"native-interaction": {status: "partial", reason: "Native links and buttons, reading order equals DOM order, 44px action targets and visible focus; breadcrumb links keep the existing inline targets.", next_step: "Decide breadcrumb target size in chrome and verify assistive technology.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Keyboard order and target measurements."}]}
			"honest-states": {status: "partial", reason: "Unsafe destinations render the shared inert label and invalid headings refuse to render; loading and error states belong to the page.", next_step: "Confirm Fathom's error pages use the same heading contract.", evidence: [{path: "appkit/ui/chrome/heading_test.go", note: "Inert action and refusal checks."}]}
			"bounded-performance": {status: "not-started", next_step: "Measure the heading within Fathom's 250 KB page budget."}
			"safe-composition": {status: "partial", reason: "All text is escaped; actions reuse form.RenderButton unchanged (unsafe destinations inert); Breadcrumbs writes every href as a link, so the heading refuses breadcrumb destinations that are not relative or HTTP(S) before output.", next_step: "Review Fathom's action and breadcrumb destinations and authorization.", evidence: [{path: "appkit/ui/chrome/heading_test.go", note: "Hostile text, javascript: action and javascript:, data:, protocol-relative and padded breadcrumb destinations."}]}
			"reproducible-delivery": {status: "partial", reason: "Deterministic golden output; no versioned consumer delivery.", next_step: "Deliver through a versioned kit revision and verify Fathom's CSS scan of chrome and form.", evidence: [{path: "appkit/ui/chrome/heading_test.go", note: "Full and minimal goldens."}]}
		}
		gates: {
			reproducibility: {status: "not-run", next_step: "Rebuild Fathom from a versioned kit revision."}
			structure: {status: "partial", reason: "One h1, order and composition are checked in Go and in one browser across three variants.", next_step: "Repeat in Fathom's pages.", evidence: [{path: "appkit/ui/chrome/heading_test.go", note: "Structure checks."}, {path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Six variant/viewport cells."}]}
			visual: {status: "partial", reason: "Wide light and narrow dark captures reviewed; no overflow; actions beside or below as designed.", next_step: "Review zoom and Fathom's layout.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Scoped layout evidence."}]}
			interaction: {status: "partial", reason: "Keyboard order and focus indication pass in one Chromium run.", next_step: "Verify assistive technology and other engines.", evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Keyboard checks."}]}
			lifecycle: {status: "not-run", next_step: "Confirm the static heading has no lifecycle surface in Fathom or record a reviewed N/A."}
			security: {status: "partial", reason: "Escaping, inert unsafe actions and refused unsafe breadcrumb destinations are tested; consumer destinations are app-owned.", next_step: "Review Fathom's action and breadcrumb destinations.", evidence: [{path: "appkit/ui/chrome/heading_test.go", note: "Hostile input and unsafe destinations."}]}
			data: {status: "not-run", next_step: "Verify Fathom's meta and freshness text against its data."}
			performance: {status: "not-run", next_step: "Measure within a named Fathom page workload."}
			integration: {status: "not-run", next_step: "Integrate in Fathom's console shell and verify its pages."}
		}
		consumers: [{
			app: "fathom"
			revision: {kind: "source", sha256: "b9630e3d2b6ec4667362cfd23e8911025093d2af3af923fce66f2b351e676272"}
			integration_status: "selected"
			evidence: [{path: "docs/assessments/2026-09-25-live-freshness-and-page-heading.md", note: "Revision is Fathom's docs/design/observatory-console.md at f8b40609, which selects FATHOM-KIT-07; nothing is integrated."}]
		}]
	},
	{
		id:                            "candidate.personal-site-series-navigation"
		kind:                          "candidate"
		owner:                         "app-kit maintainer"
		task:                          "Assess and, only when selected, adapt a linked series journey with current, previous and next destinations from the personal site."
		profile:                       "go-web"
		priority:                      "on-demand"
		core:                          false
		state:                         "deferred"
		enhancement_standard_revision: "converted-core-v1"
		capability_ids: ["interaction.chrome"]
		source_refs: [
			{path: "docs/design/theme-workbench.md", note: "Personal-site patterns remain assessed candidates, not promoted components."},
			{path: "docs/adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md", note: "Personal-site extraction remains separately admitted and bounded."},
		]
		standards: {
			"typed-config": {status: "not-started", next_step: "Freeze a selected source revision and define typed steps, current key and destinations."}
			"semantic-theme": {status: "not-started", next_step: "Map the candidate only to semantic roles and verify both retained personal identities."}
			"native-interaction": {status: "not-started", next_step: "Specify native links, aria-current, long labels, narrow layout and focus behavior."}
			"honest-states": {status: "not-started", next_step: "Define first, middle, last, empty and unavailable-destination states."}
			"bounded-performance": {status: "not-started", next_step: "Name an actual consumer workload before measuring or optimizing."}
			"safe-composition": {status: "not-started", next_step: "Resolve source rights and keep routes, icons and content consumer-owned."}
			"reproducible-delivery": {status: "not-started", next_step: "Select a target and frozen source before creating any adapter or artifact."}
		}
		gates: {
			reproducibility: {status: "not-run", next_step: "Freeze exact source, target and generated artifact revisions."}
			structure: {status: "not-run", next_step: "Define and test ordered steps, current state and destination semantics."}
			visual: {status: "not-run", next_step: "Review narrow and wide output under both selected personal themes."}
			interaction: {status: "not-run", next_step: "Test keyboard, pointer, focus and long-label navigation."}
			lifecycle: {status: "not-run", next_step: "Confirm navigation history and repeated rendering behavior for the selected profile."}
			security: {status: "not-run", next_step: "Review destinations, escaping, source rights and dependency closure."}
			data: {status: "not-run", next_step: "Verify truthful current and ordered-step inputs without app-domain inference."}
			performance: {status: "not-run", next_step: "Run only after a named task supplies a workload and budget."}
			integration: {status: "not-run", next_step: "Dogfood in an authorized consumer and retain the obsolete-fork removal and upgrade evidence."}
		}
		consumers: []
	},
]
