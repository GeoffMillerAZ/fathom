// Package goneev is the one verify.GoneEvidence both evidence sources build:
// `fathom verify --store` (cmd/fathom's cliEvidence) and the daemon's
// verify_plan (internal/mcp's verifyEvidence). Two adapters over one
// implementation is what keeps a `gone:` row's verdict the same through
// either path over one index (O1068: the daemon path had no GoneEvidence,
// so every gone: row read Unknown there while --store proved it).
//
// The rows read the corpus the index was built from (the service's
// recorded root) live, and design §3.1's first precondition holds the
// result to the index: the tree the plan was read from must be at the
// ingested commit with nothing uncommitted, and so must the indexed tree
// when it is a different checkout. A file the corpus could not read is
// named as a read failure (verify.GoneReadEvidence), never scored as a
// file without the target (I38).
package goneev

import (
	"context"
	"fmt"
	"go/build"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// Corpus is the tree a gone: row reads: every corpus path at HEAD and the
// named files' content. pkg/client.Client implements it; a file
// ReadSourceFiles leaves out of its answer was not readable.
type Corpus interface {
	SourcePaths(ctx context.Context, service string) ([]string, error)
	ReadSourceFiles(ctx context.Context, service string, paths []string) ([]core.SourceFile, error)
}

// PlanTree is the working tree a plan was read from, as design §3.1's
// first precondition compares it with the index. HeadErr / DirtyErr name
// why the head or the cleanliness could not be read — never a silent
// clean. Root is compared with the indexed root only (it is never read
// through this type), so a daemon can be told about a caller's tree
// without reading it.
type PlanTree struct {
	Root     string `json:"root,omitempty"`
	Head     string `json:"head,omitempty"`
	HeadErr  string `json:"head_error,omitempty"`
	Dirty    bool   `json:"dirty,omitempty"`
	DirtyErr string `json:"dirty_error,omitempty"`
	// DirtyPath is the first path git status reports (O1160).
	DirtyPath string `json:"dirty_path,omitempty"`
}

// ReadPlanTree reads root's HEAD and whether it carries any uncommitted
// state (staged, unstaged or untracked) through read-only git: never a
// write to the repository's index, so a daemon may read a tree it serves.
func ReadPlanTree(ctx context.Context, root string) PlanTree {
	if root == "" {
		return PlanTree{HeadErr: "no repository root resolved for the gone: precondition check"}
	}
	t := PlanTree{Root: root}
	head, err := gitro.Head(ctx, root)
	if err != nil {
		t.HeadErr = fmt.Sprintf("could not resolve git HEAD in %s: %v", root, err)
		return t
	}
	t.Head = strings.TrimSpace(head)
	out, err := gitro.Output(ctx, root, "status", "--porcelain", "--", ".")
	if err != nil {
		t.DirtyErr = fmt.Sprintf("could not check working tree cleanliness in %s: %v", root, err)
		return t
	}
	t.Dirty = len(strings.TrimSpace(string(out))) > 0
	if t.Dirty {
		t.DirtyPath = firstStatusPath(string(out))
	}
	return t
}

// firstStatusPath is the path on the first non-empty `git status
// --porcelain` line ("XY path", or "XY old -> new": the new path).
func firstStatusPath(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		if len(strings.TrimSpace(ln)) == 0 {
			continue
		}
		p := strings.TrimSpace(ln)
		if len(ln) > 3 {
			p = ln[3:]
		}
		if i := strings.LastIndex(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		return strings.Trim(p, `"`)
	}
	return ""
}

// Config is what an evidence source hands New.
type Config struct {
	Service string
	// IndexedCommit, MetaFound and MetaErr are the service's last ingest
	// record (ServiceMeta); CorpusRoot is the root it recorded, the tree
	// every corpus read comes from.
	IndexedCommit string
	MetaFound     bool
	MetaErr       error
	CorpusRoot    string
	// Plan is the tree the plan was read from.
	Plan   PlanTree
	Corpus Corpus
	// Store holds the persisted typed-pass outcome (nil: none known).
	Store store.Store
	// FindSymbol, Graph and ByKey answer the callers form: the row's
	// symbol, the call graph its typed referents come from, and each
	// canonical key's defining node.
	FindSymbol func(name string) []core.SymbolNode
	Graph      *callgraph.Graph
	ByKey      map[string]core.SymbolNode
}

// Evidence implements verify.GoneEvidence and verify.GoneReadEvidence over
// one Config. Its reads are cached for its lifetime — one verify run — so
// many gone: rows read the corpus once.
type Evidence struct {
	ctx context.Context
	cfg Config

	atHeadRead bool
	atHead     bool
	atHeadWhy  string

	pathsRead bool
	paths     []string

	goRead  bool
	goFiles []verify.GoFile

	failures map[string]bool
}

// New returns the evidence for cfg, reading nothing until a row asks.
func New(ctx context.Context, cfg Config) *Evidence {
	return &Evidence{ctx: ctx, cfg: cfg, failures: map[string]bool{}}
}

var (
	_ verify.GoneEvidence     = (*Evidence)(nil)
	_ verify.GoneReadEvidence = (*Evidence)(nil)
)

// IndexAtHead implements verify.GoneEvidence: the plan's tree must be at
// the ingested commit with no uncommitted state, and so must the indexed
// tree the rows read when it is another checkout — otherwise its live files
// are not the index's.
func (e *Evidence) IndexAtHead() (bool, string) {
	if !e.atHeadRead {
		e.atHeadRead = true
		e.atHead, e.atHeadWhy = e.indexAtHead()
	}
	return e.atHead, e.atHeadWhy
}

func (e *Evidence) indexAtHead() (bool, string) {
	p := e.cfg.Plan
	if p.HeadErr != "" {
		return false, p.HeadErr
	}
	if e.cfg.MetaErr != nil {
		return false, fmt.Sprintf("could not read ingest metadata for %q: %v", e.cfg.Service, e.cfg.MetaErr)
	}
	commit := e.cfg.IndexedCommit
	if !e.cfg.MetaFound || commit == "" {
		return false, "no commit was recorded for this service's last ingest — re-ingest before evaluating gone: rows"
	}
	if commit != p.Head {
		return false, fmt.Sprintf("the index was ingested at commit %s, HEAD is now %s — re-ingest", short(commit), short(p.Head))
	}
	if p.DirtyErr != "" {
		return false, p.DirtyErr
	}
	if p.Dirty {
		why := "the working tree has uncommitted changes since the index was ingested"
		if p.DirtyPath != "" {
			why += verify.DirtyPathMarker + p.DirtyPath + ")"
		}
		return false, why
	}
	cr := e.cfg.CorpusRoot
	if !filepath.IsAbs(cr) || sameDir(cr, p.Root) {
		return true, ""
	}
	ct := ReadPlanTree(e.ctx, cr)
	switch {
	case ct.HeadErr != "":
		return false, ct.HeadErr
	case ct.Head != commit:
		return false, fmt.Sprintf("the indexed tree %s is at commit %s, the index was ingested at %s — re-ingest", cr, short(ct.Head), short(commit))
	case ct.DirtyErr != "":
		return false, ct.DirtyErr
	case ct.Dirty:
		return false, fmt.Sprintf("the indexed tree %s has uncommitted changes since the index was ingested", cr)
	}
	return true, ""
}

// FailedTypedPackages implements verify.GoneEvidence from the store's
// PERSISTED typed-edge status (DG-10): a process's in-memory status never
// carries the pass an earlier ingest ran. The record carries one
// whole-service reason, so a failure renders as one named entry; a clean
// pass returns nil.
func (e *Evidence) FailedTypedPackages() []string {
	if e.cfg.Store == nil {
		return nil
	}
	rec, ok, err := e.cfg.Store.TypedEdgeStatus(e.ctx, e.cfg.Service)
	if err != nil || !ok || !rec.Failed {
		return nil
	}
	if rec.Reason == "" {
		return []string{"typed pass reported failed with no reason recorded"}
	}
	return []string{rec.Reason}
}

// GoFiles implements verify.GoneEvidence: every Go file in the corpus,
// each marked Excluded when go/build's MatchFile says the current build
// context (GOOS/GOARCH, no extra tags) leaves it out — the evaluation `go
// build` applies, so a build-tag-gated importer is still seen.
func (e *Evidence) GoFiles() []verify.GoFile {
	if e.goRead {
		return e.goFiles
	}
	e.goRead = true
	var want []string
	for _, p := range e.corpusPaths() {
		if strings.HasSuffix(p, ".go") {
			want = append(want, p)
		}
	}
	root := e.cfg.CorpusRoot
	if !filepath.IsAbs(root) {
		root = e.cfg.Plan.Root
	}
	for _, f := range e.read(want) {
		excluded := ""
		if root != "" {
			dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(f.Path)))
			if match, merr := build.Default.MatchFile(dir, filepath.Base(f.Path)); merr == nil && !match {
				excluded = "build-tag-excluded (go/build context mismatch)"
			}
		}
		e.goFiles = append(e.goFiles, verify.GoFile{Path: f.Path, Content: f.Content, Excluded: excluded})
	}
	return e.goFiles
}

