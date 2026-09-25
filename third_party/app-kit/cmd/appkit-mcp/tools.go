package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
)

// ---- TailwindPlus component tools ----------------------------------------

type searchInput struct {
	Query           string `json:"query,omitempty" jsonschema:"optional keyword or natural-language query matched against taxonomy, names and classes; omit or use empty text for bounded filter-only discovery"`
	Section         string `json:"section,omitempty" jsonschema:"optional filter: Application UI, Ecommerce, or Marketing"`
	Framework       string `json:"framework,omitempty" jsonschema:"optional filter: html, react, or vue"`
	TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"optional filter: 3 or 4"`
	Mode            string `json:"mode,omitempty" jsonschema:"optional filter: light, dark, system, or none (Ecommerce)"`
	Limit           int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 20)"`
	Source          string `json:"source,omitempty" jsonschema:"optional licensed corpus: \"tailwindplus\" (default) or \"matdash\" (React dashboard templates); use each result's actual Tailwind version and mode"`
}

type searchOutput struct {
	Results []index.SearchResult `json:"results"`
	Count   int                  `json:"count"`
}

type getInput struct {
	FullName        string `json:"full_name" jsonschema:"the dotted taxonomy name, e.g. 'Marketing.Page Sections.Pricing Sections.Three tiers'"`
	Framework       string `json:"framework,omitempty" jsonschema:"html, react, or vue (recommended; omitting may return any framework)"`
	TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"3 or 4 (recommended)"`
	Mode            string `json:"mode,omitempty" jsonschema:"light, dark, system, or none"`
	Source          string `json:"source,omitempty" jsonschema:"optional licensed corpus: \"tailwindplus\" (default) or \"matdash\"; use framework, tailwind_version and mode from search results"`
}

type listInput struct {
	Section         string `json:"section,omitempty" jsonschema:"optional filter: Application UI, Ecommerce, or Marketing"`
	Framework       string `json:"framework,omitempty" jsonschema:"optional filter: html, react, or vue"`
	TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"optional filter: 3 or 4"`
	Mode            string `json:"mode,omitempty" jsonschema:"optional filter: light, dark, system, or none"`
	Limit           int    `json:"limit,omitempty" jsonschema:"maximum components to list (default 200)"`
	Offset          int    `json:"offset,omitempty" jsonschema:"zero-based continuation offset; keep the same filters and server session; default 0"`
}

type listOutput struct {
	Components []index.ComponentSummary `json:"components"`
	Count      int                      `json:"count"`
	Offset     int                      `json:"offset"`
	NextOffset int                      `json:"next_offset,omitempty"`
	HasMore    bool                     `json:"has_more"`
}

type sectionsInput struct {
	Section string `json:"section,omitempty" jsonschema:"optional: list groups within this section; omit to list all sections"`
}

type sectionsOutput struct {
	Sections []index.Facet `json:"sections,omitempty"`
	Groups   []index.Facet `json:"groups,omitempty"`
}

func validateComponentAxes(section, framework string, tailwindVersion int, mode string) error {
	if section != "" && section != "Application UI" && section != "Ecommerce" && section != "Marketing" {
		return fmt.Errorf("unknown section %q; choose Application UI, Ecommerce, or Marketing", section)
	}
	if framework != "" && framework != "html" && framework != "react" && framework != "vue" {
		return fmt.Errorf("unknown framework %q; choose html, react, or vue", framework)
	}
	if tailwindVersion != 0 && tailwindVersion != 3 && tailwindVersion != 4 {
		return fmt.Errorf("unknown tailwind_version %d; choose 3 or 4", tailwindVersion)
	}
	if mode != "" && mode != "light" && mode != "dark" && mode != "system" && mode != "none" {
		return fmt.Errorf("unknown mode %q; choose light, dark, system, or none", mode)
	}
	return nil
}

func validateComponentLimit(limit, maximum int) error {
	if limit < 0 || limit > maximum {
		return fmt.Errorf("limit must be 1–%d when set", maximum)
	}
	return nil
}

