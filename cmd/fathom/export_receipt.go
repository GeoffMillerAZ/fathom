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
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdExportReceipt implements `fathom export receipt --service S --symbol
// NAME [--kind symbol_location|callers|references] [--store PATH]
// [--no-excerpts] [--keep-service] [--require-clean] [-o FILE]`, and
// `--kind certification [--test NAME] [--target ID]` in place of --symbol.
// Anchors name the service's indexed commit; a symbol with no callers is an
// absence fact carrying the index's read set. --require-clean writes the
// receipt, then exits 1 when an anchor is not committed. A Go struct field's
// callers and references receipts are refused: field references are not
// indexed.
func cmdExportReceipt(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("export receipt", stderr)
	service := fs.String("service", "", "service ID (required)")
	symbol := fs.String("symbol", "", "the symbol: a bare name or a qualified key (required unless --kind certification)")
	kind := fs.String("kind", string(core.ReceiptKindSymbolLocation), "what the receipt claims: symbol_location (where it is defined), callers (who calls it), references (which files mention it) or certification (what a test certifies and reaches)")
	test := fs.String("test", "", "certification kind: the certifying test, a bare name or a symbol key")
	target := fs.String("target", "", "certification kind: the certified target id, or kind:id")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	out := fs.String("o", "-", "output file (- is stdout)")
	noExcerpts := fs.Bool("no-excerpts", false, "keep digests only: no excerpt text leaves")
	keepService := fs.Bool("keep-service", false, "keep service names in the exported receipt (default: replace with @)")
	requireClean := fs.Bool("require-clean", false, "after writing the receipt, exit 1 when any anchor is not committed")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	k := core.ReceiptKind(*kind)
	switch k {
	case core.ReceiptKindSymbolLocation, core.ReceiptKindCallers, core.ReceiptKindReferences, core.ReceiptKindCertification:
	default:
		fmt.Fprintf(stderr, "error: export receipt --kind %q: want symbol_location, callers, references or certification\n", *kind)
		return 1
	}
	if *service == "" || *symbol == "" && k != core.ReceiptKindCertification {
		fmt.Fprintln(stderr, "error: export receipt requires --service and --symbol")
		fs.Usage()
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
	var in receipt.ExportInput
	if k == core.ReceiptKindCertification {
		in, err = exportCertificationInput(ctx, c, st, *service, *test, *target)
	} else {
		in, err = exportReceiptInput(ctx, c, st, *service, *symbol, k)
	}
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
	} else {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write %q: %v\n", *out, err)
			return 1
		}
		fmt.Fprintf(stdout, "wrote %s: %s receipt for %s, %d fact(s)\n", *out, built.Kind, built.Symbol, len(built.Facts))
	}
	if *requireClean {
		if unclean := uncleanAnchors(built); len(unclean) > 0 {
			fmt.Fprintf(stderr, "error: export receipt --require-clean: %s\n", strings.Join(unclean, ", "))
			return 1
		}
	}
	return 0
}

