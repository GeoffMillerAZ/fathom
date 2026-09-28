package main

// stanza_surface_test.go — SR-34 (surface reckoning, design §1.1): the
// fathom stanza stops telling agents to call `ingest` (auto-refresh
// queues it, RT-99) and its ROUTING instructions — the opening-moves
// table's arrow-chains — name only the default eight-tool floor
// (mcp.DefaultSurfaceTools, SR-33), never a tool a client registered at
// /rpc/agent cannot actually call. Naming the wider verb set as
// available one hop away at /rpc/extended is DISCLOSURE, not an
// instruction to call it here, so that sentence is exempt from the
// arrow-chain check below.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// arrowChainLine matches one "Opening moves" table row: "- <label>:
// <verb> -> <verb> -> <verb>" (a bare verb, or "verb(arg)" — the arg is
// dropped before the membership check).
var arrowChainLine = regexp.MustCompile(`(?m)^- [^:]+: (.+)$`)

func TestStanzaNamesOnlyDefaultSurfaceTools(t *testing.T) {
	def := map[string]bool{}
	for _, name := range mcp.DefaultSurfaceTools() {
		def[name] = true
	}

	for _, intentOn := range []bool{false, true} {
		body := fathomStanzaBody("svc", intentOn)

		matches := arrowChainLine.FindAllStringSubmatch(body, -1)
		if len(matches) == 0 {
			t.Fatal("no opening-moves arrow-chain rows found — did the table move or get renamed?")
		}
		for _, m := range matches {
			for _, step := range strings.Split(m[1], "->") {
				verb := strings.TrimSpace(step)
				if paren := strings.IndexByte(verb, '('); paren >= 0 {
					verb = verb[:paren]
				}
				if verb == "" {
					continue
				}
				if !def[verb] {
					t.Errorf("intentOn=%v: opening-moves chain names %q, which is not on the default eight-tool floor (%v) — a client registered only at /rpc/agent cannot call it", intentOn, verb, mcp.DefaultSurfaceTools())
				}
			}
		}

		// The stanza must no longer instruct a direct `ingest` call:
		// auto-refresh (RT-99) queues the re-ingest on read.
		if strings.Contains(body, "Refresh the index via the `ingest`") {
			t.Errorf("intentOn=%v: stanza still tells agents to call `ingest` directly — auto-refresh queues it now", intentOn)
		}
		if !strings.Contains(body, "auto-refresh") && !strings.Contains(body, "queues") {
			t.Errorf("intentOn=%v: stanza dropped the ingest instruction but never explains that a read auto-refreshes", intentOn)
		}

		// Disclosure of the wider set must name the opt-in surface, not
		// instruct a call on the default one.
		if !strings.Contains(body, "/rpc/extended") {
			t.Errorf("intentOn=%v: stanza never names /rpc/extended as where the wider verb set lives", intentOn)
		}
	}
}
