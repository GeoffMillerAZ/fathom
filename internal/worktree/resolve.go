package worktree

// Clone identity is shared by daemon joins, MCP paths and the edit fold.
// All git reads are local and bounded by the caller's context. Immutable
// commit facts are memoized; clones never mutate the service registry.
import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

type IndexedRoot struct {
	Service       string
	Root          string
	IndexedCommit string
}

type Resolution struct {
	IndexedRoot
	Root     string // corresponding service root in the requested checkout
	Target   string // canonical requested path
	Checkout string
	Class    string
	Note     string
}

// Resolve refuses equally strong candidates. A direct path uses the nearest
// containing service; outside recorded roots, a common git dir outranks a
// matching origin and complete root-commit set.
func Resolve(ctx context.Context, target string, services []IndexedRoot) (Resolution, bool) {
	if ctx.Err() != nil || !filepath.IsAbs(target) {
		return Resolution{}, false
	}
	target = canonical(target)
	services = append([]IndexedRoot(nil), services...)
	sort.Slice(services, func(i, j int) bool {
		if services[i].Service != services[j].Service {
			return services[i].Service < services[j].Service
		}
		return services[i].Root < services[j].Root
	})
	var direct []IndexedRoot
	for _, s := range services {
		if filepath.IsAbs(s.Root) && within(canonical(s.Root), target) {
			direct = append(direct, s)
		}
	}
	if len(direct) > 0 {
		sort.Slice(direct, func(i, j int) bool { return len(canonical(direct[i].Root)) > len(canonical(direct[j].Root)) })
		if len(direct) > 1 && canonical(direct[0].Root) == canonical(direct[1].Root) {
			return Resolution{}, false
		}
		s := direct[0]
		return Resolution{IndexedRoot: s, Root: canonical(s.Root), Target: target}, true
	}
	wt, _, ok := nearestWorkTree(target)
	if !ok || !cloneRootAllowed(wt, services) {
		return Resolution{}, false
	}
	common := commonDir(wt)
	if common == "" {
		return Resolution{}, false
	}
	origin := originFromConfig(filepath.Join(common, "config"))
	var matches []Resolution
	best := 0
	var roots string
	var rootsRead bool
	for _, s := range services {
		if ctx.Err() != nil {
			return Resolution{}, false
		}
		if !filepath.IsAbs(s.Root) {
			continue
		}
		base, _, found := nearestWorkTree(canonical(s.Root))
		if !found {
			continue
		}
		bc := commonDir(base)
		if bc == "" {
			continue
		}
		rank := 0
		class := "clone"
		if common != "" && common == bc {
			rank = 2
			class = "worktree"
		} else {
			bo := originFromConfig(filepath.Join(bc, "config"))
			if !originsMatch(origin, bo) {
				continue
			}
			if !rootsRead {
				roots, ok = RepositoryIdentity(ctx, wt)
				rootsRead = true
			}
			if !ok || roots == "" {
				continue
			}
			br, yes := RepositoryIdentity(ctx, base)
			if !yes || br != roots {
				continue
			}
			rank = 1
		}
		rel, err := filepath.Rel(base, canonical(s.Root))
		if err != nil || escapes(rel) {
			continue
		}
		cloneRoot := filepath.Join(wt, rel)
		if !within(cloneRoot, target) {
			continue
		}
		if rank > best || rank == best && len(matches) > 0 && len(cloneRoot) > len(matches[0].Root) {
			matches = nil
			best = rank
		}
		if rank == best && (len(matches) == 0 || cloneRoot == matches[0].Root) {
			matches = append(matches, Resolution{IndexedRoot: s, Root: cloneRoot, Target: target, Checkout: wt, Class: class})
		}
	}
	if len(matches) != 1 || ctx.Err() != nil {
		return Resolution{}, false
	}
	r := matches[0]
	head, err := gitro.Output(ctx, wt, "rev-parse", "HEAD")
	if err != nil {
		return Resolution{}, false
	}
	r.Note = fmt.Sprintf("clone at %s, index at %s", short(strings.TrimSpace(string(head))), short(r.IndexedCommit))
	return r, true
}

func short(s string) string {
	if s == "" {
		return "unknown"
	}
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && !escapes(rel)
}
func escapes(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel)
}

// Unchanged compares the actual clone bytes with the indexed commit, not
// with the base checkout's possibly dirty HEAD. Missing objects and files
// are unknown too. The caller still enforces its source-read exclusions.
func (r Resolution) Unchanged(ctx context.Context, rel string) (bool, string) {
	if r.Class == "" {
		return true, ""
	}
	if filepath.IsAbs(rel) || escapes(filepath.Clean(rel)) {
		return false, "unknown clone path"
	}
	full := filepath.Join(r.Root, filepath.FromSlash(rel))
	real, err := filepath.EvalSymlinks(full)
	if err != nil || !within(canonical(r.Root), real) {
		return false, "changed in clone"
	}
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 2<<20 {
		return false, "changed in clone"
	}
	data, excluded, err := corpus.ReadFileCapped(ctx, full, 2<<20)
	if err != nil || excluded != nil {
		return false, "clone file unreadable"
	}
	return r.MatchesBytes(ctx, rel, data)
}

