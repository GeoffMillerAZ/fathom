package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testDocs() *kitDocs {
	return &kitDocs{
		decisions: []decisionDoc{
			{ID: "0002", Slug: "cue-tokens", Title: "CUE tokens"},
			{ID: "0005", Slug: "loopback-invariant", Title: "The loopback invariant"},
			{ID: "0007", Slug: "cue-source-of-truth", Title: "CUE is the source of truth"},
			{ID: "0010", Slug: "component-data-pipeline", Title: "Component data pipeline"},
		},
		extra: map[string]decisionDoc{
			"decisions": {ID: "decisions", Slug: "decisions", Title: "Decision register"},
			"vision":    {ID: "vision", Slug: "vision", Title: "Vision"},
		},
	}
}

func TestFindDecision(t *testing.T) {
	d := testDocs()
	cases := []struct {
		query  string
		wantID string
		wantOK bool
	}{
		{"0005", "0005", true},                    // exact padded id
		{"5", "0005", true},                       // bare id zero-pads
		{"loopback-invariant", "0005", true},      // exact slug
		{"loopback", "0005", true},                // unambiguous substring
		{"component-data-pipeline", "0010", true}, // exact slug
		{"decisions", "decisions", true},          // extra doc
		{"vision", "vision", true},                // extra doc
		{"cue", "", false},                        // ambiguous: 0002 + 0007 → not found
		{"nonexistent", "", false},                // no match
		{"", "", false},                           // empty
		{"   ", "", false},                        // whitespace only
		{"LOOPBACK", "0005", true},                // case-insensitive
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			doc, ok := d.findDecision(c.query)
			if ok != c.wantOK {
				t.Fatalf("findDecision(%q) ok = %v, want %v (got id %q)", c.query, ok, c.wantOK, doc.ID)
			}
			if ok && doc.ID != c.wantID {
				t.Errorf("findDecision(%q) id = %q, want %q", c.query, doc.ID, c.wantID)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Governance assertions (ADR-0017 / plan multi-target-kit.md §B).
//
// These read the real docs tree rather than a fixture on purpose: the failure
// they exist to catch is a doc going stale, and a fixture cannot go stale.
// ─────────────────────────────────────────────────────────────────────────────

const kitRootRel = "../.."

// adrFiles returns docs/adr/NNNN-*.md as (id, path) pairs, skipping README and
// the template.
func adrFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join(kitRootRel, "docs", "adr")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" || name == "template.md" {
			continue
		}
		id, _, ok := strings.Cut(strings.TrimSuffix(name, ".md"), "-")
		if !ok || len(id) != 4 {
			t.Errorf("ADR filename %q is not NNNN-slug.md", name)
			continue
		}
		out[id] = filepath.Join(dir, name)
	}
	if len(out) == 0 {
		t.Fatalf("no ADRs found under %s", dir)
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// statusLine returns the ADR's `- **Status:** …` value, joined across
// continuation lines so a wrapped status is still one string.
func statusLine(md string) string {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "- **Status:**") {
			continue
		}
		status := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- **Status:**"))
		for _, cont := range lines[i+1:] {
			trimmed := strings.TrimSpace(cont)
			if trimmed == "" || strings.HasPrefix(trimmed, "- **") || strings.HasPrefix(trimmed, "#") {
				break
			}
			status += " " + trimmed
		}
		return status
	}
	return ""
}

var fourDigits = regexp.MustCompile(`\d{4}`)