// uncleanAnchors names, sorted, each anchor of r that is not committed.
func uncleanAnchors(r receipt.ExportedReceipt) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range r.Facts {
		for _, a := range f.Anchors {
			if a.State == receipt.StateCommitted {
				continue
			}
			if s := a.Path + " is " + string(a.State); !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Strings(out)
	return out
}

// exportReceiptInput gathers the receipt for symbol in service from the
// store, anchored at the service's indexed commit.
func exportReceiptInput(ctx context.Context, c *client.Client, st store.Store, service, symbol string, kind core.ReceiptKind) (receipt.ExportInput, error) {
	def, err := oneDefinition(ctx, c, service, symbol)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	if (kind == core.ReceiptKindCallers || kind == core.ReceiptKindReferences) && def.Kind == core.KindGoField {
		// The delete check's rule: no caller set, file set or absence is
		// provable over references the index never records.
		return receipt.ExportInput{}, fmt.Errorf("%s is a Go struct field and field references are not indexed: a %s receipt is withheld (export its definition with --kind %s)", def.Name, kind, core.ReceiptKindSymbolLocation)
	}
	exp, _, err := serviceExporter(ctx, c, st, service)
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

	if kind == core.ReceiptKindReferences {
		return exportReferencesInput(ctx, c, exp, service, def, r)
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

// serviceExporter anchors at service's indexed commit, and returns the
// service's symbols beside it.
func serviceExporter(ctx context.Context, c *client.Client, st store.Store, service string) (*receipt.Exporter, []core.SymbolNode, error) {
	root, indexed, found, err := c.ServiceMeta(ctx, service)
	if err != nil {
		return nil, nil, err
	}
	if !found || root == "" {
		return nil, nil, fmt.Errorf("service %q has no indexed root: ingest it first", service)
	}
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, nil, err
	}
	exp, err := receipt.NewExporter(ctx, receipt.Run{
		Dir: root, Base: root, Rev: indexed, Service: service, Nodes: nodes,
		Stored: func(p string) (string, bool) {
			fp, ok, ferr := st.Fingerprint(ctx, service, p)
			return fp, ferr == nil && ok
		},
	})
	return exp, nodes, err
}

// exportReferencesInput is the references receipt for def: the declaring
// file and the file of every typed edge into def, each a reference fact
// anchored at the whole file. Receipt verify re-derives the file set with
// go/types, which can count a mention the typed edges do not.
func exportReferencesInput(ctx context.Context, c *client.Client, exp *receipt.Exporter, service string, def core.SymbolNode, r core.Receipt) (receipt.ExportInput, error) {
	edges, err := c.TypedEdges(ctx, service)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	paths := map[string]bool{def.FilePath: true}
	for _, e := range edges {
		if e.ToSymbol == def.Key() && e.FromFile != "" {
			paths[e.FromFile] = true
		}
	}
	r.Tool = "references"
	in := receipt.ExportInput{Receipt: r}
	seen := map[string]bool{}
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		cited, err := exp.Cite(ctx, receipt.Span{Path: p})
		if err != nil {
			return receipt.ExportInput{}, err
		}
		rel := cited.Anchor.Path
		if seen[rel] {
			continue
		}
		seen[rel] = true
		in.Receipt.Files = append(in.Receipt.Files, rel)
		in.Facts = append(in.Facts, receipt.FactInput{Kind: receipt.FactReference, Subject: rel, Anchors: []receipt.CitedAnchor{cited}})
	}
	return in, nil
}

// exportCertificationInput is the certification receipt for the one
// certifications-layer claim test and target select (an empty flag
// matches any). The claim's scope is the test's file plus the target's
// files, so Reached is every symbol in a target file the test reaches over
// the call graph, typed edges included; each carries a definition fact, as
// the test does. The test's own helpers are not what it certifies, and a
// target that names no symbol (an invariant or incident, whose scope
// beyond the test file is a doc file) leaves the reach vacuous: no receipt
// can re-derive it, so the export refuses.
func exportCertificationInput(ctx context.Context, c *client.Client, st store.Store, service, test, target string) (receipt.ExportInput, error) {
	claim, m, err := oneCertification(ctx, st, service, test, target)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	exp, nodes, err := serviceExporter(ctx, c, st, service)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	edges, err := c.TypedEdges(ctx, service)
	if err != nil {
		return receipt.ExportInput{}, err
	}
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		if _, ok := byKey[n.Key()]; !ok && n.FilePath != "" {
			byKey[n.Key()] = n
		}
	}
	testNode, ok := byKey[m.TestSymbolKey]
	if !ok {
		return receipt.ExportInput{}, fmt.Errorf("certification test %s is not in the index", m.TestSymbolKey)
	}
	testCited, err := exp.Cite(ctx, nodeSpan(testNode))
	if err != nil {
		return receipt.ExportInput{}, err
	}
	r := core.Receipt{
		Kind: core.ReceiptKindCertification, Tool: "certify", Service: service,
		Symbol: receiptverify.BareSymbolName(m.TestSymbolKey), IndexCommit: exp.Commit().ID,
		Test: certificationTestName(testCited.Anchor.Path, testNode.Package, m.Test), Target: certificationTarget(m),
		Strength: m.Strength, RunDigest: m.RunDigest,
	}
	in := receipt.ExportInput{Receipt: r, Facts: []receipt.FactInput{
		{Kind: receipt.FactDefinition, Subject: m.TestSymbolKey, Anchors: []receipt.CitedAnchor{testCited}},
	}}
	// certify's own walk: each scope-file symbol is checked from the test
	// forward, rather than condensing the whole graph.
	g := callgraph.BuildBoth(nodes, edges)
	root := exp.Commit().Root
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		n := byKey[key]
		file := repoRelPath(root, n.FilePath)
		if file == testCited.Anchor.Path || !inScope(claim.Scope, file) || !callgraph.Reachable(g, m.TestSymbolKey, key) {
			continue
		}
		cited, err := exp.Cite(ctx, nodeSpan(n))
		if err != nil {
			return receipt.ExportInput{}, err
		}
		in.Receipt.Reached = append(in.Receipt.Reached, key)
		in.Facts = append(in.Facts, receipt.FactInput{Kind: receipt.FactDefinition, Subject: key, Anchors: []receipt.CitedAnchor{cited}})
	}
	if len(in.Receipt.Reached) == 0 {
		return receipt.ExportInput{}, fmt.Errorf("%s reaches no symbol outside its own file in the scope of %s (%s): the reach is vacuous, and a certification receipt has nothing to re-derive", m.Test, r.Target, strings.Join(claim.Scope, ", "))
	}
	return in, nil
}

