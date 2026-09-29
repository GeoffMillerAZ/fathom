package client

// obligations_corpus.go — RT-143 and RT-81 (docs/design/responsible-tenant.md
// §4.3, docs/plans/responsible-tenant.md). The edit hook calls obligations on
// every edit; before this file, each call's layer coverage and raw fallback
// ran client.Sources — a discovery walk plus a read of EVERY file in the
// tree — twice. One profiled hour of the daemon allocated 468.6 GB there.
//
// Now an obligations call costs the edited file plus memoized state:
//
//   - The layer's corpus PATHS come from the file_stats rows the ingest
//     wrote (RT-82) — no walk — or, for a service not ingested since
//     file_stats existed, from one names-only discovery (SourcePaths),
//     filtered by the layer's own globs, computed once per (service, store
//     watermark, layer digest) in a memo group (internal/memo:
//     single-flighted, ctx-aware waiters, byte-budgeted, evicted when the
//     watermark moves), so a burst of edits between ingests shares one
//     derivation (layerCoveragePaths).
//   - Only the files the layer selects (the governing guides: CLAUDE.md,
//     ADR.md, …) are read, each stat'ed first, capped at HotPathReadCap, and
//     kept until its (size, mtime) moves; a guide over the cap or unreadable
//     is named as unread — coverage becomes a lower bound (I38), never a
//     silent zero.
//   - The raw fallback reads only the edited file, capped the same way
//     (RT-81).
//
// A NEW guide added to the tree is seen at the next ingest (the watermark
// moves); an edit to a known guide is seen on the next call.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// obligationsMemo is Client's per-process memo for the obligations hot
// path. Bounded: one paths entry per service, and a guide-content entry per
// selected corpus file of that service.
type obligationsMemo struct {
	mu     sync.Mutex
	paths  map[string]corpusPathsEntry // service -> the guide contents at one paths key
	edited map[string]editedEntry      // absolute path -> what was derived from it at a stat
}

// editedEntry is what the obligations hot path derived from one edited
// file at one stat (design §4.3: read again only when the stat moves): its
// fingerprint hash (the freshness check) and its behaviour tokens (the raw
// fallback's relevance signal). Never the content itself.
type editedEntry struct {
	size     int64
	mtimeNs  int64
	mode     os.FileMode
	hash     string
	hashOK   bool
	tokens   []string
	tokensOK bool
}

// editedMemoMax bounds the edited-file memo; past it the memo starts over
// rather than grow (an entry is a hash and a short token list).
const editedMemoMax = 4096

func (e editedEntry) sameStat(info os.FileInfo) bool {
	return e.size == info.Size() && e.mtimeNs == info.ModTime().UnixNano() && e.mode == info.Mode()
}

// editedLookup returns the memo entry for path when it was derived at
// info's stat; a moved stat is a miss.
func (c *Client) editedLookup(path string, info os.FileInfo) (editedEntry, bool) {
	m := &c.oblMemo
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.edited[path]
	if !ok || !e.sameStat(info) {
		return editedEntry{}, false
	}
	return e, true
}

// editedStore merges update into path's entry at info's stat (an entry at
// an older stat is replaced).
func (c *Client) editedStore(path string, info os.FileInfo, update func(*editedEntry)) {
	m := &c.oblMemo
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.edited == nil || len(m.edited) >= editedMemoMax {
		m.edited = map[string]editedEntry{}
	}
	e, ok := m.edited[path]
	if !ok || !e.sameStat(info) {
		e = editedEntry{size: info.Size(), mtimeNs: info.ModTime().UnixNano(), mode: info.Mode()}
	}
	update(&e)
	m.edited[path] = e
}

type corpusPathsEntry struct {
	key  string
	docs map[string]corpusDoc // repo-relative path -> content at a stat
}

// layerPathsKey keys the layer-coverage paths group: one client's
// service at one store watermark and layer digest.
type layerPathsKey struct {
	c                *Client
	service, mark, d string
}

func (k layerPathsKey) MemoScope() (string, string) { return k.service, k.mark }

