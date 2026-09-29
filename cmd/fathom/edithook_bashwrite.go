package main

// edithook_bashwrite.go — O57 item 4/5(b): the PostToolUse Bash-write
// path. edit-hook's Edit|Write|MultiEdit matcher answers "what governs
// this file" for every editor-tool write; a Bash command that writes the
// same way (heredoc redirection, `sed -i`, `tee`, `cp`/`mv` into the
// tree, `git apply`) bypassed it entirely, so it never got obligations at
// all. cmdEditHookBashWrite parses tool_input.command (bashwrite.go) and
// serves the same payload shape edit-hook already renders — for the
// written path(s) when confident, or for the cwd's nearest guide when the
// command is write-shaped but the target cannot be extracted (git apply,
// or a target this parser does not recognize).

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// bashWriteTimeout mirrors edit-hook's own post-edit budget.
const bashWriteTimeout = 2 * time.Second

// cmdEditHookBashWrite is cmdEditHook's Bash branch: hook.FilePath is
// empty, hook.Tool is shell, hook.Command is non-empty.
//
// Three outcomes for a write-shaped command (EW-1, O122):
//   - a target resolved inside a candidate root: the rules governing it,
//     as ever;
//   - every target resolved and none inside a root (`printf … >>
//     ~/.claude/…/MEMORY.md`, `cat > /tmp/x.sh <<EOF`): silence — the write
//     is known and foreign, which is not doubt;
//   - no target could be named (`git apply`, a `$VAR` target, a cd the
//     parser could not follow): ONE line, the first time a session asks
//     from a directory, never the nearest guide's rule bodies.
//
// Every emission is a payload-cost beat (channel post-bash), so the
// instrument that prices post-edit and pre-edit payloads prices this one.
func cmdEditHookBashWrite(codec harness.Codec, hook harness.HookInput, url string, armOff bool, stdout io.Writer) int {
	deadline := time.Now().Add(bashWriteTimeout) // one shared budget for every RPC this call makes, not a fresh one each
	sessionID := hook.SessionID
	paths, writeShaped := bashWriteTargets(hook.Command)
	if !writeShaped {
		return 0 // a read-only or unrecognized command: no beat, no output — matches edit-hook's silence on a clean/irrelevant call
	}
	emitBeat("edit-hook", "seen", firingMetaArm(map[string]string{"phase": "post-bash"}, sessionID, "", armOff))
	silent := func(reason string) int {
		emitBeat("edit-hook", "answered", firingMetaArm(map[string]string{"emitted": "false", "phase": "post-bash", "reason": reason}, sessionID, "", armOff))
		return 0
	}
	if armOff {
		return silent("arm-off")
	}

	projDir := resolveProjDirFor(codec.ID(), hook.Cwd)
	if capabilityOffForFile(projDir, "", "obligations") {
		return silent("capability-off")
	}
	now := time.Now()
	if len(paths) == 0 && bashDoubtServed(sessionID, hook.Cwd, now) {
		return silent("doubt-already-served") // once per session per directory, and no daemon call to learn it
	}

	hc := &http.Client{Timeout: time.Until(deadline)}
	cands := candidatesWithDaemon(hc, url, projDir)
	if len(cands) == 0 {
		return 0
	}

	var files []string
	doubt := false
	svc := ""
	if len(paths) > 0 {
		var cand serviceCandidate
		files, cand = bashWriteRepoFiles(cands, paths)
		if len(files) == 0 {
			return silent("foreign-target") // resolved outside every candidate root: not doubt
		}
		svc = strings.ToLower(cand.name)
	} else {
		// Doubt (O57 item 4/5b): git apply, or no extractable target. Ask
		// for the cwd's nearest guide through a synthetic sentinel filename
		// inside cwd's own directory, which matches the SAME directory-
		// anchored scope globs (<dir>/**) a real file there would, with no
		// new matching logic in semantic.ObligationsFor. The answer only
		// decides whether anything governs the directory and how many
		// rules: its bodies are never served here.
		//
		// cands[0] is the NARROWEST identity guess — cwd relabeled as its
		// own root (serviceCandidates' "the Claude project IS the repo
		// root" case), which trivially relativizes to "" and never
		// differentiates a nested directory from the root. Prefer the
		// WIDEST candidate cwd actually resolves under (typically the git
		// toplevel, serviceCandidates' second guess, for a project rooted
		// in a subdirectory) so the sentinel lands where cwd really is.
		doubt = true
		relCwd, doubtRoot := "", cands[0]
		for _, c := range cands {
			if rel, ok := relativeToRoot(c.root, hook.Cwd); ok && rel != "" && rel != "." {
				relCwd, doubtRoot = rel, c
				break
			}
		}
		svc = strings.ToLower(doubtRoot.name)
		files = []string{filepath.ToSlash(filepath.Join(relCwd, "__bash_write__.go"))}
	}

	m := mergeObligationsAcrossFiles(url, svc, files, deadline, func(f string) {
		emitBeat("edit-hook", "error", firingMetaArm(map[string]string{"phase": "post-bash", "target": f}, sessionID, "", armOff))
	})
	if len(m.rules) == 0 {
		return silent("no-rules")
	}

	var text string
	if doubt {
		text = bashDoubtLine(filepath.ToSlash(filepath.Dir(files[0])), len(m.rules)+m.displaced)
	} else {
		text = bashWriteRulesPayload(m)
	}

	out, err := codec.EncodeOutput(harness.EventPostTool, harness.HookOutput{Context: text})
	if err != nil {
		return 0
	}
	costMeta := map[string]string{}
	answeredMeta := map[string]string{"emitted": "true", "phase": "post-bash"}
	if doubt {
		costMeta["doubt"] = "true"
		answeredMeta["doubt"] = "true"
		markBashDoubtServed(sessionID, hook.Cwd, now)
	}
	if m.displaced > 0 {
		answeredMeta["displaced"] = fmt.Sprint(m.displaced)
	}
	emitPayloadCost("post-bash", text, len(m.rules), "post-bash", sessionID, costMeta)
	emitBeat("edit-hook", "answered", firingMetaArm(answeredMeta, sessionID, "", armOff))
	fmt.Fprintln(stdout, string(out))
	return 0
}

