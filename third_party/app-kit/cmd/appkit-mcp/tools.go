package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
)

// ---- TailwindPlus component tools ----------------------------------------

type searchInput struct {
	Query           string `json:"query" jsonschema:"keyword or natural-language query matched against component taxonomy, names, and Tailwind utility classes"`
	Section         string `json:"section,omitempty" jsonschema:"optional filter: Application UI, Ecommerce, or Marketing"`
	Framework       string `json:"framework,omitempty" jsonschema:"optional filter: html, react, or vue"`
	TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"optional filter: 3 or 4"`
	Mode            string `json:"mode,omitempty" jsonschema:"optional filter: light, dark, system, or none (Ecommerce)"`
	Limit           int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 20)"`
	Source          string `json:"source,omitempty" jsonschema:"corpus to search: \"tailwindplus\" (default) for the TailwindPlus licensed catalog, or \"matdash\" for the MatDash dashboard-template corpus (React+Tailwind v4, mode=none)"`
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
	Source          string `json:"source,omitempty" jsonschema:"corpus to fetch from: \"tailwindplus\" (default) or \"matdash\" (MatDash dashboard templates; variants use framework=react, tailwind_version=4, mode=none)"`
}

type listInput struct {
	Section         string `json:"section,omitempty" jsonschema:"optional filter: Application UI, Ecommerce, or Marketing"`
	Framework       string `json:"framework,omitempty" jsonschema:"optional filter: html, react, or vue"`
	TailwindVersion int    `json:"tailwind_version,omitempty" jsonschema:"optional filter: 3 or 4"`
	Mode            string `json:"mode,omitempty" jsonschema:"optional filter: light, dark, system, or none"`
	Limit           int    `json:"limit,omitempty" jsonschema:"maximum components to list (default 200)"`
}

type listOutput struct {
	Components []index.ComponentSummary `json:"components"`
	Count      int                      `json:"count"`
}

type sectionsInput struct {
	Section string `json:"section,omitempty" jsonschema:"optional: list groups within this section; omit to list all sections"`
}

type sectionsOutput struct {
	Sections []index.Facet `json:"sections,omitempty"`
	Groups   []index.Facet `json:"groups,omitempty"`
}

