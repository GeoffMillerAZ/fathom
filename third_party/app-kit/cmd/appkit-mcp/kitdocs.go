package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// kitDocs holds the app-kit guidance an agent needs to use the kit well: the
// ADRs (docs/adr/NNNN-*.md), project intent, adoption guides, and binding
// conventions (CLAUDE.md, the DECISIONS register, VISION). It is loaded at
// startup from the kit checkout and degrades gracefully — if the root is wrong,
// the slices are simply empty and the tools say so.
type kitDocs struct {
	root      string
	decisions []decisionDoc
	extra     map[string]decisionDoc // stable document id -> doc
	dRange    string                 // "D1–D17", derived from DECISIONS.md — never hardcoded
}

type decisionDoc struct {
	ID    string // "0005" for ADRs; a slug for extras
	Slug  string // "loopback-invariant"
	Title string
	Body  string
}

func loadKitDocs(root string) *kitDocs {
	d := &kitDocs{root: root, extra: map[string]decisionDoc{}}

	adrDir := filepath.Join(root, "docs", "adr")
	entries, _ := os.ReadDir(adrDir)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == "README.md" || name == "template.md" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(adrDir, name))
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(name, ".md")
		id, slug, _ := strings.Cut(base, "-")
		d.decisions = append(d.decisions, decisionDoc{
			ID:    id,
			Slug:  slug,
			Title: firstHeading(string(body)),
			Body:  string(body),
		})
	}
	sort.Slice(d.decisions, func(i, j int) bool { return d.decisions[i].ID < d.decisions[j].ID })

	loadExtra := func(slug, path string) {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return
		}
		d.extra[slug] = decisionDoc{ID: slug, Slug: slug, Title: firstHeading(string(body)), Body: string(body)}
	}
	loadExtra("claude", "CLAUDE.md")
	loadExtra("decisions", filepath.Join("docs", "DECISIONS.md"))
	loadExtra("vision", "VISION.md")
	loadExtra("intent", filepath.Join("docs", "intent", "README.md"))
	for _, name := range []string{"principles", "invariants", "domain", "scope", "futures", "questions", "autonomy", "incidents", "repo"} {
		loadExtra("intent/"+name, filepath.Join("docs", "intent", name+".md"))
	}
	loadExtra("adoption", filepath.Join("docs", "design", "adoption.md"))
	loadExtra("agent-setup", filepath.Join("docs", "design", "agent-setup.md"))
	loadExtra("adoption-contract", filepath.Join("docs", "design", "adoption-contract.md"))
	loadExtra("contribution-loop", filepath.Join("docs", "design", "contribution-loop.md"))
	loadExtra("stabilization", filepath.Join("docs", "design", "stabilization-and-resumption.md"))
	loadExtra("experience-system", filepath.Join("docs", "design", "experience-system.md"))
	loadExtra("theme-workbench", filepath.Join("docs", "design", "theme-workbench.md"))
	loadExtra("app-owned-styleguide", filepath.Join("docs", "design", "app-owned-styleguide.md"))
	loadExtra("analytics-journey", filepath.Join("docs", "design", "analytics-journey.md"))
	loadExtra("viz-checks", filepath.Join("appkit", "ui", "vizcheck", "README.md"))
	loadExtra("application-shell", filepath.Join("docs", "design", "application-shell.md"))
	loadExtra("catalyst-settings-adapter", filepath.Join("docs", "design", "catalyst-settings-adapter.md"))
	loadExtra("fathom-maintenance", filepath.Join("docs", "design", "fathom-maintenance.md"))
	loadExtra("web-desktop-parity", filepath.Join("docs", "design", "web-desktop-parity.md"))
	loadExtra("design-references", filepath.Join("docs", "design", "private-references.md"))
	loadExtra("conversion-pipeline", filepath.Join("docs", "design", "conversion-pipeline.md"))
	loadExtra("just-in-time-conversion", filepath.Join("docs", "design", "just-in-time-conversion.md"))
	loadExtra("converted-component-standard", filepath.Join("docs", "design", "converted-component-standard.md"))
	loadExtra("quality-inventory", filepath.Join("internal", "qualityplan", "inventory.cue"))
	loadExtra("quality-schema", filepath.Join("internal", "qualityplan", "schema.cue"))
	for id, title := range map[string]string{"quality-inventory": "Authored standards and readiness inventory (CUE)", "quality-schema": "Authored readiness schema (CUE)"} {
		if doc, ok := d.extra[id]; ok {
			doc.Title = title
			d.extra[id] = doc
		}
	}
	loadExtra("target-evidence", filepath.Join("docs", "design", "target-evidence.md"))
	loadExtra("pricing-adapter", filepath.Join("docs", "assessments", "fixtures", "pricing-adapter", "README.md"))
	loadExtra("hero-adapter", filepath.Join("docs", "design", "hero-adapter.md"))
	loadExtra("menu-adapter", filepath.Join("docs", "design", "menu-adapter.md"))
	loadExtra("drawer-adapter", filepath.Join("docs", "design", "profile-drawer-adapter.md"))
	loadExtra("navbar-adapter", filepath.Join("docs", "design", "responsive-navbar-adapter.md"))
	loadExtra("directory-adapter", filepath.Join("docs", "assessments", "fixtures", "table-adapter", "README.md"))
	loadExtra("form-adapter", filepath.Join("docs", "assessments", "fixtures", "form-adapter", "README.md"))
	loadExtra("navigation-adapter", filepath.Join("docs", "assessments", "fixtures", "navigation-adapter", "README.md"))
	loadExtra("html-translation", filepath.Join("docs", "design", "html-translation.md"))
	loadExtra("elements-translation", filepath.Join("docs", "design", "elements-translation.md"))
	loadExtra("faq-adapter", filepath.Join("docs", "design", "faq-adapter.md"))
	loadExtra("runtime-selection", filepath.Join("docs", "design", "runtime-selection.md"))
	loadExtra("static-content-delivery", filepath.Join("docs", "design", "static-content-delivery.md"))
	loadExtra("client-editor-island", filepath.Join("docs", "design", "client-editor-island.md"))
	loadExtra("ui-components", filepath.Join("docs", "ui-components.md"))
	loadExtra("cli-kit", filepath.Join("docs", "design", "cli-kit.md"))
	loadExtra("terminal-gallery", filepath.Join("docs", "design", "terminal-gallery.md"))
	// The theming/contrast hazard checklist. It also rides along with
	// get_kit_theming, but registering it here makes get_kit_decision("hazards")
	// resolve it directly — the review checklist an agent reaches for by name.
	loadExtra("hazards", filepath.Join("docs", "theming-hazards.md"))

	d.dRange = decisionRange(d.extra["decisions"].Body)

	return d
}