// ModuleFile implements verify.GoneEvidence: go.mod's content in the
// corpus; ok is false when there is none.
func (e *Evidence) ModuleFile() ([]byte, bool) {
	files, err := e.cfg.Corpus.ReadSourceFiles(e.ctx, e.cfg.Service, []string{"go.mod"})
	if err != nil {
		e.fail(fmt.Sprintf("could not read go.mod at HEAD: %v", err))
		return nil, false
	}
	for _, f := range files {
		if f.Path == "go.mod" {
			return f.Content, true
		}
	}
	return nil, false
}

// CorpusFiles implements verify.GoneEvidence: every corpus file whose path
// is named exactly by paths or lies under one of its "dir/" entries —
// every file when paths is empty.
func (e *Evidence) CorpusFiles(paths []string) []verify.CorpusFile {
	var want []string
	for _, p := range e.corpusPaths() {
		if MatchesPaths(p, paths) {
			want = append(want, p)
		}
	}
	files := e.read(want)
	out := make([]verify.CorpusFile, 0, len(files))
	for _, f := range files {
		out = append(out, verify.CorpusFile{Path: f.Path, Content: f.Content})
	}
	return out
}

// TypedCallers implements verify.GoneEvidence: symbol's typed-tier
// referents (never heuristic or file-resolved edges), split into ordinary
// and _test.go callers. pkg, when set, narrows the matches to that
// directory the way `package:` does for a symbol row. A symbol that does
// not resolve to exactly one canonical key never reads as a proven zero:
// one unlocated referent is reported, so the row fails loud.
func (e *Evidence) TypedCallers(symbol, pkg string) (nonTest, test []verify.GoneReferent) {
	var matches []core.SymbolNode
	if e.cfg.FindSymbol != nil {
		matches = e.cfg.FindSymbol(symbol)
	}
	keys := map[string]bool{}
	for _, n := range matches {
		if pkg == "" || path.Dir(n.FilePath) == pkg {
			keys[n.Key()] = true
		}
	}
	if len(keys) != 1 || e.cfg.Graph == nil {
		return []verify.GoneReferent{{File: symbol, Line: 0}}, nil
	}
	var key string
	for k := range keys {
		key = k
	}
	for _, ci := range callgraph.CallersDetail(e.cfg.Graph, key) {
		if ci.Kind == "" || ci.Kind == core.EdgeUnresolved || ci.Kind == core.EdgeFileResolved {
			continue
		}
		n, ok := e.cfg.ByKey[ci.Key]
		if !ok {
			continue
		}
		ref := verify.GoneReferent{File: n.FilePath, Line: n.Line}
		if strings.HasSuffix(n.FilePath, "_test.go") {
			test = append(test, ref)
		} else {
			nonTest = append(nonTest, ref)
		}
	}
	sortReferents(nonTest)
	sortReferents(test)
	return nonTest, test
}

