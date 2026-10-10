package routes

// compose.go — the route seam's cross-file half (LB-F321): one service's
// per-file readings, mounts applied.

import (
	stdpath "path"
	"sort"
	"strings"
)

// maxMountDepth bounds how deep mounts compose: a file reached through
// more nested mounts than this is read as if unmounted.
const maxMountDepth = 8

// Compose applies every mount across one service's per-file readings and
// returns the service's routes at the paths they are served on:
//
//   - every route of every file a mount's Target names is served under the
//     mount's Prefix, joined to the route's path with exactly one "/";
//   - a mounted file's own mounts compose recursively (to depth 8, a file
//     never mounting itself again on one path), so a file mounted N times
//     yields its routes N times;
//   - a file no mount names keeps its local paths, and so does a file only
//     a mount cycle or the depth bound reaches (a mount cycle with no
//     unmounted file is read from its first file in path order);
//   - Handler, File and Line are unchanged;
//   - a route whose path is a deferred reference to an imported constant
//     (pathconst.go) is read at the constant's value, and left unread when
//     the constant does not resolve.
//
// A Target is resolved against the files the readings name (any file with
// a route or a mount):
//
//   - "./x" or "../x": relative to the mounting file's directory;
//   - "/x": repo-relative;
//   - otherwise a dotted module "a.b.urls": the path suffix a/b/urls.<ext>
//     or a/b/urls/__init__.py.
//
// For the first two, a Target ending in "/" names every file directly
// inside that directory; one ending in "/**" every file under it, at any
// depth to the depth bound (Symfony's recursive directory import, O698);
// one ending in "/**?dirs" the same files, each served under Prefix joined
// with its folders below the directory (@fastify/autoload's
// dirNameRoutePrefix: routes/users/index.ts under <prefix>/users).
// Otherwise the file itself, else the files whose name is x plus an
// extension, else x's index.<ext> or __init__.py. A target naming no file
// is ignored. Output is sorted (sortRoutes) and independent of input
// order.
func Compose(files []Result) []Route {
	c := composer{local: map[string][]Route{}, mounts: map[string][]Mount{}, reached: map[string]bool{}}
	known := map[string]bool{}
	consts := newPathConsts(files)
	for _, f := range files {
		for _, r := range append(append([]Route(nil), f.Routes...), f.Deferred...) {
			if isDeferred(r.Path) { // an imported path constant (pathconst.go)
				p, ok := consts.resolveImportedPath(r.File, r.Path)
				if !ok {
					continue
				}
				r.Path = p
			}
			c.local[r.File] = append(c.local[r.File], r)
			known[r.File] = true
		}
		for _, m := range f.Mounts {
			c.mounts[m.File] = append(c.mounts[m.File], m)
			known[m.File] = true
		}
	}
	for p := range known {
		c.paths = append(c.paths, p)
	}
	sort.Strings(c.paths)
	for _, p := range c.paths {
		sortRoutes(c.local[p])
		sortMounts(c.mounts[p])
	}

	mounted := map[string]bool{}
	for _, p := range c.paths {
		for _, m := range c.mounts[p] {
			for _, t := range c.resolve(m) {
				if t.file != m.File { // a file's own mount never mounts it (a directory holding it)
					mounted[t.file] = true
				}
			}
		}
	}
	for _, p := range c.paths {
		if !mounted[p] {
			c.walk(p, "", 0, map[string]bool{})
		}
	}
	for _, p := range c.paths {
		if !c.reached[p] {
			c.walk(p, "", 0, map[string]bool{})
		}
	}
	sortRoutes(c.out)
	return c.out
}

type composer struct {
	paths   []string
	local   map[string][]Route
	mounts  map[string][]Mount
	reached map[string]bool
	out     []Route
}

// walk emits file's routes under prefix, then composes its mounts.
func (c *composer) walk(file, prefix string, depth int, onPath map[string]bool) {
	c.reached[file] = true
	onPath[file] = true
	defer delete(onPath, file)
	for _, r := range c.local[file] {
		r.Path = joinPathSegments(prefix, r.Path)
		c.out = append(c.out, r)
	}
	if depth >= maxMountDepth {
		return
	}
	for _, m := range c.mounts[file] {
		next := joinPathSegments(prefix, m.Prefix)
		for _, t := range c.resolve(m) {
			if onPath[t.file] {
				continue
			}
			c.walk(t.file, joinPathSegments(next, t.folders), depth+1, onPath)
		}
	}
}

// target is one file a Mount names, with the folder path a "/**?dirs"
// Target joins to the mount's prefix ("" for every other form).
type target struct {
	file    string
	folders string
}

