package indexbench

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// ReferencesOracleSubset decides what the go/types truth is ENTITLED to
// adjudicate about the `references` tool (bench-pyramid BP-30).
//
// The first self run scored references at precision 0.158 (352 of 2,226) and
// recall 1.000. Read literally that says the tool is 84% wrong. It does not.
// The two sides are not asking the same question:
//
//   - go/types truth knows IDENTIFIER USES: the files in which a name
//     resolves, through the type checker, to one particular declared
//     function. It is silent about everything else by construction.
//   - `references` returns TEXT OCCURRENCES across code, docs and config,
//     plus index symbols whose name merely CONTAINS the query — mentions in
//     comments, in string literals, in Markdown, in a YAML key, and in a
//     same-named function in a different package. None of those is a wrong
//     answer; all of them are outside the truth's competence.
//
// Scoring the whole return against a use-set truth manufactures a precision
// failure the tool does not have. So the oracle scores only the comparable
// subset and reports the rest as UNADJUDICATED — never as correct, never as
// wrong, and never folded into a rate. A number that cannot be justified is
// not reported at all (ADR-031).
//
// Two exclusions carry the argument:
//
//   - A file go/types did not walk is unadjudicated. Non-Go files, files in
//     packages that failed to type-check, files outside the module: the
//     truth cannot contain them, so their presence in the prediction proves
//     nothing about the index.
//
//   - A bare name the module uses for MORE THAN ONE declaration is an
//     unadjudicated QUERY, dropped whole. `references` is asked for a bare
//     string; the truth is per-qualified-name. When "Close" is both a
//     function and some type's method, the tool's hits on the other one are
//     correct answers to the question it was asked and wrong answers to the
//     question the truth holds. Neither side is at fault and no honest
//     number can be computed, so the query leaves the denominator.
//
//     This catches function/method collisions, which is what it can catch:
//     publicloc.GoTruth spells a function with its bare name and no package
//     qualifier, so two packages each declaring `Run` are ONE entry in it
//     and no subset logic here can separate them. That is a limit of the
//     truth, stated rather than papered over — it is part of why the
//     references rate in docs/bench/README.md is reported as a lower bound.
//
// What remains — an unambiguously-named function, and hits in Go files the
// type checker actually walked — is a fair comparison, and that is the only
// thing the reported rate covers.
type ReferencesOracleSubset struct {
	// Names are the queries the truth can adjudicate: receiverless
	// functions whose bare name is declared exactly once in the module.
	Names []string
	// AmbiguousNames are the receiverless functions excluded because some
	// other declaration in the module shares their bare name (typically a
	// method of the same name).
	AmbiguousNames []string
	// MethodNames are the methods excluded because a method's mention in
	// source text is the bare method name, so scoring "Type.Method" as a
	// text query measures the query spelling, not the index. (This
	// exclusion predates BP-30; it is recorded here so all of them live in
	// one place.)
	MethodNames []string

	// walked is the set of index-relative Go file paths go/types read.
	walked map[string]bool
}

// NewReferencesOracleSubset derives the adjudicable subset from truth.
// prefix is Subject.Prefix — the truth's paths are module-relative and the
// index's may carry a fleet-root prefix.
func NewReferencesOracleSubset(truth publicloc.GoTruth, prefix string) ReferencesOracleSubset {
	counts := map[string]int{}
	for _, n := range declaredNames(truth) {
		counts[bareName(n)]++
	}
	sub := ReferencesOracleSubset{walked: map[string]bool{}}
	for f := range truth.DeclsByFile {
		sub.walked[prefix+f] = true
	}
	for _, files := range truth.UsesByName {
		for _, f := range files {
			sub.walked[prefix+f] = true
		}
	}
	for _, n := range declaredNames(truth) {
		switch {
		case isMethod(n):
			sub.MethodNames = append(sub.MethodNames, n)
		case counts[bareName(n)] > 1:
			sub.AmbiguousNames = append(sub.AmbiguousNames, n)
		default:
			sub.Names = append(sub.Names, n)
		}
	}
	sort.Strings(sub.Names)
	sort.Strings(sub.AmbiguousNames)
	sort.Strings(sub.MethodNames)
	return sub
}

// Adjudicable reports whether a predicted file is one the go/types truth can
// rule on: a Go file the type checker actually walked.
func (s ReferencesOracleSubset) Adjudicable(path string) bool {
	return strings.HasSuffix(path, ".go") && s.walked[path]
}

// bareName strips a package or receiver qualifier from a declared name.
func bareName(n string) string {
	if i := strings.LastIndex(n, "."); i >= 0 {
		return n[i+1:]
	}
	return n
}
