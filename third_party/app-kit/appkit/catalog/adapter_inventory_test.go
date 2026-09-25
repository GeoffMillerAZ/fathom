package catalog

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type adapterCatalogExpectation struct {
	id                string
	source            Source
	usage             []string
	dependencies      []string
	provenance        Provenance
	privateBackendGap string
}

func TestPrivateAdapterInventoryIsCLIToolingNotCertifiedOutput(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}

	wantProof := []string{"go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"}
	adapters := []adapterCatalogExpectation{
		{
			id: "reference.pricing-adapter",
			source: Source{
				Path:    "internal/designadapt/pricing_delivery.go",
				Symbols: []string{"WritePricingDelivery", "CheckPricingDelivery"},
			},
			usage: []string{
				"appkit data assets-adapt-pricing -root PRIVATE_DIRECTORY",
				"-out NEW_PRIVATE_DIRECTORY",
				"-check PRIVATE_DIRECTORY",
				"six immutable files",
				"Compile a COPY in an app-owned private module",
				"NewPricing",
				"NewPricingEnhanced",
				"Use the CLI; do not import this internal package into an app",
				"tooling, not certified web-profile compatibility",
			},
			provenance: Provenance{
				Source:   "first-party",
				License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
				Audience: "metadata available without private archives; execution requires independently verified original access",
			},
			privateBackendGap: "billing backend",
		},
		{
			id: "reference.directory-adapter",
			source: Source{
				Path: "internal/designadapt/directory_delivery.go",
				Symbols: []string{
					"WriteDirectoryDelivery", "CheckDirectoryDelivery",
					"WriteDirectoryEnhancedDelivery", "CheckDirectoryEnhancedDelivery",
				},
			},
			usage: []string{
				"appkit data assets-adapt-directory -root PRIVATE_DIRECTORY",
				"-out NEW_PRIVATE_DIRECTORY",
				"-check PRIVATE_DIRECTORY",
				"-enhanced",
				"five source-styled files",
				"eight-file base-plus-companion export",
				"Compile a COPY in an app-owned private module",
				"NewDirectoryNavigation",
				"NewDirectoryEnhanced",
				"Use the CLI, not imports of this internal package",
				"tooling, not certified web-profile compatibility",
			},
			provenance: Provenance{
				Source:   "first-party",
				License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
				Audience: "metadata available without private archives; execution requires independently verified original access",
			},
			privateBackendGap: "account backend",
		},
		{
			id: "reference.form-adapter",
			source: Source{
				Path:    "internal/designadapt/form_delivery.go",
				Symbols: []string{"WriteFormDelivery", "CheckFormDelivery"},
			},
			usage: []string{
				"get_kit_decision(\"form-adapter\")",
				"appkit data assets-adapt-form -root PRIVATE_DIRECTORY",
				"-out NEW_PRIVATE_DIRECTORY",
				"-check PRIVATE_DIRECTORY",
				"five immutable files",
				"Compile a COPY in an app-owned private module",
				"NewEmailField",
				"constructor errors before response output",
				"scanning BOTH form.go and form.templ",
				"Use the CLI; do not import this internal package into an app",
				"tooling, not certified web-profile compatibility",
			},
			provenance: Provenance{
				Source:   "first-party",
				License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
				Audience: "metadata available without private archives; execution requires independently verified original access",
			},
			privateBackendGap: "validation/auth/account backend",
		},
		{
			id: "reference.navigation-adapter",
			source: Source{
				Path:    "internal/designadapt/navigation_delivery.go",
				Symbols: []string{"WriteNavigationDelivery", "CheckNavigationDelivery"},
			},
			usage: []string{
				"get_kit_decision(\"navigation-adapter\")",
				"appkit data assets-adapt-navigation -root PRIVATE_DIRECTORY",
				"-out NEW_PRIVATE_DIRECTORY",
				"-check PRIVATE_DIRECTORY",
				"five immutable files",
				"Compile a COPY in an app-owned private module",
				"NewPageNavigationReference",
				"NewPageNavigationGET",
				"NewPageNavigationLinks",
				"scan BOTH navigation.go and navigation.templ",
				"maps stable submitted keys to allowlisted local paths",
				"owns authorization",
				"Use the CLI; do not import this internal package into an app",
				"tooling, not certified web-profile compatibility",
			},
			provenance: Provenance{
				Source:   "first-party",
				License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
				Audience: "metadata available without private archives; execution requires independently verified original access",
			},
			privateBackendGap: "router/auth backend",
		},
	}
	adapters = append(adapters, adapterCatalogExpectation{
		id:                "reference.faq-adapter",
		source:            Source{Path: "internal/designadapt/faq_delivery.go", Symbols: []string{"WriteFAQDelivery", "CheckFAQDelivery"}},
		usage:             []string{"get_kit_decision(\"faq-adapter\")", "appkit data assets-adapt-faq -root PRIVATE_DIRECTORY", "-out NEW_PRIVATE_DIRECTORY", "-check PRIVATE_DIRECTORY", "six immutable files", "Compile a COPY in an app-owned private module", "NewFAQReference", "NewFAQDetails", "no JavaScript", "distinct ID prefixes", "Use the CLI; do not import this internal package into an app", "tooling, not certified web-profile compatibility"},
		provenance:        Provenance{Source: "first-party", License: "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", Audience: "metadata available without private archives; execution requires independently verified original access"},
		privateBackendGap: "help/account backend",
	})
	adapters = append(adapters, adapterCatalogExpectation{
		id:     "reference.hero-adapter",
		source: Source{Path: "internal/designadapt/hero_delivery.go", Symbols: []string{"WriteHeroDelivery", "CheckHeroDelivery"}},
		usage: []string{
			"get_kit_decision(\"hero-adapter\")",
			"appkit data assets-adapt-hero -root PRIVATE_DIRECTORY",
			"-out NEW_PRIVATE_DIRECTORY",
			"-check PRIVATE_DIRECTORY",
			"six immutable files",
			"hero.templ, hero.go, hero.css, defaults.json, USAGE.md and artifact.json",
			"Compile a COPY in an app-owned private module with no app-kit runtime dependency",
			"all six deliberately blank URL fields",
			"constructor errors before response output",
			"NewHero preserves the original source composition",
			"NewHeroEnhanced explicitly opts into hero.css loaded after source CSS",
			"decorative images require empty alt",
			"Source asset URLs are provenance only, never downloaded or bundled",
			"Use the CLI; do not import this internal package into an app",
			"tooling, not certified web-profile compatibility",
		},
		provenance:        Provenance{Source: "first-party", License: "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", Audience: "metadata available without private archives; execution requires independently verified original access"},
		privateBackendGap: "router/backend or asset delivery",
	})
	adapters = append(adapters, adapterCatalogExpectation{
		id:     "reference.menu-adapter",
		source: Source{Path: "internal/designadapt/menu_delivery.go", Symbols: []string{"WriteMenuDelivery", "CheckMenuDelivery"}},
		usage: []string{
			"get_kit_decision(\"menu-adapter\")",
			"appkit data assets-adapt-menu -root PRIVATE_DIRECTORY",
			"-out NEW_PRIVATE_DIRECTORY",
			"-check PRIVATE_DIRECTORY",
			"six immutable files",
			"menu.templ, menu.go, menu.css, defaults.json, USAGE.md and artifact.json",
			"-repair to explicitly select the eight-file repair variant",
			"Base and repair exports reject cross-checks",
			"dispose before removal/remount",
			"Compile a COPY in an app-owned private module",
			"MenuReference preserves the constant original",
			"NewMenu binds explicit app-owned links",
			"optional local POST action with a required CSRF name/token",
			"NewMenuEnhanced explicitly opts into menu.css loaded after source CSS",
			"pinned Elements 1.0.22 delivery",
			"Use the CLI; do not import this internal package into an app",
			"tooling, not certified web-profile compatibility",
		},
		dependencies:      []string{"host-provided @tailwindplus/elements 1.0.22"},
		provenance:        Provenance{Source: "first-party", License: "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", Audience: "metadata available without private archives; execution requires independently verified original access"},
		privateBackendGap: "router/auth backend or CSRF issuer",
	})
	actualAdapterCount := 0
	adapters = append(adapters, adapterCatalogExpectation{
		id:     "reference.drawer-adapter",
		source: Source{Path: "internal/designadapt/drawer_delivery.go", Symbols: []string{"WriteDrawerDelivery", "CheckDrawerDelivery"}},
		usage: []string{
			"get_kit_decision(\"drawer-adapter\")", "appkit data assets-adapt-drawer -root PRIVATE_DIRECTORY",
			"-out NEW_PRIVATE_DIRECTORY", "-check PRIVATE_DIRECTORY", "five immutable files",
			"drawer.templ, drawer.go, defaults.json, USAGE.md and artifact.json",
			"Compile a COPY in an app-owned private module", "DrawerReference preserves the constant original",
			"optional GET navigation", "no message, call or clipboard action is inferred",
			"document-unique IDPrefix and ImageURL", "Empty actions produce a read-only drawer",
			"constructor errors before response output", "pinned Elements 1.0.22 delivery",
			"Use the CLI; do not import this internal package into an app", "tooling, not certified web-profile compatibility",
		},
		dependencies:      []string{"host-provided @tailwindplus/elements 1.0.22"},
		provenance:        Provenance{Source: "first-party", License: "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", Audience: "metadata available without private archives; execution requires independently verified original access"},
		privateBackendGap: "account/presence backend or messaging/call/clipboard behavior",
	})
	for _, entry := range c.Entries {
		if strings.HasPrefix(entry.ID, "reference.") && strings.HasSuffix(entry.ID, "-adapter") {
			actualAdapterCount++
		}
	}
	adapters = append(adapters, adapterCatalogExpectation{
		id:                "reference.navbar-adapter",
		source:            Source{Path: "internal/designadapt/navbar_delivery.go", Symbols: []string{"WriteNavbarDelivery", "CheckNavbarDelivery"}},
		usage:             []string{"get_kit_decision(\"navbar-adapter\")", "appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY", "-out NEW_PRIVATE_DIRECTORY", "-check PRIVATE_DIRECTORY", "five immutable files", "navbar.templ, navbar.go, defaults.json, USAGE.md and artifact.json", "Compile a COPY in an app-owned private module", "NavbarReference preserves the constant original", "one Links model for desktop/mobile", "exact CurrentKey; empty means none current", "no sign-out mutation or backend is inferred", "constructor errors before response output", "pinned Elements 1.0.22 delivery", "Use the CLI; do not import this internal package into an app", "tooling, not certified web-profile compatibility"},
		dependencies:      []string{"host-provided @tailwindplus/elements 1.0.22"},
		provenance:        Provenance{Source: "first-party", License: "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", Audience: "metadata available without private archives; execution requires independently verified original access"},
		privateBackendGap: "account backend or notification provider",
	})
	// ADR-0064 adds one first-party settings recipe, not another private adapter.
	if len(c.Entries) != 35 || actualAdapterCount != 9 || len(adapters) != 9 {
		t.Fatalf("catalog/private/expected counts = %d/%d/%d, want 35/9/9 (plus a separate Elements translator)", len(c.Entries), actualAdapterCount, len(adapters))
	}

	for _, want := range adapters {
		t.Run(want.id, func(t *testing.T) {
			entry, ok := c.Get(want.id)
			if !ok {
				t.Fatalf("missing first-party adapter metadata %q", want.id)
			}
			if entry.Kind != "reference" || entry.Maturity != "experimental" ||
				!slices.Equal(entry.Profiles, []string{"cli"}) || len(entry.Layers) != 0 ||
				entry.Preview != "" || len(entry.Consumers) != 0 {
				t.Fatalf("adapter overstates renderer fit, maturity, preview, or adoption: %+v", entry)
			}
			if !reflect.DeepEqual(entry.Source, want.source) {
				t.Fatalf("adapter source anchors differ\ngot:  %+v\nwant: %+v", entry.Source, want.source)
			}
			for _, phrase := range want.usage {
				if !strings.Contains(entry.Usage, phrase) {
					t.Errorf("usage omits %q", phrase)
				}
			}
			dependencies := strings.Join(entry.Dependencies, " ")
			for _, phrase := range want.dependencies {
				if !strings.Contains(dependencies, phrase) {
					t.Errorf("dependencies omit %q", phrase)
				}
			}
			if !slices.Equal(entry.Proof, wantProof) {
				t.Errorf("proof entrypoints differ\ngot:  %q\nwant: %q", entry.Proof, wantProof)
			}
			if !reflect.DeepEqual(entry.Provenance, want.provenance) {
				t.Errorf("private provenance boundary differs\ngot:  %+v\nwant: %+v", entry.Provenance, want.provenance)
			}

			gaps := strings.Join(entry.Gaps, " ")
			for _, phrase := range []string{
				"verification remains not-verified",
				"no certified target or real consumer adoption",
				"No source CSS/fonts, installed dependencies",
				want.privateBackendGap,
				"original",
				"independently retrievable",
			} {
				if !strings.Contains(gaps, phrase) {
					t.Errorf("evidence gap disclosure omits %q", phrase)
				}
			}
			for _, dependency := range entry.Dependencies {
				if strings.Contains(strings.ToLower(dependency), "app-kit") {
					t.Errorf("metadata introduces an app-kit runtime dependency: %q", dependency)
				}
			}
		})
	}
}

