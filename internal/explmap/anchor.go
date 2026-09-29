package explmap

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// AnchorTier is the rung of find_symbol's ladder that decided an anchor.
type AnchorTier string

const (
	// AnchorExact: the query is the symbol's name, its directory-qualified
	// name (`internal/fusion.ResolveImportSpecs`, the canonical spelling) or its
	// full key.
	AnchorExact AnchorTier = "exact"
	// AnchorMember: the query is a bare method name of a `Type.Method` symbol.
	AnchorMember AnchorTier = "member"
	// AnchorPackage: the query is package.Name, the Go spelling, the package
	// named by its declared name or by the tail of its directory.
	AnchorPackage AnchorTier = "package"
	// AnchorCaseInsensitive: one of the spellings above, ignoring case.
	AnchorCaseInsensitive AnchorTier = "case-insensitive"
)

// AnchorCandidateCap is how many candidates an ambiguity error lists.
const AnchorCandidateCap = 8

// ErrAnchorNotFound is a symbol anchor no tier matches.
var ErrAnchorNotFound = errors.New("no symbol matches")

// AmbiguousAnchorError is a symbol anchor that names more than one symbol at
// the first tier that matched anything. It resolves nothing: an entry filed on
// a guess would be served as if it were the right code.
type AmbiguousAnchorError struct {
	Query string
	Tier  AnchorTier
	// Candidates are the first AnchorCandidateCap matches as
	// `dir.Name (file:line)`, sorted; Total is how many there are.
	Candidates []string
	Total      int
}

func (e *AmbiguousAnchorError) Error() string {
	more := ""
	if e.Total > len(e.Candidates) {
		more = fmt.Sprintf(" and %d more", e.Total-len(e.Candidates))
	}
	return fmt.Sprintf("names %d symbols at the %s tier: %s%s; name one of them as dir.Name", e.Total, e.Tier, strings.Join(e.Candidates, ", "), more)
}

// AnchorMatch is the one symbol an anchor resolved to.
type AnchorMatch struct {
	Node core.SymbolNode
	// Key is the canonical spelling the entry stores (SymbolAnchorKey), so
	// however the caller spelled the anchor, two appends of one symbol carry one
	// key.
	Key  string
	Tier AnchorTier
}

// SymbolAnchorKey is the canonical symbol anchor: the symbol's repo-relative
// directory, a dot and its name (`internal/fusion.ResolveImportSpecs`), the
// convention a plan row's package: field and a map anchor already use; its
// package name when the file sits at the root.
func SymbolAnchorKey(n core.SymbolNode) string {
	if dir := anchorDir(n.FilePath); dir != "" {
		return dir + "." + n.Name
	}
	if n.Package != "" {
		return n.Package + "." + n.Name
	}
	return n.Name
}

func anchorDir(file string) string {
	d := path.Dir(strings.TrimPrefix(file, "/"))
	if d == "." || d == "/" {
		return ""
	}
	return d
}

// NormalizeSymbol reduces a symbol spelling to what the tiers compare: the
// service qualifier (`svc::`) and a Go doc receiver `(*T).M` are dropped.
func NormalizeSymbol(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "::"); i >= 0 {
		s = s[i+2:]
	}
	if strings.HasPrefix(s, "(") {
		if end := strings.Index(s, ")"); end > 0 {
			recv, rest := strings.TrimPrefix(s[1:end], "*"), s[end+1:]
			if recv != "" && strings.HasPrefix(rest, ".") {
				s = recv + rest
			}
		}
	}
	return s
}

// nodeForms are the spellings of one node, by tier.
type nodeForms struct {
	exact, member, pkg []string
}