// oneCertification is the one certifications-layer claim of service whose
// test and target match; none, or several, is an error naming the
// candidates.
func oneCertification(ctx context.Context, st store.Store, service, test, target string) (core.SemanticClaim, certify.Meta, error) {
	claims, err := st.ClaimsForService(ctx, service, certify.Layer)
	if err != nil {
		return core.SemanticClaim{}, certify.Meta{}, err
	}
	var candidates, matched []string
	var claim core.SemanticClaim
	var meta certify.Meta
	for _, cl := range claims {
		var m certify.Meta
		if err := json.Unmarshal(cl.Meta, &m); err != nil || m.TestSymbolKey == "" {
			continue
		}
		name := m.Test + " → " + certificationTarget(m)
		candidates = append(candidates, name)
		if test != "" && test != m.Test && test != m.TestSymbolKey {
			continue
		}
		if target != "" && target != m.TargetID && target != certificationTarget(m) {
			continue
		}
		matched = append(matched, name)
		claim, meta = cl, m
	}
	sort.Strings(candidates)
	sort.Strings(matched)
	switch len(matched) {
	case 1:
		return claim, meta, nil
	case 0:
		if len(candidates) == 0 {
			return core.SemanticClaim{}, certify.Meta{}, fmt.Errorf("service %q holds no certification", service)
		}
		return core.SemanticClaim{}, certify.Meta{}, fmt.Errorf("no certification matches --test %q --target %q; candidates: %s", test, target, strings.Join(candidates, ", "))
	}
	return core.SemanticClaim{}, certify.Meta{}, fmt.Errorf("%d certifications match --test %q --target %q (%s): pass --test and --target", len(matched), test, target, strings.Join(matched, ", "))
}

// certificationTarget is m's target as kind:id.
func certificationTarget(m certify.Meta) string {
	if m.TargetKind == "" {
		return m.TargetID
	}
	return m.TargetKind + ":" + m.TargetID
}

// certificationTestName is receipt verify's dir/pkg.Name for a test of
// package pkg declared in file; at the module root it is the bare name.
func certificationTestName(file, pkg, test string) string {
	if path.Dir(file) == "." {
		return test
	}
	return core.QualifiedPackage(file, pkg) + "." + test
}

// repoRelPath is p relative to root, slash-separated; a relative p is
// already repo-relative.
func repoRelPath(root, p string) string {
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(root, p); err == nil {
			p = rel
		}
	}
	return filepath.ToSlash(p)
}

// inScope reports whether file matches one of a claim's scope globs.
func inScope(scope []string, file string) bool {
	for _, s := range scope {
		if ok, _ := path.Match(s, file); ok || s == file {
			return true
		}
	}
	return false
}

// oneDefinition is symbol's one definition in service; none, or several
// distinct ones, is an error naming them.
func oneDefinition(ctx context.Context, c *client.Client, service, symbol string) (core.SymbolNode, error) {
	nodes, err := c.FindSymbol(ctx, service, symbol)
	if err != nil {
		return core.SymbolNode{}, err
	}
	byKey := map[string]core.SymbolNode{}
	// A same-named struct field never shadows the symbol asked for.
	for _, n := range core.WithoutShadowingFields(nodes, symbol) {
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
