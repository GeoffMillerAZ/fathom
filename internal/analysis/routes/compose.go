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
//   - Handler, File and Line are unchanged.
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
// inside that directory; otherwise the file itself, else the files whose
// name is x plus an extension, else x's index.<ext> or __init__.py. A
// target naming no file is ignored. Output is sorted (sortRoutes) and
// independent of input order.
func Compose(files []Result) []Route {
	c := composer{local: map[string][]Route{}, mounts: map[string][]Mount{}, reached: map[string]bool{}}
	known := map[string]bool{}
	for _, f := range files {
		for _, r := range f.Routes {
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
				mounted[t] = true
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
			if onPath[t] {
				continue
			}
			c.walk(t, next, depth+1, onPath)
		}
	}
}

// resolve returns the known files m's Target names, sorted.
func (c *composer) resolve(m Mount) []string {
	t := m.Target
	switch {
	case strings.HasPrefix(t, "./") || strings.HasPrefix(t, "../"):
		return c.resolvePath(stdpath.Join(stdpath.Dir(m.File), t), strings.HasSuffix(t, "/"))
	case strings.HasPrefix(t, "/"):
		return c.resolvePath(stdpath.Clean(strings.TrimPrefix(t, "/")), strings.HasSuffix(t, "/"))
	case t == "" || strings.ContainsAny(t, "/\\") || strings.HasPrefix(t, "."):
		return nil
	default:
		return c.resolveModule(strings.ReplaceAll(t, ".", "/"))
	}
}

// resolvePath resolves a cleaned repo-relative path: a directory's direct
// files, else the file, else x.<ext>, else x's index.<ext> or __init__.py.
func (c *composer) resolvePath(p string, dir bool) []string {
	if p == ".." || strings.HasPrefix(p, "../") {
		return nil
	}
	if dir {
		var out []string
		for _, f := range c.paths {
			if stdpath.Dir(f) == p {
				out = append(out, f)
			}
		}
		return out
	}
	for _, f := range c.paths {
		if f == p {
			return []string{f}
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
		return stem
	}
	return index
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
