package mcp

// Citation receipts exist only for one admitted request. Index-sourced handlers
// bind typed fields explicitly; live-tree locations never enter this path.
import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type citationStatus = core.CitationStatus

type sourceFingerprintBackend interface {
	SourceFingerprint(context.Context, string, string) (string, bool, error)
}
type sourceFingerprintEpochBackend interface {
	SourceFingerprintEpoch(context.Context, string, string, string) (string, error)
}
type citationReadKey struct{ service, file string }

// Normalize harmless path aliases without legitimizing traversal citations.
func citationKey(service, file string) citationReadKey {
	for _, part := range strings.Split(file, "/") {
		if part == ".." {
			return citationReadKey{service, file}
		}
	}
	return citationReadKey{service, path.Clean(file)}
}

type citationReadsKey struct{}
type citationReadHookKey struct{} // injected after the byte read and before the second index read

type citationFileRead struct {
	status  citationStatus
	windows []*citationWindow
	read    bool
	indexed bool
}
type citationReads struct {
	mu                sync.Mutex
	files             map[citationReadKey]*citationFileRead
	queue             *refresh.Queue
	service           string
	ingest            store.IngestState
	ingestFound       bool
	initial           citationStatus
	pending           bool
	bindings          []func()
	noLocalSourceRoot bool
}

func citationHandler(b QueryInterface, h func(context.Context, json.RawMessage) (any, *rpcError)) func(context.Context, json.RawMessage) (any, *rpcError) {
	return func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		result, rpcErr := h(ctx, args)
		if reads, _ := ctx.Value(citationReadsKey{}).(*citationReads); reads != nil {
			reads.finish(ctx, b)
			if rpcErr == nil && reads.noLocalSourceRoot {
				var body map[string]any
				switch result := result.(type) {
				case map[string]any:
					body = result
				case summaryEnvelope:
					body, _ = result.body.(map[string]any)
				case *summaryEnvelope:
					body, _ = result.body.(map[string]any)
				}
				if body != nil {
					body["citation_freshness"] = "no local source root"
				}
			}
		}
		return result, rpcErr
	}
}
func newCitationReads(ctx context.Context, b QueryInterface, service string, q *refresh.Queue) *citationReads {
	r := &citationReads{files: make(map[citationReadKey]*citationFileRead), queue: q, service: service}
	// An index without a local tree has no current bytes to compare. Keep
	// this distinction request-local, and do not probe individual citations.
	if b != nil && service != "" {
		root, _, _, err := b.ServiceMeta(ctx, service)
		r.noLocalSourceRoot = err == nil && root == ""
	}
	if r.noLocalSourceRoot {
		return r
	}
	r.ingest, r.ingestFound, r.initial, r.pending = readCitationRefresh(ctx, b, service, q)
	return r
}
func readCitationRefresh(ctx context.Context, b QueryInterface, service string, q *refresh.Queue) (store.IngestState, bool, citationStatus, bool) {
	pending := false
	if q != nil {
		state := q.StateOf(service)
		pending = state == refresh.StateQueued || state == refresh.StateRunning
	}
	if ib, ok := b.(ingestStateBackend); ok {
		row, found, err := ib.IngestState(ctx, service)
		if err != nil {
			return row, found, citationStatus{State: "unknown", Reason: "service refresh state could not be read"}, pending
		}
		return row, found, citationStatus{}, pending || (found && row.State == store.IngestInProgress)
	}
	return store.IngestState{}, false, citationStatus{}, pending
}
func (r *citationReads) finish(ctx context.Context, b QueryInterface) {
	if r.noLocalSourceRoot {
		r.mu.Lock()
		defer r.mu.Unlock()
		clear(r.files)
		r.bindings = nil
		return
	}
	row, found, status, pending := readCitationRefresh(ctx, b, r.service, r.queue)
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := found != r.ingestFound || row != r.ingest
	for _, f := range r.files {
		switch {
		case changed:
			f.status = citationStatus{State: "refreshing", Reason: "service ingest identity changed during the answer"}
		case status.State != "":
			f.status = status
		case (pending || r.pending) && f.status.State == "unknown":
			f.status = citationStatus{State: "refreshing", Reason: "source check could not complete while refresh is pending"}
		}
	}
	for _, bind := range r.bindings {
		bind()
	}
	// A background navigation context may outlive the call: release every
	// window and output binding now, rather than waiting for that context.
	clear(r.files)
	r.bindings = nil
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
func checkedCitation(ctx context.Context, b QueryInterface, service, file, before string, known bool, hash string, epochKnown bool) citationStatus {
	after, afterKnown := indexedSourceFingerprint(ctx, b, service, file)
	if !known || !afterKnown {
		return citationStatus{State: "unknown", Reason: "indexed source identity is unavailable"}
	}
	if before != after {
		return citationStatus{State: "refreshing", Reason: "indexed source identity changed during the read"}
	}
	if hash != before {
		if !epochKnown {
			return citationStatus{State: "unknown", Reason: "indexed content identity has no extractor epoch receipt"}
		}
		return citationStatus{State: "stale", Reason: "file changed since the index; read_source serves the current text"}
	}
	return citationStatus{}
}
func citationEpoch(ctx context.Context, b QueryInterface, service, file, before string) (string, bool) {
	if fp, ok := b.(sourceFingerprintEpochBackend); ok {
		epoch, err := fp.SourceFingerprintEpoch(ctx, service, file, before)
		if err != nil {
			return fusion.ExtractorEpoch, false
		}
		if epoch != "" {
			return epoch, true
		}
	}
	// A matching current-epoch digest proves equality even without a receipt;
	// a mismatch cannot distinguish changed bytes from an extractor-epoch bump.
	return fusion.ExtractorEpoch, false
}
func citationForFile(ctx context.Context, b QueryInterface, service, file string) citationStatus {
	if r, _ := ctx.Value(citationReadsKey{}).(*citationReads); r != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.file(ctx, b, service, file, true).status
	}
	_, status, _, err := readSourceBytes(ctx, b, service, file, true)
	if err != nil {
		return citationStatus{State: "unknown", Reason: "current source bytes could not be read"}
	}
	return status
}
func (r *citationReads) file(ctx context.Context, b QueryInterface, service, file string, indexed bool) *citationFileRead {
	key := citationKey(service, file)
	f := r.files[key]
	if f == nil {
		f = &citationFileRead{}
		r.files[key] = f
	}
	if r.noLocalSourceRoot {
		f.read = true
		return f
	}
	if indexed {
		if f.read && !f.indexed {
			f.status = citationStatus{State: "unknown", Reason: "source was read live without an index identity check"}
		}
		f.indexed = true
	}
	if !f.read {
		f.read = true
		_, status, errReadCitation := streamSourceFile(ctx, b, service, file, f.windows, f.indexed)
		f.status = status
		if errReadCitation != nil {
			f.status = citationStatus{State: "unknown", Reason: "current source bytes could not be read"}
		}
		if r.initial.State != "" {
			f.status = r.initial
		}
		if r.pending && f.status.State == "unknown" {
			f.status = citationStatus{State: "refreshing", Reason: "source check could not complete while refresh is pending"}
		}
	}
	return f
}

