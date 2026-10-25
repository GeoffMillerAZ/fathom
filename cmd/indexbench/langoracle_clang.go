package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// clangCallSite is one entry of the build's compiler's call-site oracle
// (calibration/oracles/clang-calls, LB-F116): the callee's name token written
// at a 1-based line and byte column, and the class the AST gives it.
type clangCallSite struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Col   int    `json:"col"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

// The disclosures of the compiler's call-site classes (LB-F116), one per
// count langbench.ScoreSCIPWith writes under SCIPOptions.ClangSites.
const (
	noteClangDependent  = "of fathom's call pairs are written only where the build's own compiler has no callee before instantiation (a member call on a template-dependent receiver such as `buf.size()`, an overloaded or argument-dependent call with dependent arguments, `T(x)` of a template parameter: CXXDependentScopeMemberExpr, UnresolvedMemberExpr, UnresolvedLookupExpr, DependentScopeDeclRefExpr, CXXUnresolvedConstructExpr in the template's pattern, which is what the index and a lexical extractor read) and are left out of both L2 counts (calibration/oracles/clang-calls, tier A)"
	noteClangAdded      = "call pairs the build's own compiler resolves at a written call are in the call truth with no callable reference of the index's behind them (a call to a member or function template's specialization, a call through a variable, field or lambda, a call in a local class's method, a constructor call the index does not reference)"
	noteClangConstructs = "constructor calls written `T(` (the compiler's CXXTemporaryObjectExpr, CXXConstructExpr or functional cast through a constructor) are in the call truth, scored against the index's class or constructor reference at the type's name"
	noteClangNotCalls   = "references the index writes at a function's declaration (`int close(int fd);`, written like a call; the build's own compiler declares the function there) are left out of the call truth"
	noteClangMacroNamed = "of fathom's definitions are written by a macro the index defines under another name (gtest's TEST(suite, name) is suite_name_Test::TestBody, reported over `TEST`): the index's callables there are named by what is written, as fathom's extractor names them, so the calls in their bodies are measured; def_mapping does not count them"
	noteCallsInactive   = "of fathom's call pairs are written only on lines the build's own preprocessor discards inside a definition it keeps (a `#ifdef _WIN32` or C++20 branch of a function's body) and are left out of both L2 counts, as a definition on such a line is"
	noteClangBlindDefs  = "of fathom's definitions that make calls hold no occurrence of the index but a macro's (code scip-clang 0.4.0 does not index, seen in the formatter<T> specializations fmt's headers and tests write and in members of nlohmann's basic_json class template) and are scored against the build's own compiler's call sites alone"
)

// clangSitesOf reads the call sites of every tool oracle that has them, per
// document.
func clangSitesOf(paths []string) (map[string][]langbench.ClangSite, error) {
	out := map[string][]langbench.ClangSite{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var probe struct {
			CallSites []clangCallSite `json:"call_sites"`
		}
		if err := json.Unmarshal(b, &probe); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		for _, s := range probe.CallSites {
			out[s.Path] = append(out[s.Path], langbench.ClangSite{Line: s.Line, Col: s.Col, Name: s.Name, Class: s.Class})
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// clangSiteNotes discloses each compiler class a subject's scoring counted.
func clangSiteNotes(name string, counts map[string]int) []string {
	var out []string
	for _, c := range []struct{ key, note string }{
		{langbench.CntClangCallsDependent, noteClangDependent},
		{langbench.CntClangCallsAdded, noteClangAdded},
		{langbench.CntClangConstructs, noteClangConstructs},
		{langbench.CntClangNotCalls, noteClangNotCalls},
		{langbench.CntScipDefsMacroNamed, noteClangMacroNamed},
		{langbench.CntClangBlindDefs, noteClangBlindDefs},
		{langbench.CntCallsInactive, noteCallsInactive},
	} {
		if n := counts[c.key]; n > 0 {
			out = append(out, fmt.Sprintf("%s: %d %s", name, n, c.note))
		}
	}
	sort.Strings(out)
	return out
}

// noteFileEdgesOracleBlind discloses the file pairs left out as ones the index
// cannot judge (LB-F296).
const noteFileEdgesOracleBlind = "file pairs fathom draws that the index's truth lacks have no occurrence of the index at any site their target's name is written in the drawing definition (a type in a template's argument list or a function template's return type, `std::shared_ptr<logger>`, which scip-clang references nowhere): pairs the oracle cannot judge, left out of file_edge_precision under LB-F203's 10% cap and listed under -unmatched's oracle_blind (LB-F296)"

// fileEdgesOracleBlind is langbench.FileEdgesOracleBlind for one subject: it
// counts and discloses the pairs it leaves out.
func fileEdgesOracleBlind(raw []byte, truth map[[2]string]bool, edges []core.Edge, nodes []core.SymbolNode, src func(string) []byte, name string, counts map[string]int, notes *[]string) ([]core.Edge, [][2]string, error) {
	kept, blind, err := langbench.FileEdgesOracleBlind(raw, truth, edges, nodes, src)
	if err != nil || len(blind) == 0 {
		return kept, blind, err
	}
	counts[langbench.CntFileEdgesOracleBlind] += len(blind)
	*notes = append(*notes, fmt.Sprintf("%s: %d %s", name, len(blind), noteFileEdgesOracleBlind))
	return kept, blind, nil
}

// feedTier is the weakest tier among the tool oracle files that carry any of
// keys (each file's own "tier", required), A when no file does: a
// metric is judged at the lowest tier of the oracles that feed it (owner ruling
// C-R6, F5), so a clang CUDA translation's preprocessor or call sites lower
// the index's tier-A metrics they settle.
func feedTier(paths []string, keys ...string) (string, error) {
	tier := "A"
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(b, &probe); err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		if !slices.ContainsFunc(keys, func(k string) bool { _, ok := probe[k]; return ok }) {
			continue
		}
		var own string
		if raw, ok := probe["tier"]; ok {
			if err := json.Unmarshal(raw, &own); err != nil {
				return "", fmt.Errorf("%s: tier: %w", p, err)
			}
		}
		if !slices.Contains([]string{"A", "B", "C", "D"}, own) {
			// Round 3 A3 (F3): an oracle that feeds a metric names its tier;
			// an untiered file is refused, never read as tier A.
			return "", fmt.Errorf("%s: feeds %v but names no tier A-D (%q)", p, keys, own)
		}
		tier = weakerTier(tier, own)
	}
	return tier, nil
}

// weakerTier is the weaker of two oracle tiers (A strongest, D weakest).
func weakerTier(a, b string) string {
	if b > a {
		return b
	}
	return a
}
