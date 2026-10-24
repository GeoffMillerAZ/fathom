package main

// RO-25's line (docs/plans/readouts.md phase 2): a command guard's optional
// `requires` names the argv tokens a command matching its command_scopes
// must carry. In rich mode, when the command that ran matches and lacks
// one, the readout says so beside the output. Advisory only: the
// PreToolUse guard path never reads requires, so nothing is denied.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// readoutRichGuardKind names the guard line in the readout row's rich field.
const readoutRichGuardKind = "guard_requires"

// readoutGuardLineCut discloses a cut (law 3: never a silent truncation).
const readoutGuardLineCut = " …[cut at the guard line's byte cap]"

// readoutGuardLine is the guard line for the command that ran in dir, ""
// when the repository has no readable guard file or no guard's requires is
// missed. A malformed guard file fails open, as on every hook path.
func readoutGuardLine(dir string, c *shellcmd.Command) string {
	root := gitToplevelByStat(dir)
	if root == "" {
		return ""
	}
	gs, ok := loadGuardSet(root)
	if !ok {
		return ""
	}
	cite := filepath.ToSlash(projectdir.Guard.Path(root))
	if rel, ok := relativeToRoot(root, projectdir.Guard.Path(root)); ok {
		cite = filepath.ToSlash(rel)
	}
	return guardLineForCommand(gs, c, cite, readoutRichLineBudget)
}

// guardLineForCommand is the first guard (file order) with requires whose
// command_scopes match a simple command of c that lacks one of its tokens,
// rendered as one line citing cite:line, or "". The match and the tokens
// are read from the parsed argv of that one command, so a token elsewhere
// on the line never satisfies it; a command with an expansion in its argv
// could carry anything and is skipped.
func guardLineForCommand(gs guardspec.GuardSet, c *shellcmd.Command, cite string, budget int) string {
	if c == nil || c.Unreadable {
		return ""
	}
	for _, g := range gs.Guards {
		if len(g.Requires) == 0 {
			continue
		}
		for _, u := range c.Units() {
			if u.Kind != shellcmd.KindSimple || len(u.Words) == 0 {
				continue
			}
			pattern, ok := guardspec.MatchCommandScope(g.CommandScopes, u)
			if !ok {
				continue
			}
			missing, known := missingRequiredTokens(u, g.Requires)
			if !known || len(missing) == 0 {
				continue
			}
			line := fmt.Sprintf("this command lacks %s, which the guard at %s:%d (%q) requires", strings.Join(missing, ", "), cite, g.Line, pattern)
			if g.Note != "" {
				line += ": " + g.Note
			}
			if len(line) > budget {
				line = clipBytes(line, budget-len(readoutGuardLineCut)) + readoutGuardLineCut
			}
			return line
		}
	}
	return ""
}

// missingRequiredTokens is the tokens of requires u does not carry, in
// requires' order. A token is carried by a word equal to it, by a flag
// token's `=value` form, or by a field of a leading assignment's value
// (GOFLAGS=-race). known is false when a word is an expansion.
func missingRequiredTokens(u *shellcmd.Unit, requires []string) (missing []string, known bool) {
	carried := func(tok string) bool {
		for _, w := range u.Words {
			if w.Text == tok || (strings.HasPrefix(tok, "-") && strings.HasPrefix(w.Text, tok+"=")) {
				return true
			}
		}
		for _, a := range u.Env {
			for _, f := range strings.Fields(a.Value.Text) {
				if f == tok || (strings.HasPrefix(tok, "-") && strings.HasPrefix(f, tok+"=")) {
					return true
				}
			}
		}
		return false
	}
	for _, w := range u.Words {
		if w.Dynamic {
			return nil, false
		}
	}
	for _, tok := range requires {
		if !carried(tok) {
			missing = append(missing, tok)
		}
	}
	return missing, true
}
