// planscontext.go — WF-2 (docs/vision/24-work-frontier.md §2): "fathom
// plans context <plan.md> --row <id>" renders one row's ctx: discovery
// pack FRESH against the live index, at read time. Never stored, never
// gating (verify never reads Ctx — see internal/verify's own doc comment
// on Task.Ctx). A pull tool, not a gate: once the row itself is found,
// this command always exits 0 — every resolution failure (an unreachable
// index, a moved/renamed pointer) is named inline in the rendered pack
// instead of failing the command.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// defaultContextBudget is docs/design/budgeted-answers.md §3's "composite"
// tool default (symbol_context, pack_context) — this command is exactly
// that shape: the point of a context pack is breadth.
const defaultContextBudget = 4000

func cmdPlansContext(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: plans context requires a plan file argument")
		printPlansContextUsage(stderr)
		return 1
	}
	planFile := args[0]

	fs := newFlagSet("plans context", stderr)
	row := fs.String("row", "", "row id to render (required)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	budget := fs.Int("budget", defaultContextBudget, "byte budget for the rendered pack (elided beyond this)")
	preflight := fs.Bool("preflight", false, "also print the row's task preflight: difficulty profile and advisory tier recommendation (WI-16, WI-17; needs --store)")
	score := fs.Bool("score", false, "with --preflight: also ask the decision port for its Score (WI-18; only with [decide] configured and the preflight site allowed)")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if *row == "" {
		fmt.Fprintln(stderr, "error: --row is required")
		fs.Usage()
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}

	raw, err := os.ReadFile(planFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading plan %q: %v\n", planFile, err)
		return 1
	}
	plan := verify.ParseMarkdownPlan(*service, raw)
	var task *verify.Task
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == *row {
			task = &plan.Tasks[i]
			break
		}
	}
	if task == nil {
		fmt.Fprintf(stderr, "error: row %q not found in %q\n", *row, planFile)
		return 1
	}

	idx, idxErr := openCtxIndex(*service, *storePath, *url)
	segs := renderContextPack(*task, planFile, idx, idxErr)
	fmt.Fprint(stdout, budgetContextPack(segs, *budget))
	if *preflight || *score {
		runPreflight(context.Background(), *task, planFile, *service, *storePath, *score, stdout, stderr)
	}
	return 0
}

func printPlansContextUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom plans context <plan.md> --row <id> --service S [--store DB | --url URL] [--budget N] [--preflight [--score]]")
}

// ─── pointer resolution ─────────────────────────────────────────────────────

// ctxIndex resolves a ctx: symbol pointer against the live index. This is
// the seam TestPlansContextRendersFreshAndNamesStalePointers exercises
// without a live daemon: direct-store mode (storeCtxIndex) IS the fake —
// the same seeding pattern every other plans test already uses
// (seedPlansFixtureStore + --store).
type ctxIndex interface {
	resolveSymbol(pkgHint, name string) (symbolnav.Context, bool)
}

// openCtxIndex opens the index in direct-store mode when storePath is
// set, else probes the daemon over HTTP (the same dual-mode shape every
// other plans subcommand uses). A non-nil error means the index itself
// could not be reached — operational, and never fatal to the command: the
// pack renderer reports it per pointer and the command still exits 0.
func openCtxIndex(service, storePath, urlStr string) (ctxIndex, error) {
	if storePath != "" {
		return newStoreCtxIndex(service, storePath)
	}
	return newHTTPCtxIndex(service, urlStr)
}

// storeCtxIndex resolves against a directly-opened store — nodes and the
// call graph are built once per command invocation.
type storeCtxIndex struct {
	nodes []core.SymbolNode
	graph *callgraph.Graph
}

func newStoreCtxIndex(service, storePath string) (*storeCtxIndex, error) {
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		return nil, fmt.Errorf("opening store %q: %w", storePath, err)
	}
	defer c.Close()
	ctx := context.Background()
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("listing symbols for %q: %w", service, err)
	}
	graph := callgraph.Build(nodes)
	// Typed edges are additive (ADR-021 §3); same graph as the daemon (INC-21).
	if typed, typedErr := c.TypedEdges(ctx, service); typedErr == nil {
		callgraph.AddTypedEdges(graph, typed)
	}
	return &storeCtxIndex{nodes: nodes, graph: graph}, nil
}