func formsOf(n core.SymbolNode) nodeForms {
	f := nodeForms{exact: []string{n.Name}}
	dir := anchorDir(n.FilePath)
	if dir != "" {
		f.exact = append(f.exact, dir+"."+n.Name)
	}
	if n.Package != "" {
		f.exact = append(f.exact, n.PackagePath()+"."+n.Name)
		if dir == "" {
			f.exact = append(f.exact, n.Package+"."+n.Name)
		} else {
			f.pkg = append(f.pkg, n.Package+"."+n.Name)
		}
	}
	if i := strings.LastIndex(n.Name, "."); i >= 0 {
		f.member = append(f.member, n.Name[i+1:])
	}
	// The tail of the directory at each path boundary: `fusion.Name` for
	// internal/fusion, `a/b.Name` and `b.Name` for cmd/a/b.
	for rest := dir; rest != ""; {
		i := strings.Index(rest, "/")
		if i < 0 {
			break
		}
		rest = rest[i+1:]
		f.pkg = append(f.pkg, rest+"."+n.Name)
	}
	return f
}

func hasForm(forms []string, q string, fold bool) bool {
	for _, f := range forms {
		if f == q || (fold && strings.EqualFold(f, q)) {
			return true
		}
	}
	return false
}

// ResolveAnchorSymbol resolves a symbol anchor over an index's symbols the way
// find_symbol does (XM-31): by tiers, the first tier that matches anything
// deciding: exact, a bare method name, package.Name, then case-insensitive. One
// distinct symbol (nodes sharing a canonical key are one, the earliest file and
// line standing for it) resolves; several return an *AmbiguousAnchorError that
// lists them; none is ErrAnchorNotFound. The result does not depend on the order
// of nodes. Deterministic, no I/O.
func ResolveAnchorSymbol(nodes []core.SymbolNode, query string) (AnchorMatch, error) {
	q := NormalizeSymbol(query)
	if q == "" || strings.ContainsAny(q, " \t\r\n") {
		return AnchorMatch{}, fmt.Errorf("%w: an anchor is one symbol spelling, without spaces", ErrAnchorNotFound)
	}
	forms := make([]nodeForms, len(nodes))
	for i, n := range nodes {
		forms[i] = formsOf(n)
	}
	tiers := []struct {
		tier AnchorTier
		fold bool
		pick func(nodeForms) []string
	}{
		{AnchorExact, false, func(f nodeForms) []string { return f.exact }},
		{AnchorMember, false, func(f nodeForms) []string { return f.member }},
		{AnchorPackage, false, func(f nodeForms) []string { return f.pkg }},
		{AnchorCaseInsensitive, true, func(f nodeForms) []string {
			return append(append(append([]string{}, f.exact...), f.member...), f.pkg...)
		}},
	}
	for _, t := range tiers {
		var hits []core.SymbolNode
		for i, n := range nodes {
			if hasForm(t.pick(forms[i]), q, t.fold) {
				hits = append(hits, n)
			}
		}
		if len(hits) == 0 {
			continue
		}
		distinct := distinctByAnchorKey(hits)
		if len(distinct) == 1 {
			return AnchorMatch{Node: distinct[0], Key: SymbolAnchorKey(distinct[0]), Tier: t.tier}, nil
		}
		labels := make([]string, len(distinct))
		for i, n := range distinct {
			labels[i] = fmt.Sprintf("%s (%s:%d)", SymbolAnchorKey(n), n.FilePath, n.Line)
		}
		total := len(labels)
		if len(labels) > AnchorCandidateCap {
			labels = labels[:AnchorCandidateCap]
		}
		return AnchorMatch{}, &AmbiguousAnchorError{Query: query, Tier: t.tier, Candidates: labels, Total: total}
	}
	return AnchorMatch{}, fmt.Errorf("%w (tried find_symbol's tiers: exact, method name, package.Name, case-insensitive)", ErrAnchorNotFound)
}

// distinctByAnchorKey keeps one node per canonical key (the earliest file, then
// line) and returns them sorted by key.
func distinctByAnchorKey(nodes []core.SymbolNode) []core.SymbolNode {
	best := map[string]core.SymbolNode{}
	for _, n := range nodes {
		k := SymbolAnchorKey(n)
		if cur, ok := best[k]; !ok || n.FilePath < cur.FilePath || (n.FilePath == cur.FilePath && n.Line < cur.Line) {
			best[k] = n
		}
	}
	keys := make([]string, 0, len(best))
	for k := range best {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]core.SymbolNode, len(keys))
	for i, k := range keys {
		out[i] = best[k]
	}
	return out
}
