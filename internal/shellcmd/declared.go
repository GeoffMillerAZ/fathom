package shellcmd

import (
	"fmt"
	"path"
	"slices"
)

// Wrapper declares either a literal argv prefix or a command whose
// ScriptArg (1-based after the command name) is a literal shell script.
// CwdArg optionally names the working-directory argument of that command.
type Wrapper struct {
	Prefix    string
	Command   string
	ScriptArg int
	CwdArg    int
}

// Unwrap applies declarations to an already lexed command. Like
// ParseCommand it is pure, never executes a command, and preserves refusal
// reasons. Built-in transparent prefixes are peeled by ParseCommand first.
// Declarations are tried in order; at most two script wrappers are opened
// along any nested path. Guards can use this without changing matching.
func Unwrap(c Command, wrappers []Wrapper) Command {
	if len(wrappers) == 0 || c.Unreadable {
		return c
	}
	c = cloneCommand(c)
	if err := unwrapAt(&c, wrappers, 0, ""); err != nil {
		return *unreadableCommand(err.Error())
	}
	return c
}

func unwrapAt(c *Command, wrappers []Wrapper, depth int, cwd string) error {
	if c.Unreadable {
		return fail(c.Reason)
	}
	for pi := range c.Pipelines {
		for ui := range c.Pipelines[pi].Units {
			u := &c.Pipelines[pi].Units[ui]
			u.Cwd = cwd
			childDepth := depth
			for u.Kind == KindSimple && len(u.Words) > 0 {
				matched := false
				for _, w := range wrappers {
					if w.Prefix != "" {
						prefix := ParseCommand(w.Prefix)
						if prefix.Unreadable || len(prefix.Pipelines) != 1 || len(prefix.Pipelines[0].Units) != 1 {
							return fail("invalid wrapper prefix")
						}
						pu := prefix.Pipelines[0].Units[0]
						if pu.Kind != KindSimple || len(pu.Words) == 0 || len(pu.Env)+len(pu.Wrappers)+len(pu.Redirects)+len(pu.Subst) > 0 || prefix.Pipelines[0].Sep != "" {
							return fail("invalid wrapper prefix")
						}
						if len(u.Words) <= len(pu.Words) {
							continue
						}
						match := true
						for i, word := range pu.Words {
							if !word.Literal() || !u.Words[i].Literal() || word.Text != u.Words[i].Text {
								match = false
								break
							}
						}
						if !match {
							continue
						}
						u.Wrappers = append(u.Wrappers, w.Prefix)
						u.Words = u.Words[len(pu.Words):]
						// A declared prefix can itself precede a built-in prefix.
						ts := make([]token, len(u.Words))
						for i, word := range u.Words {
							ts[i] = token{word: word, raw: word.Text}
						}
						peeled, env, rest, err := peel(ts)
						if err != nil {
							return err
						}
						u.Wrappers = append(u.Wrappers, peeled...)
						u.Env = append(u.Env, env...)
						u.Words = nil
						for _, t := range rest {
							u.Words = append(u.Words, t.word)
						}
						if len(u.Words) > 0 {
							if !u.Words[0].Literal() {
								return fail("command name is not literal")
							}
							if u.Name() == "eval" {
								return fail("eval runs text the lexer cannot read")
							}
							body, isShell, err := shellScript(u.Words, 0)
							if err != nil {
								return err
							}
							if isShell {
								u.Kind, u.Body = KindShell, body
							}
						}
					} else {
						if !u.Words[0].Literal() || (u.Name() != w.Command && u.Words[0].Text != w.Command) {
							continue
						}
						if depth >= 2 {
							return fail("wrapper script depth exceeds 2")
						}
						if w.ScriptArg < 1 || w.ScriptArg >= len(u.Words) {
							return fail("wrapper script argument is missing")
						}
						script := u.Words[w.ScriptArg]
						if !script.Literal() {
							return fail("wrapper script is not literal")
						}
						if w.CwdArg != 0 {
							if w.CwdArg < 1 || w.CwdArg >= len(u.Words) || !u.Words[w.CwdArg].Literal() || u.Words[w.CwdArg].Text == "" {
								return fail("wrapper cwd argument is missing or not literal")
							}
							u.Cwd = wrapperDir(cwd, u.Words[w.CwdArg].Text)
						}
						u.Kind, u.DeclaredScript, u.Script = KindShell, true, script.Text
						u.Body = parseAt(script.Text, 0)
						if u.Body.Unreadable {
							return fmt.Errorf("in wrapper %s: %s", w.Command, u.Body.Reason)
						}
						childDepth = depth + 1
					}
					matched = true
					break
				}
				if !matched {
					break
				}
			}
			if u.Body != nil {
				if err := unwrapAt(u.Body, wrappers, childDepth, u.Cwd); err != nil {
					return err
				}
			}
			for _, sub := range u.Subst {
				if err := unwrapAt(sub, wrappers, depth, cwd); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func wrapperDir(base, dir string) string {
	if path.IsAbs(dir) {
		return path.Clean(dir)
	}
	return path.Join(base, dir)
}

// Transparent exposes a declared script's pipelines when it occupies a
// whole, unredirected pipeline. A wrapper in a pipe or a negation remains
// a nested body: flattening it would change which output is classified.
func (c Command) Transparent() Command {
	c = cloneCommand(c)
	var ps []Pipeline
	for _, p := range c.Pipelines {
		if len(p.Units) == 1 && !p.Bang {
			u := p.Units[0]
			if u.DeclaredScript && u.Body != nil && len(u.Redirects) == 0 {
				body := u.Body.Transparent()
				if len(body.Pipelines) > 0 {
					last := &body.Pipelines[len(body.Pipelines)-1]
					if last.Sep != "&" {
						last.Sep = p.Sep
					}
					for i := range body.Pipelines {
						for j := range body.Pipelines[i].Units {
							v := &body.Pipelines[i].Units[j]
							v.Wrappers = append(append([]string(nil), u.Wrappers...), v.Wrappers...)
							v.Env = append(append([]Assign(nil), u.Env...), v.Env...)
						}
					}
					ps = append(ps, body.Pipelines...)
				}
				continue
			}
		}
		ps = append(ps, p)
	}
	c.Pipelines = ps
	return c
}

func cloneCommand(c Command) Command {
	c.Pipelines = slices.Clone(c.Pipelines)
	for pi := range c.Pipelines {
		p := &c.Pipelines[pi]
		p.Pipes = slices.Clone(p.Pipes)
		p.Units = slices.Clone(p.Units)
		for ui := range p.Units {
			u := &p.Units[ui]
			u.Words = slices.Clone(u.Words)
			u.Wrappers = slices.Clone(u.Wrappers)
			u.Env = slices.Clone(u.Env)
			u.Redirects = slices.Clone(u.Redirects)
			if u.Body != nil {
				body := cloneCommand(*u.Body)
				u.Body = &body
			}
			u.Subst = slices.Clone(u.Subst)
			for i, sub := range u.Subst {
				body := cloneCommand(*sub)
				u.Subst[i] = &body
			}
		}
	}
	return c
}