func (r Resolution) MatchesBytes(ctx context.Context, rel string, data []byte) (bool, string) {
	if r.Class == "" {
		return true, ""
	}
	if r.IndexedCommit == "" {
		return false, "indexed commit unknown"
	}
	tree, err := indexedTree(ctx, r.IndexedRoot.Root, r.IndexedCommit)
	blob, found := tree[filepath.ToSlash(filepath.Clean(rel))]
	if err != nil || !found || (blob.mode != "100644" && blob.mode != "100755") {
		return false, "changed in clone"
	}
	var h hash.Hash = sha1.New()
	if len(blob.oid) == 64 {
		h = sha256.New()
	}
	fmt.Fprintf(h, "blob %d%c", len(data), 0)
	h.Write(data)
	if fmt.Sprintf("%x", h.Sum(nil)) != blob.oid {
		return false, "changed in clone"
	}

	return true, ""
}

// NormalizeOrigin folds transport/user spelling for the same remote. Local
// origins deliberately have no path identity: only the root commit matches.
func NormalizeOrigin(raw string) (remote string, local bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(raw), "file:") || filepath.IsAbs(raw) || strings.HasPrefix(raw, ".") {
		return "", true
	}
	host, path := "", ""
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		host = u.Host
		switch u.Port() {
		case "22", "443", "80":
			host = u.Hostname()
		}
		path = u.Path
	} else if a, b, ok := strings.Cut(raw, ":"); ok && !strings.Contains(a, "/") {
		host, path = a, b
	} else {
		return "", true
	}
	if _, h, ok := strings.Cut(host, "@"); ok {
		host = h
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimRight(path, "/")
	return strings.ToLower(host) + "/" + path, false
}
func originsMatch(a, b string) bool {
	na, la := NormalizeOrigin(a)
	nb, lb := NormalizeOrigin(b)
	return a != "" && la || b != "" && lb || a != "" && b != "" && na != "" && na == nb
}

// RelatedTarget retains XM-37's diagnostic classification for an unborn
// checkout. Serving a tracked fold additionally calls Resolve, so an origin
// alone can label a destination but cannot authorize indexed answers.
func RelatedTarget(roots []string, target string) (class, wt, root string) {
	wt, _, ok := nearestWorkTree(target)
	if !ok {
		return "", "", ""
	}
	common := commonDir(wt)
	for _, r := range roots {
		base, _, yes := nearestWorkTree(canonical(r))
		if !yes || base == wt {
			continue
		}
		bc := commonDir(base)
		if common != "" && common == bc {
			return "worktree", wt, r
		}
		origin := originFromConfig(filepath.Join(common, "config"))
		bo := originFromConfig(filepath.Join(bc, "config"))
		norm, local := NormalizeOrigin(origin)
		localPath := strings.TrimSuffix(strings.TrimPrefix(origin, "file://"), "/.git")
		if !filepath.IsAbs(localPath) {
			localPath = filepath.Join(wt, localPath)
		}
		localRoot := canonical(localPath)
		bn, _ := NormalizeOrigin(bo)
		if origin != "" && (local && localRoot == base || !local && norm != "" && norm == bn) {
			return "clone", wt, r
		}
	}
	return "", "", ""
}

// These file-only probes are the edit hook's original fold matcher, shared
// here instead of inventing a second repository-discovery path.
func nearestWorkTree(target string) (root, dotGit string, ok bool) {
	dir := filepath.Clean(target)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		p := filepath.Join(dir, ".git")
		if _, err := os.Lstat(p); err == nil {
			return dir, p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}
func commonDir(wt string) string {
	p := filepath.Join(wt, ".git")
	fi, err := os.Lstat(p)
	if err != nil {
		return ""
	}
	if fi.IsDir() {
		return canonical(p)
	}
	if !fi.Mode().IsRegular() {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	gd, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return ""
	}
	gd = strings.TrimSpace(gd)
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(wt, gd)
	}
	gd = canonical(gd)
	b, err = os.ReadFile(filepath.Join(gd, "commondir"))
	if err != nil {
		return ""
	}
	cd := absFrom(gd, strings.TrimSpace(string(b)))
	// A linked worktree is exactly <common>/worktrees/<name>, registered
	// by git with a backlink to this checkout's .git file (XM-37).
	if filepath.Dir(gd) != filepath.Join(cd, "worktrees") {
		return ""
	}
	backlink, err := os.ReadFile(filepath.Join(gd, "gitdir"))
	if err != nil || absFrom(gd, strings.TrimSpace(string(backlink))) != canonical(p) {
		return ""
	}
	return cd
}
func originFromConfig(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			in = line == `[remote "origin"]`
			continue
		}
		if in {
			if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "url" {
				return strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
	}
	return ""
}

// RootForPath is the nearest checkout containing a file or directory.
func RootForPath(target string) string { r, _, _ := nearestWorkTree(canonical(target)); return r }

// cloneRootAllowed keeps repository identity from authorizing broad disk
// roots or a checkout that encloses another indexed service.
func cloneRootAllowed(root string, services []IndexedRoot) bool {
	root = canonical(root)
	if filepath.Dir(root) == root {
		return false
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && within(root, canonical(home)) {
		return false
	}
	for _, s := range services {
		if filepath.IsAbs(s.Root) && within(root, canonical(s.Root)) {
			return false
		}
	}
	return true
}
