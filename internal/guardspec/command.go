package guardspec

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// MatchCommandGuard returns the first declared guard matching any executable
// shell segment. Bodies and substitutions are inspected independently; quoted
// argument text never becomes a segment. Repository and user guards always retain
// their whole-string glob matches; segments only add coverage. Safety predicates
// inspect literal words independently and warn on unknown targets. No shell runs.
//
// Like the command-scopes glob convention, * spans spaces and slashes and ?
// matches one character. Leading environment assignments may be matched too.
// Peeled wrappers do not hide the executable from command scopes.
func MatchCommandGuard(gs GuardSet, command string) (Guard, bool) {
	parsed := shellcmd.ParseCommand(command)
	var units []*shellcmd.Unit
	unreadable := false
	var walk func(*shellcmd.Command, *shellcmd.Assign)
	walk = func(c *shellcmd.Command, exported *shellcmd.Assign) {
		if c == nil {
			return
		}
		if c.Unreadable {
			unreadable = true
			return
		}
		for pi, pipeline := range c.Pipelines {
			for _, u := range pipeline.Units {
				stateChange, changed := exportedLefthook(&u)
				// An export applies only within this shell, never back out of
				// a body, substitution or pipeline. Prefix assignments win.
				if exported != nil {
					u.Env = append([]shellcmd.Assign{*exported}, u.Env...)
				}
				if u.Kind == shellcmd.KindSimple {
					units = append(units, &u)
				}
				childEnv := exported
				for _, a := range u.Env {
					if a.Name == "LEFTHOOK" {
						childEnv = &a
					}
				}
				walk(u.Body, childEnv)
				for _, sub := range u.Subst {
					walk(sub, exported)
				}
				if len(pipeline.Units) == 1 && pipeline.Sep != "&" {
					conditional := pi > 0 && (c.Pipelines[pi-1].Sep == "&&" || c.Pipelines[pi-1].Sep == "||")
					if changed {
						exported = stateChange
						if conditional {
							exported = nil // whether this assignment ran is unknown
						}
					}
					if u.Kind == shellcmd.KindGroup || u.Kind == shellcmd.KindCompound {
						exported = nil // same-shell control flow may change it
					}
				}
			}
		}
	}
	walk(&parsed, nil)
	var warning *Guard
	for _, g := range gs.Guards {
		for _, u := range units {
			if g.commandCheck != "" {
				if matched, ok := matchSafetyGuard(g, u); ok {
					if matched.Enforce {
						return matched, true
					}
					if warning == nil {
						warning = &matched
					}
				}
			} else if _, ok := MatchCommandScope(g.CommandScopes, u); ok {
				return g, true
			}
		}
		if g.commandCheck == "" && commandGlobMatches(g.CommandScopes, command) {
			return g, true
		}
		if unreadable && g.commandCheck != "" && safetyGitText.MatchString(command) && safetyActionText.MatchString(command) && warning == nil {
			g.Enforce = false
			g.Note = "git-safety: command safety is unknown (unreadable shell input); run git with hooks enabled and name the target branch."
			warning = &g
		}
	}
	if warning != nil {
		return *warning, true
	}
	return Guard{}, false
}

func literalCommand(u *shellcmd.Unit) bool {
	if len(u.Words) == 0 {
		return false
	}
	for _, w := range u.Words {
		if !w.Literal() {
			return false
		}
	}
	for _, a := range u.Env {
		if !a.Value.Literal() {
			return false
		}
	}
	return true
}

// MatchCommandScope returns the first pattern matching a literal executable
// segment, including leading assignments. Edit and readout hooks share this
// rendering so quoted argument text cannot impersonate separate argv words.
func MatchCommandScope(patterns []string, u *shellcmd.Unit) (string, bool) {
	if !literalCommand(u) {
		return "", false
	}
	words := make([]string, 0, len(u.Words))
	for _, w := range u.Words {
		words = append(words, commandScopeWord(w.Text))
	}
	argv := strings.Join(words, " ")
	prefixed := argv
	for i := len(u.Env) - 1; i >= 0; i-- {
		a := u.Env[i]
		op := "="
		if a.Append {
			op = "+="
		}
		prefixed = a.Name + op + commandScopeWord(a.Value.Text) + " " + prefixed
	}
	for _, p := range patterns {
		if commandGlobMatches([]string{p}, argv) || commandGlobMatches([]string{p}, prefixed) {
			return p, true
		}
	}
	return "", false
}

// commandGlobMatches retains the original whole-command glob convention.
func commandGlobMatches(patterns []string, command string) bool {
	for _, pattern := range patterns {
		var b strings.Builder
		b.WriteString("(?s)^")
		for _, r := range pattern {
			switch r {
			case '*':
				b.WriteString(".*")
			case '?':
				b.WriteByte('.')
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
		b.WriteByte('$')
		re := regexp.MustCompile(b.String())
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// Preserve word boundaries when rendering only for glob matching. This text
// is never executed: quotes prevent one argument from impersonating argv.
func commandScopeWord(s string) string {
	if s == "" || strings.ContainsAny(s, `"'\;|&()`) || strings.IndexFunc(s, unicode.IsSpace) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

var (
	safetyGitText    = regexp.MustCompile(`\bgit\b`)
	safetyActionText = regexp.MustCompile(`\b(push|commit)\b`)
)

// exportedLefthook recognizes only this command's explicit shell state. An
// unknown value or an unset clears known state; nothing reads the host's env.
func exportedLefthook(u *shellcmd.Unit) (*shellcmd.Assign, bool) {
	if len(u.Wrappers) != 0 {
		return nil, false
	}
	if len(u.Words) == 0 {
		for i := len(u.Env) - 1; i >= 0; i-- {
			if a := u.Env[i]; a.Name == "LEFTHOOK" {
				// A bare assignment can change an already exported value.
				// It does not prove export status on its own.
				return nil, true
			}
		}
		return nil, false
	}
	if !u.Words[0].Literal() {
		return nil, false
	}
	if u.Name() != "export" && u.Name() != "unset" {
		return nil, false
	}
	var out *shellcmd.Assign
	changed := false
	for _, word := range u.Words[1:] {
		if strings.HasPrefix(word.Text, "-") && word.Text != "--" {
			return nil, true // export/unset modes do not prove a disabled export
		}
		name, value, assigned := strings.Cut(word.Text, "=")
		if name != "LEFTHOOK" {
			continue
		}
		changed = true
		out = nil
		if u.Name() == "export" && assigned && word.Literal() {
			out = &shellcmd.Assign{Name: name, Value: shellcmd.Word{Text: value}}
		}
	}
	return out, changed
}