// TestADRStatusLinesAreHonest — a status line that says "Superseded" must name
// a successor that exists, and the two supersessions ADR-0017 performed must
// still be visible in the documents they touched. The failure this catches is
// the cheap one: a status line edited to Superseded with no successor named, or
// ADR-0002's retired clause quietly un-struck so it reads as current again.
func TestADRStatusLinesAreHonest(t *testing.T) {
	adrs := adrFiles(t)

	for id, path := range adrs {
		status := statusLine(readFile(t, path))
		if status == "" {
			t.Errorf("ADR-%s has no `- **Status:**` line", id)
			continue
		}
		if !strings.Contains(strings.ToLower(status), "supersed") {
			continue
		}
		named := false
		for _, cand := range fourDigits.FindAllString(status, -1) {
			if cand == id {
				continue // its own number doesn't count as a successor
			}
			if _, ok := adrs[cand]; ok {
				named = true
			}
		}
		if !named {
			t.Errorf("ADR-%s status says superseded but names no existing successor ADR: %q", id, status)
		}
	}

	// ADR-0016's carve-out became a target class in ADR-0017.
	if s := statusLine(readFile(t, adrs["0016"])); !strings.Contains(s, "0017") {
		t.Errorf("ADR-0016 status must name ADR-0017 as its successor, got %q", s)
	}

	// ADR-0002's status must name 0017, and the clause ADR-0017 §3 retired must
	// survive ONLY inside a strikethrough. It is the sentence the kit breaks
	// four times (chat.js, overlay.js, router.js, motion.js); a reader arriving
	// at 0002 via an old citation has to see that it died.
	adr2 := readFile(t, adrs["0002"])
	if s := statusLine(adr2); !strings.Contains(s, "0017") {
		t.Errorf("ADR-0002 status must name ADR-0017, got %q", s)
	}
	const retired = "JavaScript is never written as source"
	flat := strings.Join(strings.Fields(adr2), " ")
	for i := 0; ; {
		j := strings.Index(flat[i:], retired)
		if j < 0 {
			break
		}
		at := i + j
		before := flat[max(0, at-4):at]
		if !strings.Contains(before, "~~") {
			t.Errorf("ADR-0002 still asserts %q outside a strikethrough (preceded by %q) — ADR-0017 §3 retired it", retired, before)
		}
		i = at + len(retired)
	}
	if !strings.Contains(flat, "~~") {
		t.Error("ADR-0002 contains no strikethrough at all — the retired clause was deleted rather than struck; the trail must stay readable")
	}
}

// TestDecisionsIndexCoversEveryADR — every ADR on disk has a row in both
// indexes, and the guidance footer's D-range is derived rather than typed. The
// hardcoded footer read "D1–D15" for two decisions after D16 landed, which is
// an agent being told a binding decision does not exist.
func TestDecisionsIndexCoversEveryADR(t *testing.T) {
	register := readFile(t, filepath.Join(kitRootRel, "docs", "DECISIONS.md"))
	index := readFile(t, filepath.Join(kitRootRel, "docs", "adr", "README.md"))

	for _, path := range adrFiles(t) {
		link := filepath.Base(path)
		if !strings.Contains(register, link) {
			t.Errorf("docs/DECISIONS.md has no row linking %s", link)
		}
		if !strings.Contains(index, link) {
			t.Errorf("docs/adr/README.md has no row linking %s", link)
		}
	}

	// The register's body headings (### D17 — …) are an independent witness to
	// the table's rows, so a footer derived from the table still has to agree
	// with the prose. That is what makes this non-tautological.
	bodyMax := 0
	for _, m := range regexp.MustCompile(`(?m)^### D(\d+)\b`).FindAllStringSubmatch(register, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > bodyMax {
			bodyMax = n
		}
	}
	if bodyMax == 0 {
		t.Fatal("docs/DECISIONS.md has no `### DN` body sections")
	}
	want := fmt.Sprintf("–D%d", bodyMax)
	docs := loadKitDocs(kitRootRel)
	if !strings.Contains(docs.guidance(), want) {
		t.Errorf("get_kit_guidance footer does not end at %s (dRange = %q); the register's highest body section is D%d", want, docs.dRange, bodyMax)
	}
}