// resolveSymbol narrows to nodes under pkgHint (a repo-relative
// directory — the same convention the plan grammar's own package: field
// uses) when that narrowing leaves at least one candidate; otherwise it
// falls back to the full node set, so a stale/renamed package hint never
// hides a symbol that still resolves unambiguously by bare name.
func (idx *storeCtxIndex) resolveSymbol(pkgHint, name string) (symbolnav.Context, bool) {
	candidates := idx.nodes
	if pkgHint != "" {
		if filtered := underDirNodes(idx.nodes, pkgHint); len(filtered) > 0 {
			candidates = filtered
		}
	}
	sctx := symbolnav.SymbolContext(candidates, idx.graph, name)
	return sctx, sctx.Def.Name != ""
}

// underDirNodes returns the nodes whose FilePath lies under repo-relative
// directory dir — mirrors internal/verify's own underDir (unexported
// there, and this package must not edit internal/verify to reuse it).
func underDirNodes(nodes []core.SymbolNode, dir string) []core.SymbolNode {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	var out []core.SymbolNode
	for _, n := range nodes {
		if strings.HasPrefix(n.FilePath, prefix) {
			out = append(out, n)
		}
	}
	return out
}

// httpCtxIndex resolves via the daemon's symbol_context RPC, one call per
// pointer — the same tool the MCP surface exposes, called the way
// verifyViaHTTP/daemonServiceIndexed already call the daemon (verify.go).
type httpCtxIndex struct {
	hc      *http.Client
	baseURL string
	service string
}

func newHTTPCtxIndex(service, urlStr string) (*httpCtxIndex, error) {
	hc := &http.Client{Timeout: 10 * time.Second}
	indexed, err := daemonServiceIndexed(hc, urlStr, service)
	if err != nil {
		return nil, fmt.Errorf("probing daemon for service %q: %w", service, err)
	}
	if !indexed {
		return nil, fmt.Errorf("service %q is not indexed on the daemon at %s", service, urlStr)
	}
	return &httpCtxIndex{hc: hc, baseURL: urlStr, service: service}, nil
}