// layerPathsGroup is the layer-coverage memo group (design §5: every
// derived cache is a memo group).
// Keep: a walk (the pre-file_stats path) outlives the hook that started
// it and is memoized for the next, instead of being redone per deadline
// (RT-143) — from its first instant, so its expected cost is nominal. The
// refuse drop still cancels it.
var layerPathsGroup = memo.NewGroup[layerPathsKey, []string]("layer_coverage_paths", memo.Options{WatermarkScoped: true, Keep: true, ExpectedCost: time.Nanosecond})

// corpusDoc is one guide's content as of a stat (size, mtime). fromStat is
// false for a connector with no on-disk root (faux, remote): its content is
// fixed for the life of the watermark.
type corpusDoc struct {
	size     int64
	mtimeNs  int64
	fromStat bool
	content  []byte
}

// corpusPathsBuilds counts real corpus-path walks — the RT-143 test's
// observable (a memo hit and a joined flight do not count).
var corpusPathsBuilds = struct {
	sync.Mutex
	n map[string]int
}{n: map[string]int{}}

func noteCorpusPathsBuild(service string) {
	corpusPathsBuilds.Lock()
	corpusPathsBuilds.n[service]++
	corpusPathsBuilds.Unlock()
}

// layerDigest is the layer half of the memo key: everything about the
// definition that changes which paths it selects.
func layerDigest(def semantic.LayerDef) string {
	return strings.Join([]string{
		def.Name, def.Version,
		strings.Join(def.CorpusInclude, "\x1f"),
		strings.Join(def.CorpusExclude, "\x1f"),
	}, "\x00")
}

// layerCoveragePaths returns the repo-relative paths of def's corpus for
// service, sorted: the file_stats paths (RT-82) — or, before the service's
// first ingest that wrote them, a names-only discovery (SourcePaths) — no
// file is read — filtered by def's include and exclude globs exactly as
// semantic.SelectCorpus filters a fetched corpus. Memoized per (service,
// store watermark, layer digest) in layerPathsGroup and single-flighted:
// concurrent callers at one key share one derivation, each leaving when
// its own ctx ends. A service with no store watermark (never ingested) is
// derived but never memoized (I21); an error is never memoized.
func (c *Client) layerCoveragePaths(ctx context.Context, service string, def semantic.LayerDef) ([]string, error) {
	wm, wmErr := c.StoreWatermark(ctx, service)
	if wmErr != nil {
		wm = ""
	}
	if tok := c.SuppressionToken(service); wm != "" && tok != "" {
		wm += "|suppressed:" + tok // file_stats is filtered: a suppression change is a new key
	}
	key := layerPathsKey{c: c, service: service, mark: wm, d: layerDigest(def)}
	return layerPathsGroup.Get(ctx, key, func(bctx context.Context) ([]string, int64, error) {
		paths, err := c.walkCorpusPaths(bctx, service, def)
		var size int64
		for _, p := range paths {
			size += int64(len(p)) + 16
		}
		return paths, size, err
	})
}

// pathsKeyString is the docs cache's key: guide contents are kept while
// the paths key holds, and start over when it moves.
func pathsKeyString(service, wm string, def semantic.LayerDef) string {
	return service + "\x00" + wm + "\x00" + layerDigest(def)
}

// walkCorpusPaths derives the layer's corpus paths: file_stats when the
// ingest wrote them (RT-82), else one names-only walk (RT-143).
func (c *Client) walkCorpusPaths(ctx context.Context, service string, def semantic.LayerDef) ([]string, error) {
	noteCorpusPathsBuild(service)
	var all []string
	if files, ok, err := c.st.FileStats(ctx, service); err == nil && ok {
		all = make([]string, len(files))
		for i, f := range files {
			all[i] = f.Path
		}
	} else {
		if all, err = c.SourcePaths(ctx, service); err != nil {
			return nil, err
		}
	}
	stubs := make([]core.SourceFile, len(all))
	for i, p := range all {
		stubs[i] = core.SourceFile{Path: p}
	}
	selected := semantic.SelectCorpus(stubs, def.CorpusInclude, def.CorpusExclude)
	out := make([]string, len(selected))
	for i, f := range selected {
		out[i] = f.Path
	}
	sort.Strings(out)
	return out, nil
}