// TestFindDecision_Hazards — the theming/contrast hazard checklist must resolve
// by name through get_kit_decision("hazards"). It rides along with
// get_kit_theming too, but an agent that reaches for the review checklist
// directly (the five-minute audit before shipping a theme) should find it
// without knowing which tool bundles it. loadKitDocs registers it from
// docs/theming-hazards.md; this pins that it stays reachable.
func TestFindDecision_Hazards(t *testing.T) {
	doc, ok := loadKitDocs(kitRootRel).findDecision("hazards")
	if !ok {
		t.Fatal(`get_kit_decision("hazards") does not resolve — register loadExtra("hazards", …) in loadKitDocs`)
	}
	if strings.TrimSpace(doc.Body) == "" {
		t.Error(`get_kit_decision("hazards") resolved to an empty body`)
	}
	// It is the hazard checklist, not some other doc that happened to match.
	if !strings.Contains(doc.Body, "MEASURE, NEVER READ") {
		t.Error(`the "hazards" doc is not docs/theming-hazards.md (missing its H3 heading)`)
	}
}

// TestGuidanceIsCurrent — get_kit_guidance returns app-kit/CLAUDE.md verbatim
// (kitdocs.go), so CLAUDE.md is what every agent session reads first. Fixing
// the global ~/.claude/CLAUDE.md without fixing this file silently regresses on
// the next session's first MCP call, which is exactly how the old Path A/Path B
// rule outlived the decision that retired it.
func TestGuidanceIsCurrent(t *testing.T) {
	// Markdown wrapping does not change the policy a consumer reads.
	g := strings.Join(strings.Fields(loadKitDocs(kitRootRel).guidance()), " ")
	for _, want := range []string{"appkit adoption draft", "without consumer writes", "before saving, then run adoption check", "never use drafting to approve unexplained upgrade drift"} {
		if !strings.Contains(g, want) {
			t.Errorf("guidance omits first-adoption boundary %q", want)
		}
	}

	for _, want := range []string{"ADR-0017", "L3a", "appkit vendor", "guides, it does not dictate", "Stopping is a complete adoption", "experimental Gallery analytics reference", "not promoted consumer proof", "list_design_conversions", "get_design_conversion", "neither converts, saves, executes or certifies a target", "read-only CLI static checks", "do not prove runtime adoption or agent-host activation", "no adoption MCP execution tool", "experimental read-only Charm terminal reference", "first-party catalog IDs and revision", "not browser or private-source coverage", "does not establish two-consumer adoption or host certification"} {
		if !strings.Contains(g, want) {
			t.Errorf("kit guidance does not mention %q", want)
		}
	}
	for _, stale := range []string{"D1–D15", "D1-D15"} {
		if strings.Contains(g, stale) {
			t.Errorf("kit guidance still carries the stale hardcoded D-range %q", stale)
		}
	}
	// The old rule, in the words it was written in. It said React apps could not
	// use the kit; a byte-identical copy of router.js had been in production on
	// a React site for months when it was written down.
	if strings.Contains(g, "the no-Node rule do **not** apply here") {
		t.Error("kit guidance still carries the retired Path A/Path B framing")
	}
}

// Documentation routing proof, not evidence that a consumer host enforces it.
func TestContributionGuidancePreservesOwnershipAndPromotion(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	doc := docs.extra["contribution-loop"]
	body := strings.Join(strings.Fields(doc.Body), " ")
	for _, want := range []string{"before handing off", "two consumers", "no second victim required", "local-divergence", "second-consumer result or unverified", "No automatic fleet upgrade", "Keep the generated app-kit block identical", "not installed automation", "originating-app integration nor second-app propagation"} {
		if !strings.Contains(body, want) {
			t.Errorf("contribution guidance omits %q", want)
		}
	}
	if !strings.Contains(docs.guidance(), `get_kit_decision("contribution-loop")`) {
		t.Error("guidance must route contributions through the same MCP document")
	}
	for _, path := range []string{"CLAUDE.md", "AGENTS.md"} {
		guidance := strings.Join(strings.Fields(readFile(t, filepath.Join(kitRootRel, path))), " ")
		for _, trigger := range []string{"At discovery", "before duplicating a workaround", "material UI/CLI handoff"} {
			if !strings.Contains(guidance, trigger) {
				t.Errorf("%s omits contribution trigger %q", path, trigger)
			}
		}
	}
	// Fresh adopters may start from a build guide or the terminal guide without
	// fetching MCP guidance. Keep these entry points on the canonical policy,
	// not independently maintained copies of its promotion rules.
	for _, entry := range []struct{ path, link string }{
		{"docs/getting-started.md", "(design/contribution-loop.md)"},
		{"docs/design/cli-kit.md", "(contribution-loop.md)"},
		{"docs/design/adoption-contract.md", "(contribution-loop.md)"},
	} {
		t.Run(entry.path, func(t *testing.T) {
			if !strings.Contains(readFile(t, filepath.Join(kitRootRel, entry.path)), entry.link) {
				t.Error("adopter entry point must link to the canonical contribution loop")
			}
		})
	}
}