// The recursive directory Target suffixes (Mount.Target): "/**" names
// every file under a directory, "/**?dirs" the same files with their
// folders joined to the prefix.
const (
	treeSuffix     = "/**"
	treeDirsSuffix = "/**?dirs"
)

// dirMode is how a Target names a directory's files.
type dirMode int

const (
	notDir         dirMode = iota
	dirDirect              // "x/": the files directly inside x
	dirTree                // "x/**": every file under x
	dirTreeFolders         // "x/**?dirs": every file under x, its folders joined to the prefix
)

// resolve returns the known files m's Target names, sorted.
func (c *composer) resolve(m Mount) []target {
	t, mode := m.Target, notDir
	switch {
	case strings.HasSuffix(t, treeDirsSuffix):
		t, mode = strings.TrimSuffix(t, treeDirsSuffix)+"/", dirTreeFolders
	case strings.HasSuffix(t, treeSuffix):
		t, mode = strings.TrimSuffix(t, treeSuffix)+"/", dirTree
	case strings.HasSuffix(t, "/"):
		mode = dirDirect
	}
	switch {
	case strings.HasPrefix(t, "./") || strings.HasPrefix(t, "../"):
		return c.resolvePath(stdpath.Join(stdpath.Dir(m.File), t), mode)
	case strings.HasPrefix(t, "/"):
		return c.resolvePath(stdpath.Clean(strings.TrimPrefix(t, "/")), mode)
	case mode != notDir || t == "" || strings.ContainsAny(t, "/\\") || strings.HasPrefix(t, "."):
		return nil
	default:
		return files(c.resolveModule(strings.ReplaceAll(t, ".", "/")))
	}
}

// resolvePath resolves a cleaned repo-relative path: a directory's direct
// files, or every file under it (mode), else the file, else x.<ext>, else
// x's index.<ext> or __init__.py.
func (c *composer) resolvePath(p string, mode dirMode) []target {
	if p == ".." || strings.HasPrefix(p, "../") {
		return nil
	}
	switch mode {
	case dirDirect:
		var out []target
		for _, f := range c.paths {
			if stdpath.Dir(f) == p {
				out = append(out, target{file: f})
			}
		}
		return out
	case dirTree, dirTreeFolders:
		var out []target
		for _, f := range c.paths {
			rel, ok := under(f, p)
			if !ok {
				continue
			}
			folders := stdpath.Dir(rel)
			if folders == "." {
				folders = ""
			}
			if folders != "" && strings.Count(folders, "/")+1 > maxMountDepth {
				continue // past the depth bound: the file keeps its local paths
			}
			if mode == dirTree || folders == "" {
				folders = ""
			} else {
				folders = "/" + folders
			}
			out = append(out, target{file: f, folders: folders})
		}
		return out
	}
	for _, f := range c.paths {
		if f == p {
			return []target{{file: f}}
		}
	}
	var stem, index []string
	for _, f := range c.paths {
		if stemOf(f) == p {
			stem = append(stem, f)
		}
		if stdpath.Dir(f) == p && isIndexFile(f) {
			index = append(index, f)
		}
	}
	if len(stem) > 0 {
		return files(stem)
	}
	return files(index)
}

// under reports f's path below directory dir ("." is the repository root).
func under(f, dir string) (string, bool) {
	if dir == "." {
		return f, true
	}
	rel, ok := strings.CutPrefix(f, dir+"/")
	return rel, ok
}

// files wraps paths as targets with no folders.
func files(ps []string) []target {
	if ps == nil {
		return nil
	}
	out := make([]target, len(ps))
	for i, p := range ps {
		out[i] = target{file: p}
	}
	return out
}

// resolveModule matches a dotted module's slash form as a path suffix.
func (c *composer) resolveModule(mod string) []string {
	var out []string
	for _, f := range c.paths {
		s := stemOf(f)
		if s == mod || strings.HasSuffix(s, "/"+mod) {
			out = append(out, f)
			continue
		}
		if stdpath.Base(f) == "__init__.py" {
			d := stdpath.Dir(f)
			if d == mod || strings.HasSuffix(d, "/"+mod) {
				out = append(out, f)
			}
		}
	}
	return out
}

// stemOf is p without its final extension.
func stemOf(p string) string { return strings.TrimSuffix(p, stdpath.Ext(p)) }

// isIndexFile reports a directory's index module: index.<ext> or
// __init__.py.
func isIndexFile(p string) bool {
	b := stdpath.Base(p)
	return b == "__init__.py" || stemOf(b) == "index"
}
