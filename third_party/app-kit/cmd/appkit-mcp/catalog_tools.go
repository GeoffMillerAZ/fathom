package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type capabilitySearchInput struct {
	Query    string `json:"query,omitempty" jsonschema:"task phrase; all words must match one declared task or the capability ID/name; explanatory prose is not searched"`
	Layer    string `json:"layer,omitempty" jsonschema:"exact web layer: L0, L1, L2, L3a, L3b, L4; omit for terminal capabilities"`
	Profile  string `json:"profile,omitempty" jsonschema:"exact profile: portable, go-web, static-web, react-web, cli"`
	Kind     string `json:"kind,omitempty" jsonschema:"token, primitive, recipe, or reference"`
	Maturity string `json:"maturity,omitempty" jsonschema:"experimental, available, or proven; available does not mean a complete consumer workflow was verified"`
	Limit    int    `json:"limit,omitempty" jsonschema:"page size from 1 to 50; defaults to 10"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor from the prior page; retain the same query and filters"`
}

type capabilityGetInput struct {
	ID string `json:"id" jsonschema:"exact stable capability id returned by search_capabilities"`
}

type capabilityOutput struct {
	Revision string        `json:"revision"`
	Entry    catalog.Entry `json:"entry"`
}

// registerCatalogTools serves the same embedded, validated records as the Gallery.
// It intentionally does not load or expose any licensed component corpus.
func registerCatalogTools(srv *mcp.Server) error {
	c, err := catalog.Default()
	if err != nil {
		return fmt.Errorf("load first-party catalog: %w", err)
	}
	openWorld := false
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_capabilities",
		Description: "Start here to find first-party app-kit capabilities by user task and compatible profile/layer. Returns bounded summaries with stable IDs and catalog revision, never commercial source. Valid filters with no compatible capability return an empty page; unknown filters fail. Read get_capability for integration costs, app-owned seams, proof commands and known gaps before adopting.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in capabilitySearchInput) (*mcp.CallToolResult, catalog.Page, error) {
		page, err := c.Search(catalog.Query{Text: in.Query, Layer: in.Layer, Profile: in.Profile, Kind: in.Kind, Maturity: in.Maturity, Limit: in.Limit, Cursor: in.Cursor})
		if err != nil {
			return nil, catalog.Page{}, err
		}
		body, err := json.Marshal(page)
		if err != nil {
			return nil, catalog.Page{}, err
		}
		return textResult(string(body)), page, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_capability",
		Description: "Read one first-party capability by exact stable ID. Returns its source symbols, usage, dependencies, behavior states, app-owned responsibilities, provenance and known proof gaps at the catalog revision. A proof command is a reproducible entrypoint, not a claim it ran on this consumer or host. Commercial references use get_component instead.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in capabilityGetInput) (*mcp.CallToolResult, capabilityOutput, error) {
		entry, ok := c.Get(in.ID)
		if !ok {
			return nil, capabilityOutput{}, fmt.Errorf("unknown capability %q; use search_capabilities for exact IDs", in.ID)
		}
		out := capabilityOutput{Revision: c.Revision, Entry: entry}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, capabilityOutput{}, err
		}
		return textResult(string(body)), out, nil
	})
	body, err := json.Marshal(c)
	if err != nil {
		return err
	}
	const uri = "kit://catalog"
	srv.AddResource(&mcp.Resource{URI: uri, Name: "First-party capability catalog", Description: "Versioned first-party records shared with the Gallery. Prefer bounded search_capabilities for discovery; this full snapshot is for explicit inventory and comparison.", MIMEType: "application/json", Size: int64(len(body)), Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant", "user"}}}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(body)}}}, nil
	})
	return nil
}