// bindCitation assigns the typed field immediately, then rechecks only the
// service ingest identity at call end. No JSON encoding/decoding is involved.
func bindCitation(ctx context.Context, b QueryInterface, service, file string, set func(*core.CitationStatus)) {
	if file == "" {
		return
	}
	if r, _ := ctx.Value(citationReadsKey{}).(*citationReads); r != nil && r.noLocalSourceRoot {
		set(nil)
		return
	}
	status := citationForFile(ctx, b, service, file)
	assign := func(s citationStatus) {
		if s.State == "" {
			set(nil)
		} else {
			copy := s
			set(&copy)
		}
	}
	assign(status)
	if r, _ := ctx.Value(citationReadsKey{}).(*citationReads); r != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.bindings = append(r.bindings, func() { assign(r.files[citationKey(service, file)].status) })
	}
}
func markSymbolDTOs(ctx context.Context, b QueryInterface, service string, nodes []symbolNodeDTO) []symbolNodeDTO {
	for i := range nodes {
		n := &nodes[i]
		if n.Line > 0 {
			bindCitation(ctx, b, service, n.FilePath, func(s *core.CitationStatus) { n.CitationStatus = s })
		}
	}
	return nodes
}

func withObligationSummary(ctx context.Context, summary string, body map[string]any) any {
	envelope := &summaryEnvelope{summary: summary, body: body}
	if r, _ := ctx.Value(citationReadsKey{}).(*citationReads); r != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.bindings = append(r.bindings, func() {
			rows, _ := body["rules"].([]any)
			if len(rows) == 0 {
				return
			}
			var status *core.CitationStatus
			switch row := rows[0].(type) {
			case *semantic.Obligation:
				status = row.CitationStatus
			case *obligationWithSnippet:
				status = row.CitationStatus
			}
			base := summary
			for _, state := range []string{"stale", "unknown", "refreshing"} {
				base = strings.TrimSuffix(base, " [citation "+state+"]")
			}
			if status != nil && (status.State == "stale" || status.State == "refreshing") {
				suffix := " [citation " + status.State + "]"
				runes := []rune(base)
				limit := summaryMaxChars - len(suffix)
				if len(runes) > limit {
					base = string(runes[:limit-1]) + "…"
				}
				envelope.summary = base + suffix
			} else {
				envelope.summary = base
			}
		})
	}
	return envelope
}

type symbolCandidate struct {
	CitationStatus *core.CitationStatus `json:"citation_status,omitempty"`
	File           string               `json:"file"`
	Line           int                  `json:"line"`
	SymbolKey      string               `json:"symbol_key"`
}

func citationCandidate(ctx context.Context, b QueryInterface, service string, n core.SymbolNode) *symbolCandidate {
	row := &symbolCandidate{File: n.FilePath, Line: n.Line, SymbolKey: n.Key()}
	if n.Line > 0 {
		bindCitation(ctx, b, service, n.FilePath, func(s *core.CitationStatus) { row.CitationStatus = s })
	}
	return row
}

type symbolSuggestion struct {
	CitationStatus *core.CitationStatus `json:"citation_status,omitempty"`
	FilePath       string               `json:"file_path"`
	Kind           string               `json:"kind"`
	Line           int                  `json:"line"`
	Name           string               `json:"name"`
}
