package docindex

// link.go — LinkDocSpans (KR-6, docs/design/knowledge-retrieval.md §6): a
// span is linked when it names, inside single backticks or alone on a
// fenced-code-block line, a symbol, a repo-relative path, or a contract
// key that resolves EXACTLY — never a best guess, and never a bare word
// outside backticks (KR-X2, refused: "too noisy to link; exact targets
// only"). Pure and deterministic (internal/CLAUDE.md): the actual lookups
// (does a symbol resolve to exactly one indexed match, does a path exist,
// is a contract key registered) are the caller's Resolver — this file has
// no store I/O of its own, matching RankDocs' (KR-4) own precedent of
// keeping the docindex package's ranking/linking logic testable against a
// fixed fixture rather than a live store.
//
// The write side — persisting a Link set to doc_links, and closing a link
// that stops holding while never touching an unchanged one's frozen
// target_hash_at_link (the validity-window rule §4 and §6 both cite) —
// lives in internal/store (SyncDocLinksForUnit) and internal/docindex's
// own docdrift.go (KR-7), which reads what this file computed back out to
// grade it against the live index.

import (
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// Resolver is the read side LinkDocSpans grades a backticked/fenced span
// against. Each method returns ok=false, never a guess, when the span does
// not resolve to EXACTLY one live target (§6's "resolving to exactly one
// indexed symbol", "a path that exists", "a key that... exists in
// contract_keys"): zero matches and more than one both refuse. hash is the
// target's content hash at resolution time — what a later docDrift (KR-7)
// compares a fresh resolution against to detect the target moving.
type Resolver interface {
	// ResolveSymbol resolves a bare or pkg-qualified name to the content
	// hash of the exactly-one indexed symbol it names.
	ResolveSymbol(name string) (hash string, ok bool)
	// ResolvePath resolves a repo-relative path to its content hash, when
	// the path exists.
	ResolvePath(path string) (hash string, ok bool)
	// ResolveContractKey resolves an already-parsed, normalized
	// "kind:body" contract key (syslink.ContractKey.String()) to its
	// provider's content hash, when the key is registered in
	// contract_keys. Until SG-2's contract_keys table lands, a real
	// caller's ResolveContractKey returns ok=false for every input — the
	// same "degrades to no signal, not an error" shape RankDocs (KR-4)
	// took for doc_links before KR-6 existed.
	ResolveContractKey(key string) (hash string, ok bool)
}

// Link is one resolved span: what it names, where, at what tier, and the
// target's content hash at link time. §6 defines only the exact tier for
// doc-to-code links (no heuristic tier the way route/rpc extraction has
// one) — Tier is always "exact" today, carried as a field rather than a
// hard-coded literal so a future tier never needs a shape change.
type Link struct {
	TargetKind string // "symbol" | "path" | "key"
	Target     string // the resolved, canonical form (a parsed key's String(); the span text verbatim for symbol/path)
	Line       int    // 1-based, absolute within the doc file (best-effort: the unit's own LineStart plus the span's offset within Body)
	Tier       string // always "exact" (§6)
	TargetHash string
}

var (
	// inlineBacktickRE matches inline code spans on a single line: `content`.
	// Run per-line (never across a fenced block, which is tracked
	// separately below), so `[^`]` alone is enough — no line ever contains
	// an embedded "\n" for it to over-match across.
	inlineBacktickRE = regexp.MustCompile("`([^`]+)`")
	// fenceRE matches a fenced-code-block delimiter line (``` or ~~~),
	// matching internal/analysis/docdrift's own convention exactly so the
	// two packages agree on what counts as "inside a code block".
	fenceRE = regexp.MustCompile(`^\s*(` + "`{3,}|~{3,}" + `)`)
)

// spanCandidate is one backtick/fenced-line span LinkDocSpans found,
// before resolution: its trimmed text and its 0-based line offset within
// the unit's Body.
type spanCandidate struct {
	text string
	line int
}

// scanSpanCandidates walks body and returns every span §6 makes eligible
// for linking: inline “ `...` “ spans outside any fence, and — inside a
// fenced code block — a line whose trimmed content is a single bare token
// (no interior whitespace), since a multi-word code-block line is example
// code, not a name, and linking every word in it would reopen the "bare
// word" noise §6's own table refuses. A fence delimiter line itself is
// never a candidate.
func scanSpanCandidates(body string) []spanCandidate {
	lines := strings.Split(body, "\n")
	var out []spanCandidate
	inFence := false
	for i, line := range lines {
		if fenceRE.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.ContainsAny(trimmed, " \t") {
				out = append(out, spanCandidate{text: trimmed, line: i})
			}
			continue
		}
		for _, m := range inlineBacktickRE.FindAllStringSubmatch(line, -1) {
			text := strings.TrimSpace(m[1])
			if text != "" {
				out = append(out, spanCandidate{text: text, line: i})
			}
		}
	}
	return out
}