func registerComponentTools(srv *mcp.Server, ix *index.Index, sem *semantic, mdix *index.Index) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "search_components",
		Description: "Search a component corpus by intent or keywords with optional structural filters. " +
			"Pass source=\"tailwindplus\" (default) to search the TailwindPlus licensed catalog (html/react/vue, Tailwind 3+4, multiple modes). " +
			"Pass source=\"matdash\" to search the MatDash dashboard-template corpus (React+Tailwind v4, mode=none — sidebars, topbars, widget grids, dashboard pages). " +
			"TailwindPlus ranking is BM25 — or hybrid (BM25+vector RRF) when the embeddings artifact is present. " +
			"MatDash ranking is always BM25. " +
			"Returns ranked components; follow up with get_component (same source) for the exact code. Score is BM25 (more negative = stronger; 0 for vector-only hits).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
		filters := index.Filters{
			Section: in.Section, Framework: in.Framework, TailwindVersion: in.TailwindVersion, Mode: in.Mode,
		}
		var (
			res []index.SearchResult
			err error
		)
		if in.Source == "matdash" {
			if mdix == nil {
				return nil, searchOutput{}, fmt.Errorf("MatDash index not available — run `appkit data matdash-build` to build it")
			}
			res, err = mdix.SearchComponents(in.Query, filters, in.Limit)
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
		// Default: TailwindPlus path (unchanged)
		if sem != nil {
			res, err = sem.search(ctx, ix, in.Query, filters, in.Limit)
		} else {
			res, err = ix.SearchComponents(in.Query, filters, in.Limit)
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
			"Pass source=\"tailwindplus\" (default) for TailwindPlus components; pass source=\"matdash\" for MatDash dashboard templates (framework=react, tailwind_version=4, mode=none). " +
			"If the axes leave more than one variant, returns the first and lists every available variant so you can refine.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, *index.GetResult, error) {
		if in.Source == "matdash" {
			if mdix == nil {
				return nil, nil, fmt.Errorf("MatDash index not available — run `appkit data matdash-build` to build it")
			}
			r, err := mdix.GetComponent(in.FullName, in.Framework, in.TailwindVersion, in.Mode)
			if err != nil {
				return nil, nil, err
			}
			header := fmt.Sprintf("// [matdash] %s  [%s · tailwind v%d · %s]\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
			return textResult(header + r.Code), r, nil
		}
		// Default: TailwindPlus path (unchanged)
		r, err := ix.GetComponent(in.FullName, in.Framework, in.TailwindVersion, in.Mode)
		if err != nil {
			return nil, nil, err
		}
		header := fmt.Sprintf("// %s  [%s · tailwind v%d · %s]\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
		return textResult(header + r.Code), r, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_components",
		Description: "Browse the TailwindPlus catalog structurally (no text query) with optional filters. Returns component summaries with variant counts.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, listOutput, error) {
		res, err := ix.ListComponents(index.Filters{
			Section: in.Section, Framework: in.Framework, TailwindVersion: in.TailwindVersion, Mode: in.Mode,
		}, in.Limit)
		if err != nil {
			return nil, listOutput{}, err
		}
		var b strings.Builder
		for _, c := range res {
			fmt.Fprintf(&b, "%-64s  (%d variants)\n", c.FullName, c.NumVariant)
		}
		if len(res) == 0 {
			b.WriteString("(no components match — drop a filter)\n")
		}
		return textResult(b.String()), listOutput{Components: res, Count: len(res)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sections",
		Description: "List the catalog taxonomy: top-level sections with component counts, or the groups within a given section.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in sectionsInput) (*mcp.CallToolResult, sectionsOutput, error) {
		var out sectionsOutput
		var b strings.Builder
		if in.Section == "" {
			secs, err := ix.Sections()
			if err != nil {
				return nil, out, err
			}
			out.Sections = secs
			for _, s := range secs {
				fmt.Fprintf(&b, "%-16s %d\n", s.Value, s.Count)
			}
		} else {
			groups, err := ix.Groups(in.Section)
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
}

type previewMeta struct {
	FullName        string `json:"full_name"`
	Framework       string `json:"framework"`
	TailwindVersion int    `json:"tailwind_version"`
	Mode            string `json:"mode"`
	Bytes           int    `json:"bytes"`
}

func registerPreviewTool(srv *mcp.Server, pv *index.PreviewStore, path string) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_component_preview",
		Description: "Return the standalone preview HTML document (a rendered demo page) for one component variant, by full_name + framework/tailwind_version/mode. Large output, for visual reference. The structured result reports the size; the HTML itself is the text content.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getPreviewInput) (*mcp.CallToolResult, previewMeta, error) {
		if pv == nil {
			return nil, previewMeta{}, fmt.Errorf("previews not built — run `appkit data previews` (expected at %s)", path)
		}
		r, err := pv.GetPreview(in.FullName, in.Framework, in.TailwindVersion, in.Mode)
		if err != nil {
			return nil, previewMeta{}, err
		}
		meta := previewMeta{FullName: r.FullName, Framework: r.Framework, TailwindVersion: r.TailwindVersion, Mode: r.Mode, Bytes: len(r.HTML)}
		return textResult(r.HTML), meta, nil
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
	ID string `json:"id" jsonschema:"an ADR id or slug, e.g. '0005', '5', or 'loopback-invariant'"`
}

type docOutput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type themingInput struct {
	Classes string `json:"classes,omitempty" jsonschema:"optional space-separated Tailwind class string from a component; omit for the full adaptation guide"`
}

type themingOutput struct {
	Guide     string     `json:"guide,omitempty"`     // returned when no classes were given
	Rewritten string     `json:"rewritten,omitempty"` // the adapted class string
	Verdicts  []retarget `json:"verdicts,omitempty"`  // per-class decisions
}

func registerKitTools(srv *mcp.Server, docs *kitDocs) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_guidance",
		Description: "Return how to build on app-kit: the hard rules (no Node; CUE is the source of truth; the loopback invariant; apps import the kit) and the project conventions an agent must honor when writing code for a kit app. Read this before generating kit-app code.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, docOutput, error) {
		body := docs.guidance()
		return textResult(body), docOutput{Title: "app-kit guidance", Body: body}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_kit_decisions",
		Description: "List the app-kit Architecture Decision Records (ADRs) by id and title. Use get_kit_decision to read the reasoning behind any one.",
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
		Description: "Return the full text of one app-kit ADR by id or slug (e.g. '0010', 'component-data-pipeline').",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getDecisionInput) (*mcp.CallToolResult, docOutput, error) {
		d, ok := docs.findDecision(in.ID)
		if !ok {
			return nil, docOutput{}, fmt.Errorf("no ADR matches %q; call list_kit_decisions for valid ids", in.ID)
		}
		return textResult(d.Body), docOutput{Title: d.Title, Body: d.Body}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_theming",
		Description: "How to adapt a TailwindPlus component to app-kit's semantic token theme: map palette utilities (bg-indigo-600, text-gray-900) to kit role utilities (bg-accent, text-ink) and drop dark: color variants (the kit owns light/dark via tokens). Call with no arguments for the full guide, or pass `classes` (a space-separated class string from a component) to get a per-class rewrite. get_component stays verbatim — apply this to it.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in themingInput) (*mcp.CallToolResult, themingOutput, error) {
		if strings.TrimSpace(in.Classes) == "" {
			guide := themingGuide()
			return textResult(guide), themingOutput{Guide: guide}, nil
		}
		verdicts, rewritten := RetargetClassList(in.Classes)
		var b strings.Builder
		fmt.Fprintf(&b, "%s\n\n→ %s\n\n", in.Classes, rewritten)
		for _, v := range verdicts {
			switch v.Action {
			case actionMap:
				fmt.Fprintf(&b, "  %-28s → %s\n", v.Original, v.Result)
			case actionDrop:
				fmt.Fprintf(&b, "  %-28s ✗ drop (%s)\n", v.Original, v.Note)
			}
		}
		return textResult(b.String()), themingOutput{Rewritten: rewritten, Verdicts: verdicts}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_kit_css_setup",
		Description: "How to wire app-kit's CSS into a consuming project so the kit's role utilities (bg-accent, text-ink, rounded-lg) actually render: the no-Node build (appkit gen → theme.css, appkit css), the input.css contract, the @source scanning requirement, the full utility vocabulary the kit emits, and the data-theme light/dark contract. Read this if an adapted component's classes appear to do nothing, or before standing up a new kit app's CSS.",
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
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
				{URI: "kit://adr/" + d.ID, MIMEType: "text/markdown", Text: d.Body},
			}}, nil
		})
	}
}

// textResult wraps human-readable text as the unstructured tool content;
// the structured Out value rides alongside it in StructuredContent.
func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
