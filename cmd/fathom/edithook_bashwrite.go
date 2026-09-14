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
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// bashWriteTimeout mirrors edit-hook's own post-edit budget.
const bashWriteTimeout = 2 * time.Second

// cmdEditHookBashWrite is cmdEditHook's Bash branch: hook.FilePath is
// empty, hook.Tool is shell, hook.Command is non-empty.
func cmdEditHookBashWrite(codec harness.Codec, hook harness.HookInput, url string, armOff bool, stdout io.Writer) int {
	sessionID := hook.SessionID
	paths, writeShaped := bashWriteTargets(hook.Command)
	if !writeShaped {
		return 0 // a read-only or unrecognized command: no beat, no output — matches edit-hook's silence on a clean/irrelevant call
	}
	emitBeat("edit-hook", "seen", firingMetaArm(map[string]string{"phase": "post-bash"}, sessionID, "", armOff))
	if armOff {
		emitBeat("edit-hook", "answered", firingMetaArm(map[string]string{"emitted": "false", "phase": "post-bash"}, sessionID, "", armOff))
		return 0
	}

	projDir := resolveProjDirFor(codec.ID(), hook.Cwd)
	if capabilityOffForFile(projDir, "", "obligations") {
		emitBeat("edit-hook", "answered", firingMetaArm(map[string]string{"emitted": "false", "phase": "post-bash"}, sessionID, "", armOff))
		return 0
	}

	hc := &http.Client{Timeout: bashWriteTimeout}
	cands := candidatesWithDaemon(hc, url, projDir)
	if len(cands) == 0 {
		return 0
	}
	svc := strings.ToLower(cands[0].name)

	var files []string
	doubt := false
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			files = append(files, p) // a prompt/command-relative path is already repo-relative text
			continue
		}
		if rel, ok := relativeToRoot(cands[0].root, p); ok {
			files = append(files, rel)
		}
	}
	if len(files) == 0 {
		// Doubt (O57 item 4/5b): git apply, or every extracted target fell
		// outside the repo / was absolute-and-unresolvable. Serve the
		// cwd's nearest guide instead of staying silent — a synthetic
		// sentinel filename inside cwd's own directory lets it match the
		// SAME directory-anchored scope globs (<dir>/**) a real file there
		// would, with no new matching logic in semantic.ObligationsFor.
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

	m := mergeObligationsAcrossFiles(url, svc, files, bashWriteTimeout, func(f string) {
		emitBeat("edit-hook", "error", firingMetaArm(map[string]string{"phase": "post-bash", "target": f}, sessionID, "", armOff))
	})
	if len(m.rules) == 0 {
		emitBeat("edit-hook", "answered", firingMetaArm(map[string]string{"emitted": "false", "phase": "post-bash"}, sessionID, "", armOff))
		return 0
	}

	var sb strings.Builder
	label := promptNamesLabel(m.namedFiles)
	if doubt {
		fmt.Fprintf(&sb, "⚖️ fathom obligations — this Bash command wrote something the parser could not name exactly; nearest guide (%s):", label)
	} else {
		fmt.Fprintf(&sb, "⚖️ fathom obligations — this Bash command wrote %s:", label)
	}
	for _, r := range m.rules {
		writeRuleBullet(&sb, r)
	}
	if m.displaced > 0 {
		fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", m.displaced)
	}
	if m.stale {
		sb.WriteString("\n⚠️ index is behind your working tree for one of these paths")
	}

	out, err := codec.EncodeOutput(harness.EventPostTool, harness.HookOutput{Context: sb.String()})
	if err != nil {
		return 0
	}
	answeredMeta := map[string]string{"emitted": "true", "phase": "post-bash"}
	if doubt {
		answeredMeta["doubt"] = "true"
	}
	if m.displaced > 0 {
		answeredMeta["displaced"] = fmt.Sprint(m.displaced)
	}
	emitBeat("edit-hook", "answered", firingMetaArm(answeredMeta, sessionID, "", armOff))
	fmt.Fprintln(stdout, string(out))
	return 0
}
