package fusion

// timonidispatch.go — ingest's claim of Timoni modules, bundles and runtimes
// (LB-F216, language-breadth fx wave, LB-Q-timoni). The dialect is native
// (internal/extractor/dialects/timoni): the CUE extraction plus the Timoni
// entities and references its contract carries. A .cue file is one when
// classifyCUE tags it ToolTimoni: its basename is timoni.cue, its content is a
// top-level bundle or a runtime, or it sits in a module, a directory tree under
// a timoni.cue (templates/, values.cue, images.cue, ...). The module's vendored
// dependencies under cue.mod/ stay plain CUE: they are the module's inputs, not
// its files.
//
// resolveExtractor (ingest and ExtractForBench) and ReingestFile both call
// timoniDispatch, so a Timoni file is read the same way however it arrives and
// every other .cue file reads as before.

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

var (
	timoniOnce sync.Once
	timoniExt  extractor.Extractor
)

// timoniExtractor is the dialect's extractor, built once (it holds no state).
func timoniExtractor() extractor.Extractor {
	timoniOnce.Do(func() { timoniExt = extractor.NewTimoniExtractor() })
	return timoniExt
}

// inCueModDir reports whether slash lies under a cue.mod directory.
func inCueModDir(slash string) bool {
	return strings.HasPrefix(slash, "cue.mod/") || strings.Contains(slash, "/cue.mod/")
}

// timoniDispatch returns the Timoni extractor and ToolKind when relPath is a
// .cue file of a Timoni module, bundle or runtime. idx is the sibling index
// ingest holds (it names every module root); nil reads the file alone, so only
// a file whose name or content says Timoni is claimed.
func timoniDispatch(relPath string, content []byte, idx *siblingIndex) (extractor.Extractor, ToolKind, bool) {
	if strings.ToLower(filepath.Ext(relPath)) != ".cue" {
		return nil, "", false
	}
	slash := filepath.ToSlash(relPath)
	if inCueModDir(slash) {
		return nil, "", false
	}
	if classifyCUE(slash, content, idx) != ToolTimoni {
		return nil, "", false
	}
	return timoniExtractor(), ToolTimoni, true
}

// diskTimoniIndex is a sibling index of the module roots above relPath, read
// from the repository on disk: the index ReingestFile's one-file refresh gives
// timoniDispatch where ingest holds the whole repository's.
func diskTimoniIndex(repoPath, relPath string) *siblingIndex {
	idx := &siblingIndex{timoniDirs: map[string]struct{}{}}
	segs := strings.Split(filepath.ToSlash(relPath), "/")
	for i := 0; i < len(segs)-1; i++ {
		root := "."
		if i > 0 {
			root = path.Join(segs[:i]...)
		}
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(root), "timoni.cue")); err == nil {
			idx.timoniDirs[root] = struct{}{}
		}
	}
	return idx
}
