package fusion

// scriptcalls.go — owner ruling S1's lever (wave lf7, item shell, LB-F520): a
// shell script's top-level call (a call its module symbol makes, K18) is
// stored only when its callee is a definition of the repository: in the
// script, in a file the script sources, or anywhere else the service defines
// it. A script's top level is mostly external commands (echo, grep, rm,
// mkdir), which carry no navigation value, and every stored call is a row of
// the observation's calls and of the observation_calls index. A call that
// binds (the file-resolve pass's edge into a sourced file) names a definition
// of the repository by construction, so no edge is lost; a call inside a
// function is untouched.
//
// Which definitions the repository has is known only once the extraction
// stream has ended, so a scoped script's rows (its observations and, after
// them, its fingerprint) wait in the sink until the stream's flush: its own
// definitions and every other waiting script's are known in memory, and a
// name none of them defines is looked up in the store, which by then holds
// every other file's rows (this stream's and the unchanged files'). A run
// stopped before the flush leaves the waiting scripts unfingerprinted, so the
// next ingest reads them again (RT-103). A callee no file defined when its
// script was read stays dropped until that script is read again (an edit, or
// an ExtractorEpoch bump).

import (
	"context"
	"slices"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// scriptCallDialects are the dialects whose module symbol's calls the lever
// keeps only when they bind to a definition (S1: bash; PowerShell is not in
// scope).
var scriptCallDialects = []string{"bash"}

var scriptModuleKinds = sync.OnceValue(func() map[core.SourceKind]bool {
	out := map[core.SourceKind]bool{}
	ds, err := extractor.LoadDialects()
	if err != nil {
		return out
	}
	for _, d := range ds {
		if d.ModuleSymbol != "" && slices.Contains(scriptCallDialects, d.ID) {
			out[core.SourceKind(d.Kinds[d.ModuleSymbol].SourceKind)] = true
		}
	}
	return out
})

// heldScript is one scoped script's rows, waiting for the stream's end.
type heldScript struct {
	obs []core.Observation
	fp  pendingFingerprint
}

// heldScripts is the sink's waiting scripts and the names their own
// definitions carry.
type heldScripts struct {
	files   []heldScript
	defined map[string]bool
}

// holdScript moves the file just added (its observations, the last len(nodes)
// of s.obs, one per node, and its fingerprint, the last of s.fps) into the
// hold when nodes carry a scoped module symbol.
func (s *extractSink) holdScript(nodes []core.SymbolNode) {
	kinds := scriptModuleKinds()
	if len(kinds) == 0 || !slices.ContainsFunc(nodes, func(n core.SymbolNode) bool { return n.IsModuleSymbol() && kinds[n.Kind] }) {
		return
	}
	h := &s.scripts
	if h.defined == nil {
		h.defined = map[string]bool{}
	}
	for _, n := range nodes {
		if !n.IsModuleSymbol() {
			h.defined[n.Name] = true
		}
	}
	cut := len(s.obs) - len(nodes)
	h.files = append(h.files, heldScript{obs: slices.Clone(s.obs[cut:]), fp: s.fps[len(s.fps)-1]})
	s.obs = s.obs[:cut]
	s.fps = s.fps[:len(s.fps)-1]
}

// releaseScripts filters each waiting script's module calls to the callees
// the repository defines and queues its rows, then its fingerprint, for the
// write that follows. The rows already queued are written first, so the
// store answers for every file but the waiting ones. A failed lookup keeps
// every call (no row is dropped on a guess).
func (s *extractSink) releaseScripts(ctx context.Context) error {
	h := &s.scripts
	if len(h.files) == 0 {
		return nil
	}
	if err := s.write(ctx); err != nil {
		return err
	}
	keep := h.defined
	var ask []string
	for _, f := range h.files {
		for _, o := range f.obs {
			if o.Symbol != core.ModuleSymbolName {
				continue
			}
			for _, c := range o.Calls {
				if !keep[c] {
					ask = append(ask, c)
				}
			}
		}
	}
	slices.Sort(ask)
	ask = slices.Compact(ask)
	failed := false
	if len(ask) > 0 {
		rows, err := s.p.st.SymbolSourcesForNames(ctx, s.serviceID, ask)
		failed = err != nil
		for _, r := range rows {
			keep[r.Symbol] = true
		}
	}
	for _, f := range h.files {
		for i, o := range f.obs {
			if o.Symbol == core.ModuleSymbolName && !failed {
				f.obs[i].Calls = slices.DeleteFunc(slices.Clone(o.Calls), func(c string) bool { return !keep[c] })
				if len(f.obs[i].Calls) == 0 {
					f.obs[i].Calls = nil // as a node with no calls is written
				}
			}
		}
		s.obs = append(s.obs, f.obs...)
		s.fps = append(s.fps, f.fp)
	}
	h.files, h.defined = nil, nil
	return nil
}