func TestStabilizationGuidanceRoutesResumeContract(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	doc, ok := docs.findDecision("stabilization")
	if !ok || strings.TrimSpace(doc.Body) == "" {
		t.Fatal("stabilization guide unavailable")
	}
	for _, heading := range []string{"## Current priority", "## Resume packets", "## Resume contract"} {
		if !strings.Contains(doc.Body, heading) {
			t.Errorf("stabilization guide omits %q", heading)
		}
	}
	guidance := docs.guidance()
	if !strings.Contains(guidance, `get_kit_decision("stabilization")`) || !strings.Contains(guidance, "do not resume G1-G6 as a blanket program") {
		t.Error("guidance does not route bounded, as-needed continuation")
	}
}

func TestPrivateDeliveryGuidanceKeepsVariantAndEvidenceBoundaries(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	guidance := docs.guidance()
	for _, want := range []string{"explicit private -out/-check delivery", "five-file base or eight-file enhanced private inventory", "delivery replay proves neither backend behavior, full UI quality nor target certification"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("missing private delivery guidance %q", want)
		}
	}
	if strings.Contains(guidance, "JSON-only CLI generation") {
		t.Error("guidance hides implemented private delivery")
	}
	doc, ok := docs.findDecision("directory-adapter")
	if !ok {
		t.Fatal("directory guide unavailable")
	}
	for _, want := range []string{"-out NEW_DIRECTORY", "-check DIRECTORY", "ADR-0036", "Compile a COPY", "real consumer integration remain distinct"} {
		if !strings.Contains(doc.Body, want) {
			t.Errorf("directory guide omits %q", want)
		}
	}
}

func TestMenuRepairGuidancePreservesVariantAndLifecycleBoundaries(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	for _, want := range []string{"unchanged six-file", "explicit eight-file -repair", "ADR-0051", "exports reject cross-checks", "dispose before removal/remount", "Unchanged v1 fails actual zoom", "not real-consumer acceptance or certified-target support"} {
		if !strings.Contains(docs.guidance(), want) {
			t.Errorf("menu repair guidance omits %q", want)
		}
	}
	doc, ok := docs.findDecision("menu-adapter")
	if !ok {
		t.Fatal("menu repair guide unavailable")
	}
	flat := strings.Join(strings.Fields(doc.Body), " ")
	for _, want := range []string{"-repair", "eight-file", "menu_repair.mjs", "mountMenuRepair", "disposer", "unchanged", "not a certified web component"} {
		if !strings.Contains(flat, want) {
			t.Errorf("menu repair guide omits %q", want)
		}
	}
}

func TestDrawerGuidancePreservesAppAuthorityAndEvidenceBoundaries(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	for _, want := range []string{`get_kit_decision("drawer-adapter")`, "reference.drawer-adapter", "five-file assets-adapt-drawer", "ADR-0053", "explicit six-file -repair", "ADR-0054", "data-appkit-drawer-focus", "IDPrefix/ImageURL blank", "no messaging/call/clipboard or account backend", "prevent target certification"} {
		if !strings.Contains(docs.guidance(), want) {
			t.Errorf("drawer guidance omits %q", want)
		}
	}
	doc, ok := docs.findDecision("drawer-adapter")
	if !ok {
		t.Fatal("drawer guide unavailable")
	}
	flat := strings.Join(strings.Fields(doc.Body), " ")
	for _, want := range []string{"-out NEW_PRIVATE_DIRECTORY", "-check PRIVATE_DIRECTORY", "five immutable files", "DrawerReference()", "NewDrawer(Drawer)", "No enhanced stylesheet", "runtime loading fails", "Original-photo fidelity", "zero-certified-target count", "assets-adapt-drawer -repair", "six-file", "data-appkit-drawer-focus", "prefers-color-scheme: dark"} {
		if !strings.Contains(flat, want) {
			t.Errorf("drawer guide omits %q", want)
		}
	}
}

