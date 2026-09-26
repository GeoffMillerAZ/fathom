package semantic

// contract.go — BP-19 (docs/plans/bench-pyramid.md, ADR-032 §4 D2): a
// claims-role rule is scoped at SERVE time by the contract its cited plan
// row names, not by the words an LLM read in that row's prose.
//
// O23 is the defect. Asked what governs internal/extractor/goextract.go,
// the obligations tool served six spec-bindings about C++ toolchains, CUE
// and tree-sitter and displaced 65 more; not one of them named that
// package, and the internal/CLAUDE.md extractor-wiring rule the file is
// actually governed by was not served at all. Every one of those rows had
// bound to the file because its prose contained the word "extractor" — the
// O14 defect at fleet scale.
//
// A gated plan row already carries the contract: `{id: …, symbol: Name,
// package: dir}` (docs/plans/README.md's grammar v2). That contract is the
// truth about what the row governs, and it is machine-readable, so serving
// re-derives the scope from it with the SAME tokenizer the plan gate uses
// (internal/verify.ParseAnnotation — one grammar, never two quoting rules,
// which is O4's lesson). No recompile is involved: this is a serve-time
// re-derivation over claims already in the store.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// SymbolFile resolves a plan row's {package, symbol} contract to the
// repo-relative file that DEFINES that symbol. ok=false when the index
// knows no such symbol — a symbol-only row whose symbol cannot be located
// binds to nothing rather than to a guess.
type SymbolFile func(pkg, symbol string) (file string, ok bool)

// ContractScope re-derives the scope globs a plan row binds to from the
// text of the cited line range.
//
// citedText is the exact text of a claim's primary citation — the "cites"
// link's AnchorText, or a live re-read of the cited lines. lookup resolves
// a symbol-only contract to its defining file; it may be nil, in which case
// a symbol-only row binds to nothing.
//
// The two return values are deliberately distinct questions:
//
//	ok=false — the cited text is not a plan row at all (no `{id: …}`
//	  annotation). The caller KEEPS the compile-time, LLM-proposed scope:
//	  that fallback is what lets a claims-role layer over a non-plan corpus
//	  (findings/**) go on working exactly as before.
//	ok=true, scope empty — the cited text IS a plan row and it names NO
//	  contract. Such a row binds to NOTHING and must not be served: a row
//	  with no {package} and no {symbol} has told us nothing about which code
//	  it governs, and prose is not a substitute (that is the O23 defect).
//	ok=true, scope non-empty — serve this claim against these globs only.
//
// Deterministic: same text, same answer, no clock and no network.
func ContractScope(citedText string, lookup SymbolFile) (scope []string, ok bool) {
	for _, line := range strings.Split(citedText, "\n") {
		kv, _, _ := verify.ParseAnnotation(line)
		if kv == nil || kv["id"] == "" {
			continue // not a gated plan row
		}
		ok = true
		if pkg := strings.TrimSpace(kv["package"]); pkg != "" {
			scope = append(scope, strings.TrimSuffix(strings.TrimSuffix(pkg, "/"), "/**")+"/**")
			continue
		}
		if sym := strings.TrimSpace(kv["symbol"]); sym != "" && lookup != nil {
			if file, found := lookup("", sym); found {
				scope = append(scope, file)
			}
		}
	}
	if !ok {
		return nil, false
	}
	sort.Strings(scope)
	return dedupeStrings(scope), true
}

func dedupeStrings(in []string) []string {
	if len(in) < 2 {
		return in
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}

// citedTextResolver returns a memoized resolver for "the exact text of a
// claim's primary citation", the input ContractScope parses.
//
// Two sources, in order:
//
//  1. the cites link's AnchorText — the literal cited lines as they stood
//     at compile time (T5-1/O13). This is the common case and costs no I/O.
//  2. a bounded re-read of the cited line range under root, for a legacy
//     hash-only link written before AnchorText existed. Without this
//     fallback, contract scoping would silently do nothing for exactly the
//     oldest claims — and a serve-time fix that quietly no-ops on real data
//     is the instrument-degrading-toward-healthy failure ADR-027 forbids.
//
// Files are read at most once each per ObligationsFor call. An unreadable
// file, an unparseable citation, or an empty root yields "" — the caller
// then keeps the compile-time scope, never a guess.
func citedTextResolver(root string, anchorText map[string]string) func(cite, claimID string) string {
	files := map[string][]string{}
	return func(cite, claimID string) string {
		if t := anchorText[claimID]; t != "" {
			return t
		}
		if root == "" || cite == "" {
			return ""
		}
		path, start, end, ok := parseCite(cite)
		if !ok {
			return ""
		}
		lines, cached := files[path]
		if !cached {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))); err == nil && len(data) <= citedFileMaxBytes {
				lines = strings.Split(string(data), "\n")
			}
			files[path] = lines
		}
		if start < 1 || start > len(lines) {
			return ""
		}
		if end < start || end > len(lines) {
			end = start
		}
		return strings.Join(lines[start-1:end], "\n")
	}
}

