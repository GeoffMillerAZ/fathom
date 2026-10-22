package mcp

// Citation checks are point reads against the existing fingerprints table.
// A request keeps only receipts for the bytes its snippets actually used;
// it never enumerates a corpus or retains source bytes between requests.

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Keep point checks inside the handler's admission ticket, before the
// dispatch budget is applied and before settleCall releases the ticket.
func citationHandler(b QueryInterface, h func(context.Context, json.RawMessage) (any, *rpcError)) func(context.Context, json.RawMessage) (any, *rpcError) {
	return func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		result, rpcErr := h(ctx, args)
		if rpcErr != nil {
			return nil, rpcErr
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return result, nil
		}
		marked := markCitations(ctx, b, serviceOfArgs(args), raw)
		if bytes.Equal(marked, raw) {
			return result, nil
		}
		return json.RawMessage(marked), nil
	}
}

type sourceFingerprintBackend interface {
	SourceFingerprint(context.Context, string, string) (string, bool, error)
}

type citationStatus struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type citationReadKey struct{ service, file string }
type citationReadsKey struct{}
type citationReads struct {
	mu          sync.Mutex
	statuses    map[citationReadKey]citationStatus
	queue       *refresh.Queue
	service     string
	ingest      store.IngestState
	ingestFound bool
	initial     citationStatus
}

func newCitationReads(ctx context.Context, b QueryInterface, service string, q *refresh.Queue) *citationReads {
	r := &citationReads{statuses: make(map[citationReadKey]citationStatus), queue: q, service: service}
	r.initial = citationRefreshState(ctx, b, service)
	if q != nil {
		switch q.StateOf(service) {
		case refresh.StateQueued, refresh.StateRunning:
			r.initial = citationStatus{"refreshing", "service refresh is queued or in progress"}
		}
	}
	if ib, ok := b.(ingestStateBackend); ok {
		var err error
		r.ingest, r.ingestFound, err = ib.IngestState(ctx, service)
		if err != nil {
			r.initial = citationStatus{"unknown", "service refresh state could not be read"}
		}
	}
	return r
}

func citationTool(name string) bool {
	switch name {
	case "find_symbol", "symbol_context", "obligations", "review_diff":
		return true
	}
	return false
}

func indexedSourceFingerprint(ctx context.Context, b QueryInterface, service, file string) (string, bool) {
	fp, ok := b.(sourceFingerprintBackend)
	if !ok {
		return "", false
	}
	h, found, err := fp.SourceFingerprint(ctx, service, file)
	return h, err == nil && found && h != ""
}

func citationRefreshState(ctx context.Context, b QueryInterface, service string) citationStatus {
	reads, _ := ctx.Value(citationReadsKey{}).(*citationReads)
	if reads != nil && reads.service == service && reads.initial.State != "" {
		return reads.initial
	}
	if reads != nil && reads.queue != nil {
		switch reads.queue.StateOf(service) {
		case refresh.StateQueued, refresh.StateRunning:
			return citationStatus{"refreshing", "service refresh is queued or in progress"}
		}
	}
	if ib, ok := b.(ingestStateBackend); ok {
		row, found, err := ib.IngestState(ctx, service)
		if err != nil {
			return citationStatus{"unknown", "service refresh state could not be read"}
		}
		if found && row.State == store.IngestInProgress {
			return citationStatus{"refreshing", "service ingest is in progress"}
		}
		if reads != nil && reads.service == service && (found != reads.ingestFound || row != reads.ingest) {
			return citationStatus{"refreshing", "service ingest identity changed during the answer"}
		}
	}
	return citationStatus{}
}

func checkedCitation(ctx context.Context, b QueryInterface, service, file, before string, known bool, data []byte, initial citationStatus) citationStatus {
	if initial.State != "" {
		return initial
	}
	if current := citationRefreshState(ctx, b, service); current.State != "" {
		return current
	}
	after, afterKnown := indexedSourceFingerprint(ctx, b, service, file)
	if !known || !afterKnown {
		return citationStatus{"unknown", "indexed source identity is unavailable"}
	}
	if before != after {
		return citationStatus{"refreshing", "indexed source identity changed during the read"}
	}
	if fusion.FingerprintHash(data) != before {
		return citationStatus{"stale", "file changed since the index; read_source serves the current text"}
	}
	return citationStatus{}
}

func recordCitationRead(ctx context.Context, service, file string, status citationStatus) {
	if reads, _ := ctx.Value(citationReadsKey{}).(*citationReads); reads != nil {
		reads.mu.Lock()
		defer reads.mu.Unlock()
		key := citationReadKey{service, file}
		// An uncertain earlier snippet cannot be cleared by a later read.
		if old, ok := reads.statuses[key]; !ok || old.State == "" {
			reads.statuses[key] = status
		}
	}
}

func citationForFile(ctx context.Context, b QueryInterface, service, file string) citationStatus {
	if reads, _ := ctx.Value(citationReadsKey{}).(*citationReads); reads != nil {
		reads.mu.Lock()
		status, found := reads.statuses[citationReadKey{service, file}]
		reads.mu.Unlock()
		if found {
			if current := citationRefreshState(ctx, b, service); current.State != "" {
				return current
			}
			return status
		}
	}
	// Same containment, suppression and 2 MiB ceiling as snippets, without
	// allocating line arrays for an identity-only point check.
	_, status, err := readSourceBytes(ctx, b, service, file, true)
	if err != nil {
		return citationStatus{"unknown", "current source bytes could not be read"}
	}
	return status
}

func citationFile(m map[string]any) string {
	for _, field := range []string{"src", "citation"} {
		if ref, ok := m[field].(string); ok {
			if file, _, _, ok := parseCitation(ref); ok {
				return file
			}
			if file, line, found := strings.Cut(ref, ":"); found {
				if n, err := strconv.Atoi(line); err == nil && n > 0 {
					return file
				}
			}
		}
	}
	for _, field := range []string{"file_path", "file", "path"} {
		if file, ok := m[field].(string); ok && file != "" {
			for _, key := range []string{"line", "start_line", "end_line"} {
				if line, ok := m[key].(float64); ok && line > 0 {
					return file
				}
			}
		}
	}
	// SARIF's physicalLocation carries path and region in sibling objects.
	if region, ok := m["region"].(map[string]any); ok && region["startLine"] != nil {
		if artifact, ok := m["artifactLocation"].(map[string]any); ok {
			file, _ := artifact["uri"].(string)
			return file
		}
	}
	return ""
}

// markCitations leaves clean JSON byte-identical. Only cited paths in the
// already-budgeted response are checked, in deterministic member order.
func markCitations(ctx context.Context, b QueryInterface, service string, raw []byte) []byte {
	var root any
	if json.Unmarshal(raw, &root) != nil {
		return raw
	}
	changed := false
	var visit func(any, string)
	visit = func(v any, svc string) {
		switch m := v.(type) {
		case []any:
			for _, item := range m {
				visit(item, svc)
			}
		case map[string]any:
			if own, ok := m["service"].(string); ok && own != "" {
				svc = own
			}
			keys := make([]string, 0, len(m))
			for key := range m {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				visit(m[key], svc)
			}
			if file := citationFile(m); file != "" {
				if status := citationForFile(ctx, b, svc, file); status.State != "" {
					m["citation_status"] = status
					changed = true
				}
			}
			if def, ok := m["def"].(map[string]any); ok {
				if status, ok := def["citation_status"]; ok {
					m["citation_status"] = status
				}
			}
		}
	}
	visit(root, service)
	if !changed {
		return raw
	}
	out, err := json.Marshal(root)
	if err != nil {
		return raw
	}
	return out
}