func TestNavbarGuidancePreservesRouteAndAuthorityBoundaries(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	for _, want := range []string{`get_kit_decision("navbar-adapter")`, "reference.navbar-adapter", "assets-adapt-navbar five-file -out/-check", "ADR-0057", "exact CurrentKey or none current", "never infer sign-out mutations", "does not certify the target", "nine-file -repair -runtime ABSOLUTE_ORIGINAL_ESM", "ADR-0060", "never load original and derivative in the same document", "refuse cross-variant delivery"} {
		if !strings.Contains(docs.guidance(), want) {
			t.Errorf("navbar guidance omits %q", want)
		}
	}
	doc, ok := docs.findDecision("navbar-adapter")
	if !ok {
		t.Fatal("navbar guide unavailable")
	}
	flat := strings.Join(strings.Fields(doc.Body), " ")
	for _, want := range []string{"-out NEW_PRIVATE_DIRECTORY", "-check PRIVATE_DIRECTORY", "five immutable files", "NavbarReference()", "NewNavbar(Navbar)", "when runtime loading fails", "not a certified web component", "no inferred home link", "-repair -runtime ABSOLUTE_ORIGINAL_ESM", "nine-file private variant", "Do not supply the delivered derivative as the original", "Serve elements-navbar.js once"} {
		if !strings.Contains(flat, want) {
			t.Errorf("navbar guide omits %q", want)
		}
	}
}

func TestFormAdapterGuidancePreservesCSSAndOwnershipBoundaries(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	for _, want := range []string{`get_kit_decision("form-adapter")`, "reference.form-adapter", "Scan both form.go and form.templ", "app owns unique IDs, validation, authorization and save/retry", "production backend or certified target"} {
		if !strings.Contains(docs.guidance(), want) {
			t.Errorf("form guidance omits %q", want)
		}
	}
	doc, ok := docs.findDecision("form-adapter")
	if !ok {
		t.Fatal("form guide unavailable")
	}
	for _, want := range []string{"assets-adapt-form", "form.go and form.templ", "NewEmailField", "Error empty", "distinct IDs", "validation", "No package installation", "not a performance guarantee"} {
		if !strings.Contains(doc.Body, want) {
			t.Errorf("form guide omits %q", want)
		}
	}
}

func TestNavigationAdapterGuidancePreservesRoutingAndEvidenceBoundaries(t *testing.T) {
	docs := loadKitDocs(kitRootRel)
	for _, want := range []string{`get_kit_decision("navigation-adapter")`, "reference.navigation-adapter", "assets-adapt-navigation five-file -out/-check delivery", "Reference, GET and Links variants", "Scan both navigation.go and navigation.templ", "canonical local paths", "app owns authorization, the truthful current key and submitted key-to-route mapping", "native select type-ahead is proven", "OS popup-arrow behavior, consumer adoption and target certification remain unproven"} {
		if !strings.Contains(docs.guidance(), want) {
			t.Errorf("navigation guidance omits %q", want)
		}
	}
	doc, ok := docs.findDecision("navigation-adapter")
	if !ok {
		t.Fatal("navigation guide unavailable")
	}
	flat := strings.Join(strings.Fields(doc.Body), " ")
	for _, want := range []string{"assets-adapt-navigation", "five-file delivery", "navigation.go` and `navigation.templ", "NewPageNavigationReference", "NewPageNavigationGET", "NewPageNavigationLinks", "map it to a route", "authorization", "native keyboard type-ahead/submission", "OS native-popup arrow navigation", "consumer adoption claim", "certified target"} {
		if !strings.Contains(flat, want) {
			t.Errorf("navigation guide omits %q", want)
		}
	}
}

