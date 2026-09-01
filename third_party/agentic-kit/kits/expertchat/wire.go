package expertchat

import (
	"encoding/json"
	"fmt"
)

// Wire shapes for HTTPRetriever's module-host client — kept in this one
// small file per the founding plan's instruction ("keep the wire shapes in
// one small file — they mirror fathom's spec, cite it in a comment").
//
// # Reconciled 2026-07-28 — this is no longer an assumption
//
// This file previously documented its payload shape as expertchat's OWN
// guess at what fathom's `/rpc/modules` surface would look like once built.
// It is now the PINNED, final module_query v0 contract
// (docs/plans/gke-module-host.md, Phase G / GKE-MH-5), reconciled against
// fathom's own implementation of the same tool (fathom is aligning its side
// to the identical field names, with a round-trip test of its own — so
// drift between the two repos would show up as a decode failure here, not
// silently). Update this file's comment (and moduleQueryToolName below) the
// day that changes; nothing else does.
//
// # The envelope (unchanged from the pre-reconciliation revision)
//
// This is a JSON-RPC 2.0 envelope. Fathom's HTTP MCP server dispatches
// exactly three methods — "initialize", "tools/list", "tools/call"
// (sextant/internal/mcp/dispatch.go's registerBuiltins) — there is no
// bespoke per-feature verb space; every tool invocation goes through
// "tools/call" with {name, arguments} params, and every successful result
// comes back wrapped in the MCP content envelope
// {"content":[{"type":"text","text":"<json>"}]} (dispatch.go's
// handleToolsCallWithCtx, the final json.Marshal(result)+wrap). This file's
// rpcRequest/rpcResponse/rpcError/toolsCallParams/mcpToolResult types encode
// exactly that shape.
//
// # The module_query v0 payload (pinned 2026-07-28)
//
// Request arguments (moduleQueryArguments): {"text": string REQUIRED,
// "module": string optional, "max": int optional}. "text" is the free-text
// query; "module" scopes to one mounted module (empty searches every
// mounted module, the federated case, docs/vision/
// 12-governed-knowledge-engine.md §3); "max" caps result count.
//
// Result, once unwrapped from the content envelope (modulesQueryResult):
// {"results": [wireModuleResult, ...]} — one entry PER QUERIED MODULE, not
// one entry per claim. Each wireModuleResult carries that module's identity
// (module/service/watermark) plus its claims:
//
//   - "claims": [{"claim_id","text","src","anchor","tier","status","kind"}]
//     — the actual citable evidence. One EvidenceItem per claim entry
//     (wireClaim.toEvidenceItem), inheriting its containing result's
//     module/watermark. "anchor" may be legitimately empty (legacy
//     hash-only claims, per fathom's own store format) — validCiteIdentity
//     does not require it, only Module/ClaimID/Tier. "status": "" means
//     FRESH per fathom's serving semantics (not "unknown") — normalized to
//     StatusFresh right here at decode time, so nothing downstream ever
//     sees an empty Status. "service" and "kind" are decoded but not yet
//     mapped onto EvidenceItem (no target field names them in the pinned
//     contract) — reserved for a future use, never silently dropped from
//     the wire type even though unused today.
//   - "nodes": informational — symbols are NOT claims and are never treated
//     as citable evidence. Decoded as opaque json.RawMessage and discarded;
//     see wireModuleResult.Nodes.
//   - "staleness": not yet surfaced as a Citation-level detail. Decoded as
//     opaque json.RawMessage and currently unused — reserved, not a guess
//     at a future shape.
//   - "error": per-MODULE degradation (e.g. one federated module is
//     unreachable while others answer normally) — a module reporting error
//     is skipped (its claims, if any, are excluded — an errored module's
//     data is not trustworthy enough to cite) WITHOUT failing the whole
//     Retrieve call, UNLESS every queried module errored, in which case
//     Retrieve returns an aggregate error (nothing usable came back at
//     all). See httpretriever.go's Retrieve for the exact accounting.
//
// ModuleVersion is set to "" for every EvidenceItem produced here: the
// module manifest loader does not exist yet in fathom (docs/spec/
// expert-module-v0.md §7's documented honest gap — "no manifest loader
// ships in this repo"), so there is no manifest `version` field to read.
// The field is kept (not removed) because the manifest loader landing is
// the documented future path, not a redesign.

// moduleQueryToolName is the tool this client invokes via tools/call to
// query the module host. Pinned 2026-07-28 to fathom's decided name
// (docs/plans/gke-module-host.md, GKE-MH-5).
const moduleQueryToolName = "module_query"