// citedFileMaxBytes bounds the legacy-citation re-read above. A governing
// document larger than this is not a plan file, and a serve-time hot path
// must never pull an arbitrarily large file into the shared daemon.
const citedFileMaxBytes = 1 << 20

// parseCite splits a "cites" TargetRef — "path#Lstart-Lend" or "path#Lstart"
// (core.CiteRef's shape) — into its parts.
func parseCite(ref string) (path string, start, end int, ok bool) {
	path, frag, found := strings.Cut(ref, "#L")
	if !found || path == "" {
		return "", 0, 0, false
	}
	lo, hi, ranged := strings.Cut(frag, "-L")
	start, err := strconv.Atoi(lo)
	if err != nil || start < 1 {
		return "", 0, 0, false
	}
	end = start
	if ranged {
		if end, err = strconv.Atoi(hi); err != nil {
			return "", 0, 0, false
		}
	}
	return path, start, end, true
}

// symbolFileLookup returns a lazily-populated SymbolFile over the service's
// indexed symbols — the resolution a symbol-only plan row needs
// ("{id: X, symbol: Name}" with no package). It is lazy on purpose: the vast
// majority of gated rows name a package, and the observation scan behind
// this is the most expensive read in the function. A service with no
// symbol-only rows never pays for it.
func symbolFileLookup(ctx context.Context, st store.Store, serviceID string) SymbolFile {
	var (
		loaded bool
		byName map[string]string
	)
	return func(pkg, symbol string) (string, bool) {
		if !loaded {
			loaded = true
			byName = map[string]string{}
			obs, err := st.QueryObservations(ctx, serviceID, "")
			if err != nil {
				return "", false
			}
			for _, o := range obs {
				if o.Symbol == "" || o.SourceRef == "" {
					continue
				}
				file := o.SourceRef
				if i := strings.LastIndex(file, ":"); i > 0 {
					if _, err := strconv.Atoi(file[i+1:]); err == nil {
						file = file[:i]
					}
				}
				key := o.Package + "\x00" + o.Symbol
				// Deterministic on a duplicate symbol name: the
				// lexicographically first defining file wins, never
				// whichever row the store happened to return first.
				if prev, seen := byName[key]; !seen || file < prev {
					byName[key] = file
				}
				bare := "\x00" + o.Symbol
				if prev, seen := byName[bare]; !seen || file < prev {
					byName[bare] = file
				}
			}
		}
		file, ok := byName[pkg+"\x00"+symbol]
		return file, ok
	}
}

// SymbolNamesForFile returns the distinct symbol names the index has
// recorded as declared in file (a SourceRef's ":line" suffix ignored) —
// the index-backed half of relevance.go's optional signal (wave O2, item
// 1): a governing rule that names the file's own type or function exactly
// is as strong a lexical signal as one that names its stem. A caller
// (pkg/client.Client.RawFallbackObligations) resolves these and passes
// them to semantic.RawFallbackRules as extraTokens — RawFallbackRules
// itself never touches a store directly. Since RT-81 it reads only file's
// own rows (store.SymbolNamesForSource, the (service_id, source_ref)
// index) — never the service's whole observation set, which grew every
// edit's cost with the tree. Returns nil (never an error) on a store read
// failure or when file declares nothing the index has seen, the same
// honest-absence contract symbolFileLookup's own lookup makes. Sorted.
func SymbolNamesForFile(ctx context.Context, st store.Store, serviceID, file string) []string {
	names, err := st.SymbolNamesForSource(ctx, serviceID, file)
	if err != nil || len(names) == 0 {
		return nil
	}
	return names
}