func TestNavigationAdapterExactMetadata(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get("reference.navigation-adapter")
	want := Entry{
		ID:       "reference.navigation-adapter",
		Name:     "Private page-navigation adapter CLI",
		Kind:     "reference",
		Summary:  "Generate and check private P06 page-navigation source through a first-party CLI; Reference, GET and Links have scoped synthetic evidence, not certified web support.",
		Tasks:    []string{"private adapter", "page navigation", "responsive navigation", "current page", "Go templ source generation", "licensed design delivery", "native GET navigation", "static links", "source replay", "export check"},
		Layers:   []string{},
		Profiles: []string{"cli"},
		Dependencies: []string{
			"verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/navigation/tabs/01-tabs-with-underline.html",
			"app-owned Go module and pinned templ v0.3.1020",
			"app-owned Tailwind 4 build scanning both navigation.go and navigation.templ plus deliberate font assets",
			"macOS/Linux for secure delivery checks",
		},
		Source: Source{Path: "internal/designadapt/navigation_delivery.go", Symbols: []string{"WriteNavigationDelivery", "CheckNavigationDelivery"}},
		Usage:  "Read get_kit_decision(\"navigation-adapter\"). Run appkit data assets-adapt-navigation -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export five immutable files. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module. NewPageNavigationReference preserves the unbound source specimen; NewPageNavigationGET adds native GET selection; NewPageNavigationLinks adds wrapping static links. Handle constructor errors before response output and scan BOTH navigation.go and navigation.templ. The app maps stable submitted keys to allowlisted local paths and owns authorization. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
		States: []string{"default private JSON", "source-style unbound Reference", "native GET selection", "wrapping static Links", "new private five-file export", "matching replay", "source or delivery refusal"},
		AppOwns: []string{
			"licensed original access and private output retention",
			"module/tool/CSS/font pins and checksums",
			"copy outside immutable input before compiling",
			"local paths and truthful current key",
			"submitted-key cardinality, allowlist mapping and redirects",
			"routing and authorization",
			"consumer task and visual acceptance",
		},
		Maturity: "experimental",
		Proof:    []string{"go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"},
		Gaps: []string{
			"verification remains not-verified; no certified target or real consumer adoption is claimed.",
			"Default tests skip paid-source probes; docs/assessments/2026-09-14-navigation-typed-adapter.md records explicit scoped evidence.",
			"No source CSS/fonts, installed dependencies, router/auth backend or automatic submitted-key-to-path mapping is supplied.",
			"The original remains independently retrievable; Reference retains its unbound select and source sizing, while GET/Links usability changes are adaptations rather than source fidelity.",
		},
		Provenance: Provenance{
			Source:   "first-party",
			License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
			Audience: "metadata available without private archives; execution requires independently verified original access",
		},
		Consumers: []string{},
		Preview:   "",
	}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("navigation adapter metadata differs\ngot:  %+v\nwant: %+v", got, want)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<nav", "<select", "<option", "<a ", "class=", "sourceExpected", "sourceDefaults"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("navigation metadata contains source/default body marker %q", forbidden)
		}
	}
}