// decisionRange derives the D-register's span ("D1–D17") from the register
// itself. It was a hardcoded string until 2026-08-07, by which time it said
// "D1–D15" against a file containing D16 — an agent was being told two
// decisions did not exist. A derived range cannot go stale, so the honesty
// here is structural rather than a thing someone has to remember.
//
// It reads the index-table rows (`| D7 | … |`), not the body headings, because
// the table is the register's canonical list. Returns "" when nothing parses,
// and guidance() then omits the range rather than inventing one.
func decisionRange(md string) string {
	lo, hi := 0, 0
	for _, line := range strings.Split(md, "\n") {
		s := strings.TrimSpace(line)
		if !strings.HasPrefix(s, "| D") {
			continue
		}
		field, _, ok := strings.Cut(strings.TrimPrefix(s, "| "), "|")
		if !ok {
			continue
		}
		num := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(field), "D"))
		n, err := strconv.Atoi(num)
		if err != nil || n <= 0 {
			continue
		}
		if lo == 0 || n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	if hi == 0 {
		return ""
	}
	return fmt.Sprintf("D%d–D%d", lo, hi)
}

// guidance returns the primary "how to build on app-kit" text: CLAUDE.md (the
// hard rules and workflow) with a pointer to the ADRs and decision register.
func (d *kitDocs) guidance() string {
	var b strings.Builder
	b.WriteString("Host setup and repeatable protocol checks: get_kit_decision(\"agent-setup\"). Codex and Claude Code use different registration files; a connection is not adoption proof.\n")
	if claude, ok := d.extra["claude"]; ok {
		b.WriteString(claude.Body)
	} else {
		b.WriteString("app-kit guidance unavailable: run appkit-mcp with -kit-root pointing at the app-kit checkout.\n")
	}
	b.WriteString("\n---\nStart with get_kit_decision(\"intent\") for the project-intent router. Select the target and adopted layers, then follow only the applicable intent and runtime profile; the full Go server stack is not a requirement for every consumer. Intent documents are available as get_kit_decision(\"intent/<name>\") and kit://intent/<name> resources. For the consumer workflow call get_kit_decision(\"adoption\"); for the experience system call get_kit_decision(\"experience-system\"). Discover implemented UI packages with get_kit_decision(\"ui-components\") and terminal conventions with get_kit_decision(\"cli-kit\").\nFor the binding decisions")
	if d.dRange != "" {
		b.WriteString(" (" + d.dRange + ")")
	}
	b.WriteString(" call get_kit_decision(\"decisions\"); for the north-star vision call get_kit_decision(\"vision\"); for any architecture decision call list_kit_decisions then get_kit_decision(<id>).\n")
	b.WriteString("For complete private design references, call get_kit_decision(\"design-references\"). Select a visual reference separately from the implementation capability; preserve its hierarchy, typography, spacing and interaction states during adaptation.\n")
	b.WriteString("For source conversions and agent repairs, read get_kit_decision(\"conversion-pipeline\"); for static, Go/Datastar and client-heavy choices, read get_kit_decision(\"runtime-selection\"). Originals remain first-class choices alongside adaptations. get_kit_decision(\"html-translation\") documents the experimental syntax-only CLI; there is no translation MCP tool or target certification yet. get_kit_decision(\"target-evidence\") describes the internal receipt integrity checker, not semantic proof or a certification tool. Do not infer compatibility from compilation or change a test oracle to obtain a pass.\n")
	b.WriteString("For the private Go-hosted React/Tiptap client-owned editor rehearsal, read get_kit_decision(\"client-editor-island\"). It is not a public widget API, certified conversion or save backend.\n")
	b.WriteString("Core-first enhancement follows get_kit_decision(\"just-in-time-conversion\") and get_kit_decision(\"converted-component-standard\"). Use list_quality_work for bounded discovery by scope/capability with expected_catalog_revision from get_capability.revision, then get_quality_work with expected_revision=revision and expected_catalog_revision=catalog_revision from the list for one item from the embedded authored snapshot (rebuild to refresh; lookup does not check evidence files or certify gates). Inspect or edit the full authored CUE work queue with get_kit_decision(\"quality-inventory\") and its validation rules with get_kit_decision(\"quality-schema\"). Standards implementation, gate claims and consumer adoption are separate; schema validity never certifies quality. Convert the long tail only for a named adopter or owner-selected batch, preserve imported originals and failed receipts, and return improvements through the contribution loop.\n")
	b.WriteString("For current stabilization and as-needed continuation, read get_kit_decision(\"stabilization\"). Resume a named consumer task or explicit owner-funded maintenance item; do not resume G1-G6 as a blanket program.\n")
	b.WriteString("For the separate runtime-bound Elements syntax rule, read get_kit_decision(\"elements-translation\") or get_capability(\"reference.elements-translator\"). Its CLI declares a pinned host-provided runtime, remains not-verified and does not enter the legacy syntax@ store.\n")
	b.WriteString("Find first-party private adapter tooling with search_capabilities(query=\"private adapter\", profile=\"cli\", kind=\"reference\"), then get_capability for reference.hero-adapter, reference.menu-adapter, reference.drawer-adapter, reference.navbar-adapter, reference.pricing-adapter, reference.directory-adapter, reference.form-adapter, reference.navigation-adapter or reference.faq-adapter. These metadata-only CLI records do not advertise certified web-profile support, expose licensed bodies or execute conversion.\n")
	b.WriteString("For P09, get_kit_decision(\"navbar-adapter\") explains unchanged assets-adapt-navbar five-file -out/-check delivery under ADR-0057 and explicit nine-file -repair -runtime ABSOLUTE_ORIGINAL_ESM delivery under ADR-0060. NavbarReference keeps the original; NewNavbar shares one route model across desktop/mobile, with an exact CurrentKey or none current. NewNavbarWithRoutes adds persistent native route access. The repair requires the exact original module as an explicit input and delivers its separately replayed focus derivative; never load original and derivative in the same document. Compile/scan both delivered templates, load source CSS then repair CSS, and serve the selected module once. Base/repair checks refuse cross-variant delivery. Defaults leave identity, landmark, route and image bindings incomplete. Optional account/notification GET destinations never infer sign-out mutations or a backend. The app supplies unique IDs, accurate current state, images and fonts. Browser screening does not certify the target; original imagery, full accessibility/performance/lifecycle and consumer acceptance remain open.\n")
	b.WriteString("For the private P08 profile-drawer candidate, get_kit_decision(\"drawer-adapter\") documents unchanged five-file assets-adapt-drawer -out/-check delivery under ADR-0053 and the explicit six-file -repair variant under ADR-0054. Select the app-owned data-appkit-drawer-focus wrapper and load its CSS after source CSS; no extra constructor or JavaScript is added. DrawerReference keeps the original; NewDrawer binds explicit identity, image semantics and optional GET navigation. Defaults leave IDPrefix/ImageURL blank, actions absent and online state unclaimed. Empty actions yield read-only output. The app owns truthful presence, image rights, destinations and pinned Elements 1.0.22 delivery; no messaging/call/clipboard or account backend is inferred. Original focus failures remain recorded; repair has scoped normal/actual200% evidence. Missing original imagery and open browser/lifecycle/consumer gates prevent target certification.\n")
	b.WriteString("For the private P01 hero candidate, get_kit_decision(\"hero-adapter\") documents typed copy, all six required app-owned URLs, explicit image alt/decorative semantics and six-file assets-adapt-hero -out/-check delivery. NewHero preserves the source; NewHeroEnhanced explicitly adds scoped CSS. Original image URLs are provenance only, never silently downloaded or bundled. Neither variant is a certified web target.\n")
	b.WriteString("For the private P07 dropdown-menu candidate, get_kit_decision(\"menu-adapter\") documents unchanged six-file assets-adapt-menu -out/-check delivery and the explicit eight-file -repair variant under ADR-0051. The repair envelope retains the exact base plus hash-bound module/CSS; base and repair exports reject cross-checks. MenuReference and NewMenu remain unmounted; NewMenuEnhanced may explicitly mountMenuRepair after Elements is defined and dispose before removal/remount. The app supplies pinned Elements 1.0.22, runtime-failure fallback, authorization and CSRF issuance for an optional canonical local POST action. Unchanged v1 fails actual zoom; repair has scoped Chrome evidence, not real-consumer acceptance or certified-target support.\n")
	b.WriteString("For app-owned FAQ content and repeated instances, read get_kit_decision(\"faq-adapter\"). Choose faithful Elements Reference or explicit JavaScript-free native Details; keep document-wide ID prefixes unique. No implicit runtime fallback or state persistence is supplied.\n")
	b.WriteString("For the separate experimental fixed static-reading/FAQ example, read get_kit_decision(\"static-content-delivery\"). Go runs at build time; visitors need HTML/CSS only. Follow its explicit CSS build, fresh output directory and hosting-prefix/404 requirements. This is not a generic site generator, CMS or certified static profile.\n")
	b.WriteString("For the experimental private typed P02 component, get_kit_decision(\"pricing-adapter\") documents JSON-default CLI generation, explicit private -out/-check delivery, faithful versus opt-in enhanced rendering, required CSS and evidence gaps. It is not a certified catalog target.\n")
	b.WriteString("For the private P04 directory candidate, get_kit_decision(\"directory-adapter\") documents unchanged JSON defaults, bounded typed rows/native Add/Edit navigation and an opt-in -enhanced companion. Explicit -out/-check selects its five-file base or eight-file enhanced private inventory. The original remains separate; delivery replay proves neither backend behavior, full UI quality nor target certification.\n")
	b.WriteString("For the private P05 email-field candidate, get_kit_decision(\"form-adapter\") documents typed error/neutral presentation, five-file -out/-check delivery and native recovery evidence. Scan both form.go and form.templ for CSS classes. The app owns unique IDs, validation, authorization and save/retry; neither delivery nor the synthetic fixture is a production backend or certified target.\n")
	b.WriteString("For the private P06 page-navigation candidate, get_kit_decision(\"navigation-adapter\") documents assets-adapt-navigation five-file -out/-check delivery and the explicit Reference, GET and Links variants. Scan both navigation.go and navigation.templ for CSS classes. Operational destinations are limited to canonical local paths; the app owns authorization, the truthful current key and submitted key-to-route mapping. Synthetic native select type-ahead is proven, but OS popup-arrow behavior, consumer adoption and target certification remain unproven.\n")
	b.WriteString("Saved private attempts are discoverable through list_design_conversions and get_design_conversion. These read-only tools preserve exact original references and distinguish current replay from stale transformer identity; neither converts, saves, executes or certifies a target. Use get_design_asset_file for the linked original.\n")
	b.WriteString("For the experimental Gallery analytics reference, read get_kit_decision(\"analytics-journey\"); this is not promoted consumer proof.\n")
	b.WriteString("For opt-in static chart/table/source checks with scoped SVG coverage, read get_kit_decision(\"viz-checks\"). Supply an independent fixture; these checks do not prove geometry, computed visibility, accessibility or source truth.\n")
	b.WriteString("For the experimental profile/settings/help shell workflow, read get_kit_decision(\"application-shell\"). It composes existing primitives around synthetic state; consumers keep their real identity, permissions, persistence and support policy.\n")
	b.WriteString("For P10's private complete native settings contract, read get_kit_decision(\"catalyst-settings-adapter\"): preserve all five sections, the native CSS companion and app-owned policy; this document does not expose a public component or certify the target. For independently usable browser workflows and explicit native exceptions, read get_kit_decision(\"web-desktop-parity\"); it is a conformance contract, not shipped platform support. For kit-local Fathom configuration and stale-evidence limits, read get_kit_decision(\"fathom-maintenance\"); it does not authorize consumer or global configuration changes.\n")
	b.WriteString("For the experimental read-only Charm terminal reference, read get_kit_decision(\"terminal-gallery\"). It browses first-party catalog IDs and revision, not browser or private-source coverage; it does not establish two-consumer adoption or host certification.\n")
	b.WriteString("For personal-site Synthwave/Indigo identities and named themes, read get_kit_decision(\"theme-workbench\"). For experimental local app-owned token preview, validation and reviewed snapshot export, read get_kit_decision(\"app-owned-styleguide\"). Neither installs fonts, saves preferences or migrates a consumer.\n")
	b.WriteString("For explicit consumer selections and read-only CLI static checks, read get_kit_decision(\"adoption-contract\"). These checks do not prove runtime adoption or agent-host activation; there is no adoption MCP execution tool.\n")
	b.WriteString("For a first selection, appkit adoption draft prints an explicit v2 manifest without consumer writes. Review metadata, requirements and file hashes before saving, then run adoption check; never use drafting to approve unexplained upgrade drift.\n")
	return b.String()
}

// findDecision resolves an ADR by id (e.g. "0005", "5") or slug (e.g.
// "loopback-invariant", "loopback"), and also matches the extra docs
// ("intent", "intent/scope", "adoption", "claude", "decisions", "vision").
func (d *kitDocs) findDecision(query string) (decisionDoc, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return decisionDoc{}, false
	}
	if doc, ok := d.extra[q]; ok {
		return doc, true
	}
	qpad := q
	for len(qpad) < 4 && isAllDigits(qpad) {
		qpad = "0" + qpad
	}
	// An exact id or slug always wins.
	for _, doc := range d.decisions {
		if doc.ID == q || doc.ID == qpad || doc.Slug == q {
			return doc, true
		}
	}
	// Otherwise accept a substring match only when it is unambiguous; a partial
	// like "cue" matches both 0002 and 0007, so return not-found and let the
	// caller list_kit_decisions rather than silently picking the lowest id.
	var match decisionDoc
	n := 0
	for _, doc := range d.decisions {
		if strings.Contains(doc.Slug, q) {
			match, n = doc, n+1
		}
	}
	if n == 1 {
		return match, true
	}
	return decisionDoc{}, false
}

func firstHeading(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "# "))
		}
	}
	return ""
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
