// First-party capabilities only. Commercial source stays in optional private corpora.
// Tasks are positive, coherent capability phrases (ADR-0062). Keep related facets
// together; never merge user workflows with unrelated source-delivery obligations.
{
  "schema_version": 1,
  "entries": [
    {
      "id": "reference.pricing-adapter",
      "name": "Private pricing adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P02 pricing source through a first-party CLI; source-styled/enhanced outputs remain unverified candidates.",
      "tasks": ["private adapter", "pricing", "Go templ source generation", "licensed design delivery", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675, html/sections/pricing/05-single-price-with-details.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 build scanning generated templates and deliberate font assets", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/pricing_delivery.go", "symbols": ["WritePricingDelivery", "CheckPricingDelivery"]},
      "usage": "Read get_kit_decision(\"pricing-adapter\"). Run appkit data assets-adapt-pricing -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export six immutable files. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module; call NewPricing or explicitly NewPricingEnhanced, binding real destinations and app-formatted data. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON", "source-styled or explicit enhanced constructor", "new private export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/CSS/font pins and checksums", "copy outside immutable input before compiling", "destinations and data", "billing/auth/backend behavior if needed", "consumer task and visual acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; docs/assessments/2026-09-14-pricing-private-delivery.md records explicit scoped evidence.", "No source CSS/fonts, installed dependencies, billing backend or catalog conversion write is supplied.", "The original stays independently retrievable; source-styled target-size and long-copy failures are not silently repaired."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.directory-adapter",
      "name": "Private directory/table adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P04 responsive directory/table source through a first-party CLI; the enhanced companion is opt-in, not a certified target.",
      "tasks": ["private adapter", "directory", "responsive table", "Go templ source generation", "licensed design delivery", "native Add Edit navigation", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/lists/tables/07-with-stacked-columns-on-mobile.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 build scanning both selected templates and deliberate font assets", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/directory_delivery.go", "symbols": ["WriteDirectoryDelivery", "CheckDirectoryDelivery", "WriteDirectoryEnhancedDelivery", "CheckDirectoryEnhancedDelivery"]},
      "usage": "Read get_kit_decision(\"directory-adapter\"). Run appkit data assets-adapt-directory -root PRIVATE_DIRECTORY for private JSON. Add -out NEW_PRIVATE_DIRECTORY for five source-styled files; add -enhanced for the distinct eight-file base-plus-companion export. Check with -check PRIVATE_DIRECTORY and the same -enhanced choice; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module; bind AddURL and every EditURL, then call NewDirectoryNavigation or explicitly NewDirectoryEnhanced. Use the CLI, not imports of this internal package. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default or enhanced private JSON", "source-styled five-file delivery", "enhanced eight-file delivery", "matching replay", "wrong variant or drift refusal", "empty/partial/long rows supplied by app"],
      "app_owns": ["licensed original access and private output retention", "module/tool/CSS/font pins and checksums", "copy outside immutable input before compiling", "labels and bounded 0..100 rows", "Add/Edit destinations and authorization", "empty/partial explanation, persistence and consumer acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; docs/assessments/2026-09-14-directory-private-delivery.md records cached standalone-module proof, not cold-cache or filesystem/network isolation.", "No source CSS/fonts, installed dependencies, account backend, sorting/filtering/pagination or export engine is supplied.", "The original remains independently retrievable; docs/assessments/2026-09-14-directory-enhanced-adapter.md separates original target-size failures from scoped enhanced browser evidence."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.form-adapter",
      "name": "Private form/email-field adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P05 email-field source through a first-party CLI; error and authored neutral states remain unverified candidates.",
      "tasks": ["private adapter", "form", "email field", "validation error", "Go templ source generation", "licensed design delivery", "native form recovery", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/forms/input-groups/03-input-with-validation-error.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 build scanning both form.go and form.templ plus deliberate font assets", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/form_delivery.go", "symbols": ["WriteFormDelivery", "CheckFormDelivery"]},
      "usage": "Read get_kit_decision(\"form-adapter\"). Run appkit data assets-adapt-form -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export five immutable files. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module; call NewEmailField and handle constructor errors before response output. Supply Tailwind CSS scanning BOTH form.go and form.templ. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON", "source error or authored neutral field", "new private five-file export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/CSS/font pins and checksums", "copy outside immutable input before compiling", "distinct field IDs and reserved error IDs", "form action and validation", "authorization and CSRF", "save/retry behavior and success truth", "consumer task and visual acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; docs/assessments/2026-09-14-form-typed-adapter.md records explicit scoped evidence.", "No source CSS/fonts, installed dependencies, validation/auth/account backend, persistence or email delivery service is supplied.", "The original remains independently retrievable; the clean state is an authored adaptation and source sizing limitations are not silently repaired."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.navigation-adapter",
      "name": "Private page-navigation adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P06 page-navigation source through a first-party CLI; Reference, GET and Links have scoped synthetic evidence, not certified web support.",
      "tasks": ["private adapter", "page navigation", "responsive navigation", "current page", "Go templ source generation", "licensed design delivery", "native GET navigation", "static links", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/navigation/tabs/01-tabs-with-underline.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 build scanning both navigation.go and navigation.templ plus deliberate font assets", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/navigation_delivery.go", "symbols": ["WriteNavigationDelivery", "CheckNavigationDelivery"]},
      "usage": "Read get_kit_decision(\"navigation-adapter\"). Run appkit data assets-adapt-navigation -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export five immutable files. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module. NewPageNavigationReference preserves the unbound source specimen; NewPageNavigationGET adds native GET selection; NewPageNavigationLinks adds wrapping static links. Handle constructor errors before response output and scan BOTH navigation.go and navigation.templ. The app maps stable submitted keys to allowlisted local paths and owns authorization. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON", "source-style unbound Reference", "native GET selection", "wrapping static Links", "new private five-file export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/CSS/font pins and checksums", "copy outside immutable input before compiling", "local paths and truthful current key", "submitted-key cardinality, allowlist mapping and redirects", "routing and authorization", "consumer task and visual acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; docs/assessments/2026-09-14-navigation-typed-adapter.md records explicit scoped evidence.", "No source CSS/fonts, installed dependencies, router/auth backend or automatic submitted-key-to-path mapping is supplied.", "The original remains independently retrievable; Reference retains its unbound select and source sizing, while GET/Links usability changes are adaptations rather than source fidelity."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.faq-adapter",
      "name": "Private FAQ adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P03 typed FAQ source; faithful Elements and JavaScript-free native disclosures are explicit choices, not certified web support.",
      "tasks": ["private adapter", "FAQ help private adapter", "typed disclosures", "native details", "repeated instances", "Go templ source generation", "licensed design delivery", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675, html/sections/faq-sections/02-centered-accordion.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 scanning faq.templ and faq_model.go, source fonts, and native companion faq.css", "Reference alone needs host-provided @tailwindplus/elements 1.0.22 matching artifact runtime hashes; Details needs no JavaScript", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/faq_delivery.go", "symbols": ["WriteFAQDelivery", "CheckFAQDelivery"]},
      "usage": "Read get_kit_decision(\"faq-adapter\"). Run appkit data assets-adapt-faq -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export six immutable files. Use -check PRIVATE_DIRECTORY for exact current-source replay; out/check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module. NewFAQReference keeps original Elements behavior; NewFAQDetails explicitly uses native disclosures with faq.css and no JavaScript. Handle constructor errors before response output; use unique item keys and distinct ID prefixes across instances. The app owns content, runtime/CSS/font delivery and state policy. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON", "source-styled Elements Reference", "JavaScript-free native Details", "independent initial open states", "new private six-file export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/CSS/font pins and checksums", "copy outside immutable input before compiling", "plain content and unique document-wide prefixes", "Reference runtime delivery and CSP", "empty states and persisted state policy", "consumer task and visual acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; actual browser and standalone evidence must be read separately.", "No source CSS/fonts, installed dependencies, help/account backend or automatic state persistence is supplied.", "The original remains independently retrievable; Reference preserves source limitations, while Details is an explicit usability/semantic adaptation. Full accessibility, cross-browser and Datastar lifecycle remain unproven."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.hero-adapter",
      "name": "Private split-hero adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P01 split-hero source through a first-party CLI; source, compiler, DOM and replay evidence do not certify browser or consumer fit.",
      "tasks": ["private adapter", "split hero", "Go templ source generation", "licensed design delivery", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675, html/sections/heroes/09-split-with-image.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 build scanning hero.templ and hero.go, source CSS, deliberate fonts, and optional hero.css", "app-owned explicit routes and image assets; source asset URLs are provenance only and are not bundled", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/hero_delivery.go", "symbols": ["WriteHeroDelivery", "CheckHeroDelivery"]},
      "usage": "Read get_kit_decision(\"hero-adapter\"). Run appkit data assets-adapt-hero -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export six immutable files: hero.templ, hero.go, hero.css, defaults.json, USAGE.md and artifact.json. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module with no app-kit runtime dependency. Supply all six deliberately blank URL fields and handle constructor errors before response output. NewHero preserves the original source composition; NewHeroEnhanced explicitly opts into hero.css loaded after source CSS. Set ImageDecorative explicitly: decorative images require empty alt; otherwise supply plain alt text. Source asset URLs are provenance only, never downloaded or bundled. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON with six blank URLs", "source-faithful NewHero", "opt-in NewHeroEnhanced with companion CSS", "explicit decorative or described image", "new private six-file export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/Tailwind CSS/font pins and checksums", "copy outside immutable input before compiling", "all six routes and image URLs plus asset rights and delivery", "explicit decorative-image and alt decision", "consumer task, browser, visual and accessibility acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Frozen source identity, generated compiler and DOM comparison, private replay, and scoped browser and standalone consumer proof are available in the guide; they do not establish full target certification.", "No source CSS/fonts, installed dependencies, downloaded image assets, router/backend or asset delivery is supplied.", "The original remains independently retrievable; NewHero is source-faithful and NewHeroEnhanced is an opt-in CSS adaptation. Consumer task, browser, visual and accessibility certification remain unproven."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.menu-adapter",
      "name": "Private dropdown-menu adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P07 dropdown-menu source through a first-party CLI; constant reference and typed Elements variants remain unverified candidates.",
      "tasks": ["private adapter", "dropdown menu", "Go templ source generation", "licensed design delivery", "native POST and CSRF", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/elements/dropdowns/01-simple.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 scanning menu.templ and menu.go, source CSS/fonts, and optional menu.css", "host-provided @tailwindplus/elements 1.0.22 matching artifact runtime hashes", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/menu_delivery.go", "symbols": ["WriteMenuDelivery", "CheckMenuDelivery"]},
      "usage": "Read get_kit_decision(\"menu-adapter\"). Run appkit data assets-adapt-menu -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export six immutable files: menu.templ, menu.go, menu.css, defaults.json, USAGE.md and artifact.json. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Add -repair to explicitly select the eight-file repair variant, including menu_repair.mjs and menu_repair.css with nested unchanged base identity. Base and repair exports reject cross-checks. Compile a COPY in an app-owned private module. MenuReference preserves the constant original; NewMenu binds explicit app-owned links and an optional local POST action with a required CSRF name/token; NewMenuEnhanced explicitly opts into menu.css loaded after source CSS. For -repair load menu_repair.css last, explicitly mountMenuRepair after Elements is defined, and dispose before removal/remount; reviewed gap/edge defaults are 8/16px. Handle constructor errors before response output. The app owns action authorization, CSRF issuance, destinations and pinned Elements 1.0.22 delivery. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON with blank destinations and CSRF fields", "constant MenuReference", "typed NewMenu with links and optional disabled POST", "opt-in NewMenuEnhanced with companion CSS", "new private six-file export", "explicit eight-file repair variant with app-owned mount/dispose", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/Tailwind CSS/font/runtime pins and checksums", "copy outside immutable input before compiling", "link destinations and local POST action", "authorization and CSRF issuance/rotation", "menu state policy, consumer task, browser, visual and accessibility acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; source/compiler/DOM, private replay, scoped browser actions and two cold standalone render builds must be read separately in the guide. Unchanged v1 fails actual 200% focus/containment; the explicit repair has 314 normal and 210 zoom/resize/lifecycle checks, not full target certification.", "No source CSS/fonts, installed dependencies, Elements runtime, router/auth backend or CSRF issuer is supplied.", "The original remains independently retrievable as MenuReference; NewMenu is an explicit app-action adaptation and NewMenuEnhanced adds opt-in CSS. Repair mounting, disposal and runtime-failure fallback remain app-owned."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.drawer-adapter",
      "name": "Private profile-drawer adapter CLI",
      "kind": "reference",
      "summary": "Generate and check private P08 profile-drawer source with typed identity, details and explicit navigation; nested Elements behavior is experimental, not certified web support.",
      "tasks": ["private adapter", "profile drawer", "nested menu", "read-only identity", "Go templ source generation", "licensed design delivery", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/overlays/drawers/07-user-profile-example.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 scanning drawer.templ, source CSS/fonts and explicit image assets", "host-provided @tailwindplus/elements 1.0.22 matching artifact runtime hashes", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/drawer_delivery.go", "symbols": ["WriteDrawerDelivery", "CheckDrawerDelivery"]},
      "usage": "Read get_kit_decision(\"drawer-adapter\"). Run appkit data assets-adapt-drawer -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY to export five immutable files: drawer.templ, drawer.go, defaults.json, USAGE.md and artifact.json. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module. DrawerReference preserves the constant original. NewDrawer binds explicit identity, image semantics, details and optional GET navigation; no message, call or clipboard action is inferred. Supply a document-unique IDPrefix and ImageURL; defaults leave both blank, actions absent and online state unclaimed. Empty actions produce a read-only drawer. Handle constructor errors before response output. The app owns truthful data, image rights, destinations, runtime failure and pinned Elements 1.0.22 delivery. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["default private JSON with unbound instance/image", "constant DrawerReference", "typed NewDrawer with app-owned navigation", "read-only without actions", "optional truthful online label", "new private five-file export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private output retention", "module/tool/Tailwind CSS/font/runtime pins and checksums", "copy outside immutable input before compiling", "document-wide ID namespace and image rights/alt semantics", "truthful identity, presence and details", "navigation destinations, authorization and runtime-failure fallback", "consumer task, browser, visual and accessibility acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; read source/compiler/DOM, private replay and browser observations separately in the guide.", "No source CSS/fonts, installed dependencies, image download, account/presence backend or messaging/call/clipboard behavior is supplied.", "The original remains independently retrievable. Original-photo fidelity, full accessibility/zoom, runtime-failure, dynamic lifecycle, performance and standalone consumer acceptance remain open."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.navbar-adapter",
      "name": "Private responsive-navbar adapter CLI",
      "kind": "reference",
      "summary": "Generate private P09 navigation with one typed desktop/mobile route model and optional account destinations; pinned Elements behavior remains experimental.",
      "tasks": ["private adapter", "responsive navbar", "mobile disclosure", "account navigation", "Go templ source generation", "licensed design delivery", "source replay", "export check"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified original application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8, html/navigation/navbars/04-simple-with-menu-button-on-left.html", "app-owned Go module and pinned templ v0.3.1020", "app-owned Tailwind 4 scanning navbar.templ, fonts and explicit brand/avatar images", "host-provided @tailwindplus/elements 1.0.22 matching artifact runtime hashes", "macOS/Linux for secure delivery checks"],
      "source": {"path": "internal/designadapt/navbar_delivery.go", "symbols": ["WriteNavbarDelivery", "CheckNavbarDelivery"]},
      "usage": "Read get_kit_decision(\"navbar-adapter\"). Run appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY for private JSON, or add -out NEW_PRIVATE_DIRECTORY for five immutable files: navbar.templ, navbar.go, defaults.json, USAGE.md and artifact.json. Use -check PRIVATE_DIRECTORY for exact current-source replay; -out and -check are mutually exclusive and silent on success. Exit 1 means drift, 2 invalid request or could-not-check. Compile a COPY in an app-owned private module. NavbarReference preserves the constant original. NewNavbar uses one Links model for desktop/mobile and an exact CurrentKey; empty means none current. Bind document-unique IDPrefix, landmark/mobile labels, brand imagery and routes. Optional Notifications and Account expose GET destinations only; no sign-out mutation or backend is inferred. Defaults remain unbound and optional controls absent. Handle constructor errors before response output. The app owns truthful route state, approved images, destinations, fallback and pinned Elements 1.0.22 delivery. Use the CLI; do not import this internal package into an app. Metadata describes tooling, not certified web-profile compatibility.",
      "states": ["unbound private JSON", "constant NavbarReference", "typed desktop/mobile shared route model", "explicit current or no current link", "optional account and notification navigation", "new private five-file export", "matching replay", "source or delivery refusal"],
      "app_owns": ["licensed original access and private derivative retention", "module/tool/CSS/font/runtime pins", "compile a copy outside immutable input", "document-wide ID namespace and accurate navigation labels", "truthful current route, destinations and authorization", "brand/avatar asset rights and delivery", "runtime-failure fallback and consumer acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["verification remains not-verified; no certified target or real consumer adoption is claimed.", "Default tests skip paid-source probes; compiler/DOM, delivery and browser evidence must be read separately in the guide.", "No source CSS/fonts, installed dependencies, runtime installation, image download, account backend or notification provider is supplied.", "The original remains independently retrievable. Original imagery, full accessibility/zoom, runtime failure, performance, standalone and consumer acceptance remain open."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "reference.elements-translator",
      "name": "Private Elements HTML-to-templ CLI",
      "kind": "reference",
      "summary": "Translate a bounded Elements HTML grammar into private constant-only templ with an explicit pinned browser runtime; no target certification.",
      "tasks": ["Elements conversion", "HTML to templ", "FAQ disclosure", "dropdown menu", "nested dialog", "mobile navigation", "runtime preservation", "deterministic syntax"],
      "layers": [],
      "profiles": ["cli"],
      "dependencies": ["verified private HTML archive member", "host-provided @tailwindplus/elements 1.0.22 matching receipt ESM and tarball hashes", "app-owned pinned templ v0.3.1020, CSS and source font/image assets"],
      "source": {"path": "internal/designconvert/elements.go", "symbols": ["TranslateElements", "VerifyElementsReplay"]},
      "usage": "Read get_kit_decision(\"elements-translation\") and ADR-0040. Run appkit data assets-translate-elements -id ARCHIVE_ID -path MEMBER -root PRIVATE_DIRECTORY. Output is one private elements-syntax JSON receipt; exit 0 means translated syntax, 1 unsupported, 2 invalid invocation or could-not-check. No -save, installation or runtime loading occurs. Review and supply the pinned Elements module separately, compile only in a private app-owned module, and verify the app's CSS/assets/actions and task behavior. Existing assets-translate and syntax@ discovery are unchanged. Use the CLI, not imports of this internal tooling. Catalog metadata describes CLI availability, not certified web-component fit.",
      "states": ["deterministic private JSON", "unsupported grammar without partial output", "explicit runtime requirement", "source or invocation refusal"],
      "app_owns": ["licensed original access and private derivative retention", "pinned runtime delivery and CSP", "CSS/font/image closure", "unique IDs and repeated-instance composition", "routes, forms, authorization and data", "consumer visual, accessibility and task acceptance"],
      "maturity": "experimental",
      "proof": ["go test ./internal/designconvert", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"],
      "gaps": ["Every receipt remains not-verified; no certified target or consumer adoption is claimed.", "Default tests skip explicit paid-source/browser probes; those require private artifacts and named evidence.", "No runtime installation, typed app data, persistent Elements catalog or application backend is supplied.", "Source small controls, missing imagery, complete focus/zoom and Datastar lifecycle remain separate acceptance work."],
      "provenance": {"source": "first-party", "license": "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions", "audience": "metadata available without private archives; execution requires independently verified original access"},
      "consumers": [],
      "preview": ""
    },
    {
      "id": "recipe.styleguide.local",
      "name": "Local app-owned theme review",
      "kind": "recipe",
      "summary": "Preview bounded app token changes against real app specimens and export an acknowledged source-preserving review bundle.",
      "tasks": ["living style guide", "theme editor", "token diff", "theme preview", "theme export", "app customization"],
      "layers": ["L4"],
      "profiles": ["go-web"],
      "dependencies": ["explicit trusted CUE token source and selected kit identity", "app-owned synthetic specimen callbacks and compiled CSS", "exact local loopback origin; no browser JavaScript required"],
      "source": {"path": "apps/styleguide/handler.go", "symbols": ["New", "Options", "Specimen"]},
      "usage": "Read app-owned-styleguide guidance. Configure NewEngine with immutable source, approved font IDs and selected contrast checks; mount New at the configured base path. Gallery opts in with -theme-source and /theme-lab. Export is a concrete candidate snapshot, never an automatic source patch. Fathom's verified-scoped working-tree return and its remaining limits are recorded in docs/assessments/2026-09-22-fathom-guide-only.md; that evidence does not promote this experimental recipe.",
      "states": ["source baseline", "candidate preview", "invalid edits", "failed or unknown contrast", "stale acknowledgement", "deterministic export", "reset"],
      "app_owns": ["source and selected kit identity", "authentication wrapper", "CSS delivery and scan coverage", "side-effect-free synthetic fixtures", "contrast pair selection", "review and explicit source application"],
      "maturity": "experimental",
      "proof": ["go test ./apps/styleguide", "go test ./apps/gallery", "go test ./apps/gallery/cmd/gallery"],
      "gaps": ["RAC and Fathom have scoped integrations; released propagation, reusable-widget promotion and full two-consumer acceptance remain open.", "Fathom's selected light/wide/dark/narrow preview, refusal, repair, reset and pointer/Enter export checks pass; large-value chart geometry, actual zoom, full accessibility, memory/cancellation and full G3/G4 remain open.", "Preview specimens use the static system-font profile without font/image assets, scripts or forms; the guide's own native editing/export forms remain available."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": ["Resume as Code", "Fathom"],
      "preview": ""
    },
    {
      "id": "foundation.theme.geoff-synthwave",
      "name": "Geoff Synthwave",
      "kind": "token",
      "summary": "Use the personal site's white-neon, pink and purple identity without copying its application.",
      "tasks": [
        "theme",
        "personal website",
        "branding",
        "geoff-synthwave",
        "living style guide",
        "appearance"
      ],
      "layers": [
        "L0",
        "L1"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS; rebuild older utility CSS once for runtime font/radius bindings",
        "Inter font stack declared only; app owns any font delivery"
      ],
      "source": {
        "path": "appkit/ui/themes/geoffsynthwave/geoffsynthwave.go",
        "symbols": [
          "CSS",
          "CSSHandler"
        ]
      },
      "usage": "Serve geoffsynthwave.CSSHandler() after utility CSS, or vendor --include tokens --theme geoff-synthwave. Read theme-workbench guidance. Optional NeonCSS() supplies static ak-neon-title glow, separately from the palette.",
      "states": [
        "dark-only under light/dark/system requests"
      ],
      "app_owns": [
        "application identity selection",
        "font delivery",
        "CSS asset delivery",
        "persistence",
        "domain behavior"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/themes/geoffsynthwave",
        "go test ./apps/gallery",
        "go test ./cmd/appkit"
      ],
      "gaps": [
        "Selected token contrast and generated bytes are tested, not complete browser accessibility or website fidelity.",
        "No consumer migration; local guide editing/export is separately experimental."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/themes?palette=geoff-synthwave"
    },
    {
      "id": "foundation.theme.geoff-indigo",
      "name": "Geoff Indigo",
      "kind": "token",
      "summary": "Use the newer personal dashboard's gray and indigo identity through shared semantic roles.",
      "tasks": [
        "theme",
        "personal website",
        "branding",
        "geoff-indigo",
        "living style guide",
        "appearance"
      ],
      "layers": [
        "L0",
        "L1"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS; rebuild older utility CSS once for runtime font/radius bindings",
        "Inter font stack declared only; app owns any font delivery"
      ],
      "source": {
        "path": "appkit/ui/themes/geoffindigo/geoffindigo.go",
        "symbols": [
          "CSS",
          "CSSHandler"
        ]
      },
      "usage": "Serve geoffindigo.CSSHandler() after utility CSS, or vendor --include tokens --theme geoff-indigo. Read theme-workbench guidance. No decorative effect or application layout is implied.",
      "states": [
        "dark-only under light/dark/system requests"
      ],
      "app_owns": [
        "application identity selection",
        "font delivery",
        "CSS asset delivery",
        "persistence",
        "domain behavior"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/themes/geoffindigo",
        "go test ./apps/gallery",
        "go test ./cmd/appkit"
      ],
      "gaps": [
        "Selected token contrast and generated bytes are tested, not complete browser accessibility or website fidelity.",
        "No consumer migration; local guide editing/export is separately experimental."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/themes?palette=geoff-indigo"
    },
    {
      "id": "foundation.tokens",
      "name": "Semantic theme tokens",
      "kind": "token",
      "summary": "Carry meaningful color, typography and shape across light and dark themes.",
      "tasks": [
        "theme",
        "appearance",
        "design tokens",
        "contrast",
        "branding"
      ],
      "layers": [
        "L0",
        "L1"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/tokens/tokens.go",
        "symbols": [
          "Load",
          "ThemeCSS"
        ]
      },
      "usage": "Load CUE tokens with tokens.Load; serve tokens.ThemeCSS() once. Map consumer identity through semantic roles, not hard-coded widget colors.",
      "states": [
        "light",
        "dark"
      ],
      "app_owns": [
        "brand palette",
        "CSS asset delivery"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/tokens"
      ],
      "gaps": [
        "CUE generation requires the kit toolchain; consuming generated CSS does not."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/tokens"
    },
    {
      "id": "analytics.bars",
      "name": "Labeled comparisons",
      "kind": "primitive",
      "summary": "Compare named values using deterministic, portable SVG.",
      "tasks": [
        "analytics",
        "dashboard",
        "bar chart",
        "horizontal bar chart or graph comparison",
        "comparison",
        "distribution"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/bars.go",
        "symbols": [
          "Bars"
        ]
      },
      "usage": "Call viz.Bars with labeled values and BarsOptions{ShowValue:true}; include units, period and a text/table alternative in the surrounding view. Set Fluid to fill the container width from the W viewBox (the drawing, text included, scales uniformly).",
      "states": [
        "empty",
        "zero",
        "positive values"
      ],
      "app_owns": [
        "aggregation",
        "units",
        "period",
        "missingness",
        "authorization"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "The primitive does not enforce analytical semantics; the app supplies them.",
        "Fluid (FATHOM-F0-KIT-04) fills the container width by scaling the whole drawing: text shrinks below W and grows above it, and nothing reflows or wraps. Choose W near the narrowest container, or keep a fixed-width chart in a keyboard-scrollable region beside a table alternative."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Throughline",
        "GGB",
        "RAC"
      ],
      "preview": "/components"
    },
    {
      "id": "analytics.timeline",
      "name": "Temporal exploration",
      "kind": "primitive",
      "summary": "Show events and spans with honest temporal gaps and multiple layout modes.",
      "tasks": [
        "analytics",
        "timeline",
        "events",
        "history",
        "temporal exploration",
        "activity history"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/timeline.go",
        "symbols": [
          "Timeline"
        ]
      },
      "usage": "Pass stable events and TimelineOptions to viz.Timeline. Use the documented event filter for period/lane/search; retain an accessible event listing. Set Fluid to fill the container width from the W viewBox (the drawing, text included, scales uniformly).",
      "states": [
        "proportional",
        "sequence",
        "condensed",
        "swimlanes",
        "density"
      ],
      "app_owns": [
        "event identity",
        "timezones",
        "filter persistence",
        "permissions"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "SVG output alone does not implement keyboard drill-down or application query semantics.",
        "Fluid (FATHOM-F0-KIT-04) fills the container width by scaling the whole drawing: text shrinks below W and grows above it, and nothing reflows or wraps. Choose W near the narrowest container, or keep a fixed-width chart in a keyboard-scrollable region beside a table alternative."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Throughline"
      ],
      "preview": "/components"
    },
    {
      "id": "analytics.graph",
      "name": "Layered graph",
      "kind": "primitive",
      "summary": "Draw a bounded, deterministic left-to-right graph or one node's callers and callees as portable SVG with a matching table alternative.",
      "tasks": [
        "layered graph or layered DAG",
        "dependency graph",
        "directed acyclic graph (DAG) or flow diagram",
        "call graph or code neighbourhood",
        "call graph or code neighborhood",
        "ego graph of callers and callees",
        "blast radius graph"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors",
        "app CSS for the class-free table alternative"
      ],
      "source": {
        "path": "appkit/ui/viz/graph.go",
        "symbols": [
          "LayoutGraph",
          "LayoutEgo",
          "GraphLayout.SVG",
          "GraphLayout.Table",
          "GraphLayout.Notes"
        ]
      },
      "usage": "Call viz.LayoutGraph with GraphNode and GraphEdge values, or viz.LayoutEgo with a focus ID and EgoOptions.Depth; handle a refusal wrapping ErrInvalidGraph before writing any response. Render GraphLayout.SVG and GraphLayout.Table from the same layout and show GraphLayout.Notes beside the figure. Name what solid, dashed and emphasized edges mean with GraphOptions.SolidLabel, GraphOptions.DashedLabel and GraphOptions.EmphasisLabel. Choose GraphOptions.RankMode given with GraphOptions.RankLabels for app-owned columns; GraphOptions.MaxPerRank bounds each column with a stated +N more aggregate. Wrap the SVG in a keyboard-scrollable region for narrow screens and style the class-free table with app CSS, giving its links at least 44px-high hit areas; once any node links, the SVG node boxes are 44px high too. Gallery specimens: /graph.",
      "states": [
        "empty graph said in words",
        "labelled empty column drawn as None",
        "column over MaxPerRank collapsed into +N more with a note",
        "cycle-closing edges drawn right to left and counted",
        "duplicate edges merged and counted",
        "edges into a +N more node kept apart by line style and emphasis, each with its own count",
        "repeated labels qualified by group or ID in the table",
        "ego nodes beyond depth omitted and counted",
        "invalid input refused before output"
      ],
      "app_owns": [
        "node and edge identity",
        "meaning of solid, dashed and emphasized edges",
        "depth, filtering and per-column bound choices",
        "link destinations and authorization",
        "table styling and placement",
        "data freshness and completeness"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/viz",
        "go test ./apps/gallery"
      ],
      "gaps": [
        "Experimental with one originating consumer: Fathom selected it for its observatory console code neighbourhood (vision 35, FATHOM-KIT-02) but has not integrated it, so no consumer is listed; promotion needs a second consumer.",
        "Sugiyama-lite: no dummy nodes or edge routing, so edges spanning several columns and cycle-closing edges can pass behind intermediate nodes; crossing reduction is four barycenter sweeps, not an optimum.",
        "Label widths are estimated per display cell; long labels truncate with an ellipsis and stay whole in the tooltip and table. Browser checks cover Chromium at 1280 and 390 px in light and dark only (docs/assessments/2026-09-25-layered-graph.md).",
        "The SVG is one role=img picture with title and desc and an aria-hidden drawing; nodes and links are reachable by keyboard and assistive technology only through the table alternative. No screen-reader session was run.",
        "Legend, aggregate and empty-column wording (Focus, +N more, None) is English and fixed.",
        "No pan, zoom, aggregate expansion or node-by-node keyboard traversal; no render-time or memory budget was measured beyond the input bounds of 2000 nodes, 8000 edges and 64 columns."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/graph"
    },
    {
      "id": "analytics.stat",
      "name": "KPI with context",
      "kind": "primitive",
      "summary": "Present a key value with a labeled change and optional trend.",
      "tasks": [
        "analytics",
        "dashboard",
        "KPI",
        "key performance indicator KPI summary",
        "metric",
        "summary"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/stat/stat.go",
        "symbols": [
          "Stat",
          "Render"
        ]
      },
      "usage": "Compose stat.Stat with app-calculated value and delta; call stat.Render. Scan appkit/ui/stat/*.go in appkit css.",
      "states": [
        "value",
        "delta",
        "trend"
      ],
      "app_owns": [
        "unit",
        "comparison baseline",
        "freshness",
        "missing data"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/stat"
      ],
      "gaps": [
        "A tile is not a complete analytical journey."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Throughline"
      ],
      "preview": "/components"
    },
    {
      "id": "analytics.comparison",
      "name": "Meaning-bearing analytics",
      "kind": "recipe",
      "summary": "An experimental typed comparison composition with explicit units, windows, freshness, missing values, table and CSV agreement.",
      "tasks": [
        "analytics comparison",
        "signed diverging bars",
        "negative value chart",
        "analytics CSV export",
        "spreadsheet table export",
        "analytics row detail links",
        "metric contract",
        "period filter",
        "chart table CSV",
        "missing data",
        "source freshness",
        "explanation detail export"
      ],
      "layers": ["L3b"],
      "profiles": ["go-web"],
      "dependencies": [
        "Go for typed dataset validation and CSV export",
        "theme CSS and appkit css scanning of appkit/ui/analytics and appkit/ui/vizframe when using RenderBars or RenderDivergingBars"
      ],
      "source": {
        "path": "appkit/ui/analytics/analytics.go",
        "symbols": ["Dataset", "Contract", "Window", "Row"]
      },
      "usage": "Build an analytics.Dataset from app-selected rows and call Dataset.Validate with an explicit clock. Implementation anchors: appkit/ui/analytics/analytics.go#Dataset.Validate, appkit/ui/analytics/render.go#RenderBars, appkit/ui/analytics/diverging.go#RenderDivergingBars, appkit/ui/analytics/csv.go#WriteCSV. Use analytics.RenderBars for nonnegative data or opt into analytics.RenderDivergingBars for signed single-dataset observations with a centered symmetric scale; Max is the absolute extent and must not clip rounded magnitudes. Both include context and a visible table; optional Row.DetailHref links a table label to an app-owned root-relative URL, including missing/fallback rows. Empty keeps plain labels; invalid links refuse before output. Missing-reason cells reserve 12rem with native scrolling; rebuild CSS and review changed missing-row snapshots and selected pins (ADR-0076). analytics.WriteCSV exports the same selection without row-detail URLs. Experimental analytics.WriteTableCSV emits one 27-column spreadsheet table with repeated context and the same presentation cells; empty selections contain only headers, so use WriteCSV for empty-selection provenance. Both retain source_href context; app-owned Row.DetailHref is omitted. Gallery /analytics/recipe/table.csv demonstrates the opt-in format; old downloads stay unchanged (ADR-0077). New Row fields require named-field migration for positional literals. Read appkit/ui/analytics/README.md. For L3b, call get_kit_css_setup, include theme CSS and scan appkit/ui/analytics/*.go plus appkit/ui/vizframe/*.go relative to the selected kit source root. Open /analytics/recipe or /analytics/signed for exact renderer specimens and matching CSV; over-wide labels/values retain an explicit table-only fallback. Gallery /analytics composes separate current/prior datasets, linked detail and a rectangular CSV adapter, not a reusable query engine. Use an independent fixture for static checks; read appkit/ui/vizcheck/README.md for static-only evidence limits, not geometry or runtime proof.",
      "states": [
        "empty selection",
        "measured zero",
        "missing value with reason",
        "fresh or stale source",
        "unknown source freshness",
        "partial or unknown coverage",
        "invalid contract refusal"
      ],
      "app_owns": [
        "selection and aggregation",
        "source and metric truth",
        "authorization",
        "queries and filter routes",
        "detail and export adapters",
        "Row.DetailHref root-relative destinations, authorization and return context; empty preserves plain labels and accepted URLs do not enter CSV",
        "observation clock and comparison windows"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/analytics",
        "go test ./appkit/ui/vizcheck",
        "go test ./apps/gallery"
      ],
      "gaps": [
        "This is a bounded experiment, not a permanently promoted reusable workflow; independent consumer adoption remains unproven.",
        "Browser task, keyboard, contrast, visual fidelity and measured performance evidence remain separate requirements.",
        "Each dataset is bounded to 200 rows, 2048 bytes per text field, 0–2 decimals and numeric magnitude 1e12; applications select or aggregate larger inputs.",
        "RenderBars still refuses negative values; opt-in RenderDivergingBars and CSV support them. Signed rendering owns one dataset, not paired series or domain calculations. Rounding and formula-safe text encoding are not a lossless source export.",
        "Static chart text/table/source checks do not verify geometry, computed visibility, accessibility or source truth; dynamic charts need post-update browser evidence."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/analytics/recipe"
    },
    {
      "id": "shell.appearance",
      "name": "Browser appearance preference",
      "kind": "primitive",
      "summary": "Share light, dark and system preference with browser-local persistence, visible failure state and retry.",
      "tasks": [
        "appearance preference",
        "dark mode",
        "light dark system theme preference with browser storage recovery and cross-tab synchronization",
        "light dark system",
        "theme persistence",
        "storage recovery",
        "cross-tab synchronization"
      ],
      "layers": ["L2"],
      "profiles": ["portable", "go-web", "static-web", "react-web"],
      "dependencies": [
        "browser JavaScript and app-owned script delivery",
        "theme CSS honoring the root data-theme attribute"
      ],
      "source": {
        "path": "appkit/ui/appearance/appearance.go",
        "symbols": ["Script", "BootstrapScript", "Handler"]
      },
      "usage": "Serve the dependency-free body from appearance.Script once; Go hosts can use appearance.Handler. appearance.BootstrapScript configures and runs the same runtime before CSS, so do not add a second copy. Choose an app-owned storage key or session-only state, then connect controls through the documented browser global/events. Optional appearance.RenderControl is a separate Go L3b convenience requiring a scan of appkit/ui/appearance/*.go; this entry's portable fit covers the L2 script only. The /shell/settings Gallery preview uses the optional Go L3b renderer.",
      "states": [
        "light",
        "dark",
        "system",
        "saved",
        "session-only",
        "unsaved or unavailable storage",
        "invalid preference",
        "retry",
        "cross-tab update"
      ],
      "app_owns": [
        "storage key and persistence policy",
        "script delivery and CSP authorization",
        "visual identity",
        "non-CSS redraw and dynamic control mounting"
      ],
      "maturity": "available",
      "proof": ["go test ./appkit/ui/appearance"],
      "gaps": [
        "No account or settings backend is included; independent consumer integration and browser task proof remain separate.",
        "The package's JavaScript state-machine test requires Node and explicitly skips when unavailable; it is not a real-browser test."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/shell/settings"
    },
    {
      "id": "shell.layout",
      "name": "Application shell",
      "kind": "primitive",
      "summary": "Compose responsive sidebar, top bar and content with typed grouped navigation and a current-route indicator.",
      "tasks": [
        "application shell",
        "mobile application shell navigation with current route and grouped links",
        "navigation",
        "typed navigation",
        "current route",
        "grouped links",
        "sidebar",
        "dashboard",
        "mobile"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/dashboard/shell.go",
        "symbols": [
          "Shell",
          "RenderShell"
        ]
      },
      "usage": "Build dashboard.NavGroup and dashboard.NavItem values and pass dashboard.RenderNav the app's current path; dashboard.MatchPath supports app-owned compositions. Supply that navigation to dashboard.RenderShell with app-owned content. The shell includes a native mobile disclosure; enable its skip link for a full-page shell. Scan appkit/ui/dashboard/*.go and include theme CSS.",
      "states": [
        "wide",
        "narrow",
        "light",
        "dark",
        "current route",
        "exact or subtree matching",
        "unsafe destination disabled",
        "mobile disclosure"
      ],
      "app_owns": [
        "routes",
        "current path and permitted destinations",
        "identity",
        "authorization",
        "content"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/dashboard"
      ],
      "gaps": [
        "Typed navigation shares link rendering and route matching, not routing, permissions, account or settings backends. Appearance behavior is a separate capability.",
        "HTML slots and navigation icons require caller review. Consumer history does not prove fresh keyboard, narrow-screen or workflow adoption results.",
        "Gallery /shell composes this primitive in a synthetic workflow; real account/settings backends and consumer/browser proof are not supplied."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "CA",
        "GGB"
      ],
      "preview": "/shell"
    },
    {
      "id": "shell.account-workflow",
      "name": "Profile, settings and help journey",
      "kind": "recipe",
      "summary": "Reuse typed account destinations and semantic feedback; inspect the complete synthetic profile/settings/help recovery journey.",
      "tasks": ["account workflow", "profile settings", "help about", "save retry", "unsaved draft", "permission recovery"],
      "layers": ["L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS and kit source scanning", "ordinary Go HTTP forms", "optional appearance and Gallery-only draft scripts", "optional Gallery-only Activity Datastar observer; no required consumer runtime dependency"],
      "source": {"path": "appkit/ui/dashboard/account.go", "symbols": ["AccountDestinations", "Notice", "RenderNotice"]},
      "usage": "Define optional dashboard.AccountDestinations once and pass its Group result to dashboard.RenderNav with primary groups. Custom task links retain app-owned URL validation; raw model fields are not a sanitizer. Compose dashboard.RenderShell with reviewed content and dashboard.RenderNotice for explicit status/error/conflict feedback; scan appkit/ui/dashboard/*.go for CSS. Read docs/design/application-shell.md and the compiled ExampleAccountDestinations_Group. Gallery /shell demonstrates forms, versioned saves and recovery; those controllers remain Gallery-only. This is an experimental composition with exported presentation helpers, not an account-backend API, authorization policy or production session implementation. Keep app identity/schema/persistence/recovery and never copy demo permission controls.",
      "states": ["saved", "unsaved draft", "validation error", "save progress", "interrupted navigation", "save failure and retry", "conflict review", "read-only", "signed-out", "paused loading specimen", "load failure", "light/dark/system", "narrow navigation"],
      "app_owns": ["identity provider", "authorized destinations", "profile/settings schema", "validation and persistence", "concurrency policy", "support and diagnostic redaction"],
      "maturity": "experimental",
      "proof": ["go test ./apps/gallery", "go test ./appkit/ui/dashboard", "go test ./appkit/ui/form", "go test ./appkit/ui/appearance"],
      "gaps": ["Synthetic process-memory sessions and draft scripts remain Gallery-only; exported presentation helpers provide no production auth, persistence, save/recovery controller or automatic live-region updates.", "Browser keyboard and narrow/theme checks have scoped Gallery evidence; full accessibility, physical/IME input, human task/performance and consumer acceptance remain open. See docs/design/application-shell.md for exact boundaries.", "Optional Activity observer has scoped browser evidence, not full browser/native conformance or a consumer streaming API.", "Two consumer integrations and their preserved domain behavior are still required before permanent opinionated component promotion; ADR-0063 bounds the incubation."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": [],
      "preview": "/shell"
    },
    {
      "id": "shell.settings-sections",
      "name": "Responsive settings sections",
      "kind": "recipe",
      "summary": "Compose responsive description/control sections with escaped headings and app-owned form content.",
      "tasks": ["settings sections", "description control layout", "responsive preference sections"],
      "layers": ["L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS", "appkit/ui/form/*.go Tailwind source scanning"],
      "source": {"path": "appkit/ui/form/settings.go", "symbols": ["SettingsSection", "RenderSettingsSections"]},
      "usage": "Compose form.SettingsSection values and call form.RenderSettingsSections before writing a response; handle invalid or duplicate heading IDs and blank titles. Place the h3 headings beneath an app-owned h2 with document-unique IDs. Content is reviewed HTML from labeled form.RenderField controls or app-owned information, not an untrusted string. Scan appkit/ui/form/*.go and include theme CSS. Read docs/design/application-shell.md and ExampleRenderSettingsSections. No form wrapper, fieldset, label, submit action, JavaScript or account backend is supplied; schema, authorization, saving and recovery stay app-owned.",
      "states": ["narrow stacked", "wide two-column", "long labels", "light/dark", "empty list", "invalid input refusal"],
      "app_owns": ["heading hierarchy and document-wide IDs", "reviewed HTML content and field labels", "form/action and authorization", "validation, persistence and recovery"],
      "maturity": "experimental",
      "proof": ["go test ./appkit/ui/form", "go test ./apps/gallery"],
      "gaps": ["Layout only; no production account backend or save controller.", "Full accessibility, other engines, physical input and two consumer integrations remain open; ADR-0064 bounds incubation."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": [],
      "preview": "/shell/settings"
    },
    {
      "id": "interaction.forms",
      "name": "Forms and validation vocabulary",
      "kind": "primitive",
      "summary": "Build labeled controls with consistent help and error relationships.",
      "tasks": [
        "form",
        "input",
        "settings",
        "profile",
        "validation",
        "save"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/form/form.go",
        "symbols": [
          "Field",
          "Input",
          "Select",
          "Button"
        ]
      },
      "usage": "Wrap controls with form.Field and use its help/error plumbing. Select refuses silent substitution of unavailable nonempty values with a disabled empty prompt; Placeholder owns initial empty selection. Apps explain unavailable values and explicitly choose any valid fallback. Scan appkit/ui/form/*.go; keep persistence and validation policy in the app. Review ADR-0083 and selected metadata pins when upgrading.",
      "states": [
        "default",
        "disabled",
        "help",
        "error"
      ],
      "app_owns": [
        "schema",
        "authorization",
        "saving",
        "recovery"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/form"
      ],
      "gaps": [
        "Form controls do not provide an account or settings backend."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Throughline"
      ],
      "preview": "/forms"
    },
    {
      "id": "interaction.overlays",
      "name": "Dialogs, drawers and notifications",
      "kind": "primitive",
      "summary": "Add native-dialog surfaces with optional dependency-free interaction.",
      "tasks": [
        "dialog",
        "dialog drawer and toast notifications",
        "drawer",
        "toast",
        "notification",
        "confirmation"
      ],
      "layers": [
        "L2",
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning",
        "embedded interaction script"
      ],
      "source": {
        "path": "appkit/ui/overlay/overlay.go",
        "symbols": [
          "Dialog",
          "Drawer",
          "Script"
        ]
      },
      "usage": "Render the native dialog or drawer, include overlay.Script() once, and scan appkit/ui/overlay/*.go. Supply explicit labels and action forms.",
      "states": [
        "closed",
        "open",
        "dismissed",
        "notification"
      ],
      "app_owns": [
        "actions",
        "authorization",
        "destructive consequences"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/overlay"
      ],
      "gaps": [
        "Consumer must verify initial focus, focus return and validation for its own content."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/overlays"
    },
    {
      "id": "interaction.command-palette",
      "name": "Command palette",
      "kind": "primitive",
      "summary": "Open a keyboard search dialog from anywhere, show server-rendered grouped results and degrade to an ordinary GET search page.",
      "tasks": ["command palette", "command menu", "Mod+K, Cmd+K or Ctrl+K command palette", "keyboard shortcut search", "keyboard search dialog", "search palette", "search modal", "global site search", "quick open", "quick navigation search", "grouped search results", "search page without JavaScript"],
      "layers": ["L2", "L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS", "appkit css source scanning of appkit/ui/palette/*.go and appkit/ui/overlay/*.go", "embedded palette.js; overlay.js is not required", "one app-owned GET search route"],
      "source": {"path": "appkit/ui/palette/palette.go", "symbols": ["Palette", "RenderTrigger", "RenderDialog", "RenderResults", "RenderSearchForm", "WantsFragment", "Script", "Handler"]},
      "usage": "Render palette.RenderTrigger where the search control belongs and palette.RenderDialog once per page, then inline palette.Script or mount palette.Handler once; the AppKitPalette global needs no framework. Scan appkit/ui/palette/*.go and appkit/ui/overlay/*.go and include the theme CSS. Serve one app-owned GET route at Palette.Action: when palette.WantsFragment reports the palette request, write the palette.RenderResults fragment; otherwise render the full search page with palette.RenderSearchForm and the same results at HeadingLevel 2, and send Vary: X-AppKit-Palette. The Action query carries app context as hidden fields. Set Results.Query to the request's query parameter: the script inserts only a response that is exactly one fragment answering its query. After swapping palette or trigger markup, call AppKitPalette.init. Handle refused configuration or results before writing a response. The app decides what is searched, grouping, ranking and destinations; the script fetches with debounce, aborts stale requests, upgrades the fragment to listbox options and moves aria-activedescendant. Read the package doc and Example for the keyboard contract and data-palette hooks; Gallery /palette reproduces every state.",
      "states": ["closed", "idle", "debounced loading", "grouped results", "no results", "partial results notice", "no results while a source was not searched (partial)", "request error, timeout or non-fragment answer", "stale results fall back to the search page", "no-JavaScript search page", "light/dark", "narrow"],
      "app_owns": ["search route, authorization and data sources", "grouping, ranking and result destinations", "full search page layout", "caching and Vary", "shortcut conflicts with app key bindings", "localization of built-in secondary copy"],
      "maturity": "experimental",
      "proof": ["go test ./appkit/ui/palette", "go test ./apps/gallery"],
      "gaps": ["Fathom is the originating consumer (vision 35 observatory console, FATHOM-KIT-01) and has not integrated this revision; experimental until two consumers integrate it (ADR-0017 L3b bar).", "Browser evidence covers the Gallery specimen in one Chromium build: shortcut open, focus, arrow selection, Enter, Escape focus return, stale-request cancellation, timeout/error/empty/partial states, redirects to full pages, re-mount and late triggers with init, 390px width and light/dark. Screen-reader output, other engines, touch/IME input, zoom and performance budgets are unverified.", "The shared overlay dialog close button is a 24px target, below the kit's 44px convention; Escape and backdrop clicks also close.", "Built-in secondary copy (Searching…, result counts, key hints, Search failed) is English; label, placeholder, idle and error text are configurable.", "Script lifecycle tests (node --test appkit/ui/palette/palette_test.mjs) use DOM doubles, not a browser."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": [],
      "preview": "/palette"
    },
    {
      "id": "interaction.callouts",
      "name": "Actionable messages",
      "kind": "primitive",
      "summary": "Communicate information, success, caution and failure without color alone.",
      "tasks": [
        "error",
        "display warning and help feedback",
        "warning",
        "help",
        "message",
        "empty state",
        "feedback"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/callout/callout.go",
        "symbols": [
          "Callout",
          "Render"
        ]
      },
      "usage": "Use callout.Render with a semantic role, plain-language body and an app-owned recovery action. Scan appkit/ui/callout/*.go.",
      "states": [
        "info",
        "success",
        "caution",
        "danger",
        "note"
      ],
      "app_owns": [
        "message wording",
        "remedy",
        "error redaction"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/callout"
      ],
      "gaps": [
        "Escaped Body is the default; BodyHTML is trusted application markup only."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/messaging"
    },
    {
      "id": "interaction.chrome",
      "name": "Navigation and workflow chrome",
      "kind": "primitive",
      "summary": "Compose breadcrumbs, tabs, pagination, disclosures and empty/loading states.",
      "tasks": [
        "breadcrumb",
        "breadcrumb tabs and pagination navigation",
        "tabs",
        "pagination",
        "empty state",
        "loading",
        "navigation"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/chrome/chrome.go",
        "symbols": [
          "Breadcrumbs"
        ]
      },
      "usage": "Use plain-link chrome controls with app-owned URLs. Scan appkit/ui/chrome/*.go; preserve filter context in links. Breadcrumb links render 44px minimum targets with a focus-visible outline.",
      "states": [
        "active",
        "empty",
        "loading",
        "expanded"
      ],
      "app_owns": [
        "routing",
        "page count",
        "search state",
        "copy"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/chrome"
      ],
      "gaps": [
        "A skeleton does not implement loading, retry or state preservation.",
        "RenderPagination links are px-2 py-1 (about 28px high), below the kit's 44px target floor; Fathom's console enlarges them locally.",
        "FATHOM-F0-KIT-03 (breadcrumb target size and focus) comes from Fathom's unpublished console wave F0, which still wraps the trail locally until it adopts this revision."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/layout"
    },
    {
      "id": "interaction.live-freshness",
      "name": "Live freshness regions",
      "kind": "primitive",
      "summary": "Mark server-rendered panels as having newer data from an app-owned server-sent event stream, with a polite announcement and a refresh link, without re-rendering or moving them.",
      "tasks": ["live freshness", "live updates", "server-sent events subscriber", "SSE subscriber", "event stream subscriber", "updated panel refresh link", "stale panel notification", "reconnect with Last-Event-ID"],
      "layers": ["L2", "L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS", "appkit css source scanning of appkit/ui/live/*.go", "embedded live.js served by Handler or inlined with Script", "an app-owned text/event-stream endpoint"],
      "source": {"path": "appkit/ui/live/live.go", "symbols": ["Script", "Handler", "Region", "RenderRegion", "RegionAttrs", "RenderStatus", "Page", "RenderPageStatus"]},
      "usage": "Render live.RenderPageStatus once per page with the app-owned stream URL and followed event names. Wrap each panel with live.RenderRegion, or put live.RegionAttrs on an app-owned wrapper with live.RenderStatus inside it, giving document-unique IDs, source keys, a label and required as-of text; handle refusals before writing the response. Serve live.Script through live.Handler at an app-owned path and load it once: it auto-starts from the page status, or the AppKitLive global's start({url, events, sources}) maps custom payloads; that explicit start adopts the render-time cursor of the page status with the same URL (pass lastEventId otherwise) and connects once the document is parsed. Emit events whose JSON data names source or sources (* after an unreplayable gap), send comment keepalives more often than 60 s and replay from the Last-Event-ID header, which carries the id's UTF-8 bytes. Refresh reloads the page at the region; event data and region HTML are never inserted. Read the package doc for the keyboard and connection contract; scan appkit/ui/live/*.go and include theme CSS.",
      "states": ["no script or not started: as-of text only", "connecting", "live", "region updated with refresh link", "several regions in one announcement", "reconnecting with bounded, jittered backoff", "accept-then-close server exhausts attempts", "paused while hidden", "manually stopped and resumed", "unavailable after refusal or exhausted attempts", "unsupported browser or refused configuration: no resume control", "missed events replayed", "non-ASCII event ids", "gap wildcard", "no movement at 320 px", "invalid input refusal"],
      "app_owns": ["event stream endpoint, names, retention and replay", "source keys and event-to-source mapping", "stream authorization, origin and CORS policy", "as-of wording and time zone", "refresh destination and page rendering", "script delivery and CSP"],
      "maturity": "experimental",
      "proof": ["go test ./appkit/ui/live", "go test ./apps/gallery"],
      "gaps": [
        "Fathom (vision 35 observatory console, FATHOM-KIT-06) is the originating consumer; its /events adoption is not yet integrated or verified against this revision, and permanent promotion needs a second consumer.",
        "Browser evidence is one local Chromium run of the Gallery fixture (docs/assessments/2026-09-25-live-freshness-and-page-heading.md); screen-reader speech, other engines, real network loss, long-running memory and CSP-restricted hosts are unverified.",
        "The updated layer relies on the scanned invisible utility; without the L3b CSS build it would show. The script is not packaged for non-Go vendoring, so no portable profile is claimed.",
        "No event taxonomy, replay store, authorization or content refresh is supplied; a server that cannot replay a gap must send *.",
        "Connection messages are English only; live.js and live.go keep one tested message list so the reserved line height stays exact, and localized copy would need both."
      ],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": ["Fathom"],
      "preview": "/live"
    },
    {
      "id": "interaction.page-heading",
      "name": "Page heading",
      "kind": "primitive",
      "summary": "Render a page's single h1 with an optional breadcrumb trail, eyebrow, meta line and actions that wrap below the title on narrow screens.",
      "tasks": ["page heading", "page title with breadcrumbs", "page header actions", "heading meta line", "page eyebrow"],
      "layers": ["L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS", "appkit css source scanning of appkit/ui/chrome/*.go and appkit/ui/form/*.go"],
      "source": {"path": "appkit/ui/chrome/heading.go", "symbols": ["PageHeading", "RenderPageHeading"]},
      "usage": "Build chrome.PageHeading with a plain-text Title and optional ID, Breadcrumbs (chrome.Crumb values rendered by chrome.Breadcrumbs; a destination that is not relative or HTTP(S) is refused), Eyebrow, up to eight Meta items and up to four Actions: Button values from the form package, rendered unchanged by its RenderButton (Href for links, Type button for script-owned actions). Call chrome.RenderPageHeading before writing the response and handle its error; it writes the page's one h1. Below the sm breakpoint the actions wrap under the title. Scan appkit/ui/chrome/*.go and appkit/ui/form/*.go and include theme CSS.",
      "states": ["title only", "breadcrumbs, eyebrow, meta and actions", "narrow: actions below the title", "wide: actions at the row end", "long unbroken title and meta", "unsafe action link inert", "unsafe breadcrumb destination refused", "invalid input refusal", "light/dark"],
      "app_owns": ["page title and the heading hierarchy below the h1", "breadcrumb destinations", "action destinations, forms, scripts and authorization", "meta wording and freshness"],
      "maturity": "experimental",
      "proof": ["go test ./appkit/ui/chrome", "go test ./apps/gallery"],
      "gaps": [
        "Fathom (vision 35 observatory console, FATHOM-KIT-07) is the originating consumer; its integration is not yet performed or verified against this revision, and permanent promotion needs a second consumer.",
        "Breadcrumb links keep the existing Breadcrumbs inline targets and hover-only styling; they are not raised to 44px.",
        "Browser evidence is one local Chromium run of the Gallery specimen at wide/narrow and light/dark (docs/assessments/2026-09-25-live-freshness-and-page-heading.md); zoom, other engines and assistive technology are unverified.",
        "Meta items are plain text; links or rich meta need an app-owned composition."
      ],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": ["Fathom"],
      "preview": "/page-heading"
    },
    {
      "id": "content.docs",
      "name": "Readable documentation",
      "kind": "primitive",
      "summary": "Add token-themed prose and a responsive table of contents.",
      "tasks": [
        "help",
        "help page",
        "about",
        "documentation",
        "table of contents",
        "prose",
        "find fixed static content example",
        "find fixed FAQ example without JavaScript"
      ],
      "layers": [
        "L1",
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/docs/toc.go",
        "symbols": [
          "RenderToC",
          "RenderToCPanel",
          "RenderToCRail"
        ]
      },
      "usage": "Scan appkit/ui/docs/*.go; import prose.css when using its prose treatment. Supply stable heading IDs and focusable fragment targets. Gallery /content composes a native reading task with app-owned content, destination policy and synthetic recovery. For the separate experimental fixed Gallery exporter, call get_kit_decision(\"static-content-delivery\") or follow docs/design/static-content-delivery.md, then run go run ./apps/gallery/cmd/static-content. Observe its CSS build prerequisite and fresh absolute output paths. It is a bounded example, not a generic site generator, CMS, or certified static profile.",
      "states": [
        "wide",
        "narrow",
        "long headings"
      ],
      "app_owns": [
        "content",
        "anchor IDs",
        "support links"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/docs"
      ],
      "gaps": [
        "Named one-consumer admission exception remains documented in ui-components.md."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "platform-engineering portal"
      ],
      "preview": "/content"
    },
    {
      "id": "content.code-excerpt",
      "name": "Code excerpt viewer",
      "kind": "primitive",
      "summary": "Show numbered source lines with labelled per-line state, line anchors and permalinks in a keyboard-scrollable viewport.",
      "tasks": ["code excerpt", "source code excerpt", "source code viewer", "code line numbers", "line anchors", "line permalinks", "changed lines", "not executed lines", "unexecuted changed lines highlighted", "code review excerpt"],
      "layers": ["L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS", "appkit/ui/code/*.go Tailwind source scanning"],
      "source": {"path": "appkit/ui/code/excerpt.go", "symbols": ["Excerpt", "Line", "LineState", "StateLabels", "SplitLines", "RenderExcerpt", "MaxLines"]},
      "usage": "Number app-supplied source text with code.SplitLines, set Line.State to code.LineChanged, code.LineNotExecuted or code.LineHighlighted for the lines the app has classified, and call code.RenderExcerpt before writing a response; handle its error (blank Label, non-increasing numbers, unknown state, unsafe SourceHref, more than MaxLines lines). Line ids are Excerpt.AnchorPrefix+\"L\"+number, so give each excerpt on a page its own prefix. Override wording with code.StateLabels. Scan appkit/ui/code/*.go and include theme CSS. Hidden, format, tag, line-separator and direction-changing characters are shown as code points in the lines, Caption and SourceLabel. Rows are 24px, so each line-number permalink is a 48×24px target. Gallery /agentic/code shows every state, a stated gap, revealed hidden characters, the empty state and a refusal. No syntax highlighting, JavaScript, file fetching or authorization is supplied.",
      "states": ["normal", "changed", "not executed", "highlighted", "targeted line", "stated gap between lines", "revealed hidden or direction-changing characters", "long lines scroll horizontally", "empty excerpt", "invalid input refusal", "light/dark", "narrow"],
      "app_owns": ["source text, revision and line-ending normalization", "which lines carry which state and what the state means", "anchor prefix uniqueness within the document", "source link destination and authorization"],
      "maturity": "experimental",
      "proof": ["go test ./appkit/ui/code", "go test ./apps/gallery"],
      "gaps": ["No syntax highlighting engine: text renders escaped in the monospace font. Review excerpts do not need one; add highlighting only for a named consumer task.", "Every line-number permalink is a native tab stop, so long excerpts add many stops; MaxLines bounds input but no measured performance budget exists.", "Divergence: line-number permalinks are 48×24px at a 24px row pitch. That meets WCAG 2.2 2.5.8 (24px minimum) but not the 44px floor the kit uses for standalone controls; dense code rows cannot reach 44px.", "Hidden-character reveal classifies by the unicode tables of the Go release that builds the kit (Unicode 15.0 in Go 1.26); characters assigned later are not classified.", "Scoped Chrome checks cover keyboard scrolling, permalinks, focus, narrow and both themes at the default palette; screen readers, zoom, other engines, forced colors and other palettes remain unverified.", "One originating consumer (Fathom FATHOM-KIT-04) has not integrated it yet; a second consumer is required before promotion."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": [],
      "preview": "/agentic/code"
    },
    {
      "id": "data.table",
      "name": "Structured data table",
      "kind": "primitive",
      "summary": "Present record comparisons with semantic headings and optional sort links.",
      "tasks": [
        "analytics",
        "table",
        "records",
        "data browser",
        "sort data browser records",
        "sortable records",
        "sort"
      ],
      "layers": [
        "L3b",
        "L4"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning",
        "templ"
      ],
      "source": {
        "path": "appkit/ui/table/table_templ.go",
        "symbols": [
          "Table"
        ]
      },
      "usage": "Render table.Table with typed columns and rows. Managed Row.Href accepts relative web references or absolute HTTP(S); refused nonempty input becomes inert Open unavailable text. Actions infer one aligned column; RowHrefs can reserve it for empty/unlinked results. Rebuild templ/scanned CSS and review density, snapshots and app-owned footer spans. Queries, sort URLs, custom actions and authorization remain app-owned. Header cells carry scope=\"col\" and the scroll wrapper is positioned, so a table wider than a narrow screen scrolls inside it instead of widening the page.",
      "states": [
        "rows",
        "empty",
        "sorted"
      ],
      "app_owns": [
        "query",
        "filter",
        "pagination",
        "selection",
        "permissions"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/table"
      ],
      "gaps": [
        "No client virtualization or query backend is included.",
        "Managed row-link evidence is scoped; full table accessibility, sorting/selection workflows and consumer acceptance remain separate.",
        "There is no row-header option: body cells are all <td>, so a table whose first column names its row needs app-owned markup for scope=\"row\".",
        "The positioned scroll wrapper also contains absolutely positioned cell content (for example a popover in RowAction or HTMLCells) that previously escaped it; fixed-position overlays are unaffected.",
        "FATHOM-F0-KIT-01/02 (header scopes, narrow-width containment) come from Fathom's unpublished console wave F0, which still carries local workarounds until it adopts this revision."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Throughline"
      ],
      "preview": "/layout"
    },
    {
      "id": "work.kanban",
      "name": "Workflow board",
      "kind": "primitive",
      "summary": "Arrange app-owned records into clear stages using the shared card surface.",
      "tasks": [
        "board",
        "kanban",
        "kanban stages",
        "task board",
        "workflow",
        "stages"
      ],
      "layers": [
        "L3b",
        "L4"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning",
        "templ"
      ],
      "source": {
        "path": "appkit/ui/kanban/kanban_templ.go",
        "symbols": [
          "Board"
        ]
      },
      "usage": "Render kanban.Board with columns and cards, matching StageID to Column.ID. Scan kanban and card sources and use templ.",
      "states": [
        "columns",
        "cards",
        "empty stage"
      ],
      "app_owns": [
        "stage policy",
        "move authorization",
        "persistence"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/kanban"
      ],
      "gaps": [
        "Drag-and-drop or keyboard move workflows require app-owned integration."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "RAC"
      ],
      "preview": "/layout"
    },
    {
      "id": "work.approval",
      "name": "Human approval surface",
      "kind": "primitive",
      "summary": "Show the proposed action, risk and reviewable decision context.",
      "tasks": [
        "approval",
        "review queue",
        "agent",
        "human in the loop",
        "review",
        "risk"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/hitl/approval.go",
        "symbols": [
          "ApprovalCard"
        ]
      },
      "usage": "Compose hitl.ApprovalCard with app-owned action forms and risk context. The card owns its bordered surface: do not wrap it in another dashboard widget. For priority queues, keep engine order in a single-column ordered list with a queue heading, count and honest empty state; see docs/design/experience-system.md. Keep execution and consequence previews app-owned. Scan appkit/ui/hitl/*.go.",
      "states": [
        "pending",
        "review"
      ],
      "app_owns": [
        "authorization",
        "approval policy",
        "action execution",
        "audit trail"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/hitl"
      ],
      "gaps": [
        "Presentation does not authorize or execute an action."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/agentic"
    },
    {
      "id": "work.unified-diff",
      "name": "Unified text diff",
      "kind": "primitive",
      "summary": "Review a line-level text change as unified hunks with numbered +/- lines and expandable unchanged context.",
      "tasks": ["unified diff", "text diff", "line diff", "line-level diff", "diff hunks", "diff viewer", "unified text diff renderer", "proposed text change review", "code review diff"],
      "layers": ["L3b"],
      "profiles": ["go-web"],
      "dependencies": ["theme CSS", "appkit/ui/hitl/*.go Tailwind source scanning"],
      "source": {"path": "appkit/ui/hitl/unified.go", "symbols": ["UnifiedDiff", "RenderUnifiedDiff"]},
      "usage": "Pass the complete current and proposed texts as UnifiedDiff.Old and UnifiedDiff.New, or precomputed hunks from hitl.DiffText and LineDiff.Hunks (or an app's own diff) as UnifiedDiff.Hunks, and call hitl.RenderUnifiedDiff before writing a response; handle its error. hitl.DiffText is a deterministic Myers line diff bounded by hitl.MaxDiffLines per side; beyond its edit budget it reports LineDiff.Replaced and the page says so. With precomputed hunks, pass LineDiff.Replaced as UnifiedDiff.Replaced so the page still says so. UnifiedDiff.Context 0 keeps hitl.DefaultContext lines, hitl.NoContext keeps none. Unchanged runs collapse behind native details. Precomputed hunks may be a subset of one comparison: the ranges between them are stated per side as not included, never as unchanged; overlapping hunks and a missing final newline anywhere but a side's last line are refused. Hidden, format, tag, line-separator and direction-changing characters are shown as code points in the lines, Label and side names. Pair with hitl.DiffView for field-level changes and hitl.ApprovalCard for the decision. Scan appkit/ui/hitl/*.go. Gallery /agentic/code shows computed, precomputed and unchanged diffs.",
      "states": ["added", "removed", "unchanged context", "collapsed unchanged run", "expanded unchanged run", "omitted range between precomputed hunks", "no newline at end of file", "no changes", "diff budget exceeded", "revealed hidden or direction-changing characters", "invalid input refusal", "light/dark", "narrow"],
      "app_owns": ["both texts and their revisions", "whether a change is allowed, applied or audited", "line-ending normalization", "side names and labels"],
      "maturity": "experimental",
      "proof": ["go test ./appkit/ui/hitl", "go test ./apps/gallery"],
      "gaps": ["No syntax highlighting, word-level intra-line diff, side-by-side view, rename detection or patch application.", "The renderer cannot know whether precomputed hunks are a subset or where the texts end: added/removed counts cover the supplied hunks, and lines after the last hunk are not mentioned.", "Hidden-character reveal classifies by the unicode tables of the Go release that builds the kit (Unicode 15.0 in Go 1.26); characters assigned later are not classified.", "Collapsed runs carry their full text in the HTML; inputs are bounded by MaxDiffLines and a 1000-edit Myers budget, but no measured size or render budget exists.", "Scoped Chrome checks cover keyboard disclosure, focus, narrow and both themes at the default palette; screen readers, zoom, other engines, forced colors and other palettes remain unverified.", "One originating consumer (Fathom FATHOM-KIT-05) has not integrated it yet; a second consumer is required before promotion."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": [],
      "preview": "/agentic/code"
    },
    {
      "id": "cli.command",
      "name": "Opinionated command chrome",
      "kind": "primitive",
      "summary": "Give Cobra commands coherent help, versions, completion and signal-aware exit behavior.",
      "tasks": [
        "CLI",
        "command",
        "cobra",
        "Cobra command help version and completion",
        "help",
        "version",
        "completion"
      ],
      "layers": [],
      "profiles": [
        "cli"
      ],
      "dependencies": [
        "Go; see go.mod and docs/design/cli-kit.md"
      ],
      "source": {
        "path": "appkit/cli/command/command.go",
        "symbols": [
          "New",
          "Execute"
        ]
      },
      "usage": "Create command.New(command.App{...}), attach commands, and pass command.Execute's exit code to the process boundary. See docs/design/cli-kit.md.",
      "states": [
        "help",
        "usage error",
        "cancellation",
        "machine invocation"
      ],
      "app_owns": [
        "commands",
        "domain errors",
        "side effects"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/cli/command"
      ],
      "gaps": [
        "Fang is contained behind the kit API; dependency maturity does not establish terminal support on every host."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "bosun"
      ],
      "preview": ""
    },
    {
      "id": "cli.output",
      "name": "Predictable diagnostic output",
      "kind": "primitive",
      "summary": "Keep plain text and NDJSON stable while enriching interactive terminals.",
      "tasks": [
        "CLI",
        "JSON",
        "NDJSON",
        "NDJSON diagnostics",
        "diagnostics",
        "doctor",
        "pipes",
        "NO_COLOR"
      ],
      "layers": [],
      "profiles": [
        "cli"
      ],
      "dependencies": [
        "Go; see go.mod and docs/design/cli-kit.md"
      ],
      "source": {
        "path": "appkit/cli/output/table.go",
        "symbols": [
          "RenderStatusTable"
        ]
      },
      "usage": "Render output.StatusRow values using output.RenderStatusTable and output.Detect. Use output.WriteNDJSON for machine output, never styled terminal text.",
      "states": [
        "terminal",
        "redirected",
        "NO_COLOR",
        "NDJSON"
      ],
      "app_owns": [
        "status meaning",
        "exit code",
        "sensitive data redaction"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/cli/output"
      ],
      "gaps": [
        "Choose the output mode explicitly; automatic color detection is not automatic format selection."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "bosun"
      ],
      "preview": ""
    },
    {
      "id": "cli.config",
      "name": "Explainable CUE configuration",
      "kind": "primitive",
      "summary": "Validate configuration layers and explain where each value came from.",
      "tasks": [
        "CLI",
        "CUE",
        "explain CUE command configuration and value provenance",
        "configuration",
        "settings",
        "explain",
        "deterministic"
      ],
      "layers": [],
      "profiles": [
        "cli"
      ],
      "dependencies": [
        "Go; see go.mod and docs/design/cli-kit.md"
      ],
      "source": {
        "path": "appkit/cli/config/config.go",
        "symbols": [
          "Load"
        ]
      },
      "usage": "Call config.Load with the app schema and Host; use Result.Explain for provenance. Conflicting CUE layers refuse rather than silently overwriting.",
      "states": [
        "absent layer",
        "valid",
        "conflict",
        "explain"
      ],
      "app_owns": [
        "schema",
        "secrets",
        "override policy"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/cli/config"
      ],
      "gaps": [
        "CUE unification is not last-writer-wins configuration."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "bosun"
      ],
      "preview": ""
    },
    {
      "id": "cli.tui",
      "name": "Charm interaction runner",
      "kind": "primitive",
      "summary": "Run Bubble Tea models with consistent lifecycle and deterministic test defaults.",
      "tasks": [
        "CLI",
        "TUI",
        "Charm interactive terminal with keyboard and mouse",
        "Charm",
        "keyboard",
        "mouse",
        "interactive",
        "terminal"
      ],
      "layers": [],
      "profiles": [
        "cli"
      ],
      "dependencies": [
        "Go; see go.mod and docs/design/cli-kit.md"
      ],
      "source": {
        "path": "appkit/cli/tui/run.go",
        "symbols": [
          "Run",
          "WithMouse",
          "WithAltScreen"
        ]
      },
      "usage": "Call tui.Run only for tasks that need persistent interaction; test with tui.NewTest. The experimental apps/terminal reference demonstrates live capture opt-out using model-owned mouse mode instead of forcing tui.WithMouse. Read docs/design/terminal-gallery.md.",
      "states": [
        "running",
        "canceled",
        "resized"
      ],
      "app_owns": [
        "model",
        "keyboard paths",
        "mouse hit testing",
        "selection",
        "terminal matrix"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/cli/tui",
        "go test ./apps/terminal",
        "go test ./apps/terminal/cmd/terminal"
      ],
      "gaps": [
        "apps/terminal is an experimental first-party catalog reference, not a permanent widget API or proof of two CLI integrations. Terminal restoration, emulator rendering and accessibility need target-specific evidence."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "thumperctl"
      ],
      "preview": ""
    },
    {
      "id": "cli.theme",
      "name": "Shared terminal palette",
      "kind": "token",
      "summary": "Reuse the semantic theme roles in terminal help and interactive views.",
      "tasks": [
        "CLI",
        "theme",
        "color",
        "Charm",
        "palette",
        "Charm terminal theme palette"
      ],
      "layers": [
        "L0"
      ],
      "profiles": [
        "cli"
      ],
      "dependencies": [
        "Go; see go.mod and docs/design/cli-kit.md"
      ],
      "source": {
        "path": "appkit/cli/theme/theme.go",
        "symbols": [
          "Styles"
        ]
      },
      "usage": "Use theme.Styles(isDark) instead of literal ANSI colors; respect output color profiles and NO_COLOR.",
      "states": [
        "light",
        "dark",
        "no color"
      ],
      "app_owns": [
        "terminal profile",
        "density"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/cli/theme"
      ],
      "gaps": [
        "Terminal rendering differs by host and color capability."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "bosun"
      ],
      "preview": ""
    },
    {
      "id": "viz.heatmap",
      "name": "Heatmap with cell truth states",
      "kind": "primitive",
      "summary": "Render a matrix as themed intensity cells and, with truth states, keep a measured zero distinct from missing, unknown and error cells.",
      "tasks": [
        "heatmap",
        "matrix heatmap",
        "coverage matrix",
        "cell truth states",
        "missing unknown and error cells",
        "intensity grid"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/heatmap.go",
        "symbols": [
          "Heatmap",
          "HeatmapOptions",
          "HeatmapCell",
          "CellState",
          "CellZero",
          "CellMissing",
          "CellUnknown",
          "CellError"
        ]
      },
      "usage": "Call viz.Heatmap with rows and HeatmapOptions. For truth states set HeatmapOptions.States (row-major, parallel to rows; the grid spans both) or HeatmapOptions.TruthStates: viz.CellMissing and viz.CellUnknown cells are hatched, viz.CellError carries a danger cross, a measured zero is a ramp cell marked 0, NaN becomes missing and an infinite value or unrecognized state becomes unknown. Each cell gets a title with row, column, state and HeatmapCell.Reason; a legend and desc summary name the states present. Leave both unset to keep the original bytes. Call HeatmapOptions.Validate in tests or admission to refuse contradictory input; the renderer itself never refuses. Titles are pointer-only, so render the same cells as an app-owned table (Gallery /components/heatmap), and size Cell so x labels fit.",
      "states": [
        "value",
        "measured zero",
        "missing: dashed outline and hatch",
        "unknown: dotted outline, cross-hatch and ?",
        "error: danger outline and cross",
        "empty grid",
        "all missing",
        "legacy NaN hole without truth states"
      ],
      "app_owns": [
        "cell semantics and reasons",
        "units and period",
        "table alternative for keyboard and screen-reader access",
        "value range",
        "label length and cell size"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/viz",
        "go test ./apps/gallery"
      ],
      "gaps": [
        "Truth states are new (FATHOM-KIT-03, requested for Fathom's coverage matrix): no app consumes them yet, and no published Fathom revision records the selection.",
        "Per-cell titles are pointer tooltips inside role=img; keyboard and assistive-technology users need the app's table alternative. There is no in-chart keyboard navigation.",
        "The value ramp's low end is below 3:1 on the surface (docs/theming-hazards.md); a measured zero relies on its 0 glyph, which is omitted in cells under 14px unless ShowValues is set, so there a zero looks like the ramp's lowest values.",
        "X labels and ShowValues labels are centered without wrapping or fitting: at the 26px default a label wider than about four characters (a long layer name, -12.25 or -0.004) runs into its neighbor. Legend text width is estimated, not measured.",
        "Without truth states the original bytes are kept, including unguarded infinite-value handling; use truth states or Validate.",
        "Scoped Gallery checks (light/dark, 1440/390px, keyboard scroll regions) are recorded in the quality inventory; no consumer, screen-reader or performance acceptance is claimed."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/components/heatmap"
    },
    {
      "id": "viz.trend-sparkline",
      "name": "Inline trend sparkline",
      "kind": "primitive",
      "summary": "Draw a compact trend line with an optional area fill for inline and KPI contexts.",
      "tasks": [
        "sparkline",
        "inline trend line",
        "trend line graph",
        "compact trend"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/sparkline.go",
        "symbols": [
          "Sparkline",
          "SparklineOptions"
        ]
      },
      "usage": "Call viz.Sparkline with ordered values and SparklineOptions (stroke role, optional fill role, accessible label). NaN values are filtered out of the line rather than drawn as zero. The stat tile composes it; put the current value, units and period in surrounding text. Set Fluid to fill the container width; W:H sets the aspect ratio and the stroke scales with the drawing.",
      "states": [
        "empty",
        "single value",
        "flat series",
        "with area fill"
      ],
      "app_owns": [
        "time axis and period",
        "units",
        "meaning of dropped points",
        "numeric label"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "No axes, labels or gap marking: a filtered NaN point simply disappears from the line.",
        "role=img with a title only; the app supplies the value and period as text."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Fathom"
      ],
      "preview": "/components"
    },
    {
      "id": "viz.composition",
      "name": "Part-to-whole composition",
      "kind": "primitive",
      "summary": "Show shares of a whole as one stacked bar or a donut ring with semantic segment roles.",
      "tasks": [
        "stacked bar",
        "donut",
        "donut ring",
        "part to whole composition",
        "share breakdown"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/stackbar.go",
        "symbols": [
          "StackBar",
          "Segment",
          "StackBarOptions"
        ]
      },
      "usage": "Pass viz.Segment values with Label and Role to viz.StackBar for one horizontal bar or to viz.Donut for a ring with optional DonutOptions.CenterText. A zero total renders an empty track or ring. Segment labels become hover titles only, so show counts and shares as text or a table beside the figure.",
      "states": [
        "empty or zero total",
        "single segment",
        "multiple segments",
        "center text"
      ],
      "app_owns": [
        "segment order and roles",
        "totals and percentages",
        "visible legend",
        "units"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "No visible legend or percentage labels; segment labels are pointer titles.",
        "Meaning must not rest on segment color alone; the app supplies text."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/components"
    },
    {
      "id": "viz.scores",
      "name": "Scores, ratings and meters",
      "kind": "primitive",
      "summary": "Show a single score as a dial, rating pips or a labeled value against a maximum, optionally graded by threshold.",
      "tasks": [
        "gauge",
        "score dial",
        "rating pips",
        "confidence rating",
        "progress meter",
        "value against maximum"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/gauge.go",
        "symbols": [
          "Gauge",
          "GaugeOptions",
          "GradedRole"
        ]
      },
      "usage": "Use viz.Gauge for a 180-degree dial, viz.Rating for n-of-M pips and viz.MeterRow for a labeled value/max bar. GaugeOptions.Graded and viz.GradedRole map a 0..1 fraction to danger, caution and positive at fixed thirds. State the scale, what the thresholds mean and the source beside the mark.",
      "states": [
        "zero",
        "full",
        "value above maximum clamped",
        "graded thresholds",
        "caption"
      ],
      "app_owns": [
        "scale and thresholds meaning",
        "what the score measures",
        "source and period"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "Graded thresholds are fixed at about one and two thirds; they are not configurable.",
        "A light-scheme caution mark measures below 3:1 on the surface (docs/theming-hazards.md), so print the value too."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Fathom"
      ],
      "preview": "/components"
    },
    {
      "id": "viz.stages",
      "name": "Lifecycle steps and funnels",
      "kind": "primitive",
      "summary": "Show ordered stages as a done/current/upcoming stepper or a narrowing funnel of counts.",
      "tasks": [
        "stepper",
        "lifecycle progress steps",
        "funnel",
        "conversion funnel",
        "intake to outcome stages"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/steps.go",
        "symbols": [
          "Steps",
          "Step",
          "StepsOptions",
          "StepState"
        ]
      },
      "usage": "Pass ordered viz.Step values to viz.Steps with done, current or upcoming states; pass viz.Stage counts to viz.Funnel, whose bar widths are relative to the first stage (the largest value when the first is zero). Keep stage names and counts available as text.",
      "states": [
        "done",
        "current",
        "upcoming",
        "empty",
        "zero first stage"
      ],
      "app_owns": [
        "stage order and meaning",
        "counts and conversion math",
        "current-step truth"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "The stepper is a picture, not a navigable wizard: no links or keyboard interaction.",
        "The funnel does not compute or print conversion rates."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/components"
    },
    {
      "id": "viz.outcome",
      "name": "Ranges, waterfalls and sensitivity",
      "kind": "primitive",
      "summary": "Explain outcomes with low-high range bars, running-total waterfalls, reward-versus-cost bars and tornado sensitivity swings.",
      "tasks": [
        "waterfall",
        "running total bridge",
        "tornado sensitivity",
        "sensitivity analysis",
        "low high range bars",
        "interval estimate",
        "cost versus reward"
      ],
      "layers": [
        "L3a"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "generated theme CSS when using token colors"
      ],
      "source": {
        "path": "appkit/ui/viz/waterfall.go",
        "symbols": [
          "Waterfall",
          "WaterfallStep",
          "WaterfallOptions"
        ]
      },
      "usage": "viz.Waterfall draws running-total columns from signed WaterfallStep.Delta values and Total steps; viz.RangeBars draws low-high intervals with an optional most-likely point; viz.CostReward draws diverging reward and cost bars; viz.Tornado sorts swings around TornadoOptions.Baseline. Keep the underlying figures in a table and state units and assumptions.",
      "states": [
        "positive and negative deltas",
        "subtotal and total",
        "range without a point estimate",
        "sorted swings",
        "empty"
      ],
      "app_owns": [
        "estimates and assumptions",
        "units and currency prefix",
        "baseline choice",
        "uncertainty meaning"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/viz"
      ],
      "gaps": [
        "The renderers draw supplied numbers; they compute no ranges or sensitivities, only the waterfall's running sum.",
        "role=img with a title only; the app supplies a table alternative."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/components"
    },
    {
      "id": "viz.provenance-frame",
      "name": "Provenance frame",
      "kind": "primitive",
      "summary": "Wrap a figure with a mandatory source line whose kind is published, derived or estimate, and refuse unsourced figures.",
      "tasks": [
        "provenance frame",
        "chart source line",
        "figure citation",
        "data provenance"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/vizframe/vizframe.go",
        "symbols": [
          "Render",
          "MustRender",
          "Options",
          "Published",
          "Derived",
          "Estimate"
        ]
      },
      "usage": "Wrap chart markup with vizframe.Render and vizframe.Options, giving a Source built by vizframe.Published, vizframe.Derived or vizframe.Estimate; Render returns empty output rather than an unsourced figure and vizframe.MustRender panics for tests. Use vizframe.CarriesMechanism only for diagrams that state no quantity. Wrap wide bodies in a named focusable scroll region and scan appkit/ui/vizframe/*.go.",
      "states": [
        "published",
        "derived",
        "estimate",
        "draft",
        "mechanism waiver",
        "refused unsourced figure"
      ],
      "app_owns": [
        "source truth and links",
        "figure body",
        "layout and scrolling"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/vizframe"
      ],
      "gaps": [
        "The frame checks that a source is stated, not that it is true.",
        "Body sizing and scrolling remain the caller's composition."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Fathom"
      ],
      "preview": "/provenance"
    },
    {
      "id": "work.agent-trace",
      "name": "Agent run inspection",
      "kind": "primitive",
      "summary": "Show what an agent did: a run event rail, one tool invocation with collapsible arguments and result, and a before/after field diff.",
      "tasks": [
        "agent run trace",
        "tool call inspection",
        "proposed change diff",
        "before and after field diff",
        "agent activity rail"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/hitl/trace.go",
        "symbols": [
          "RunTrace",
          "TraceStep",
          "TraceState"
        ]
      },
      "usage": "Render run steps with hitl.RunTrace, one invocation with hitl.RenderToolCall and hitl.ToolCall (native details for arguments and result), and field-level edits with hitl.DiffView and hitl.Change, where an empty side marks an addition or removal. Pair a diff with hitl.RenderApprovalCard for decisions. Text is escaped; scan appkit/ui/hitl/*.go.",
      "states": [
        "trace done, active, failed, pending",
        "empty run",
        "tool ok, error, running, pending",
        "addition",
        "removal",
        "changed field"
      ],
      "app_owns": [
        "run data and redaction of arguments and results",
        "tool status truth",
        "authorization of follow-up actions",
        "live updates"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/hitl"
      ],
      "gaps": [
        "Static rendering only: no streaming, pagination or line-level text diff.",
        "Arguments and results are shown as supplied; redacting secrets is app-owned."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [
        "Fathom"
      ],
      "preview": "/agentic"
    },
    {
      "id": "foundation.motion",
      "name": "Motion vocabulary",
      "kind": "primitive",
      "summary": "Two-state transitions, bounded staggering and a structural reduced-motion cancel as plain CSS with an optional helper script.",
      "tasks": [
        "motion",
        "reduced motion",
        "state transition animation",
        "staggered reveal",
        "animated count"
      ],
      "layers": [
        "L1",
        "L2"
      ],
      "profiles": [
        "portable",
        "go-web",
        "static-web",
        "react-web"
      ],
      "dependencies": [
        "theme CSS for role colors",
        "motion.css after the theme layer; the scripts only for their optional helpers"
      ],
      "source": {
        "path": "appkit/ui/motion/motion.go",
        "symbols": [
          "Root",
          "StateB",
          "RootClass",
          "Step",
          "CSS",
          "Script",
          "CountScript"
        ]
      },
      "usage": "Import motion.css after the theme layer with no source scanning, serve motion.CSS or motion.CSSHandler, or vendor it with appkit vendor --include motion. Put motion.Root on a subtree, toggle motion.StateB, and give each moving element one of motion.Fade, motion.Tint, motion.Shift or motion.Anim with motion.WhenA or motion.WhenB; stagger with motion.Step up to motion.MaxStep. The optional motion.Script helper exposes the AppKitMotion browser global; motion.CountScript adds the counting tween.",
      "states": [
        "state A",
        "state B",
        "reduced motion",
        "staggered",
        "without script"
      ],
      "app_owns": [
        "when to change state",
        "which elements move",
        "script delivery and CSP"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/motion"
      ],
      "gaps": [
        "CSS transitions only: no physics, sequencing or scroll-linked animation.",
        "The reduced-motion cancel is pinned by stylesheet tests; operating-system and browser behavior across hosts is not certified."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/motion"
    },
    {
      "id": "ops.topology",
      "name": "Fleet health topology",
      "kind": "primitive",
      "summary": "Show a region, cluster and workload hierarchy as collapsible rows with app-supplied health badges, metrics and drill-through links.",
      "tasks": [
        "topology",
        "topology tree",
        "status tree",
        "hierarchical status"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/topology/tree.go",
        "symbols": [
          "Render",
          "Node",
          "Options"
        ]
      },
      "usage": "Pass nested topology.Node values with name, status, metric, link and children to topology.Render with topology.Options. topology.Healthy, topology.Degraded and topology.Failed map to positive, caution and danger badges; an empty or unrecognized status renders as topology.Unknown, a neutral badge that never reads as healthy. Groups use native details, so no JavaScript is needed. Scan appkit/ui/topology/*.go and appkit/ui/callout/*.go.",
      "states": [
        "healthy",
        "degraded",
        "failed",
        "unknown",
        "collapsed",
        "expanded",
        "leaf",
        "drill-through link"
      ],
      "app_owns": [
        "health computation and roll-up",
        "drill-through routes and authorization",
        "refresh cadence"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/topology"
      ],
      "gaps": [
        "No Gallery specimen or browser evidence: keyboard, narrow and theme behavior are unverified.",
        "It declares tree and treeitem roles without arrow-key tree navigation; assistive-technology behavior is unproven.",
        "A tree of health rows, not a node-link graph: no edges between siblings, graph layout or graph navigation.",
        "Parent status is not rolled up from children; the app supplies each node's status."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": ""
    },
    {
      "id": "content.explainer",
      "name": "Self-contained explainer document",
      "kind": "primitive",
      "summary": "Assemble a single offline HTML explainer page with inline styles, SVG figures, cards, tables and an optional export toolbar.",
      "tasks": [
        "explainer",
        "single file explainer page",
        "offline concept document",
        "shareable design explainer"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "none at runtime: the document inlines its own CSS and the export toolbar script"
      ],
      "source": {
        "path": "appkit/ui/explainer/explainer.go",
        "symbols": [
          "Render",
          "Doc",
          "Section",
          "Figure",
          "Card",
          "Table"
        ]
      },
      "usage": "Build an explainer.Doc of explainer.Section values and call explainer.Render for a complete HTML document; compose bodies with explainer.Figure, explainer.SVG, explainer.Grid, explainer.Card and explainer.Table. Doc.Lead, Doc.Foot, Section.Intro and Section.Body are trusted HTML; other text is escaped. The document inlines its own dark design and reduced-motion escape instead of the kit theme and motion layers.",
      "states": [
        "sections",
        "figures and tables",
        "export toolbar",
        "reduced motion"
      ],
      "app_owns": [
        "content truth and review of trusted HTML",
        "file delivery",
        "branding accents"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/explainer"
      ],
      "gaps": [
        "No Gallery specimen or browser evidence.",
        "The fixed dark design and hex accents do not follow the app's token theme.",
        "Trusted HTML fields require caller review."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": ""
    },
    {
      "id": "content.export-toolbar",
      "name": "Offline export toolbar",
      "kind": "primitive",
      "summary": "Add Save as PDF and Save HTML buttons as one inline script that works in themed pages and standalone documents.",
      "tasks": [
        "save as PDF",
        "save HTML copy",
        "offline export toolbar",
        "print to PDF"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "browser JavaScript and the browser print dialog",
        "CSP that permits the inline script"
      ],
      "source": {
        "path": "appkit/ui/export/export.go",
        "symbols": [
          "Toolbar",
          "Options"
        ]
      },
      "usage": "Append export.Toolbar with export.Options before the closing body tag; the zero value enables both buttons. Options select the PDF and HTML buttons, page width, file name and a hex hover accent. The toolbar injects its own inline styles and hides itself in print output.",
      "states": [
        "both buttons",
        "PDF only",
        "HTML only",
        "printing"
      ],
      "app_owns": [
        "what the page exposes when saved",
        "file names",
        "script policy"
      ],
      "maturity": "experimental",
      "proof": [
        "go test ./appkit/ui/export"
      ],
      "gaps": [
        "No Gallery specimen or browser evidence; PDF output depends on each browser's print engine.",
        "The accent is a literal hex color by contract, outside the token theme.",
        "Save HTML copies the current page, including any data rendered into it."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": ""
    },
    {
      "id": "content.details",
      "name": "Disclosures and description lists",
      "kind": "primitive",
      "summary": "Reveal optional detail behind a native show-more disclosure and present aligned term and detail metadata.",
      "tasks": [
        "show more disclosure",
        "collapsible section",
        "description list",
        "term and detail metadata",
        "key value details"
      ],
      "layers": [
        "L3b"
      ],
      "profiles": [
        "go-web"
      ],
      "dependencies": [
        "theme CSS",
        "appkit css source scanning"
      ],
      "source": {
        "path": "appkit/ui/chrome/disclosure.go",
        "symbols": [
          "RenderDisclosure",
          "Disclosure",
          "RenderDescriptionList",
          "RenderDescriptionListCard",
          "DescItem"
        ]
      },
      "usage": "Call chrome.RenderDisclosure with chrome.Disclosure for a native details section, and chrome.RenderDescriptionList or chrome.RenderDescriptionListCard with chrome.DescItem terms and details. Disclosure.Body and DescItem.DetailHTML are trusted HTML; other text is escaped. No JavaScript; scan appkit/ui/chrome/*.go. Breadcrumbs, tabs, pagination and empty states are the separate interaction.chrome capability.",
      "states": [
        "collapsed",
        "expanded",
        "empty list",
        "card"
      ],
      "app_owns": [
        "content and review of trusted HTML",
        "open-state persistence",
        "term wording"
      ],
      "maturity": "available",
      "proof": [
        "go test ./appkit/ui/chrome"
      ],
      "gaps": [
        "Open state is not persisted across navigation.",
        "Trusted HTML fields require caller review."
      ],
      "provenance": {
        "source": "first-party",
        "license": "Project-private use; no redistribution grant asserted",
        "audience": "repository consumers"
      },
      "consumers": [],
      "preview": "/agentic"
    }
  ]
}