// contractKeyKinds is syslink.Kinds() as a lookup set, computed once — the
// prefix test scanSpanCandidates' output is classified against before ever
// calling syslink.ParseContractKey, so a symbol or path that happens to
// contain a colon (neither grammar allows one, but a resolver is free to
// be stricter than its interface requires) is never misparsed as an
// attempted contract key.
var contractKeyKinds = func() map[string]bool {
	m := make(map[string]bool, len(syslink.Kinds()))
	for _, k := range syslink.Kinds() {
		m[string(k)] = true
	}
	return m
}()

// looksLikeContractKey reports whether text's shape ("kind:body") names
// one of the system graph's eleven kinds before the first ':' — the signal
// LinkDocSpans uses to try contract-key resolution FIRST and, on that
// attempt's failure, refuse the span outright rather than falling through
// to path or symbol resolution (a span shaped like a contract key is
// never coincidentally also a path or a symbol name in this corpus).
func looksLikeContractKey(text string) bool {
	i := strings.IndexByte(text, ':')
	return i > 0 && contractKeyKinds[text[:i]]
}

// classifySpan resolves one candidate span's text against r, trying
// contract key, then path, then symbol — the first exact resolution wins.
// A span shaped like a contract key that fails to parse or to resolve is
// refused outright (never re-tried as a path or symbol); a span that
// resolves as neither a path nor a symbol is refused. Returns ok=false for
// anything not resolved exactly.
func classifySpan(text string, r Resolver) (kind, target, hash string, ok bool) {
	if looksLikeContractKey(text) {
		key, err := syslink.ParseContractKey(text)
		if err != nil {
			return "", "", "", false
		}
		if h, ok2 := r.ResolveContractKey(key.String()); ok2 {
			return "key", key.String(), h, true
		}
		return "", "", "", false
	}
	if h, ok2 := r.ResolvePath(text); ok2 {
		return "path", text, h, true
	}
	if h, ok2 := r.ResolveSymbol(text); ok2 {
		return "symbol", text, h, true
	}
	return "", "", "", false
}

// LinkDocSpans resolves every eligible span in u.Body against r and
// returns the ones that resolve exactly, each tagged with its 1-based
// absolute line (u.LineStart plus the span's own offset — best-effort:
// §3's per-kind splitters do not all keep Body's first line perfectly
// aligned with LineStart, so this is a navigation aid, not a byte-exact
// citation). A nil Resolver yields nil, never a panic (I38).
func LinkDocSpans(u Unit, r Resolver) []Link {
	if r == nil {
		return nil
	}
	candidates := scanSpanCandidates(u.Body)
	var out []Link
	for _, c := range candidates {
		kind, target, hash, ok := classifySpan(c.text, r)
		if !ok {
			continue
		}
		out = append(out, Link{
			TargetKind: kind,
			Target:     target,
			Line:       u.LineStart + c.line,
			Tier:       "exact",
			TargetHash: hash,
		})
	}
	return out
}
