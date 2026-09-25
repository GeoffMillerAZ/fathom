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
