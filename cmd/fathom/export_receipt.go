// export_receipt.go — `fathom export receipt` (EVX-7, docs/plans/
// portable-receipts.md; docs/vision/43-portable-receipts.md D3, D4, D11):
// a store-direct answer for one symbol, written as the core.Receipt JSON
// `fathom receipt verify` reads, with anchored facts beside it. Every
// exported receipt is built by receipt.BuildReceipt, the one allowlist
// constructor; this file only gathers what it is handed.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdExportReceipt implements `fathom export receipt --service S --symbol
// NAME [--kind symbol_location|callers] [--store PATH] [--no-excerpts]
// [--keep-service] [-o FILE]`. Anchors name the service's indexed commit; a symbol with no
// callers is an absence fact carrying the index's read set.
func cmdExportReceipt(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("export receipt", stderr)
	service := fs.String("service", "", "service ID (required)")
	symbol := fs.String("symbol", "", "the symbol: a bare name or a qualified key (required)")
	kind := fs.String("kind", string(core.ReceiptKindSymbolLocation), "what the receipt claims: symbol_location (where it is defined) or callers (who calls it)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	out := fs.String("o", "-", "output file (- is stdout)")
	noExcerpts := fs.Bool("no-excerpts", false, "keep digests only: no excerpt text leaves")
	keepService := fs.Bool("keep-service", false, "keep service names in the exported receipt (default: replace with @)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" || *symbol == "" {
		fmt.Fprintln(stderr, "error: export receipt requires --service and --symbol")
		fs.Usage()
		return 1
	}
	k := core.ReceiptKind(*kind)
	if k != core.ReceiptKindSymbolLocation && k != core.ReceiptKindCallers {
		fmt.Fprintf(stderr, "error: export receipt --kind %q: want symbol_location or callers\n", *kind)
		return 1
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	ctx := context.Background()
	in, err := exportReceiptInput(ctx, c, st, *service, *symbol, k)
	if err != nil {
		fmt.Fprintf(stderr, "error: export receipt: %v\n", err)
		return 1
	}
	in.NoExcerpts = *noExcerpts
	built, err := receipt.BuildReceipt(in)
	if err != nil {
		fmt.Fprintf(stderr, "error: export receipt: %v\n", err)
		return 1
	}
	receipt.RedactReceipt(&built, *keepService)
	data, err := json.MarshalIndent(built, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: export receipt: %v\n", err)
		return 1
	}
	data = append(data, '\n')
	if *out == "-" {
		_, _ = stdout.Write(data)
		return 0
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write %q: %v\n", *out, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s: %s receipt for %s, %d fact(s)\n", *out, built.Kind, built.Symbol, len(built.Facts))
	return 0
}

// exportReceiptInput gathers the receipt for symbol in service from the
// store, anchored at the service's indexed commit.
func exportReceiptInput(ctx context.Context, c *client.Client, st store.Store, service, symbol string, kind core.ReceiptKind) (receipt.ExportInput, error) {
	root, indexed, found, err := c.ServiceMeta(ctx, service)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	if !found || root == "" {
		return receipt.ExportInput{}, fmt.Errorf("service %q has no indexed root: ingest it first", service)
	}
	def, err := oneDefinition(ctx, c, service, symbol)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	exp, err := receipt.NewExporter(ctx, receipt.Run{
		Dir: root, Base: root, Rev: indexed, Service: service, Nodes: nodes,
		Stored: func(p string) (string, bool) {
			fp, ok, ferr := st.Fingerprint(ctx, service, p)
			return fp, ferr == nil && ok
		},
	})
	if err != nil {
		return receipt.ExportInput{}, err
	}
	r := core.Receipt{Kind: kind, Service: service, Symbol: receiptverify.BareSymbolName(def.Key()), IndexCommit: exp.Commit().ID}

	if kind == core.ReceiptKindSymbolLocation {
		cited, err := exp.Cite(ctx, nodeSpan(def))
		if err != nil {
			return receipt.ExportInput{}, err
		}
		r.Tool, r.File, r.Line = "find_symbol", cited.Anchor.Path, def.Line
		return receipt.ExportInput{Receipt: r, Facts: []receipt.FactInput{
			{Kind: receipt.FactDefinition, Subject: def.Key(), Anchors: []receipt.CitedAnchor{cited}},
		}}, nil
	}

	r.Tool = "find_callers"
	// find_callers matches a call by the bare name, as receipt verify
	// re-derives it.
	callers, err := c.FindCallers(ctx, service, r.Symbol)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	in := receipt.ExportInput{Receipt: r}
	seen := map[string]bool{}
	for _, n := range callers {
		key := n.Key()
		if seen[key] || n.FilePath == "" {
			continue
		}
		seen[key] = true
		cited, err := exp.Cite(ctx, nodeSpan(n))
		if err != nil {
			return receipt.ExportInput{}, err
		}
		in.Receipt.Callers = append(in.Receipt.Callers, key)
		in.Facts = append(in.Facts, receipt.FactInput{Kind: receipt.FactCaller, Subject: key, Anchors: []receipt.CitedAnchor{cited}})
	}
	if len(in.Facts) > 0 {
		return in, nil
	}
	// No callers: the absence carries the index's read set.
	refs, err := st.ListSourceRefs(ctx, service)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	tree, err := exp.Tree(ctx)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	reads := exp.IndexReadSet(ctx, refs, func(ctx context.Context, ref string) (string, bool, error) {
		return st.Fingerprint(ctx, service, ref)
	})
	in.Facts = []receipt.FactInput{{
		Kind: receipt.FactAbsence, Subject: def.Key(), Reads: &reads, TreeHash: tree,
		Tool: "fathom/find_callers", Noun: "caller(s)",
	}}
	return in, nil
}

// oneDefinition is symbol's one definition in service; none, or several
// distinct ones, is an error naming them.
func oneDefinition(ctx context.Context, c *client.Client, service, symbol string) (core.SymbolNode, error) {
	nodes, err := c.FindSymbol(ctx, service, symbol)
	if err != nil {
		return core.SymbolNode{}, err
	}
	byKey := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.FilePath == "" {
			continue
		}
		if _, ok := byKey[n.Key()]; !ok {
			byKey[n.Key()] = n
		}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	switch len(keys) {
	case 0:
		return core.SymbolNode{}, fmt.Errorf("no definition of %q in service %q", symbol, service)
	case 1:
		return byKey[keys[0]], nil
	}
	if len(keys) > 5 {
		keys = append(keys[:5], "…")
	}
	return core.SymbolNode{}, fmt.Errorf("%q names %d definitions (%s): pass one qualified key", symbol, len(byKey), strings.Join(keys, ", "))
}

// nodeSpan is n's definition span; a node with no line is its whole file.
func nodeSpan(n core.SymbolNode) receipt.Span {
	if n.Line <= 0 {
		return receipt.Span{Path: n.FilePath}
	}
	return receipt.Span{Path: n.FilePath, StartLine: n.Line, EndLine: max(n.EndLine, n.Line)}
}