// corpusSkip is one selected corpus file layerCorpus could not read.
type corpusSkip struct {
	path, reason string
}

func (k corpusSkip) String() string { return k.path + ": " + k.reason }

// skipStrings renders skips as "path: reason" lines (UnreadCorpus's shape).
func skipStrings(skips []corpusSkip) []string {
	if len(skips) == 0 {
		return nil
	}
	out := make([]string, len(skips))
	for i, k := range skips {
		out[i] = k.String()
	}
	return out
}

// layerCorpus returns def's selected corpus for service with contents, and
// the paths it could not read with why, in path order. Only the
// selected files are ever read; each is stat'ed first, skipped over
// HotPathReadCap, and re-read only when its (size, mtime) moved since the
// memoized copy. A selected path that no longer exists is simply absent —
// it is not part of the tree any more.
func (c *Client) layerCorpus(ctx context.Context, service string, def semantic.LayerDef) ([]core.SourceFile, []corpusSkip, error) {
	paths, err := c.layerCoveragePaths(ctx, service, def)
	if err != nil {
		return nil, nil, err
	}
	wm, _ := c.StoreWatermark(ctx, service)
	docsKey := ""
	if wm != "" {
		docsKey = pathsKeyString(service, wm, def)
	}
	root := c.serviceRoot(ctx, service)
	onDisk := false
	if root != "" {
		if st, serr := os.Stat(root); serr == nil && st.IsDir() {
			onDisk = true
		}
	}

	files := make([]core.SourceFile, 0, len(paths))
	var unread []corpusSkip
	for _, p := range paths {
		doc, cached := c.cachedCorpusDoc(service, docsKey, p)
		// RT-153: the corpus read is a tree producer. Before any file it
		// has to read (a memoized copy costs nothing), ask the producer
		// governor; a halt is the named error, never a shorter corpus.
		if !cached || onDisk {
			if gErr := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); gErr != nil {
				return nil, nil, gErr
			}
		}
		switch {
		case onDisk:
			abs := filepath.Join(root, filepath.FromSlash(p))
			info, serr := os.Stat(abs)
			if serr != nil {
				if errors.Is(serr, fs.ErrNotExist) {
					continue
				}
				unread = append(unread, corpusSkip{p, serr.Error()})
				continue
			}
			fresh := func(d corpusDoc) bool {
				return d.fromStat && d.size == info.Size() && d.mtimeNs == info.ModTime().UnixNano()
			}
			if !(cached && fresh(doc)) {
				type guide struct {
					doc     corpusDoc
					skipped string
				}
				v, rerr := c.flightDerive(ctx, "doc\x00"+service+"\x00"+docsKey+"\x00"+p, func() (any, error) {
					if d, ok := c.cachedCorpusDoc(service, docsKey, p); ok && fresh(d) {
						return guide{doc: d}, nil
					}
					data, rinfo, skipped, rerr := readFileCapped(ctx, abs, HotPathReadCap)
					if rerr != nil || skipped != "" {
						return guide{skipped: skipped}, rerr
					}
					d := corpusDoc{size: rinfo.Size(), mtimeNs: rinfo.ModTime().UnixNano(), fromStat: true, content: data}
					c.storeCorpusDoc(service, docsKey, p, d)
					return guide{doc: d}, nil
				})
				if rerr != nil {
					if errors.Is(rerr, fs.ErrNotExist) {
						continue
					}
					unread = append(unread, corpusSkip{p, rerr.Error()})
					continue
				}
				g := v.(guide)
				if g.skipped != "" {
					unread = append(unread, corpusSkip{p, g.skipped})
					continue
				}
				doc = g.doc
			}
		case !cached:
			data, ferr := c.conn.Fetch(ctx, core.SourceRef{Service: service, Path: p})
			if ferr != nil {
				unread = append(unread, corpusSkip{p, ferr.Error()})
				continue
			}
			if int64(len(data)) > HotPathReadCap {
				unread = append(unread, corpusSkip{p, oversizeNote(int64(len(data)), HotPathReadCap)})
				continue
			}
			doc = corpusDoc{content: data}
			c.storeCorpusDoc(service, docsKey, p, doc)
		}
		lang := connector.LanguageForPath(p)
		files = append(files, core.SourceFile{Path: p, Content: doc.content, Lang: lang})
	}
	return files, unread, nil
}