func TestFormAdapterExactMetadata(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get("reference.form-adapter")
	want := Entry{
		ID:       "reference.form-adapter",
		Name:     "Private form/email-field adapter CLI",
		Kind:     "reference",
		Summary:  "Generate and check private P05 email-field source through a first-party CLI; error and authored neutral states remain unverified candidates.",
		Tasks:    []string{"private adapter", "form", "email field", "validation error", "Go templ source generation", "licensed design delivery", "native form recovery", "source replay", "export check"},
		Layers:   []string{},
		Profiles: []string{"cli"},
		Dependencies: []string{
			"verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/forms/input-groups/03-input-with-validation-error.html",
			"app-owned Go module and pinned templ v0.3.1020",
			"app-owned Tailwind 4 build scanning both form.go and form.templ plus deliberate font assets",
			"macOS/Linux for secure delivery checks",
		},
		Source: Source{Path: "internal/designadapt/form_delivery.go", Symbols: []string{"WriteFormDelivery", "CheckFormDelivery"}},
		Usage:  "Read get_kit_decision(\"form-adapter\"). Run appkit data assets-adapt-form -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export five immutable files. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module; call NewEmailField and handle constructor errors before response output. Supply Tailwind CSS scanning BOTH form.go and form.templ. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
		States: []string{"default private JSON", "source error or authored neutral field", "new private five-file export", "matching replay", "source or delivery refusal"},
		AppOwns: []string{
			"licensed original access and private output retention",
			"module/tool/CSS/font pins and checksums",
			"copy outside immutable input before compiling",
			"distinct field IDs and reserved error IDs",
			"form action and validation",
			"authorization and CSRF",
			"save/retry behavior and success truth",
			"consumer task and visual acceptance",
		},
		Maturity: "experimental",
		Proof:    []string{"go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"},
		Gaps: []string{
			"verification remains not-verified; no certified target or real consumer adoption is claimed.",
			"Default tests skip paid-source probes; docs/assessments/2026-09-14-form-typed-adapter.md records explicit scoped evidence.",
			"No source CSS/fonts, installed dependencies, validation/auth/account backend, persistence or email delivery service is supplied.",
			"The original remains independently retrievable; the clean state is an authored adaptation and source sizing limitations are not silently repaired.",
		},
		Provenance: Provenance{
			Source:   "first-party",
			License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
			Audience: "metadata available without private archives; execution requires independently verified original access",
		},
		Consumers: []string{},
		Preview:   "",
	}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("form adapter metadata differs\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestPrivateAdapterSearchIsBoundedAndRefusesUnsupportedPromotion(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}

	adapters := []struct {
		id, query string
	}{
		{"reference.pricing-adapter", "pricing private adapter"},
		{"reference.directory-adapter", "directory table private adapter"},
		{"reference.form-adapter", "form email field private adapter"},
		{"reference.navigation-adapter", "page navigation private adapter"},
		{"reference.faq-adapter", "FAQ help private adapter"},
		{"reference.hero-adapter", "split hero private adapter"},
		{"reference.menu-adapter", "dropdown menu private adapter"},
		{"reference.drawer-adapter", "profile drawer private adapter"},
		{"reference.navbar-adapter", "responsive navbar private adapter"},
	}
	if len(adapters) != 9 {
		t.Fatalf("private adapter search count = %d, want 9", len(adapters))
	}
	for _, adapter := range adapters {
		t.Run(adapter.id, func(t *testing.T) {
			for _, query := range []Query{
				{Text: adapter.query},
				{Text: adapter.query, Profile: "cli"},
			} {
				page, err := c.Search(query)
				if err != nil {
					t.Fatal(err)
				}
				if page.Revision != c.Revision || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != adapter.id {
					t.Fatalf("adapter is not exactly discoverable for %+v: %+v", query, page)
				}
				body, err := json.Marshal(page)
				if err != nil {
					t.Fatal(err)
				}
				if len(body) > 4<<10 {
					t.Fatalf("adapter summary exceeds bounded discovery budget: %d bytes", len(body))
				}
				for _, forbidden := range []string{
					`"source"`, `"usage"`, `"dependencies"`, `"proof"`, `"gaps"`, `"provenance"`,
					"WritePricingDelivery", "WriteDirectoryDelivery", "WriteFormDelivery", "WriteNavigationDelivery", "frozen original",
				} {
					if strings.Contains(string(body), forbidden) {
						t.Errorf("bounded discovery exposed exact contract/source material %q", forbidden)
					}
				}
			}

			incompatible := []Query{
				{Text: adapter.query, Profile: "go-web"},
				{Text: adapter.query, Profile: "static-web"},
				{Text: adapter.query, Profile: "react-web"},
				{Text: adapter.query, Maturity: "available"},
				{Text: adapter.query, Maturity: "proven"},
				{Text: adapter.query, Kind: "recipe"},
				{Text: adapter.query, Layer: "L4"},
			}
			for _, query := range incompatible {
				page, err := c.Search(query)
				if err != nil {
					t.Fatal(err)
				}
				if page.Revision != c.Revision || page.Total != 0 || len(page.Items) != 0 || page.NextCursor != "" {
					t.Errorf("unsupported fit promoted for %+v: %+v", query, page)
				}
			}
		})
	}
}
