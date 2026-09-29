package expertchat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// HTTPRetriever is a Retriever that speaks the wire.go JSON-RPC contract to
// a module host over HTTP. See wire.go for exactly what wire contract is
// assumed and why.
//
// Safe for concurrent use: HTTPRetriever holds no mutable state after
// construction (http.Client itself is documented safe for concurrent use).
type HTTPRetriever struct {
	// URL is the module host's JSON-RPC endpoint (an ordinary MCP-shaped
	// HTTP endpoint, caller-supplied — expertchat never assumes or defaults
	// a host). Required.
	URL string

	// Module scopes every query to one mounted module. Empty (the default)
	// searches every module the host has mounted.
	Module string

	// HTTPClient performs the request. Nil means http.DefaultClient — the
	// one documented bare-default this kit allows, matching the same
	// "kits do this routinely" seam AGENTS.md's determinism rule permits.
	HTTPClient *http.Client
}

// httpClient returns r.HTTPClient, or http.DefaultClient when unset.
func (r *HTTPRetriever) httpClient() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return http.DefaultClient
}

// Retrieve performs one "tools/call" JSON-RPC call (invoking
// moduleQueryToolName) and returns the decoded items, sorted
// deterministically by (Module, ClaimID) — a network hop is not required to
// preserve any particular order, and the kit's determinism contract
// requires sorted outputs regardless of what the host returns them in.
func (r *HTTPRetriever) Retrieve(ctx context.Context, query string, limit int) ([]EvidenceItem, error) {
	if r.URL == "" {
		return nil, fmt.Errorf("expertchat: HTTPRetriever.URL must not be empty")
	}

	args, err := json.Marshal(moduleQueryArguments{Text: query, Module: r.Module, Max: limit})
	if err != nil {
		return nil, fmt.Errorf("expertchat: encode %s arguments: %w", moduleQueryToolName, err)
	}
	params, err := json.Marshal(toolsCallParams{Name: moduleQueryToolName, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("expertchat: encode tools/call params: %w", err)
	}
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("expertchat: encode JSON-RPC request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("expertchat: build HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("expertchat: module host request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("expertchat: read module host response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expertchat: module host returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("expertchat: decode JSON-RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("expertchat: module host error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var toolResult mcpToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("expertchat: decode tools/call result envelope: %w", err)
	}
	if toolResult.IsError {
		text, _ := toolResult.firstText()
		return nil, fmt.Errorf("expertchat: %s reported a tool error: %s", moduleQueryToolName, text)
	}
	text, err := toolResult.firstText()
	if err != nil {
		return nil, fmt.Errorf("expertchat: %s: %w", moduleQueryToolName, err)
	}

	var result modulesQueryResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("expertchat: decode %s result: %w", moduleQueryToolName, err)
	}

	// Per-module degradation accounting (module_query v0's "error" field,
	// pinned 2026-07-28): a module reporting Error is skipped — its claims
	// (if any) are excluded, since an errored module's data is not
	// trustworthy enough to cite — WITHOUT failing the whole call, unless
	// EVERY queried module errored, in which case nothing usable came back
	// at all and Retrieve fails loudly rather than silently returning an
	// empty result that looks identical to "the query legitimately matched
	// nothing."
	if len(result.Results) > 0 {
		allErrored := true
		for _, mr := range result.Results {
			if mr.Error == "" {
				allErrored = false
				break
			}
		}
		if allErrored {
			first := result.Results[0]
			return nil, fmt.Errorf("expertchat: %s: all %d queried module(s) errored; e.g. module=%q: %s",
				moduleQueryToolName, len(result.Results), first.Module, first.Error)
		}
	}

	var items []EvidenceItem
	for _, mr := range result.Results {
		if mr.Error != "" {
			continue // skipped-module degradation — see the accounting above
		}
		for _, c := range mr.Claims {
			items = append(items, c.toEvidenceItem(mr.Module, mr.Watermark))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Module != items[j].Module {
			return items[i].Module < items[j].Module
		}
		return items[i].ClaimID < items[j].ClaimID
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
