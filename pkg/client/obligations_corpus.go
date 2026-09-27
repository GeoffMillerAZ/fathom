package client

// obligations_corpus.go — RT-143 and RT-81 (docs/design/responsible-tenant.md
// §4.3, docs/plans/responsible-tenant.md). The edit hook calls obligations on
// every edit; before this file, each call's layer coverage and raw fallback
// ran client.Sources — a discovery walk plus a read of EVERY file in the
// tree — twice. One profiled hour of the daemon allocated 468.6 GB there.
//
// Now an obligations call costs the edited file plus memoized state:
//
//   - The layer's corpus PATHS come from a names-only discovery
//     (SourcePaths), filtered by the layer's own globs, computed once per
//     (service, store watermark, layer digest) and single-flighted, so a
//     burst of edits between ingests shares one walk (layerCoveragePaths).
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

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// obligationsMemo is Client's per-process memo for the obligations hot
// path. Bounded: one paths entry per service, and a guide-content entry per
// selected corpus file of that service.
type obligationsMemo struct {
	mu     sync.Mutex
	paths  map[string]corpusPathsEntry // service -> the layer's corpus paths at one key
	flight map[string]*corpusPathsCall // full key -> the in-flight walk
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
	key   string
	paths []string
	docs  map[string]corpusDoc // repo-relative path -> content at a stat
}

type corpusPathsCall struct {
	done  chan struct{}
	paths []string
	err   error
}

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
// service, sorted: a names-only discovery (SourcePaths — no file is read)
// filtered by def's include and exclude globs exactly as semantic.
// SelectCorpus filters a fetched corpus. Memoized per (service, store
// watermark, layer digest) and single-flighted: concurrent callers at one
// key share one walk. A service with no store watermark (never ingested)
// cannot be memoized, but its concurrent callers still share one walk. A
// discovery error is returned and never memoized. Every caller — the one
// that started the walk included — leaves when its own ctx is cancelled;
// the walk itself runs detached and still memoizes its result.
func (c *Client) layerCoveragePaths(ctx context.Context, service string, def semantic.LayerDef) ([]string, error) {
	wm, wmErr := c.StoreWatermark(ctx, service)
	memoize := wmErr == nil && wm != ""
	key := service + "\x00" + wm + "\x00" + layerDigest(def)
	if !memoize {
		key = service + "\x00<no watermark>\x00" + layerDigest(def)
	}

	m := &c.oblMemo
	m.mu.Lock()
	if e, ok := m.paths[service]; memoize && ok && e.key == key {
		m.mu.Unlock()
		return e.paths, nil
	}
	call, joined := m.flight[key]
	if !joined {
		if m.flight == nil {
			m.flight = map[string]*corpusPathsCall{}
		}
		call = &corpusPathsCall{done: make(chan struct{})}
		m.flight[key] = call
		// The walk runs detached from every caller's cancellation: callers
		// joined to this flight must not inherit the first caller's
		// cancel, and a cancelled first caller still leaves a finished
		// walk for the next one.
		go c.runCorpusPathsWalk(context.WithoutCancel(ctx), service, def, key, memoize, call)
	}
	m.mu.Unlock()

	select {
	case <-call.done:
		return call.paths, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// runCorpusPathsWalk is one flight's walk: it records the result (memoized
// only when the key carries a watermark) and releases the flight.
func (c *Client) runCorpusPathsWalk(ctx context.Context, service string, def semantic.LayerDef, key string, memoize bool, call *corpusPathsCall) {
	call.paths, call.err = c.walkCorpusPaths(ctx, service, def)

	m := &c.oblMemo
	m.mu.Lock()
	delete(m.flight, key)
	if memoize && call.err == nil {
		if m.paths == nil {
			m.paths = map[string]corpusPathsEntry{}
		}
		// A new key starts a fresh guide-content cache: the path set may
		// have changed, and a stale entry must not outlive its path.
		m.paths[service] = corpusPathsEntry{key: key, paths: call.paths, docs: map[string]corpusDoc{}}
	}
	m.mu.Unlock()
	close(call.done)
}

// walkCorpusPaths is the one names-only walk layerCoveragePaths memoizes.
func (c *Client) walkCorpusPaths(ctx context.Context, service string, def semantic.LayerDef) ([]string, error) {
	noteCorpusPathsBuild(service)
	all, err := c.SourcePaths(ctx, service)
	if err != nil {
		return nil, err
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
		doc, cached := c.cachedCorpusDoc(service, p)
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
			if !(cached && doc.fromStat && doc.size == info.Size() && doc.mtimeNs == info.ModTime().UnixNano()) {
				data, rinfo, skipped, rerr := readFileCapped(abs, HotPathReadCap)
				if rerr != nil {
					if errors.Is(rerr, fs.ErrNotExist) {
						continue
					}
					unread = append(unread, corpusSkip{p, rerr.Error()})
					continue
				}
				if skipped != "" {
					unread = append(unread, corpusSkip{p, skipped})
					continue
				}
				doc = corpusDoc{size: rinfo.Size(), mtimeNs: rinfo.ModTime().UnixNano(), fromStat: true, content: data}
				c.storeCorpusDoc(service, p, doc)
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
			c.storeCorpusDoc(service, p, doc)
		}
		lang := connector.LanguageForPath(p)
		files = append(files, core.SourceFile{Path: p, Content: doc.content, Lang: lang})
	}
	return files, unread, nil
}

func (c *Client) cachedCorpusDoc(service, path string) (corpusDoc, bool) {
	m := &c.oblMemo
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.paths[service]
	if !ok {
		return corpusDoc{}, false
	}
	d, ok := e.docs[path]
	return d, ok
}

func (c *Client) storeCorpusDoc(service, path string, d corpusDoc) {
	m := &c.oblMemo
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.paths[service]; ok && e.docs != nil {
		e.docs[path] = d
	}
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
