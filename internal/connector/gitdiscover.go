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

// gitRootIgnored reports whether an enclosing repository's .gitignore
// excludes the corpus root as a whole. That repository's rules do not
// govern a corpus it ignores (a scratch clone inside a checkout, say):
// discovery walks it instead, and the refresh path agrees. false outside a
// work tree, at a repository's top level, or on any git failure.
func gitRootIgnored(ctx context.Context, root string) bool {
	out, err := gitro.Output(ctx, root, "rev-parse", "--show-prefix")
	if err != nil {
		return false
	}
	prefix := strings.TrimSpace(string(out))
	if prefix == "" {
		return false
	}
	top, err := gitro.Output(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	return gitIgnored(strings.TrimSpace(string(top)), prefix)
}

// ownWorkTree reports whether dir is the top level of its own git work
// tree (a nested repository or a submodule), not merely inside another.
func ownWorkTree(ctx context.Context, dir string) bool {
	out, err := gitro.Output(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top, err1 := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	self, err2 := filepath.EvalSymlinks(dir)
	return err1 == nil && err2 == nil && top == self
}

// discoverViaGit lists root's corpus through git. ok is false when root is
// not a git work tree, when an enclosing repository ignores root as a
// whole, or when git lists nothing (the caller walks instead — an empty
// discovery would make the ingest purge every stored row); an error from a
// work tree is returned. A nested repository or submodule, which git lists
// as one directory entry, is listed through its own git; one that cannot
// be is counted (GitUnlisted). Paths are repo-relative to root,
// slash-separated, sorted, regular files only.
func discoverViaGit(ctx context.Context, root string, ignore *filter.Matcher, include map[string]bool) (paths []string, ex Exclusions, ok bool, err error) {
	if !isGitWorkTree(ctx, root) || gitRootIgnored(ctx, root) {
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
	for i := 0; i < len(candidates); i++ {
		if i%256 == 0 {
			if cerr := ctx.Err(); cerr != nil {
				return nil, ex, true, cerr
			}
		}
		rel := strings.TrimSuffix(filepath.ToSlash(candidates[i]), "/")
		if rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		if dir, why := excludedDirOf(rel+"/_", ignore, include); why != "" {
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
		abs := filepath.Join(root, filepath.FromSlash(rel))
		fi, lerr := os.Lstat(abs)
		if lerr != nil {
			continue // deleted from the work tree
		}
		if fi.IsDir() {
			// A nested repository or a submodule: git lists it as one
			// entry. Its own git lists its files (its own .gitignore
			// applies); one it cannot list is counted, never dropped
			// silently.
			if !ownWorkTree(ctx, abs) {
				ex.GitUnlisted++
				continue
			}
			sub, serr := gitro.Output(ctx, abs, "ls-files", "-co", "--exclude-standard", "-z", "--", ".")
			if serr != nil {
				ex.GitUnlisted++
				continue
			}
			for _, p := range splitNUL(sub) {
				candidates = append(candidates, rel+"/"+filepath.ToSlash(p))
			}
			if ign, ierr := gitro.Output(ctx, abs, "ls-files", "-o", "-i", "--exclude-standard", "--directory", "-z", "--", "."); ierr == nil {
				ex.GitIgnored += len(splitNUL(ign))
			}
			continue
		}
		if !fi.Mode().IsRegular() {
			continue // a symlink
		}
		paths = append(paths, rel)
	}
	if len(paths) == 0 {
		// Nothing listed: walk rather than hand the ingest an empty
		// corpus it would take as every stored file deleted.
		return nil, Exclusions{}, false, nil
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

// refreshRootIgnored is gitRootIgnored for the one-file refresh path,
// bounded like its check-ignore.
func refreshRootIgnored(root string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), gitCheckIgnoreTimeout)
	defer cancel()
	return gitRootIgnored(ctx, root)
}