// bashDoubtLine is the whole doubt payload: one line, no rule bodies. dir is
// the directory the sentinel landed in, repo-relative ("." at the root).
func bashDoubtLine(dir string, rules int) string {
	if dir == "" {
		dir = "."
	}
	return fmt.Sprintf("⚖️ fathom obligations — this Bash command wrote something the hook could not name; %d rule(s) govern %s (shown once per directory this session): call `obligations` with the file it wrote.", rules, dir)
}

// bashWriteRulesPayload renders the rules governing the file(s) a Bash
// command wrote: the same bullets edit-hook renders for an Edit.
func bashWriteRulesPayload(m mergedObligations) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "⚖️ fathom obligations — this Bash command wrote %s:", promptNamesLabel(m.namedFiles))
	for _, r := range m.rules {
		writeRuleBullet(&sb, r)
	}
	if m.displaced > 0 {
		fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", m.displaced)
	}
	if m.stale {
		sb.WriteString("\n⚠️ index is behind your working tree for one of these paths")
	}
	return sb.String()
}

// bashWriteRepoFiles resolves the confident targets bashWriteTargets named
// to repo-relative files under one candidate root, and returns that
// candidate (its service name is what obligations is asked). A relative
// target is already repo-relative text (bashwrite.go threads cd across the
// command), unless it climbs out with `..`; a `~` target is expanded to the
// home directory first; an absolute one must relativize under a candidate
// root. files is empty when every target resolved outside all of them.
func bashWriteRepoFiles(cands []serviceCandidate, paths []string) ([]string, serviceCandidate) {
	chosen := cands[0]
	chosenSet := false
	var files []string
	for _, p := range paths {
		p = expandBashTilde(p)
		if p == "" {
			continue // a `~user` form: never guessed at
		}
		if !filepath.IsAbs(p) {
			if p == ".." || strings.HasPrefix(p, "../") {
				continue // climbs out of the tree the command started in
			}
			files = append(files, p)
			continue
		}
		for _, c := range cands {
			if chosenSet && c.root != chosen.root {
				continue // one service per call: later targets must sit under the first one's root
			}
			if rel, ok := relativeToRoot(c.root, p); ok {
				chosen, chosenSet = c, true
				files = append(files, filepath.ToSlash(rel))
				break
			}
		}
	}
	return files, chosen
}

// expandBashTilde expands a leading `~` or `~/` to the user's home
// directory, as the shell would have before running the command. Any other
// tilde form (`~user/…`) returns "" (unresolvable here); a path without a
// leading tilde is returned unchanged.
func expandBashTilde(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
