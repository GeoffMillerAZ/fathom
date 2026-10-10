package main

// edithook_ignored.go — EW-55: an edit to a gitignored file (workspace/,
// build output, generated code) is not governed by anything the index
// holds, so the hook stays silent before any daemon traffic instead of
// asking the daemon to prove it. R-b (owner ruling, 2026-10-10): that skip is
// for GENERIC obligation injection only. On --pre a guard whose scopes name
// the file still prints (renderPreEdit matches the guard first), without the
// per-file refresh an unindexed file would otherwise trigger.
//
// Mechanism (measured, BenchmarkGitIgnoreDecision): `git check-ignore -q`
// costs a process spawn on EVERY edit, well past the 5 ms p50 budget. The
// decision is therefore made in process — the .gitignore files from the
// repo root down to the file's directory plus .git/info/exclude, evaluated
// root-to-leaf with git's own rules (last match wins, deeper files
// override, an ignored directory ignores everything under it) — and
// `git check-ignore` is spawned only to CONFIRM a positive, which makes
// tracked-but-ignored files and the user's global excludes exact at a cost
// paid only on edits that are about to be skipped. Nothing is cached:
// every call recomputes (TABLE FREEZE). Any error fails open (not ignored).

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// skipIgnoredEditTarget reports whether the edit of filePath should be
// silent because git ignores it, after emitting the beat that names why.
// phase is "post", "pre" or "post-bash".
func skipIgnoredEditTarget(projDir, filePath, sessionID, phase string) bool {
	root, rel, ok := gitRootAndRel(projDir, filePath)
	if !ok || !gitIgnoredFile(root, rel) {
		return false
	}
	emitBeat("edit-hook", "answered", firingMeta(map[string]string{"emitted": "false", "phase": phase, "reason": "gitignored"}, sessionID, filePath))
	return true
}

// gitRootAndRel finds the work-tree root holding filePath (the nearest
// ancestor directory with a .git entry, stat only) and the slash-separated
// path of filePath under it. ok=false when filePath sits under no
// candidate root or no repository.
func gitRootAndRel(projDir, filePath string) (string, string, bool) {
	if filePath == "" || !filepath.IsAbs(filePath) {
		return "", "", false
	}
	inside := false
	for _, cand := range serviceCandidates(projDir) {
		if _, ok := relativeToRoot(cand.root, filePath); ok {
			inside = true
			break
		}
	}
	if !inside {
		return "", "", false
	}
	for dir := filepath.Dir(filepath.Clean(filePath)); ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			rel, err := filepath.Rel(dir, filePath)
			if err != nil || escapesRoot(rel) {
				return "", "", false
			}
			return dir, filepath.ToSlash(rel), true
		}
		if parent := filepath.Dir(dir); parent == dir {
			return "", "", false
		}
	}
}

// gitIgnoredFile is the in-process verdict, confirmed by git.
func gitIgnoredFile(root, rel string) bool {
	if !gitignoredInProcess(root, rel) {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// exit 0 = ignored; 1 = not ignored (a tracked file is never reported
	// ignored); anything else (no git, 128) fails open.
	return exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "-q", "--", rel).Run() == nil
}

type ignoreRule struct {
	base    string   // directory (slash path, "" = root) the pattern is relative to
	segs    []string // pattern split on "/"
	anchor  bool     // matched against the path under base, not the basename
	negate  bool
	dirOnly bool
}

// gitignoredInProcess reports whether rel (slash path under root) is
// ignored by the repo's .gitignore files and .git/info/exclude.
func gitignoredInProcess(root, rel string) bool {
	parts := strings.Split(rel, "/")
	var rules []ignoreRule
	rules = append(rules, readIgnoreFile(filepath.Join(root, ".git", "info", "exclude"), "")...)
	rules = append(rules, readIgnoreFile(filepath.Join(root, ".gitignore"), "")...)
	// Walk the path root-to-leaf; each directory contributes its own
	// .gitignore before its children are judged.
	for i := range parts {
		cur := strings.Join(parts[:i+1], "/")
		isDir := i < len(parts)-1
		if !isDir {
			if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(cur))); err == nil && st.IsDir() {
				isDir = true
			}
		}
		if ignoredBy(rules, cur, isDir) {
			return true // an ignored directory ignores everything under it
		}
		if i < len(parts)-1 {
			rules = append(rules, readIgnoreFile(filepath.Join(root, filepath.FromSlash(cur), ".gitignore"), cur)...)
		}
	}
	return false
}

func ignoredBy(rules []ignoreRule, p string, isDir bool) bool {
	ignored := false
	for _, r := range rules { // precedence order: later wins
		if r.dirOnly && !isDir {
			continue
		}
		sub := p
		if r.base != "" {
			rest, ok := strings.CutPrefix(p, r.base+"/")
			if !ok {
				continue
			}
			sub = rest
		}
		var hit bool
		if r.anchor {
			hit = matchSegs(r.segs, strings.Split(sub, "/"))
		} else {
			hit = matchSegs(r.segs, []string{path.Base(sub)})
		}
		if hit {
			ignored = !r.negate
		}
	}
	return ignored
}

func readIgnoreFile(file, base string) []ignoreRule {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []ignoreRule
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		for strings.HasSuffix(line, " ") && !strings.HasSuffix(line, "\\ ") {
			line = line[:len(line)-1]
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{base: base}
		if rest, ok := strings.CutPrefix(line, "!"); ok {
			r.negate, line = true, rest
		}
		if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
			line = line[1:]
		}
		if rest, ok := strings.CutSuffix(line, "/"); ok {
			r.dirOnly, line = true, rest
		}
		if line == "" {
			continue
		}
		r.anchor = strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		r.segs = strings.Split(line, "/")
		out = append(out, r)
	}
	return out
}

// matchSegs matches slash-split pattern segments against path segments;
// a "**" segment spans zero or more whole segments. A pattern that ends
// before the path does matches the path's leading segments only through
// the directory walk in gitignoredInProcess, so the match is whole-path.
func matchSegs(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegs(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
		return false
	}
	return matchSegs(pat[1:], segs[1:])
}

// dropIgnoredWriteTargets removes the repo-relative Bash write targets git
// ignores (EW-55); an empty result reads as "foreign-target" silence.
func dropIgnoredWriteTargets(root string, files []string) []string {
	kept := files[:0:0]
	for _, f := range files {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, filepath.FromSlash(f))
		}
		if r, rel, ok := gitRootAndRel(root, abs); ok && gitIgnoredFile(r, rel) {
			continue
		}
		kept = append(kept, f)
	}
	return kept
}