func registerComponentTools(srv *mcp.Server, ix *index.Index, sem *semantic, mdix *index.Index) {
	closedWorld := false
	openWorld := true
	localReadOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}
	searchReadOnly := localReadOnly
	if sem != nil {
		// Hybrid search may call the configured Ollama endpoint before falling
		// back to the local BM25 index, so it is not closed-world in this mode.
		searchReadOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}
	}
	corpus := func(source string) (*index.Index, error) {
		switch source {
		case "", "tailwindplus":
			if ix == nil {
				return nil, fmt.Errorf("TailwindPlus index unavailable; set -index / APPKIT_TWPLUS_INDEX to a valid index or run `appkit data build` with your licensed source; first-party get_kit_guidance remains available")
			}
			return ix, nil
		case "matdash":
			if mdix == nil {
				return nil, fmt.Errorf("MatDash index unavailable; set -matdash-index / APPKIT_MATDASH_INDEX to a valid index or run `appkit data matdash-build` with your licensed source; first-party get_kit_guidance remains available")
			}
			return mdix, nil
		default:
			return nil, fmt.Errorf("unknown component source %q; choose tailwindplus or matdash", source)
		}
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "search_components",
		Description: "Search a component corpus by intent or keywords with optional structural filters. " +
			"Pass source=\"tailwindplus\" (default) to search the TailwindPlus licensed catalog (html/react/vue, Tailwind 3+4, multiple modes). " +
			"Pass source=\"matdash\" to search the MatDash dashboard-template corpus (React — sidebars, topbars, widget grids, dashboard pages); use the Tailwind version and mode returned by the index. " +
			"TailwindPlus ranking is BM25 — or hybrid (BM25+vector RRF) when the embeddings artifact is present. " +
			"MatDash ranking is always BM25. Both corpora are optional licensed references; an absent source returns a tool error. " +
			"Returns at most 20 ranked components; follow up with get_component (same source) for the exact code. Score is BM25 (more negative = stronger; 0 for vector-only hits).",
		Annotations: searchReadOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
		if err := validateComponentAxes(in.Section, in.Framework, in.TailwindVersion, in.Mode); err != nil {
			return nil, searchOutput{}, err
		}
		if err := validateComponentLimit(in.Limit, 20); err != nil {
			return nil, searchOutput{}, err
		}
		selected, err := corpus(in.Source)
		if err != nil {
			return nil, searchOutput{}, err
		}
		filters := index.Filters{
			Section: in.Section, Framework: in.Framework, TailwindVersion: in.TailwindVersion, Mode: in.Mode,
		}
		var res []index.SearchResult
		if in.Source == "matdash" {
			res, err = selected.SearchComponents(in.Query, filters, in.Limit)
			if err != nil {
				return nil, searchOutput{}, err
			}
			var b strings.Builder
			for _, r := range res {
				fmt.Fprintf(&b, "[matdash] %-58s  %s/tw%d/%s\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
			}
			if len(res) == 0 {
				b.WriteString("(no matches — try broader keywords or drop a filter)\n")
			}
			return textResult(b.String()), searchOutput{Results: res, Count: len(res)}, nil
		}
		// Empty text is structural/filter-only discovery, even when optional
		// embeddings are configured; it must not turn a local browse into a
		// network call or semantic-only ranking.
		if strings.TrimSpace(in.Query) == "" {
			res, err = selected.SearchComponents(in.Query, filters, in.Limit)
		} else if sem != nil {
			res, err = sem.search(ctx, selected, in.Query, filters, in.Limit)
		} else {
			res, err = selected.SearchComponents(in.Query, filters, in.Limit)
		}
		if err != nil {
			return nil, searchOutput{}, err
		}
		var b strings.Builder
		for _, r := range res {
			fmt.Fprintf(&b, "%-58s  %s/tw%d/%s\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
		}
		if len(res) == 0 {
			b.WriteString("(no matches — try broader keywords or drop a filter)\n")
		}
		return textResult(b.String()), searchOutput{Results: res, Count: len(res)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_component",
		Description: "Return the full source code for one component variant by its full_name and (recommended) framework/tailwind_version/mode. " +
			"Pass source=\"tailwindplus\" (default) for TailwindPlus components; pass source=\"matdash\" for MatDash dashboard templates, using the framework, Tailwind version and mode returned by search. Both licensed sources are optional; an absent source returns a tool error. " +
			"If the axes leave more than one variant, returns the first and lists every available variant so you can refine.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, *index.GetResult, error) {
		if err := validateComponentAxes("", in.Framework, in.TailwindVersion, in.Mode); err != nil {
			return nil, nil, err
		}
		selected, err := corpus(in.Source)
		if err != nil {
			return nil, nil, err
		}
		if in.Source == "matdash" {
			r, err := selected.GetComponent(in.FullName, in.Framework, in.TailwindVersion, in.Mode)
			if err != nil {
				return nil, nil, err
			}
			header := fmt.Sprintf("// [matdash] %s  [%s · tailwind v%d · %s]\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
			return textResult(header + r.Code), r, nil
		}
		// Default: TailwindPlus path (unchanged)
		r, err := selected.GetComponent(in.FullName, in.Framework, in.TailwindVersion, in.Mode)
		if err != nil {
			return nil, nil, err
		}
		header := fmt.Sprintf("// %s  [%s · tailwind v%d · %s]\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
		return textResult(header + r.Code), r, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_components",
		Description: "Browse the TailwindPlus catalog structurally (no text query) with optional filters. Returns at most 200 component summaries with variant counts; when has_more is true, continue at next_offset with the same filters and server session. Offset paging assumes the read-only index is unchanged for that session; it is not snapshot certification.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, listOutput, error) {
		if err := validateComponentAxes(in.Section, in.Framework, in.TailwindVersion, in.Mode); err != nil {
			return nil, listOutput{}, err
		}
		if err := validateComponentLimit(in.Limit, 200); err != nil {
			return nil, listOutput{}, err
		}
		effectiveLimit := in.Limit
		if effectiveLimit == 0 {
			effectiveLimit = 200
		}
		if in.Offset < 0 || in.Offset > math.MaxInt-effectiveLimit-1 {
			return nil, listOutput{}, fmt.Errorf("offset must be non-negative and leave room for a bounded page")
		}
		selected, err := corpus("tailwindplus")
		if err != nil {
			return nil, listOutput{}, err
		}
		res, err := selected.ListComponents(index.Filters{
			Section: in.Section, Framework: in.Framework, TailwindVersion: in.TailwindVersion, Mode: in.Mode,
		}, effectiveLimit+1, in.Offset)
		if err != nil {
			return nil, listOutput{}, err
		}
		out := listOutput{Offset: in.Offset}
		if len(res) > effectiveLimit {
			out.HasMore = true
			res = res[:effectiveLimit]
			out.NextOffset = in.Offset + len(res)
		}
		out.Components = res
		out.Count = len(res)
		var b strings.Builder
		for _, c := range res {
			fmt.Fprintf(&b, "%-64s  (%d variants)\n", c.FullName, c.NumVariant)
		}
		if len(res) == 0 {
			b.WriteString("(no components match — drop a filter)\n")
		}
		if out.HasMore {
			fmt.Fprintf(&b, "(more available — continue with offset %d and the same filters)\n", out.NextOffset)
		}
		return textResult(b.String()), out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sections",
		Description: "List the optional TailwindPlus catalog taxonomy: top-level sections with component counts, or the groups within a given section.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in sectionsInput) (*mcp.CallToolResult, sectionsOutput, error) {
		var out sectionsOutput
		if err := validateComponentAxes(in.Section, "", 0, ""); err != nil {
			return nil, out, err
		}
		selected, err := corpus("tailwindplus")
		if err != nil {
			return nil, out, err
		}
		var b strings.Builder
		if in.Section == "" {
			secs, err := selected.Sections()
			if err != nil {
				return nil, out, err
			}
			out.Sections = secs
			for _, s := range secs {
				fmt.Fprintf(&b, "%-16s %d\n", s.Value, s.Count)
			}
		} else {
			groups, err := selected.Groups(in.Section)
			if err != nil {
				return nil, out, err
			}
			out.Groups = groups
			for _, g := range groups {
				fmt.Fprintf(&b, "%-32s %d\n", g.Value, g.Count)
			}
		}
		if b.Len() == 0 {
			b.WriteString("(no matches)\n")
		}
		return textResult(b.String()), out, nil
	})
}

// ---- preview tool ---------------------------------------------------------

type getPreviewInput struct {
	FullName        string `json:"full_name" jsonschema:"the dotted taxonomy name, e.g. 'Marketing.Page Sections.Pricing Sections.Three tiers'"`
	Framework       string `json:"framework,omitempty" jsonschema:"html, react, or vue"`
	TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"3 or 4"`
	Mode            string `json:"mode,omitempty" jsonschema:"light, dark, system, or none"`
	Offset          int    `json:"offset,omitempty" jsonschema:"zero-based byte offset at a UTF-8 boundary; default 0"`
	MaxBytes        int    `json:"max_bytes,omitempty" jsonschema:"maximum UTF-8-safe bytes to return; default 32768, maximum 65536"`
	ExpectedSHA256  string `json:"expected_sha256,omitempty" jsonschema:"optional full-preview SHA-256 from the prior page; stale or malformed values are refused"`
}

type previewMeta struct {
	FullName        string `json:"full_name"`
	Framework       string `json:"framework"`
	TailwindVersion int    `json:"tailwind_version"`
	Mode            string `json:"mode"`
	Bytes           int    `json:"bytes"` // total bytes in the immutable full preview
	Offset          int    `json:"offset"`
	ReturnedBytes   int    `json:"returned_bytes"`
	NextOffset      int    `json:"next_offset,omitempty"`
	HasMore         bool   `json:"has_more"`
	SHA256          string `json:"sha256"`
}

const (
	defaultPreviewPageBytes = 32 << 10
	maxPreviewPageBytes     = 64 << 10
)

func previewPage(r *index.PreviewResult, in getPreviewInput) (string, previewMeta, error) {
	meta := previewMeta{FullName: r.FullName, Framework: r.Framework, TailwindVersion: r.TailwindVersion,
		Mode: r.Mode, Bytes: len(r.HTML)}
	if !utf8.ValidString(r.HTML) {
		return "", previewMeta{}, fmt.Errorf("preview contains invalid UTF-8; rebuild the previews artifact")
	}
	sum := sha256.Sum256([]byte(r.HTML))
	meta.SHA256 = fmt.Sprintf("%x", sum)
	if in.ExpectedSHA256 != "" {
		if len(in.ExpectedSHA256) != 64 || strings.Trim(in.ExpectedSHA256, "0123456789abcdef") != "" {
			return "", previewMeta{}, fmt.Errorf("expected_sha256 must be 64 lowercase hexadecimal characters")
		}
		if in.ExpectedSHA256 != meta.SHA256 {
			return "", previewMeta{}, fmt.Errorf("expected_sha256 does not match the current preview; restart at offset 0")
		}
	}
	if in.Offset < 0 || in.Offset >= len(r.HTML) {
		return "", previewMeta{}, fmt.Errorf("offset must be between 0 and %d", len(r.HTML)-1)
	}
	if !utf8.RuneStart(r.HTML[in.Offset]) {
		return "", previewMeta{}, fmt.Errorf("offset %d is inside a UTF-8 encoding; use a next_offset returned by this tool", in.Offset)
	}
	pageBytes := in.MaxBytes
	if pageBytes == 0 {
		pageBytes = defaultPreviewPageBytes
	}
	if pageBytes < 1 || pageBytes > maxPreviewPageBytes {
		return "", previewMeta{}, fmt.Errorf("max_bytes must be 1–%d", maxPreviewPageBytes)
	}
	end := min(in.Offset+pageBytes, len(r.HTML))
	for end > in.Offset && end < len(r.HTML) && !utf8.RuneStart(r.HTML[end]) {
		end--
	}
	if end == in.Offset {
		return "", previewMeta{}, fmt.Errorf("max_bytes %d cannot include the next complete UTF-8 encoding", pageBytes)
	}
	meta.Offset = in.Offset
	meta.ReturnedBytes = end - in.Offset
	meta.HasMore = end < len(r.HTML)
	if meta.HasMore {
		meta.NextOffset = end
	}
	return r.HTML[in.Offset:end], meta, nil
}

func registerPreviewTool(srv *mcp.Server, pv *index.PreviewStore, path string) {
	openWorld := false
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_component_preview",
		Description: "Read a byte-exact UTF-8 page of the standalone preview HTML for one component variant. Defaults to 32 KiB, maximum 64 KiB; continue with next_offset and expected_sha256. The structured result reports total and returned bytes; the original HTML slice is the text content.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getPreviewInput) (*mcp.CallToolResult, previewMeta, error) {
		if err := validateComponentAxes("", in.Framework, in.TailwindVersion, in.Mode); err != nil {
			return nil, previewMeta{}, err
		}
		if pv == nil {
			return nil, previewMeta{}, fmt.Errorf("previews not built — run `appkit data previews` (expected at %s)", path)
		}
		r, err := pv.GetPreview(in.FullName, in.Framework, in.TailwindVersion, in.Mode)
		if err != nil {
			return nil, previewMeta{}, err
		}
		body, meta, err := previewPage(r, in)
		if err != nil {
			return nil, previewMeta{}, err
		}
		return textResult(body), meta, nil
	})
}

// ---- app-kit guidance tools ----------------------------------------------

type emptyInput struct{}

type decisionListOutput struct {
	Decisions []decisionMeta `json:"decisions"`
}

type decisionMeta struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type getDecisionInput struct {
	ID string `json:"id" jsonschema:"a document id such as 'intent', 'intent/scope', 'adoption', or 'experience-system'; or an ADR id or slug such as '0005', '5', or 'loopback-invariant'"`
}

type docOutput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type themingInput struct {
	Classes string `json:"classes,omitempty" jsonschema:"optional Tailwind class string to preserve and review; omit for the adaptation guide; no automatic palette rewrite or dark-class removal"`
}

type themingOutput struct {
	Guide     string     `json:"guide,omitempty"`     // returned when no classes were given
	Rewritten string     `json:"rewritten,omitempty"` // the exact original class string
	Verdicts  []retarget `json:"verdicts,omitempty"`  // keep verdicts with optional review advice
}

func registerKitTools(srv *mcp.Server, docs *kitDocs) {
	openWorld := false
	localReadOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_guidance",
		Description: "Return how to build on app-kit: the project-intent router, consumer adoption workflow, and binding conventions. Select the app's target, adopted layers, and applicable runtime profile before generating code; stack-specific rules apply only to that profile. Read this before generating kit-app code.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, docOutput, error) {
		body := docs.guidance()
		return textResult(body), docOutput{Title: "app-kit guidance", Body: body}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_kit_decisions",
		Description: "List the app-kit Architecture Decision Records (ADRs) by id and title. Use get_kit_decision to read the reasoning behind any one.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, decisionListOutput, error) {
		var out decisionListOutput
		var b strings.Builder
		for _, d := range docs.decisions {
			out.Decisions = append(out.Decisions, decisionMeta{ID: d.ID, Title: d.Title})
			fmt.Fprintf(&b, "%s  %s\n", d.ID, d.Title)
		}
		if len(docs.decisions) == 0 {
			b.WriteString("(no ADRs found — set -kit-root to the app-kit checkout)\n")
		}
		return textResult(b.String()), out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_decision",
		Description: "Return a current app-kit intent document, adoption guide, or ADR. Start with id='intent' for the router, then use 'intent/principles', 'intent/invariants', 'intent/domain', 'intent/scope', 'intent/futures', 'intent/questions', 'intent/autonomy', 'intent/incidents', or 'intent/repo'. Use 'adoption' for the consumer workflow, 'experience-system' for experience design, 'ui-components' for implemented UI packages, and 'cli-kit' for terminal conventions. ADRs resolve by id or slug (e.g. '0010', 'component-data-pipeline'); 'decisions', 'vision', and 'hazards' also resolve.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getDecisionInput) (*mcp.CallToolResult, docOutput, error) {
		d, ok := docs.findDecision(in.ID)
		if !ok {
			return nil, docOutput{}, fmt.Errorf("no kit document matches %q; start with get_kit_guidance for document ids or list_kit_decisions for ADR ids; check -kit-root if a documented id is unavailable", in.ID)
		}
		return textResult(d.Body), docOutput{Title: d.Title, Body: d.Body}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_theming",
		Description: "Review a design reference for app-kit while preserving the original design. Call with no arguments for the adaptation guide, or pass classes for keep verdicts and explicit Review notes; rewritten preserves the exact original string. No automatic palette rewrite or dark-class removal: semantic intent, text-on-fill contrast and distinct interaction states need the consumer's explicit role mapping. Source-reference selection and renderer selection are separate decisions; build/runtime rules apply to the selected layers/profile. get_component stays verbatim.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in themingInput) (*mcp.CallToolResult, themingOutput, error) {
		if strings.TrimSpace(in.Classes) == "" {
			guide := themingGuide()
			return textResult(guide), themingOutput{Guide: guide}, nil
		}
		verdicts, rewritten := RetargetClassList(in.Classes)
		var b strings.Builder
		fmt.Fprintf(&b, "Original classes preserved (no automatic rewrite):\n%s\n\n", rewritten)
		for _, v := range verdicts {
			switch v.Action {
			case actionMap:
				fmt.Fprintf(&b, "  %-28s → %s\n", v.Original, v.Result)
			case actionDrop:
				fmt.Fprintf(&b, "  %-28s ✗ drop (%s)\n", v.Original, v.Note)
			case actionKeep:
				if v.Note != "" {
					fmt.Fprintf(&b, "  %-28s keep — %s\n", v.Original, v.Note)
				}
			}
		}
		return textResult(b.String()), themingOutput{Rewritten: rewritten, Verdicts: verdicts}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_css_setup",
		Description: "How to wire app-kit's CSS into a consuming project so the kit's role utilities (bg-accent, text-ink, rounded-lg) actually render: the no-Node build (appkit gen → theme.css, appkit css), the input.css contract, the @source scanning requirement, the full utility vocabulary the kit emits, and the data-theme light/dark contract. Read this if an adapted component's classes appear to do nothing, or before standing up a new kit app's CSS.",
		Annotations: localReadOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, docOutput, error) {
		body := cssSetupGuide()
		return textResult(body), docOutput{Title: "app-kit CSS setup", Body: body}, nil
	})

	// Expose each ADR as an MCP resource as well, so hosts that browse
	// resources surface them (D11: decisions as resources).
	for _, d := range docs.decisions {
		srv.AddResource(&mcp.Resource{
			URI:         "kit://adr/" + d.ID,
			Name:        "ADR " + d.ID + ": " + d.Title,
			Description: "app-kit architecture decision record " + d.ID,
			MIMEType:    "text/markdown",
			Size:        int64(len(d.Body)),
			Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant", "user"}},
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
				{URI: "kit://adr/" + d.ID, MIMEType: "text/markdown", Text: d.Body},
			}}, nil
		})
	}

	// Intent and adoption use the same loaded bodies through tools and resources.
	// Keep registration stable and expose only the explicit loadKitDocs allowlist.
	ids := make([]string, 0, len(docs.extra))
	for id := range docs.extra {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := docs.extra[id]
		uri := "kit://" + id
		mimeType := "text/markdown"
		if id == "quality-inventory" || id == "quality-schema" {
			mimeType = "text/plain"
		}
		srv.AddResource(&mcp.Resource{
			URI: uri, Name: d.Title, Description: "app-kit document " + id, MIMEType: mimeType,
			Size:        int64(len(d.Body)),
			Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant", "user"}},
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
				{URI: uri, MIMEType: mimeType, Text: d.Body},
			}}, nil
		})
	}
}

// textResult wraps human-readable text as the unstructured tool content;
// the structured Out value rides alongside it in StructuredContent.
func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