func TestCatalystSettingsAdapterGuidancePreservesCompositionAndAuthority(t *testing.T) {
	doc, ok := loadKitDocs(kitRootRel).findDecision("catalyst-settings-adapter")
	if !ok {
		t.Fatal(`get_kit_decision("catalyst-settings-adapter") does not resolve the root intent router's P10 guide`)
	}
	flat := strings.Join(strings.Fields(doc.Body), " ")
	for _, want := range []string{
		"SettingsNativeStyles()",
		"all five sections",
		"Consumers own that shell, labels/values, allowed options, auth, routes and persistence",
		"CLI/catalog exposure and certification remain open",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("Catalyst settings guide omits %q", want)
		}
	}
}

// Exercise the actual MCP boundary: an agent that only has the server must be
// able to follow the router and retrieve the same text as a local file reader.
func TestIntentDocumentsThroughMCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv := mcp.NewServer(&mcp.Implementation{Name: "appkit-test", Version: "test"}, nil)
	docs := loadKitDocs(kitRootRel)
	registerKitTools(srv, docs)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "kit-docs-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]*mcp.Resource{}
	for _, resource := range resources.Resources {
		listed[resource.URI] = resource
	}
	paths := map[string]string{
		"intent":                       "docs/intent/README.md",
		"adoption":                     "docs/design/adoption.md",
		"agent-setup":                  "docs/design/agent-setup.md",
		"adoption-contract":            "docs/design/adoption-contract.md",
		"contribution-loop":            "docs/design/contribution-loop.md",
		"stabilization":                "docs/design/stabilization-and-resumption.md",
		"experience-system":            "docs/design/experience-system.md",
		"theme-workbench":              "docs/design/theme-workbench.md",
		"app-owned-styleguide":         "docs/design/app-owned-styleguide.md",
		"analytics-journey":            "docs/design/analytics-journey.md",
		"viz-checks":                   "appkit/ui/vizcheck/README.md",
		"application-shell":            "docs/design/application-shell.md",
		"catalyst-settings-adapter":    "docs/design/catalyst-settings-adapter.md",
		"design-references":            "docs/design/private-references.md",
		"fathom-maintenance":           "docs/design/fathom-maintenance.md",
		"conversion-pipeline":          "docs/design/conversion-pipeline.md",
		"just-in-time-conversion":      "docs/design/just-in-time-conversion.md",
		"converted-component-standard": "docs/design/converted-component-standard.md",
		"quality-inventory":            "internal/qualityplan/inventory.cue",
		"quality-schema":               "internal/qualityplan/schema.cue",
		"target-evidence":              "docs/design/target-evidence.md",
		"pricing-adapter":              "docs/assessments/fixtures/pricing-adapter/README.md",
		"hero-adapter":                 "docs/design/hero-adapter.md",
		"menu-adapter":                 "docs/design/menu-adapter.md",
		"drawer-adapter":               "docs/design/profile-drawer-adapter.md",
		"navbar-adapter":               "docs/design/responsive-navbar-adapter.md",
		"directory-adapter":            "docs/assessments/fixtures/table-adapter/README.md",
		"form-adapter":                 "docs/assessments/fixtures/form-adapter/README.md",
		"navigation-adapter":           "docs/assessments/fixtures/navigation-adapter/README.md",
		"html-translation":             "docs/design/html-translation.md",
		"elements-translation":         "docs/design/elements-translation.md",
		"faq-adapter":                  "docs/design/faq-adapter.md",
		"runtime-selection":            "docs/design/runtime-selection.md",
		"static-content-delivery":      "docs/design/static-content-delivery.md",
		"client-editor-island":         "docs/design/client-editor-island.md",
		"ui-components":                "docs/ui-components.md",
		"cli-kit":                      "docs/design/cli-kit.md",
		"terminal-gallery":             "docs/design/terminal-gallery.md",
		"web-desktop-parity":           "docs/design/web-desktop-parity.md",
	}
	for _, name := range []string{"principles", "invariants", "domain", "scope", "futures", "questions", "autonomy", "incidents", "repo"} {
		paths["intent/"+name] = "docs/intent/" + name + ".md"
	}
	router := readFile(t, filepath.Join(kitRootRel, "docs/intent/README.md"))
	for _, match := range regexp.MustCompile(`\.\./design/([A-Za-z0-9._-]+\.md)(?:#[^)]*)?`).FindAllStringSubmatch(router, -1) {
		rel := filepath.ToSlash(filepath.Join("docs/design", match[1]))
		want := readFile(t, filepath.Join(kitRootRel, rel))
		matchedID := ""
		for id, doc := range docs.extra {
			if doc.Body == want {
				matchedID = id
				break
			}
		}
		if matchedID == "" {
			t.Errorf("root intent router design document %s is not loaded as an exact MCP document", rel)
			continue
		}
		paths[matchedID] = rel
	}
	for id, path := range paths {
		t.Run(id, func(t *testing.T) {
			want := readFile(t, filepath.Join(kitRootRel, path))
			if strings.TrimSpace(want) == "" {
				t.Fatalf("%s is empty", path)
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "get_kit_decision", Arguments: map[string]any{"id": id},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("get_kit_decision(%q): %v", id, result.GetError())
			}
			if len(result.Content) != 1 {
				t.Fatalf("get_kit_decision(%q): got %d content blocks", id, len(result.Content))
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok || content.Text != want {
				t.Errorf("get_kit_decision(%q) text differs from %s", id, path)
			}
			encoded, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var doc docOutput
			if err := json.Unmarshal(encoded, &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Body != want {
				t.Errorf("get_kit_decision(%q) structured body differs from %s", id, path)
			}
			wantMIME := "text/markdown"
			if id == "quality-inventory" || id == "quality-schema" {
				wantMIME = "text/plain"
			}
			uri := "kit://" + id
			if advertised := listed[uri]; advertised == nil {
				t.Errorf("ListResources does not advertise %s", uri)
			} else {
				if advertised.MIMEType != wantMIME || advertised.Size != int64(len(want)) {
					t.Errorf("%s metadata = MIME %q, size %d; want MIME %q, size %d", uri, advertised.MIMEType, advertised.Size, wantMIME, len(want))
				}
				if advertised.Annotations == nil || len(advertised.Annotations.Audience) != 2 {
					t.Errorf("%s must declare its agent and human audiences", uri)
				}
			}
			resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
			if err != nil {
				t.Fatal(err)
			}
			if len(resource.Contents) != 1 || resource.Contents[0].URI != uri || resource.Contents[0].MIMEType != wantMIME || resource.Contents[0].Text != want {
				t.Errorf("ReadResource(%q) differs from %s", uri, path)
			}
		})
	}

	t.Run("candidate-only-cli-origin-policy", func(t *testing.T) {
		want := readFile(t, filepath.Join(kitRootRel, "docs/adr/0052-default-off-cli-invocation-origins.md"))
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_kit_decision", Arguments: map[string]any{"id": "0052"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError || len(result.Content) != 1 {
			t.Fatalf("get_kit_decision(0052): %v", result)
		}
		body, ok := result.Content[0].(*mcp.TextContent)
		if !ok || body.Text != want {
			t.Fatal("candidate-only origin policy differs from its decision record")
		}
	})
	t.Run("profile-drawer-authority-policy", func(t *testing.T) {
		want := readFile(t, filepath.Join(kitRootRel, "docs/adr/0053-explicit-profile-drawer-and-app-owned-navigation.md"))
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_kit_decision", Arguments: map[string]any{"id": "0053"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError || len(result.Content) != 1 {
			t.Fatalf("get_kit_decision(0053): %v", result)
		}
		body, ok := result.Content[0].(*mcp.TextContent)
		if !ok || body.Text != want {
			t.Fatal("profile drawer policy differs from its decision record")
		}
	})

	guidance, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_kit_guidance", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if guidance.IsError || len(guidance.Content) != 1 {
		t.Fatalf("get_kit_guidance: %v", guidance.GetError())
	}
	body := guidance.Content[0].(*mcp.TextContent).Text
	for _, id := range []string{"just-in-time-conversion", "converted-component-standard", "quality-inventory", "quality-schema", "static-content-delivery"} {
		if !strings.Contains(body, "get_kit_decision(\""+id+"\")") {
			t.Errorf("get_kit_guidance does not route to %q", id)
		}
	}
	for _, id := range []string{"intent", "adoption", "adoption-contract", "contribution-loop", "experience-system", "theme-workbench", "app-owned-styleguide", "analytics-journey", "viz-checks", "application-shell", "catalyst-settings-adapter", "design-references", "fathom-maintenance", "conversion-pipeline", "target-evidence", "directory-adapter", "form-adapter", "navigation-adapter", "faq-adapter", "hero-adapter", "menu-adapter", "drawer-adapter", "navbar-adapter", "html-translation", "elements-translation", "runtime-selection", "client-editor-island", "ui-components", "cli-kit", "terminal-gallery", "web-desktop-parity"} {
		if !strings.Contains(body, "get_kit_decision(\""+id+"\")") {
			t.Errorf("get_kit_guidance does not route to %q", id)
		}
	}
	for _, boundary := range []string{"private Go-hosted React/Tiptap client-owned editor rehearsal", "not a public widget API, certified conversion or save backend"} {
		if !strings.Contains(body, boundary) {
			t.Errorf("client editor guidance omits %q", boundary)
		}
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if strings.Contains(tool.Name, "adoption") {
			t.Errorf("adoption contract must remain documentation, not a new MCP execution tool: %s", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Errorf("kit guidance tool %s must advertise local read-only behavior", tool.Name)
		}
		if tool.Name == "get_kit_guidance" && !strings.Contains(tool.Description, "runtime profile") {
			t.Error("get_kit_guidance tool description does not scope stack rules by runtime profile")
		}
	}
}