func (c *Client) cachedCorpusDoc(service, key, path string) (corpusDoc, bool) {
	m := &c.oblMemo
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.paths[service]
	if !ok || key == "" || e.key != key {
		return corpusDoc{}, false
	}
	d, ok := e.docs[path]
	return d, ok
}

// storeCorpusDoc keeps a guide's content under the current paths key; a
// new key (the watermark or layer moved) starts the service's cache over,
// so a stale entry never outlives its path. Uncached when the service has
// no watermark (an empty key segment is never a key, I21).
func (c *Client) storeCorpusDoc(service, key, path string, d corpusDoc) {
	if key == "" {
		return
	}
	m := &c.oblMemo
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.paths == nil {
		m.paths = map[string]corpusPathsEntry{}
	}
	e, ok := m.paths[service]
	if !ok || e.key != key {
		e = corpusPathsEntry{key: key, docs: map[string]corpusDoc{}}
		m.paths[service] = e
	}
	e.docs[path] = d
}

// serviceRoot is the service's recorded corpus root, or "".
func (c *Client) serviceRoot(ctx context.Context, service string) string {
	if meta, found, err := c.st.GetServiceMeta(ctx, service); err == nil && found {
		return meta.Path
	}
	return ""
}

// skipPaths returns skips' paths, in order.
func skipPaths(skips []corpusSkip) []string {
	out := make([]string, len(skips))
	for i, k := range skips {
		out[i] = k.path
	}
	return out
}

// probeAncestorGuides adds to corpus the guides a nested-guide include
// ("**/CLAUDE.md") selects in the edited file's own ancestor directories
// that the paths do not list yet — a guide written since the service's
// last ingest (RT-82: file_stats paths are the ingest's). Bounded: one
// stat per (ancestor, literal include), a read only for a guide that
// exists, under the hot-path cap; never a walk. The raw fallback's
// "a brand-new nested guide is served before any ingest" (ADR-037) holds.
func (c *Client) probeAncestorGuides(ctx context.Context, service string, def semantic.LayerDef, file string, corpus []core.SourceFile) []core.SourceFile {
	root := c.serviceRoot(ctx, service)
	if root == "" || file == "" || filepath.IsAbs(file) {
		return corpus
	}
	have := make(map[string]bool, len(corpus))
	for _, f := range corpus {
		have[f.Path] = true
	}
	var names []string
	for _, inc := range def.CorpusInclude {
		rest, ok := strings.CutPrefix(inc, "**/")
		if ok && rest != "" && !strings.ContainsAny(rest, "*?[/") {
			names = append(names, rest)
		}
	}
	if len(names) == 0 {
		return corpus
	}
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(file)))
	for {
		for _, name := range names {
			p := name
			if dir != "." && dir != "" {
				p = dir + "/" + name
			}
			if have[p] {
				continue
			}
			if len(semantic.SelectCorpus([]core.SourceFile{{Path: p}}, def.CorpusInclude, def.CorpusExclude)) == 0 {
				continue
			}
			abs := filepath.Join(root, filepath.FromSlash(p))
			if info, err := os.Stat(abs); err != nil || !info.Mode().IsRegular() {
				continue
			}
			data, _, skipped, err := readFileCapped(ctx, abs, HotPathReadCap)
			if err != nil || skipped != "" {
				continue
			}
			have[p] = true
			corpus = append(corpus, core.SourceFile{Path: p, Content: data, Lang: connector.LanguageForPath(p)})
		}
		if dir == "." || dir == "" || dir == "/" {
			break
		}
		dir = filepath.ToSlash(filepath.Dir(filepath.FromSlash(dir)))
	}
	// ADR-059: the root guides under .claude/ the connector never indexes.
	for _, f := range semantic.RootGuideExtras(root, def.CorpusInclude, def.CorpusExclude, have) {
		have[f.Path] = true
		corpus = append(corpus, f)
	}
	sort.Slice(corpus, func(i, j int) bool { return corpus[i].Path < corpus[j].Path })
	return corpus
}
