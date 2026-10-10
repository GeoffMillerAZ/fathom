package langbench

import (
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Raw counts a subject records; metrics are ratios of their sums, so a
// reading merged over subjects is exact (never an average of ratios).
const (
	CntOracleDefs      = "oracle_defs"
	CntFathomDefs      = "fathom_defs"
	CntMatched         = "matched"
	CntKindEqual       = "kind_equal"
	CntRangeEligible   = "range_eligible"
	CntRangeOK         = "range_ok"
	CntCallables       = "callables"
	CntCallablesSigned = "callables_signed"

	CntCallTruth        = "call_truth"
	CntCallTruthFound   = "call_truth_found"
	CntCallFathom       = "call_fathom"
	CntCallFathomInTrue = "call_fathom_in_truth"

	// CntScipCallsUnwritten counts callable references left out of call
	// truth because no call is written at their range (SCIPOptions).
	CntScipCallsUnwritten = "scip_calls_unwritten"
	// CntScipCalledNames counts references to a non-callable symbol (a
	// class, a field or variable, an interface method scip-go writes as a
	// term, a local) the call truth takes as a call because a call is
	// written at their range (SCIPOptions.CalledNames, LB-F65).
	CntScipCalledNames = "scip_called_names"

	// The build's compiler as the C/C++ call-site oracle (LB-F116,
	// SCIPOptions.ClangSites): fathom's pairs whose every written site has no
	// callee before instantiation, left out of both counts; truth pairs the
	// compiler gives that the index's callable references do not (constructor
	// calls the index references as a class counted apart); references the
	// index writes where the compiler has no call (a declaration), left out
	// of the truth; scip-clang definitions a macro writes, named by what is
	// written; fathom's definitions in code the index holds no occurrence in.
	CntClangCallsDependent = "clang_calls_dependent"
	CntClangCallsAdded     = "clang_calls_added"
	CntClangConstructs     = "clang_constructs"
	CntClangNotCalls       = "clang_not_calls"
	CntScipDefsMacroNamed  = "scip_defs_macro_named"
	CntClangBlindDefs      = "clang_blind_defs"
	// CntCallsInactive counts fathom's pairs written only on lines the build's
	// preprocessor discards (SCIPOptions.ActiveLines, with ClangSites): left
	// out of both counts, as a definition on such a line is.
	CntCallsInactive = "calls_inactive"

	CntScipDefs       = "scip_defs"
	CntScipDefsMapped = "scip_defs_mapped"
	CntScipEdges      = "scip_edges"
	CntScipEdgesTyped = "scip_edges_typed"
	// CntScipEdgesParamTarget counts references to a parameter or type
	// parameter the index defines (internal/scip Report.Skipped
	// ["parameter-target"], LB-F239): out of scip_edges, as def_mapping
	// leaves parameters out of scip_defs, and disclosed here.
	CntScipEdgesParamTarget = "scip_edges_param_target"

	CntFileEdgesTruth  = "file_edges_truth"
	CntFileEdgesFathom = "file_edges_fathom"
	CntFileEdgesAgree  = "file_edges_agree"

	CntRefTruth        = "ref_truth"
	CntRefTruthFound   = "ref_truth_found"
	CntRefFathom       = "ref_fathom"
	CntRefFathomInTrue = "ref_fathom_in_truth"
)

// Families group counts by the oracle that produced them; a subject
// records the tier each family was measured at.
const (
	FamilyL1        = "l1"
	FamilyCalls     = "calls"
	FamilyDefMap    = "defmap"
	FamilyTyped     = "typed"
	FamilyFileEdges = "file_edges"
	FamilyRefs      = "refs"
)

// BareName is a qualified name's last segment (§1.4: qualified names are
// compared on the last segment).
func BareName(q string) string {
	if op, ok := operatorName(q); ok {
		return op
	}
	i := strings.LastIndexAny(q, ".:#/\\ )")
	return q[i+1:]
}

// dottedPath is a name made only of identifiers joined by dots: a language's own expanded name (an Ada child
// unit, GNATCOLL.Arg_Lists), which an oracle writes whole.
var dottedPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+$`)

// oracleMatchName is the form of an oracle's name the harness compares with the dialect's last segment (§1.4:
// qualified names are compared on the last segment): a dotted identifier path is cut to its last identifier,
// anything else (a selector, an npm name, an export path, a name with other punctuation) is left as the oracle
// wrote it, which is why name_match "identity" exists for the dialects that need those compared whole.
func oracleMatchName(name string) string {
	if dottedPath.MatchString(name) {
		return name[strings.LastIndexByte(name, '.')+1:]
	}
	return name
}

// operatorName recognizes a symbolic operator's name in q (C++ and C#:
// "Box.operator+", ctags' "operator +", "ns::Foo::operator()") and returns
// its one canonical spelling, "operator" followed by the symbol without
// spaces: tools disagree on the space (source `operator+`, universal-ctags
// `operator +`) and the ")" closing "operator()" is part of the name, not a
// separator (LB-Q-cpp). Named operators ("operator new") and conversions
// ("operator bool") are not symbolic; they keep the last-segment rule, and so
// does an identifier that merely contains the word.
func operatorName(q string) (string, bool) {
	const kw = "operator"
	for from := len(q); from > 0; {
		i := strings.LastIndex(q[:from], kw)
		if i < 0 {
			return "", false
		}
		from = i
		if i > 0 && !strings.ContainsRune(".:#/\\ ", rune(q[i-1])) {
			continue // "my_operator": not the keyword
		}
		rest := strings.TrimLeft(q[i+len(kw):], " ")
		if rest == "" || !strings.ContainsRune("+-*/%^&|~!=<>,([", rune(rest[0])) {
			continue
		}
		return kw + strings.ReplaceAll(rest, " ", ""), true
	}
	return "", false
}

// L1Result is one subject's §1.4 definition matching.
type L1Result struct {
	Counts       map[string]int
	Excluded     map[string]int
	Intersection []string
	// DialectOnly are fathom's definitions inside the kind intersection
	// the oracle matched nothing to (name in the dialect's match form) —
	// the only candidates an adjudication may admit (D3).
	DialectOnly []OracleDef
	// OracleOnly are the oracle's definitions inside the intersection no
	// fathom definition matched (the recall misses), for diagnosis.
	OracleOnly []OracleDef
}

type scoredDef struct {
	path, name, canonical string
	line, end             int
	callable, signed      bool
	matched               bool
	// asWritten and lineAsWritten are the dialect's own name and line when the
	// comparison uses ctags' rendition of it (ctagsRenditions); fragments are
	// the rendition's further tags, the same definition's.
	asWritten     string
	lineAsWritten int
	fragments     []ctagsTag
}

// ScoreL1 matches fathom's definitions against an oracle's (§1.4): same
// file, same bare name, |Δline| ≤ 2, nearest first, one-to-one.
// Definitions whose kind maps outside the kind intersection on either side
// are excluded from both counts and tallied as "<side>:<kind>".
//
// Against universal-ctags, a dialect whose lang tag has a ctags rendition
// (ctagsRenditions: CSS, K19, LB-F423) is compared in the tags ctags writes
// for each definition: the first tag is the match key, at its own line; when
// it matches, the rendition's further tags (a selector ctags splits at a comma
// inside :not() or :is()) are the same definition, each consuming one oracle
// tag of its name within two lines, left out of the oracle count and tallied
// as "ctags:selector-fragment".
func ScoreL1(m Manifest, oracle string, nodes []core.SymbolNode, defs []OracleDef) L1Result {
	inter := m.Intersection(oracle)
	in := map[string]bool{}
	for _, c := range inter {
		in[c] = true
	}
	res := L1Result{Counts: map[string]int{}, Excluded: map[string]int{}, Intersection: inter}
	var render func(string) []ctagsTag
	if oracle == "ctags" {
		render = ctagsRenditions[m.LangTag]
	}

	var mine []*scoredDef
	for _, n := range nodes {
		c, callable, ok := m.CanonicalForSource(string(n.Kind))
		if !ok || !in[c] {
			res.Excluded["fathom:"+string(n.Kind)]++
			continue
		}
		d := &scoredDef{path: n.FilePath, name: m.MatchName(n.Name), canonical: c, line: n.Line, end: n.EndLine,
			callable: callable, signed: strings.TrimSpace(n.Signature) != ""}
		if render != nil {
			if tags := render(n.Name); len(tags) > 0 {
				d.asWritten, d.lineAsWritten = d.name, d.line
				d.name, d.line, d.fragments = tags[0].name, n.Line+tags[0].line, tags[1:]
			}
		}
		mine = append(mine, d)
	}
	var theirs []*scoredDef
	for _, d := range defs {
		if d.Generated {
			res.Excluded[oracle+":macro-generated"]++
			continue
		}
		c, ok := m.OracleCanonical(oracle, d.Kind, d.Scoped)
		if !ok || !in[c] {
			res.Excluded[oracle+":"+d.Kind]++
			continue
		}
		name := d.Name
		if op, ok := operatorName(name); ok && m.NameMatch != NameMatchIdentity {
			name = op // one spelling per operator on both sides (BareName's rule)
		} else if m.NameMatch != NameMatchIdentity {
			name = oracleMatchName(name)
		}
		theirs = append(theirs, &scoredDef{path: d.Path, name: name, canonical: c, line: d.Line, end: d.End})
	}
	res.Counts[CntFathomDefs] = len(mine)
	res.Counts[CntOracleDefs] = len(theirs)
	for _, d := range mine {
		if d.callable {
			res.Counts[CntCallables]++
			if d.signed {
				res.Counts[CntCallablesSigned]++
			}
		}
	}

	type key struct{ path, name string }
	byKey := map[key][]*scoredDef{}
	for _, d := range theirs {
		byKey[key{d.path, d.name}] = append(byKey[key{d.path, d.name}], d)
	}
	type pair struct {
		f, o *scoredDef
		dist int
	}
	var pairs []pair
	for _, f := range mine {
		for _, o := range byKey[key{f.path, f.name}] {
			dist := f.line - o.line
			if dist < 0 {
				dist = -dist
			}
			if dist <= 2 {
				pairs = append(pairs, pair{f, o, dist})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		a, b := pairs[i], pairs[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		if a.f.path != b.f.path {
			return a.f.path < b.f.path
		}
		if a.o.line != b.o.line {
			return a.o.line < b.o.line
		}
		return a.f.line < b.f.line
	})
	for _, p := range pairs {
		if p.f.matched || p.o.matched {
			continue
		}
		p.f.matched, p.o.matched = true, true
		res.Counts[CntMatched]++
		if p.f.canonical == p.o.canonical {
			res.Counts[CntKindEqual]++
		}
		if p.o.end > 0 {
			res.Counts[CntRangeEligible]++
			d := p.f.end - p.o.end
			if p.f.end > 0 && d >= -1 && d <= 1 {
				res.Counts[CntRangeOK]++
			}
		}
	}
	// A matched definition's further rendition tags: each consumes the nearest
	// unmatched oracle tag of its name within two lines, in file and line order.
	var split []*scoredDef
	for _, f := range mine {
		if f.matched && len(f.fragments) > 0 {
			split = append(split, f)
		}
	}
	sort.SliceStable(split, func(i, j int) bool {
		if split[i].path != split[j].path {
			return split[i].path < split[j].path
		}
		return split[i].lineAsWritten < split[j].lineAsWritten
	})
	for _, f := range split {
		for _, frag := range f.fragments {
			var best *scoredDef
			bestDist := 3
			for _, o := range byKey[key{f.path, frag.name}] {
				dist := f.lineAsWritten + frag.line - o.line
				if dist < 0 {
					dist = -dist
				}
				if !o.matched && dist < bestDist {
					best, bestDist = o, dist
				}
			}
			if best != nil {
				best.matched = true
				res.Counts[CntOracleDefs]--
				res.Excluded[oracle+":selector-fragment"]++
			}
		}
	}
	for _, d := range theirs {
		if !d.matched {
			res.OracleOnly = append(res.OracleOnly, OracleDef{Path: d.path, Name: d.name, Line: d.line, End: d.end, Kind: d.canonical})
		}
	}
	for _, d := range mine {
		if !d.matched {
			name, line := d.name, d.line
			if d.asWritten != "" {
				name, line = d.asWritten, d.lineAsWritten
			}
			res.DialectOnly = append(res.DialectOnly, OracleDef{Path: d.path, Name: name, Line: line, End: d.end, Kind: d.canonical})
		}
	}
	return res
}

// CallPair is (enclosing definition's bare name, callee bare name) in a
// file — the §1.4 unit of call truth.
type CallPair struct {
	Path, Caller, Callee string
}

// FathomCallPairs returns the extractor's call pairs over files, each
// callee reduced to its last dotted segment (§1.1 L2).
func FathomCallPairs(nodes []core.SymbolNode, files map[string]bool) map[CallPair]bool {
	return callPairs(nodes, files, BareName)
}

func callPairs(nodes []core.SymbolNode, files map[string]bool, name func(string) string) map[CallPair]bool {
	out := map[CallPair]bool{}
	for _, n := range nodes {
		if files != nil && !files[n.FilePath] {
			continue
		}
		for _, c := range n.Calls {
			callee := name(c)
			if callee == "" {
				continue
			}
			out[CallPair{n.FilePath, name(n.Name), callee}] = true
		}
	}
	return out
}

// ScorePairs counts truth against found for one pair family, writing the
// four counts under the given keys.
func ScorePairs(truth, found map[CallPair]bool, counts map[string]int, kTruth, kTruthFound, kFound, kFoundInTruth string) {
	counts[kTruth] += len(truth)
	counts[kFound] += len(found)
	for p := range truth {
		if found[p] {
			counts[kTruthFound]++
		}
	}
	for p := range found {
		if truth[p] {
			counts[kFoundInTruth]++
		}
	}
}