// resolveSymbol calls symbol_context with the bare name only — the RPC
// tool has no package-directory filter parameter, so pkgHint is unused in
// HTTP mode (direct-store mode is the more precise seam; this is the
// fallback when only a daemon is available).
func (idx *httpCtxIndex) resolveSymbol(_ string, name string) (symbolnav.Context, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name":      "symbol_context",
			"arguments": map[string]any{"service": idx.service, "symbol": name, "snippet": false},
		},
	})
	if err != nil {
		return symbolnav.Context{}, false
	}
	req, err := originHeader(idx.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return symbolnav.Context{}, false
	}
	resp, err := idx.hc.Do(req)
	if err != nil {
		return symbolnav.Context{}, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return symbolnav.Context{}, false
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return symbolnav.Context{}, false
	}
	var payload struct {
		Def *struct {
			Name      string `json:"name"`
			FilePath  string `json:"file_path"`
			Line      int    `json:"line"`
			EndLine   int    `json:"end_line"`
			Signature string `json:"signature"`
		} `json:"def"`
		Callers []string `json:"callers"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil || payload.Def == nil || payload.Def.Name == "" {
		return symbolnav.Context{}, false
	}
	return symbolnav.Context{
		Def: core.SymbolNode{
			Name: payload.Def.Name, FilePath: payload.Def.FilePath,
			Line: payload.Def.Line, EndLine: payload.Def.EndLine, Signature: payload.Def.Signature,
		},
		Callers: payload.Callers,
	}, true
}

// resolveFilePointer tries ptr as a file path — first relative to the
// process's working directory (the common case: an agent's cwd is the
// repo root), then relative to the plan file's own directory. Returns the
// line count and true on success.
func resolveFilePointer(planFile, ptr string) (lines int, ok bool) {
	candidates := []string{ptr}
	if !filepath.IsAbs(ptr) {
		candidates = append(candidates, filepath.Join(filepath.Dir(planFile), ptr))
	}
	for _, c := range candidates {
		if n, err := countLines(c); err == nil {
			return n, true
		}
	}
	return 0, false
}

// countLines reads path and returns its line count. This is a cheap
// existence+size probe, not a source viewer — no cap is needed because a
// ctx: file pointer names one source file, never a directory or a binary
// blob worth guarding against.
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	return strings.Count(string(data), "\n") + 1, nil
}

// splitPointer splits a "pkg.Name" ctx pointer on its LAST dot — the same
// convention the plan grammar's package: field uses (a repo-relative
// directory), so "internal/verify.ParseMarkdownPlan" splits into
// ("internal/verify", "ParseMarkdownPlan"). No dot: the whole pointer is
// tried as a bare symbol name with no package hint.
func splitPointer(ptr string) (pkgHint, name string) {
	if i := strings.LastIndex(ptr, "."); i >= 0 {
		return ptr[:i], ptr[i+1:]
	}
	return "", ptr
}

// ─── rendering ───────────────────────────────────────────────────────────────

// renderContextPack renders task's pack as a slice of segments: segment 0
// is always the row header (id/title/reason/trigger) and survives any
// budget; each remaining segment is one ctx: pointer's render, in
// document order — a resolved symbol (signature, file:line, direct
// callers), a resolved file (line count), or "unresolved" naming the
// pointer that moved/renamed/couldn't be reached.
func renderContextPack(t verify.Task, planFile string, idx ctxIndex, idxErr error) []string {
	var head strings.Builder
	fmt.Fprintf(&head, "row %s: %s\n", t.ID, t.Title)
	if t.Reason != "" {
		fmt.Fprintf(&head, "reason: %s\n", t.Reason)
	}
	if t.Trigger != "" {
		fmt.Fprintf(&head, "trigger: %s\n", t.Trigger)
	}
	segs := []string{head.String()}

	if len(t.Ctx) == 0 {
		return append(segs, "ctx: (none — this row carries no ctx: pointers)\n")
	}

	for _, ptr := range t.Ctx {
		var b strings.Builder
		if lines, ok := resolveFilePointer(planFile, ptr); ok {
			fmt.Fprintf(&b, "\nctx %s: file, %d line(s)\n", ptr, lines)
			segs = append(segs, b.String())
			continue
		}
		if idxErr != nil {
			fmt.Fprintf(&b, "\nctx %s: unresolved (index unreachable: %v)\n", ptr, idxErr)
			segs = append(segs, b.String())
			continue
		}
		pkgHint, name := splitPointer(ptr)
		if sctx, ok := idx.resolveSymbol(pkgHint, name); ok {
			sig := sctx.Def.Signature
			if sig == "" {
				sig = "(no signature recorded)"
			}
			fmt.Fprintf(&b, "\nctx %s: symbol\n  signature: %s\n  at: %s:%d\n", ptr, sig, sctx.Def.FilePath, sctx.Def.Line)
			if len(sctx.Callers) == 0 {
				b.WriteString("  callers: (none)\n")
			} else {
				fmt.Fprintf(&b, "  callers: %s\n", strings.Join(sctx.Callers, ", "))
			}
			segs = append(segs, b.String())
			continue
		}
		fmt.Fprintf(&b, "\nctx %s: unresolved (moved/renamed since planning)\n", ptr)
		segs = append(segs, b.String())
	}
	return segs
}

// budgetContextPack joins segs up to budget bytes — a target, not a hard
// cap: the header segment (segs[0]) always survives even if it alone
// exceeds budget (docs/design/budgeted-answers.md §2 rule 1, "the
// smallest useful response exceeds the budget, return it and say so").
// Any segment left out is named in a trailing elision block —
// budgeted-answers.md §2's contract shape, rendered as plain text for
// this command's plain-markdown output — never silently dropped.
func budgetContextPack(segs []string, budget int) string {
	if len(segs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(segs[0])
	included := 1
	for _, s := range segs[1:] {
		if budget > 0 && b.Len()+len(s) > budget {
			break
		}
		b.WriteString(s)
		included++
	}
	total := len(segs) - 1 // ctx pointers only, excluding the header
	dropped := len(segs) - included
	if dropped > 0 {
		fmt.Fprintf(&b, "\n[elided: %d of %d ctx pointer(s) dropped — ordering: document order (ctx: field order) — re-run with a larger --budget to see the rest]\n",
			dropped, total)
	}
	return b.String()
}
