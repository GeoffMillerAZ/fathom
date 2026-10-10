package main

// edithook_dest.go — what every edit-hook answer records about itself.
//
// GL-24 (docs/plans/governance-learning.md, INC-27): 21,247 hook answers from
// 2026-10-03 to 2026-10-10 said nothing and carried no reason, so the 2.3%
// emission share could not be called a leak or a correct silence. Every
// answered beat with emitted=false now carries a reason from one closed
// vocabulary (silenceReason), the one the Bash-write path already used plus
// the reasons the other paths had left unnamed.
//
// XM-36 (docs/plans/exploration-maps.md Wave 2d): every file-target answer
// records where the write went (destinationClass) and the target path, the
// post-bash writes included, so XM-37's fold and XM-M8's reading count the
// clone and worktree traffic the hook answers silently today. Recording
// only: nothing here changes what the agent is told.

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Silence reasons: the closed vocabulary of an edit-hook answer that said
// nothing. knownSilenceReason is the vocabulary; every emitted=false answer
// names exactly one of these.
const (
	silenceArmOff             = "arm-off"              // the kill switch or an OFF measurement arm
	silenceCapabilityOff      = "capability-off"       // `capabilities set obligations|guards off`
	silenceGitignored         = "gitignored"           // git ignores the target (EW-55, GL-25)
	silenceForeignTarget      = "foreign-target"       // the target sits under no candidate root of the project
	silenceDoubtAlreadyServed = "doubt-already-served" // the unnamed-write line was already shown for this directory
	silenceNoRules            = "no-rules"             // resolved, and nothing governs it or cites a spec
	silenceNoGuard            = "no-guard"             // --pre: no guard names the file or command
	silenceGuardSilent        = "guard-silent"         // --pre: a guard matched and had nothing to say
	silenceUnresolved         = "unresolved"           // --pre: the daemon could not resolve the guarded file's service
	silenceNoService          = "no-service"           // no candidate service at all for the project directory
	silenceDaemonError        = "daemon-error"         // every daemon call failed; an error beat names why
	silenceEncodeFailed       = "encode-failed"        // the harness codec could not encode the answer
)

// knownSilenceReason reports whether r is in the closed vocabulary. A switch,
// not a package-level map (tenancy lint).
func knownSilenceReason(r string) bool {
	switch r {
	case silenceArmOff, silenceCapabilityOff, silenceGitignored, silenceForeignTarget,
		silenceDoubtAlreadyServed, silenceNoRules, silenceNoGuard, silenceGuardSilent,
		silenceUnresolved, silenceNoService, silenceDaemonError, silenceEncodeFailed:
		return true
	}
	return false
}

// silentAnswerMeta is the meta of an answered beat that said nothing: it
// sets emitted=false and the reason. meta may be nil.
func silentAnswerMeta(meta map[string]string, reason string) map[string]string {
	if meta == nil {
		meta = map[string]string{}
	}
	meta["emitted"] = "false"
	meta["reason"] = reason
	return meta
}

// Destination classes (XM-36): where a file-target answer's target sits.
const (
	destIndexed    = "indexed"    // under a candidate root of the project (the repository the hook serves)
	destClone      = "clone"      // under another clone of that repository
	destWorktree   = "worktree"   // under a git worktree of it
	destGitignored = "gitignored" // inside it, and ignored by git
	destScratch    = "scratch"    // a temp directory or a session scratchpad
	destOutside    = "outside"    // anywhere else
)

// withDestination stamps a file-target answer's meta with the target's
// destination class and its absolute path. An empty target leaves meta as it
// is: an answer that names no file has no destination.
func withDestination(meta map[string]string, projDir, target string) map[string]string {
	if target == "" {
		return meta
	}
	if meta == nil {
		meta = map[string]string{}
	}
	meta["dest_class"] = destinationClass(projDir, target)
	meta["dest_path"] = target
	return meta
}

