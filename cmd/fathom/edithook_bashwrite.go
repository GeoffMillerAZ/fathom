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
	"strconv"
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
//
// pinText (PR-3, edithook_pin.go) is the commit pin line's text for the
// same call: it rides this payload's envelope when there is one, and goes
// out alone when this path is silent.
func cmdEditHookBashWrite(codec harness.Codec, hook harness.HookInput, url string, armOff bool, stdout io.Writer, pinText string) int {
	pinCarried := false
	defer func() {
		if !pinCarried {
			emitPinContext(codec, pinText, stdout)
		}
	}()
	deadline := time.Now().Add(bashWriteTimeout) // one shared budget for every RPC this call makes, not a fresh one each
	sessionID := hook.SessionID
	paths, writeShaped := bashWriteTargets(hook.Command)
	if !writeShaped {
		return 0 // a read-only or unrecognized command: no beat, no output — matches edit-hook's silence on a clean/irrelevant call
	}
	emitBeat("edit-hook", "seen", firingMetaArm(map[string]string{"phase": "post-bash"}, sessionID, "", armOff))
	projDir := resolveProjDirFor(codec.ID(), hook.Cwd)
	// GL-25 (G3): a relative target is relative to the directory the
	// command ran in, never to the project root — `sed -i … NOTES.md` run
	// from workspace/notes/ wrote workspace/notes/NOTES.md, and resolving
	// it at the root served the root's rules for a file nobody wrote.
	cwd := hook.Cwd
	if cwd == "" {
		cwd = projDir
	}
	targets := resolveBashWriteTargets(cwd, paths)
	// XM-36: every answer that names a target records where it went.
	answered := func(meta map[string]string) map[string]string {
		if len(targets) > 0 {
			meta = withDestination(meta, projDir, targets[0])
			if len(targets) > 1 {
				meta["dest_targets"] = strconv.Itoa(len(targets))
			}
		}
		return meta
	}
	silent := func(reason string) int {
		emitBeat("edit-hook", "answered", firingMetaArm(answered(silentAnswerMeta(map[string]string{"phase": "post-bash"}, reason)), sessionID, "", armOff))
		return 0
	}
	if armOff {
		return silent(silenceArmOff)
	}

	if capabilityOffForFile(projDir, "", "obligations") {
		return silent(silenceCapabilityOff)
	}
	// GL-25: the one gitignore skip an Edit takes (editTargetIgnored), before
	// any daemon request — for the named targets, and for the doubt
	// branch's own directory.
	if len(targets) > 0 {
		kept, ignored := dropIgnoredWriteTargets(projDir, targets)
		if len(kept) == 0 && ignored > 0 {
			return silent(silenceGitignored)
		}
		targets = kept
	} else if len(paths) == 0 && editTargetIgnored(projDir, filepath.Clean(cwd)) {
		return silent(silenceGitignored)
	}
	now := time.Now()
	if len(paths) == 0 && bashDoubtServed(sessionID, hook.Cwd, now) {
		return silent(silenceDoubtAlreadyServed) // once per session per directory, and no daemon call to learn it
	}

	hc := &http.Client{Timeout: time.Until(deadline)}
	cands := candidatesWithDaemon(hc, url, projDir)
	if len(cands) == 0 {
		return silent(silenceNoService)
	}

	var files []string
	doubt := false
	svc := ""
	if len(paths) > 0 {
		var cand serviceCandidate
		files, cand = bashWriteRepoFiles(cands, targets)
		if len(files) == 0 {
			return silent(silenceForeignTarget) // resolved outside every candidate root: not doubt
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
		if m.resolved == 0 && m.total > 0 {
			return silent(silenceDaemonError) // every fetch failed or ran out of time; each failure has its error beat
		}
		return silent(silenceNoRules)
	}

	var text string
	if doubt {
		text = bashDoubtLine(filepath.ToSlash(filepath.Dir(files[0])), len(m.rules)+m.displaced)
	} else {
		text = bashWriteRulesPayload(m)
	}

	if pinText != "" {
		text += "\n" + pinText
	}
	out, err := codec.EncodeOutput(harness.EventPostTool, harness.HookOutput{Context: text})
	if err != nil {
		return silent(silenceEncodeFailed)
	}
	pinCarried = true
	costMeta := map[string]string{}
	answeredMeta := map[string]string{"emitted": "true", "phase": "post-bash"}
	if doubt {
		costMeta["doubt"] = "true"
		answeredMeta["doubt"] = "true"
		markBashDoubtServed(sessionID, hook.Cwd, now)
	} else {
		// GL-24: the rules this write was served are rule-outcome rows, as an
		// Edit's are (trackRuleOutcome, keyed per written file by the first
		// served claim). A doubt line serves no rule body, so it records none.
		for _, f := range m.namedFiles {
			trackRuleOutcome(sessionID, svc, f, m.topClaims[f])
		}
		answeredMeta["claims"] = strconv.Itoa(claimBearing(m.rules)) // rule_outcome_writer reads it
	}
	if m.displaced > 0 {
		answeredMeta["displaced"] = fmt.Sprint(m.displaced)
	}
	emitPayloadCost("post-bash", text, len(m.rules), "post-bash", sessionID, costMeta)
	emitBeat("edit-hook", "answered", firingMetaArm(answered(answeredMeta), sessionID, "", armOff))
	fmt.Fprintln(stdout, string(out))
	return 0
}

// resolveBashWriteTargets makes every confident target bashWriteTargets
// named absolute: a `~` target is expanded to the home directory, and a
// relative one is joined onto cwd, the directory the command ran in
// (bashwrite.go already threaded the command's own cd's into it). A `~user`
// form is dropped, never guessed at.
func resolveBashWriteTargets(cwd string, paths []string) []string {
	var out []string
	for _, p := range paths {
		p = expandBashTilde(p)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			if cwd == "" {
				continue
			}
			p = filepath.Join(cwd, filepath.FromSlash(p))
		}
		out = append(out, filepath.Clean(p))
	}
	return out
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

// bashWriteRepoFiles relativizes the absolute targets resolveBashWriteTargets
// made to repo-relative files under one candidate root, and returns that
// candidate (its service name is what obligations is asked). files is empty
// when every target sits outside all of them.
func bashWriteRepoFiles(cands []serviceCandidate, targets []string) ([]string, serviceCandidate) {
	chosen := cands[0]
	chosenSet := false
	var files []string
	for _, p := range targets {
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