// rpcRequest is a JSON-RPC 2.0 request envelope.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is a JSON-RPC 2.0 response envelope. Exactly one of Result and
// Error is set on a well-formed response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolsCallParams is the "tools/call" request payload — fathom's ONLY
// dispatch verb for invoking a tool (sextant/internal/mcp/dispatch.go's
// handleToolsCallWithCtx: `{Name string; Arguments json.RawMessage}`).
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// mcpContentItem is one element of the MCP content envelope.
type mcpContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// mcpToolResult is the envelope every successful "tools/call" result is
// wrapped in (dispatch.go: `{"content":[{"type":"text","text":"<json>"}]}`).
// The actual tool result is JSON-encoded as a STRING inside Content[0].Text
// — it must be unmarshaled a second time to reach modulesQueryResult.
type mcpToolResult struct {
	Content []mcpContentItem `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

// firstText returns the text of the first "text" content item, or an error
// if none is present — an empty or missing content array is a malformed
// tool result, never silently treated as "no items."
func (r mcpToolResult) firstText() (string, error) {
	for _, c := range r.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("tool result content has no text item")
}

// moduleQueryArguments is the "arguments" object passed to moduleQueryToolName
// via tools/call — the pinned module_query v0 request shape.
type moduleQueryArguments struct {
	// Text is the free-text query. REQUIRED on the wire (no omitempty: an
	// empty Text would silently encode as an absent field, which the module
	// host's REQUIRED validation should reject loudly rather than expertchat
	// hiding the omission client-side).
	Text string `json:"text"`

	// Module scopes the query to one mounted module. Empty searches every
	// module the host has mounted (the federated case, docs/vision/
	// 12-governed-knowledge-engine.md §3).
	Module string `json:"module,omitempty"`

	// Max caps the number of items returned. 0 means "no limit."
	Max int `json:"max,omitempty"`
}

// modulesQueryResult is the tool's result payload, once unwrapped from the
// MCP content envelope: one wireModuleResult per queried module, not one per
// claim.
type modulesQueryResult struct {
	Results []wireModuleResult `json:"results"`
}

// wireModuleResult is one queried module's contribution: its identity
// (module/service/watermark), the claims it returned, and (see the file
// doc) nodes/staleness/error fields that are decoded but not folded into
// EvidenceItem the same way claims are.
type wireModuleResult struct {
	// Module is the module identifier every claim in Claims inherits as
	// EvidenceItem.Module.
	Module string `json:"module"`

	// Service is decoded but not currently mapped onto EvidenceItem — no
	// field in the pinned contract names a target for it. Reserved.
	Service string `json:"service"`

	// Watermark is this module's corpus watermark, inherited by every claim
	// in Claims as EvidenceItem.Watermark.
	Watermark string `json:"watermark"`

	// Claims are this module's citable claims — the only source of
	// EvidenceItem values this type produces.
	Claims []wireClaim `json:"claims"`

	// Nodes is informational only (symbol/graph data): a module_query
	// result carries it, but symbols are NOT claims and this client never
	// treats Nodes as citable evidence. Decoded as opaque JSON so a
	// malformed Nodes payload doesn't fail decoding of the Claims that
	// matter, and intentionally never converted to anything else.
	Nodes json.RawMessage `json:"nodes,omitempty"`

	// Staleness carries module-level staleness detail beyond each claim's
	// own Status. Not yet surfaced anywhere (reserved for a future
	// Citation-level detail field) — decoded opaquely, never guessed at.
	Staleness json.RawMessage `json:"staleness,omitempty"`

	// Error is non-empty when THIS module failed to answer (per-module
	// degradation in a federated query) — see Retrieve's accounting: a
	// module reporting Error is skipped, not fatal, unless every queried
	// module errored.
	Error string `json:"error,omitempty"`
}

// wireClaim is one claim on the wire, within a wireModuleResult. Fields not
// carrying Module/ModuleVersion/Watermark directly — those are inherited
// from the containing wireModuleResult by toEvidenceItem.
type wireClaim struct {
	ClaimID string `json:"claim_id"`
	Text    string `json:"text"`
	Src     string `json:"src"`
	Anchor  string `json:"anchor"`
	Tier    string `json:"tier"`
	Status  string `json:"status"`

	// Kind is decoded but not currently mapped onto EvidenceItem — no field
	// in the pinned contract names a target for it. Reserved.
	Kind string `json:"kind"`
}

// toEvidenceItem converts a wire claim to the package's domain type, given
// the module/watermark of its containing wireModuleResult. Status "" is
// normalized to StatusFresh here — fathom's serving semantics document an
// empty status as fresh, not "unknown" — so this is the ONE place that
// translation happens; EvidenceItem.Status is documented as never empty for
// a well-formed value (types.go), and every other code path in this package
// relies on that already being true by the time an EvidenceItem exists.
func (c wireClaim) toEvidenceItem(module, watermark string) EvidenceItem {
	status := c.Status
	if status == "" {
		status = StatusFresh
	}
	return EvidenceItem{
		Module: module,
		// ModuleVersion is always "" from this wire path — see the file doc,
		// "the module manifest loader does not exist yet."
		ModuleVersion: "",
		Watermark:     watermark,
		ClaimID:       c.ClaimID,
		Tier:          c.Tier,
		Status:        status,
		SourceRef:     c.Src,
		Anchor:        c.Anchor,
		Text:          c.Text,
	}
}
