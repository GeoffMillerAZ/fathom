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
      "usage": "Read app-owned-styleguide guidance. Configure NewEngine with immutable source, approved font IDs and selected contrast checks; mount New at the configured base path. Gallery opts in with -theme-source and /theme-lab. Export is a concrete candidate snapshot, never an automatic source patch.",
      "states": ["source baseline", "candidate preview", "invalid edits", "failed or unknown contrast", "stale acknowledgement", "deterministic export", "reset"],
      "app_owns": ["source and selected kit identity", "authentication wrapper", "CSS delivery and scan coverage", "side-effect-free synthetic fixtures", "contrast pair selection", "review and explicit source application"],
      "maturity": "experimental",
      "proof": ["go test ./apps/styleguide", "go test ./apps/gallery", "go test ./apps/gallery/cmd/gallery"],
      "gaps": ["No real-consumer migration or reusable-widget promotion.", "Browser geometry, keyboard and rendered accessibility are unverified.", "Static system-font profile; no font/image assets or interactive scripts/forms."],
      "provenance": {"source": "first-party", "license": "Project-private use; no redistribution grant asserted", "audience": "repository consumers"},
      "consumers": [],
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
      "usage": "Call viz.Bars with labeled values and BarsOptions{ShowValue:true}; include units, period and a text/table alternative in the surrounding view.",
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
        "The primitive does not enforce analytical semantics; the app supplies them."
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
      "usage": "Pass stable events and TimelineOptions to viz.Timeline. Use the documented event filter for period/lane/search; retain an accessible event listing.",
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
        "SVG output alone does not implement keyboard drill-down or application query semantics."
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
        "analytics CSV export",
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
        "theme CSS and appkit css scanning of appkit/ui/analytics and appkit/ui/vizframe when using the optional RenderBars composition"
      ],
      "source": {
        "path": "appkit/ui/analytics/analytics.go",
        "symbols": ["Dataset", "Contract", "Window"]
      },
      "usage": "Build an analytics.Dataset from app-selected rows and call Dataset.Validate with an explicit clock. Exact implementation anchors: appkit/ui/analytics/analytics.go#Dataset.Validate, appkit/ui/analytics/render.go#RenderBars, and appkit/ui/analytics/csv.go#WriteCSV. Use analytics.WriteCSV for that selection or analytics.RenderBars for an optional chart/context/table composition; read appkit/ui/analytics/README.md. For the L3b renderer, call get_kit_css_setup, include the theme CSS, and scan appkit/ui/analytics/*.go plus appkit/ui/vizframe/*.go relative to the selected kit source root. Open /analytics/recipe for the exact single-dataset renderer and matching WriteCSV download; over-wide plot labels/values retain an explicitly explained table-only fallback. The Gallery /analytics journey composes separate current/prior datasets, lower-level bars, linked detail and its own rectangular CSV adapter; it is not the exact RenderBars renderer or a reusable query engine. The opt-in static checker compares a selected composition against an independent fixture; read appkit/ui/vizcheck/README.md for static-only evidence limits.",
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
        "RenderBars refuses negative values; CSV supports them. Presentation rounding and formula-safe text encoding are not a lossless source export.",
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
          "Button"
        ]
      },
      "usage": "Wrap controls with form.Field and use its help/error plumbing. Scan appkit/ui/form/*.go; keep persistence and validation policy in the app.",
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
      "usage": "Use plain-link chrome controls with app-owned URLs. Scan appkit/ui/chrome/*.go; preserve filter context in links.",
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
        "A skeleton does not implement loading, retry or state preservation."
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
        "prose"
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
      "usage": "Scan appkit/ui/docs/*.go; import prose.css when using its prose treatment. Supply stable heading IDs and focusable fragment targets. Gallery /content composes a native reading task with app-owned content, destination policy and synthetic recovery; it is not a public site generator or static-host delivery proof.",
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
      "usage": "Render table.Table with typed columns and rows. Use templ and scan both kit and app component sources; queries and sort URLs remain app-owned.",
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
        "No client virtualization or query backend is included."
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
    }
  ]
}
