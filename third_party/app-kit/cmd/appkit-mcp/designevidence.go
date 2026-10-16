package main

import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type designEvidenceSearchInput struct {
	ArchiveID string `json:"archive_id,omitempty" jsonschema:"optional exact source archive ID"`
	SubjectID string `json:"subject_id,omitempty" jsonschema:"optional exact evidence subject ID"`
	Profile   string `json:"profile,omitempty" jsonschema:"optional exact target profile"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"continuation bound to the same filters, effective limit and revision"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, default 10 and maximum 50"`
}

type designEvidenceGetInput struct {
	ID string `json:"id" jsonschema:"exact evidence@SHA256 ID from search_design_evidence"`
}

func registerDesignEvidenceTools(srv *mcp.Server, root string) {
	openWorld := false
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}
	mcp.AddTool(srv, &mcp.Tool{Name: "search_design_evidence", Annotations: annotations,
		Description: "List private target-evidence Recorded metadata, not freshly checked evidence, current transformer output, semantic validation or target certification. Exact archive_id, subject_id and profile filters; default limit 10, max 50, cursor bound to filters and revision. Every recorded result remains verification=not-verified regardless of claimed pass statuses. Returns no source or evidence bytes and performs no writes or execution; use get_design_evidence for an exact historical integrity and original-membership check."},
		func(_ context.Context, _ *mcp.CallToolRequest, in designEvidenceSearchInput) (*mcp.CallToolResult, designevidence.Page, error) {
			catalog, err := designevidence.Load(root)
			if err != nil {
				return nil, designevidence.Page{}, fmt.Errorf("private design evidence unavailable: %w", err)
			}
			out, err := catalog.Search(designevidence.Query{ArchiveID: in.ArchiveID, SubjectID: in.SubjectID, Profile: in.Profile, Cursor: in.Cursor, Limit: in.Limit})
			if err != nil {
				return nil, out, err
			}
			return assetResult(out)
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "get_design_evidence", Annotations: annotations,
		Description: "Read one private historical target-evidence record after exact stored bundle integrity recheck and current original archive membership check. Returns contract, receipt, Recorded metadata, Rechecked integrity metadata and original_membership=matched, but no source/evidence bytes. It does not run a transformer or semantic verifier, broaden a profile, authenticate reviewers or certify compatibility; verification is always not-verified regardless of claimed pass statuses. Read-only: no store creation, writes, downloads or execution."},
		func(_ context.Context, _ *mcp.CallToolRequest, in designEvidenceGetInput) (*mcp.CallToolResult, designevidence.Detail, error) {
			out, err := designevidence.Read(root, in.ID)
			if err != nil {
				return nil, out, err
			}
			return assetResult(out)
		})
}
