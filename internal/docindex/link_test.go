package docindex

import "testing"

// fakeResolver is a fixed-fixture Resolver for LinkDocSpans tests — no
// store, no I/O, matching internal/AGENTS.md's "pure, deterministic"
// discipline for this package's analyzers.
type fakeResolver struct {
	symbols map[string]string // name -> hash, present only when exactly one indexed symbol matches
	paths   map[string]string // path -> hash, present only when the path exists
	keys    map[string]string // normalized "kind:body" -> hash, present only when registered in contract_keys
}

func (f fakeResolver) ResolveSymbol(name string) (string, bool) {
	h, ok := f.symbols[name]
	return h, ok
}

func (f fakeResolver) ResolvePath(path string) (string, bool) {
	h, ok := f.paths[path]
	return h, ok
}

func (f fakeResolver) ResolveContractKey(key string) (string, bool) {
	h, ok := f.keys[key]
	return h, ok
}

// TestLinkDocSpansExactOnly pins KR-6 (design §6): a symbol, path or
// contract key inside backticks (or alone on a fenced-code-block line)
// resolves ONLY when the resolver reports exactly one match — an ambiguous
// symbol name, a path that does not exist, or a key that is not registered
// each produce no link, never a best guess. A bare word outside backticks
// is never linked at all, even when it names a real symbol (KR-X2: "too
// noisy to link; exact targets only").
func TestLinkDocSpansExactOnly(t *testing.T) {
	r := fakeResolver{
		symbols: map[string]string{"LinkDocSpans": "sym-hash-1"}, // "Ambiguous" deliberately absent: ambiguous, resolver refuses
		paths:   map[string]string{"internal/docindex/link.go": "path-hash-1"},
		keys:    map[string]string{"route:GET /health": "key-hash-1"}, // "cli:unregistered command" deliberately absent
	}

	body := "See `LinkDocSpans` and `Ambiguous` in `internal/docindex/link.go` " +
		"or `internal/docindex/missing.go`, wired to `route:GET /health` " +
		"but never `cli:unregistered command`. Plain LinkDocSpans in prose " +
		"outside backticks must not link.\n" +
		"```\n" +
		"internal/docindex/link.go\n" +
		"a whole line of prose with spaces\n" +
		"```\n"
	u := Unit{HeadingPath: "Test", LineStart: 10, LineEnd: 16, Kind: KindSection, Body: body}

	links := LinkDocSpans(u, r)

	byTarget := make(map[string]Link, len(links))
	for _, l := range links {
		byTarget[l.TargetKind+":"+l.Target] = l
	}

	// Resolved exactly-one cases link, at tier exact, with the resolver's hash.
	for _, want := range []struct{ kind, target, hash string }{
		{"symbol", "LinkDocSpans", "sym-hash-1"},
		{"path", "internal/docindex/link.go", "path-hash-1"},
		{"key", "route:GET /health", "key-hash-1"},
	} {
		got, ok := byTarget[want.kind+":"+want.target]
		if !ok {
			t.Fatalf("LinkDocSpans: missing expected %s link for %q; got %+v", want.kind, want.target, links)
		}
		if got.Tier != "exact" {
			t.Errorf("%s link tier = %q, want exact", want.target, got.Tier)
		}
		if got.TargetHash != want.hash {
			t.Errorf("%s link hash = %q, want %q", want.target, got.TargetHash, want.hash)
		}
	}

	// Ambiguous symbol, missing path, unregistered key: no link at all —
	// never a best guess at some other kind.
	for _, refused := range []string{"symbol:Ambiguous", "path:internal/docindex/missing.go", "key:cli:unregistered command"} {
		if _, ok := byTarget[refused]; ok {
			t.Errorf("LinkDocSpans: unresolved span %q was linked anyway: %+v", refused, byTarget[refused])
		}
	}

	// The fenced-code-block line that is a lone path resolves too — but the
	// multi-word fenced line ("a whole line of prose with spaces") does
	// not, since it is not a single bare token.
	if _, ok := byTarget["path:internal/docindex/link.go"]; !ok {
		t.Fatal("LinkDocSpans: expected the fenced-block lone-path line to link")
	}
	for _, l := range links {
		if l.Target == "a whole line of prose with spaces" {
			t.Errorf("LinkDocSpans: multi-word fenced line was linked: %+v", l)
		}
	}

	// The bare "LinkDocSpans" occurrence outside backticks in the trailing
	// prose sentence must not produce a SECOND symbol link — exactly one
	// symbol link total, from the backticked span.
	n := 0
	for _, l := range links {
		if l.TargetKind == "symbol" && l.Target == "LinkDocSpans" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("LinkDocSpans: %d symbol links for LinkDocSpans, want exactly 1 (bare prose word must not link)", n)
	}

	// Every link's Line falls within the unit's own line range.
	for _, l := range links {
		if l.Line < u.LineStart || l.Line > u.LineEnd {
			t.Errorf("link %+v: Line %d outside unit range [%d,%d]", l, l.Line, u.LineStart, u.LineEnd)
		}
	}
}

// TestLinkDocSpansNilResolver pins that a nil Resolver yields no links,
// never a nil-deref panic (I38: bad input is refused cleanly, not silently
// misread).
func TestLinkDocSpansNilResolver(t *testing.T) {
	u := Unit{Body: "`LinkDocSpans`", LineStart: 1, LineEnd: 1}
	if got := LinkDocSpans(u, nil); got != nil {
		t.Fatalf("LinkDocSpans(nil resolver) = %+v, want nil", got)
	}
}
