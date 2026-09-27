package connector

// gitdiscover.go — RT-68 (docs/design/responsible-tenant.md §4.2): in a git
// work tree, discovery can list the corpus through git — tracked files
// plus untracked files .gitignore does not exclude (`git ls-files -co
// --exclude-standard -z`, via internal/gitro: a read that never writes the
// repository) — instead of walking every directory, so a tree's build
// output, caches and scratch never enter the corpus because they happen
// not to be on the default list. ls-files lists COMMITTED files too, so
// skipDir (vendor, testdata, …), the default cache list and the scoped
// ignore files still apply to every path it returns: a committed vendor/
// or testdata/ stays out. `[connector] include` re-includes a directory
// .gitignore excludes. Every exclusion is counted (I25, I38).
//
// Off by default: a corpus opts in with `[connector] respect_gitignore =
// true` until RT-M11 has measured, for every registered service, that no
// service loses more than 1% of its observations outside gitignored
// directories.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// isGitWorkTree reports whether root is inside a git work tree.
func isGitWorkTree(ctx context.Context, root string) bool {
	out, err := gitro.Output(ctx, root, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// splitNUL splits git's -z output into its entries.
func splitNUL(out []byte) []string {
	var paths []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) > 0 {
			paths = append(paths, string(p))
		}
	}
	return paths
}

// discoverViaGit lists root's corpus through git. ok is false when root is
// not a git work tree (the caller walks instead); an error from a work
// tree is returned. Paths are repo-relative to root, slash-separated,
// sorted, regular files only.
func discoverViaGit(ctx context.Context, root string, ignore *filter.Matcher, include map[string]bool) (paths []string, ex Exclusions, ok bool, err error) {
	if !isGitWorkTree(ctx, root) {
		return nil, ex, false, nil
	}
	ex.Git = true
	out, err := gitro.Output(ctx, root, "ls-files", "-co", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return nil, ex, true, err
	}
	candidates := splitNUL(out)

	// `[connector] include` re-includes directories .gitignore excludes.
	if len(include) > 0 {
		var specs []string
		for e := range include {
			if strings.Contains(e, "/") {
				specs = append(specs, e+"/**")
			} else {
				specs = append(specs, "**/"+e+"/**")
			}
		}
		sort.Strings(specs)
		args := []string{"ls-files", "-o", "-i", "--exclude-standard", "-z", "--"}
		for _, s := range specs {
			args = append(args, ":(glob)"+s)
		}
		if extra, xerr := gitro.Output(ctx, root, args...); xerr == nil {
			candidates = append(candidates, splitNUL(extra)...)
		}
	}
	// Count what .gitignore excluded (untracked, directories collapsed).
	if ign, ierr := gitro.Output(ctx, root, "ls-files", "-o", "-i", "--exclude-standard", "--directory", "-z", "--", "."); ierr == nil {
		ex.GitIgnored = len(splitNUL(ign))
	}

	defaultSeen := map[string]bool{}
	skippedSeen := map[string]bool{}
	ignoredSeen := map[string]bool{}
	seen := make(map[string]bool, len(candidates))
	for i, rel := range candidates {
		if i%256 == 0 {
			if cerr := ctx.Err(); cerr != nil {
				return nil, ex, true, cerr
			}
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		if dir, why := excludedDirOf(rel, ignore, include); why != "" {
			switch why {
			case "skip":
				skippedSeen[dir] = true
			case "default":
				defaultSeen[dir] = true
			default:
				ignoredSeen[dir] = true
			}
			continue
		}
		if ignore != nil && ignore.Match(rel) {
			continue
		}
		fi, lerr := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if lerr != nil || !fi.Mode().IsRegular() {
			continue // deleted from the work tree, a symlink, or a submodule
		}
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	ex.SkippedDirs, ex.IgnoredDirs = len(skippedSeen), len(ignoredSeen)
	for d := range defaultSeen {
		ex.DefaultIgnored = append(ex.DefaultIgnored, d)
	}
	sort.Strings(ex.DefaultIgnored)
	return paths, ex, true, nil
}

// excludedDirOf names the first ancestor directory of rel the walk would
// have pruned, and why: "skip" (skipDir), "default" (the default cache
// list, not re-included) or "ignore" (a scoped ignore file).
func excludedDirOf(rel string, ignore *filter.Matcher, include map[string]bool) (string, string) {
	segs := strings.Split(rel, "/")
	for i := 0; i < len(segs)-1; i++ {
		name := segs[i]
		prefix := strings.Join(segs[:i+1], "/")
		switch {
		case skipDir(name):
			return prefix, "skip"
		case defaultIgnoredDir(name, prefix, include):
			return prefix, "default"
		case ignore != nil && ignore.Match(prefix+"/"):
			return prefix, "ignore"
		}
	}
	return "", ""
}

// gitIgnored reports whether git ignores rel in the work tree at root
// (false outside a work tree or on any git failure: never excluded by a
// guess).
func gitIgnored(root, rel string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), gitCheckIgnoreTimeout)
	defer cancel()
	cmd, cleanup, err := gitro.Command(ctx, root, "check-ignore", "-q", "--", rel)
	defer cleanup()
	if err != nil {
		return false
	}
	return cmd.Run() == nil // exit 0: ignored; 1: not ignored; 128: error
}

// gitCheckIgnoreTimeout bounds the one-file check the refresh path runs.
const gitCheckIgnoreTimeout = 5 * time.Second