// destinationClass classifies an absolute target path against the project's
// candidate roots (serviceCandidates: the project directory and its git
// toplevel). File reads and stats only, never a git process, except the one
// `git check-ignore` that confirms an in-process ignore verdict
// (gitIgnoredFile). In order:
//
//   - worktree: the nearest work tree holding the target is a linked worktree
//     (a `.git` file) whose common git directory is a candidate root's;
//   - clone: the nearest work tree is a separate checkout whose `origin`
//     names the same repository as a candidate root's `origin`, or the
//     candidate root itself;
//   - gitignored / indexed: the target sits under a candidate root and git
//     ignores it, or does not;
//   - scratch: the target sits under a temp directory or a scratchpad;
//   - outside: anything else.
func destinationClass(projDir, target string) string {
	if target == "" || !filepath.IsAbs(target) {
		return destOutside
	}
	var roots []string
	for _, c := range serviceCandidates(projDir) {
		roots = append(roots, c.root)
	}
	if wt, dotGit, ok := nearestWorkTree(target); ok {
		if !rootListed(roots, wt) {
			if fi, err := os.Lstat(dotGit); err == nil && !fi.IsDir() {
				if common := linkedCommonDir(wt, dotGit); common != "" && commonDirOfRoots(roots, common) {
					return destWorktree
				}
			} else if err == nil && sameRepository(roots, wt) {
				return destClone
			}
		}
	}
	for _, r := range roots {
		if _, ok := relativeToRoot(r, target); ok {
			if editTargetIgnored(projDir, target) {
				return destGitignored
			}
			return destIndexed
		}
	}
	if scratchPath(target) {
		return destScratch
	}
	return destOutside
}

// nearestWorkTree walks up from target's directory to the first directory
// holding a `.git` entry (a directory for a main work tree, a file for a
// linked worktree or a submodule).
func nearestWorkTree(target string) (root, dotGit string, ok bool) {
	for dir := filepath.Dir(filepath.Clean(target)); ; dir = filepath.Dir(dir) {
		p := filepath.Join(dir, ".git")
		if _, err := os.Lstat(p); err == nil {
			return dir, p, true
		}
		if parent := filepath.Dir(dir); parent == dir {
			return "", "", false
		}
	}
}

// rootListed reports whether dir is one of roots, symlinks resolved.
func rootListed(roots []string, dir string) bool {
	rd := resolveSymlinks(dir)
	for _, r := range roots {
		if r == dir || resolveSymlinks(r) == rd {
			return true
		}
	}
	return false
}

// linkedCommonDir reads a linked worktree's `.git` file ("gitdir: <path>")
// and returns the common git directory it shares with its main work tree
// (the gitdir's commondir file, else the parent of its worktrees/ entry), or
// "" when the file names no worktree.
func linkedCommonDir(wt, dotGit string) string {
	raw, err := os.ReadFile(dotGit)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	gd, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return ""
	}
	gd = strings.TrimSpace(gd)
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(wt, gd)
	}
	if c, err := os.ReadFile(filepath.Join(gd, "commondir")); err == nil {
		cd := strings.TrimSpace(string(c))
		if !filepath.IsAbs(cd) {
			cd = filepath.Join(gd, cd)
		}
		return filepath.Clean(cd)
	}
	if i := strings.LastIndex(filepath.ToSlash(gd), "/worktrees/"); i >= 0 {
		return filepath.Clean(gd[:i])
	}
	return ""
}

// commonDirOfRoots reports whether common is the .git directory of one of
// roots.
func commonDirOfRoots(roots []string, common string) bool {
	rc := resolveSymlinks(common)
	for _, r := range roots {
		g := filepath.Join(r, ".git")
		if g == common || resolveSymlinks(g) == rc {
			return true
		}
	}
	return false
}

// sameRepository reports whether the checkout at wt is a clone of a
// candidate root: its `origin` names the same repository as the root's
// `origin` (normalizeOriginURL), or names the root's own path.
func sameRepository(roots []string, wt string) bool {
	origin := originFromConfig(filepath.Join(wt, ".git", "config"))
	if origin == "" {
		return false
	}
	norm := normalizeOriginURL(origin)
	local := ""
	if filepath.IsAbs(origin) || strings.HasPrefix(origin, "file://") {
		local = resolveSymlinks(strings.TrimSuffix(strings.TrimPrefix(origin, "file://"), "/.git"))
	}
	for _, r := range roots {
		if local != "" && resolveSymlinks(r) == local {
			return true
		}
		if ro := originFromConfig(filepath.Join(r, ".git", "config")); ro != "" && normalizeOriginURL(ro) == norm {
			return true
		}
	}
	return false
}

// originFromConfig reads the url of [remote "origin"] from a git config
// file; "" when the file or the remote is absent.
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
		if !in {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "url" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// scratchPath reports whether p sits under the OS temp directory, /tmp, or a
// directory whose name says it is scratch (tmp, temp, scratch, scratchpad).
func scratchPath(p string) bool {
	rp := resolveSymlinks(p)
	for _, base := range []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders"} {
		if base == "" {
			continue
		}
		for _, b := range []string{base, resolveSymlinks(base)} {
			if _, ok := relativeToRoot(b, rp); ok {
				return true
			}
		}
	}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(rp)), "/") {
		switch strings.ToLower(seg) {
		case "tmp", "temp", "scratch", "scratchpad":
			return true
		}
	}
	return false
}