// GoneReadFailures implements verify.GoneReadEvidence: every corpus read
// that failed or came back short, sorted.
func (e *Evidence) GoneReadFailures() []string {
	out := make([]string, 0, len(e.failures))
	for f := range e.failures {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// MatchesPaths reports whether p is named exactly by one of paths or lies
// under one of its "dir/" entries; an empty paths matches everything (the
// pattern form's whole-corpus default).
func MatchesPaths(p string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, want := range paths {
		if p == want || (strings.HasSuffix(want, "/") && strings.HasPrefix(p, want)) {
			return true
		}
	}
	return false
}

// KeyIndex maps each node's canonical key to the node (TypedCallers'
// referent locations).
func KeyIndex(nodes []core.SymbolNode) map[string]core.SymbolNode {
	out := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		out[n.Key()] = n
	}
	return out
}

func (e *Evidence) corpusPaths() []string {
	if e.pathsRead {
		return e.paths
	}
	e.pathsRead = true
	if e.cfg.Corpus == nil {
		e.fail("no corpus is available to this evidence source")
		return nil
	}
	ps, err := e.cfg.Corpus.SourcePaths(e.ctx, e.cfg.Service)
	if err != nil {
		e.fail(fmt.Sprintf("could not list the corpus at HEAD: %v", err))
		return nil
	}
	e.paths = append([]string(nil), ps...)
	sort.Strings(e.paths)
	return e.paths
}

// read reads want (sorted), recording a failure for a read error or for
// any file the corpus left out of its answer.
func (e *Evidence) read(want []string) []core.SourceFile {
	if len(want) == 0 || e.cfg.Corpus == nil {
		return nil
	}
	files, err := e.cfg.Corpus.ReadSourceFiles(e.ctx, e.cfg.Service, want)
	if err != nil {
		e.fail(fmt.Sprintf("could not read the corpus at HEAD: %v", err))
	}
	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[f.Path] = true
	}
	var missing []string
	for _, p := range want {
		if !got[p] {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		e.fail(fmt.Sprintf("%d of %d corpus file(s) could not be read (first: %s)", len(missing), len(want), missing[0]))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func (e *Evidence) fail(reason string) { e.failures[reason] = true }

func sortReferents(refs []verify.GoneReferent) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].Line < refs[j].Line
	})
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if ra, err := filepath.EvalSymlinks(ca); err == nil {
		ca = ra
	}
	if rb, err := filepath.EvalSymlinks(cb); err == nil {
		cb = rb
	}
	return ca == cb
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