func TestClientEditorIslandThroughMCPWithoutLicensedCorpus(t *testing.T) {
	root := t.TempDir()
	const body = "# Synthetic client editor island\n\nPrivate rehearsal only; no licensed source corpus.\n"
	path := filepath.Join(root, "docs", "design", "client-editor-island.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv := mcp.NewServer(&mcp.Implementation{Name: "appkit-test", Version: "test"}, nil)
	registerKitTools(srv, loadKitDocs(root))
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "client-editor-doc-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_kit_decision", Arguments: map[string]any{"id": "client-editor-island"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var content *mcp.TextContent
	textOK := false
	if len(result.Content) == 1 {
		content, textOK = result.Content[0].(*mcp.TextContent)
	}
	if result.IsError || len(result.Content) != 1 || !textOK || content.Text != body {
		t.Fatalf("synthetic client editor decision unavailable without licensed corpus: %+v", result)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var doc docOutput
	if err := json.Unmarshal(encoded, &doc); err != nil || doc.Body != body {
		t.Fatalf("synthetic client editor structured body differs: %v, %+v", err, doc)
	}
	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, candidate := range resources.Resources {
		listed = listed || candidate.URI == "kit://client-editor-island"
	}
	if !listed {
		t.Fatal("synthetic client editor resource is not advertised")
	}
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "kit://client-editor-island"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resource.Contents) != 1 || resource.Contents[0].Text != body {
		t.Fatalf("synthetic client editor resource differs: %+v", resource)
	}
}

func TestMissingKitRootDoesNotInventIntent(t *testing.T) {
	docs := loadKitDocs(t.TempDir())
	for _, id := range []string{"intent", "adoption-contract", "analytics-journey", "viz-checks", "application-shell", "catalyst-settings-adapter", "fathom-maintenance", "web-desktop-parity", "terminal-gallery", "directory-adapter", "form-adapter", "navigation-adapter", "client-editor-island", "static-content-delivery"} {
		if _, ok := docs.findDecision(id); ok {
			t.Errorf("missing %s must not resolve to a fabricated document", id)
		}
	}
	if !strings.Contains(docs.guidance(), "guidance unavailable") {
		t.Error("missing kit root must still explain guidance availability")
	}
}
