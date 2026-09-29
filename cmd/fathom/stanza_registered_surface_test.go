package main

// stanza_registered_surface_test.go — EW-5 (efficiency wave, ADR-049
// amendment of 2026-09-29): every tool the rendered CLAUDE.md / AGENTS.md
// stanza names is one the registration serves, and the refresh line names a
// path that works there.
//
// `fathom init` registers the daemon at /rpc/agent, the eight-tool default
// (mcp.DefaultSurfaceTools). A tool named to an agent that is not on it is a
// call that fails: the `ingest` refresh instruction and `references` were the
// wasted calls the wave set out to remove, and `ingest` is served only from
// /rpc/extended (and the bare /rpc). TestStanzaNamesOnlyDefaultSurfaceTools
// (SR-34) pins the opening-moves arrow chains of the template; this test holds
// the RENDERED result the wiring engine writes (stanzaPlan, the plan --apply
// executes), for every registered harness, and holds the agent-facing files
// this repository checks in, whose hand-written lines sit beside the stanza.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

var (
	// stanzaBacktickIdent is a backticked lowercase identifier: the form an
	// agent reads as "a tool I can call" (`find_symbol`, `obligations`).
	stanzaBacktickIdent = regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	// stanzaBareToolIdent is an underscore-joined lowercase word cited bare.
	stanzaBareToolIdent = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)
)

// offDefaultSurfaceTools is every tool a curated surface serves beyond the
// default eight: the names an agent registered at /rpc/agent cannot call.
func offDefaultSurfaceTools() map[string]bool {
	def := map[string]bool{}
	for _, n := range mcp.DefaultSurfaceTools() {
		def[n] = true
	}
	off := map[string]bool{}
	for _, list := range [][]string{mcp.ExtendedSurfaceTools(), mcp.AgentSurfaceTools()} {
		for _, n := range list {
			if !def[n] {
				off[n] = true
			}
		}
	}
	return off
}

// stanzaToolCitations returns the off-default tool names a text cites: a
// backticked identifier, or a bare underscore-joined one.
func stanzaToolCitations(text string, off map[string]bool) []string {
	seen := map[string]bool{}
	for _, m := range stanzaBacktickIdent.FindAllStringSubmatch(text, -1) {
		if off[m[1]] {
			seen[m[1]] = true
		}
	}
	for _, name := range stanzaBareToolIdent.FindAllString(stanzaBacktickIdent.ReplaceAllString(text, " "), -1) {
		if off[name] {
			seen[name] = true
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func TestStanzaNamesOnlyRegisteredSurfaceTools(t *testing.T) {
	def := map[string]bool{}
	for _, n := range mcp.DefaultSurfaceTools() {
		def[n] = true
	}
	off := offDefaultSurfaceTools()

	// 1. The rendered plan: the region stanzaPlan would write into a repo
	// with no instruction file — AGENTS.md alone (ADR-059).
	plan, err := stanzaPlan(t.TempDir(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	rendered := map[string]string{}
	for _, it := range plan.Items {
		rendered[it.File] = it.After
	}
	if strings.TrimSpace(rendered["AGENTS.md"]) == "" {
		t.Fatalf("stanzaPlan rendered nothing for AGENTS.md (items: %+v)", plan.Items)
	}

	// 2. Plus every registered harness's body, with and without the intent
	// sentence: the hook paragraphs differ by harness, the routing must not.
	bodies := map[string]string{}
	for f, after := range rendered {
		bodies["plan:"+f] = after
	}
	for _, id := range harness.IDs() {
		for _, intentOn := range []bool{false, true} {
			key := id + "/plain"
			if intentOn {
				key = id + "/intent"
			}
			bodies[key] = fathomStanzaBodyFor(id, "svc", intentOn)
		}
	}
	for name, body := range bodies {
		for _, m := range stanzaBacktickIdent.FindAllStringSubmatch(body, -1) {
			if !def[m[1]] {
				t.Errorf("%s: the stanza names `%s`, which the default eight-tool surface (%v) does not serve", name, m[1], mcp.DefaultSurfaceTools())
			}
		}
		for _, n := range stanzaToolCitations(body, off) {
			t.Errorf("%s: the stanza cites %s, served only off the default surface", name, n)
		}

		// The refresh paragraph routes through what the default surface
		// does: a read queues the re-ingest itself (RT-99), nothing to call.
		var refresh string
		for _, para := range strings.Split(body, "\n\n") {
			if strings.Contains(para, "re-ingest") || strings.Contains(strings.ToLower(para), "refresh") {
				refresh = para
				break
			}
		}
		flat := strings.Join(strings.Fields(refresh), " ")
		if !strings.Contains(flat, "refreshes itself") || !strings.Contains(flat, "nothing to call") {
			t.Errorf("%s: the refresh line %q must say the index refreshes itself and there is nothing to call", name, flat)
		}
	}
	if _, ok := rendered["CLAUDE.md"]; ok || len(rendered) != 1 {
		t.Errorf("a repo with no CLAUDE file must get the stanza in AGENTS.md only, and no CLAUDE.md (items: %+v)", plan.Items)
	}

	// 3. What this repository actually checks in beside the stanza.
	t.Run("repo agent files", checkRepoAgentFilesNameOnlyServedTools)
}

// checkRepoAgentFilesNameOnlyServedTools holds this repository's own
// AGENTS.md — the one instruction file (ADR-059) — and any root CLAUDE file,
// the whole files, not only the marked stanza, to the same rule: a line that
// names a tool served beyond the default eight must also name the surface
// that serves it, or it teaches a call that fails (the hand-written routing
// rows told sessions to refresh via `ingest` and to resolve via `references`,
// neither on /rpc/agent).
func checkRepoAgentFilesNameOnlyServedTools(t *testing.T) {
	root := repoRootForDocs(t)
	off := offDefaultSurfaceTools()
	for _, f := range append([]string{agentsMDFile}, claudeFiles()...) {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if os.IsNotExist(err) && f != agentsMDFile {
			continue // ADR-059: a CLAUDE file is optional
		}
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		// AGENTS.md wraps its rows across lines; join a bullet or table row
		// back into one unit before reading it.
		for i, unit := range agentFileUnits(string(raw)) {
			names := stanzaToolCitations(unit, off)
			if len(names) == 0 {
				continue
			}
			if strings.Contains(unit, "/rpc/extended") || strings.Contains(unit, "/rpc/companion") {
				continue
			}
			t.Errorf("%s unit %d names %v, served only off the default surface, without saying which surface serves them: %q", f, i+1, names, strings.TrimSpace(unit))
		}
	}
}

// agentFileUnits splits an agent-config file into the units a reader takes
// as one sentence of guidance: a paragraph (blank-line separated), with a
// list bullet or table row starting a unit of its own.
func agentFileUnits(text string) []string {
	var units []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			units = append(units, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, ln := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(ln)
		switch {
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "#"):
			flush()
			cur = append(cur, trimmed)
		default:
			cur = append(cur, trimmed)
		}
	}
	flush()
	return units
}
