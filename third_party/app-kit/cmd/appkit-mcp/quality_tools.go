package main

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/app-kit/internal/qualityplan"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type qualityListInput struct {
	CapabilityID string `json:"capability_id,omitempty" jsonschema:"optional exact first-party capability ID from search_capabilities; unknown IDs fail; a known capability can have no planned work"`
	Scope        string `json:"scope,omitempty" jsonschema:"core (default), on-demand (non-core), or all; selects authored work items, not the entire commercial catalog"`
}

type qualityGetInput struct {
	ID               string `json:"id" jsonschema:"exact work-item ID from list_quality_work, for example core.analytics"`
	ExpectedRevision string `json:"expected_revision,omitempty" jsonschema:"optional revision from list_quality_work; mismatch refuses retrieval instead of silently substituting a changed plan"`
}

// registerQualityTools projects the existing CUE-authored work queue. It neither
// executes quality gates nor validates evidence files on an adopting host.
func registerQualityTools(srv *mcp.Server) error {
	plan, err := qualityplan.Default()
	if err != nil {
		return err
	}
	openWorld := false
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_quality_work",
		Description: "List compact authored starter/JIT work items, optionally joined to an exact first-party capability. Core is the default; on-demand shows explicitly recorded non-core work, not every licensed component. Returns a content revision for get_quality_work. Planning states are not a readiness score, certification, or live evidence check. No commercial source, network, or evidence execution; at most 32 records and 64 KiB of JSON per representation.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in qualityListInput) (*mcp.CallToolResult, qualityplan.ListResult, error) {
		out, err := plan.List(in.CapabilityID, in.Scope)
		if err != nil {
			return nil, qualityplan.ListResult{}, err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, qualityplan.ListResult{}, err
		}
		return textResult(string(body)), out, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_quality_work",
		Description: "Get one exact authored work item: all seven enhancement standards, nine gate statuses, reasons, next steps, evidence references and recorded consumer integration statuses. Includes standard definitions and backlog policy from the same embedded CUE revision. Partial/failed/not-run remain distinct; selected, integrated and verified-scoped are not adopted; absent records do not mean no consumer exists. Referenced files, hashes and runtime claims are not checked by this lookup. Follow the JIT/contribution guides before improving or adopting. At most 128 KiB of JSON per representation; oversize results fail rather than truncate.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in qualityGetInput) (*mcp.CallToolResult, qualityplan.DetailResult, error) {
		out, err := plan.Get(in.ID, in.ExpectedRevision)
		if err != nil {
			return nil, qualityplan.DetailResult{}, err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, qualityplan.DetailResult{}, err
		}
		return textResult(string(body)), out, nil
	})
	return nil
}
