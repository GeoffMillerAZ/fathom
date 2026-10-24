package shellcmd

import "path"

// Command is one lexed command line (or a nested body).
type Command struct {
	// Pipelines are the top-level pipelines in source order; nil when
	// Unreadable.
	Pipelines []Pipeline
	// Unreadable is true when the lexer could not read the text; Reason
	// names the first cause.
	Unreadable bool
	Reason     string
	// Refusal distinguishes a declared-wrapper refusal from ordinary syntax errors.
	Refusal Refusal
}

// Refusal is the typed source of an unreadable declared command.
type Refusal uint8

const (
	RefusalNone Refusal = iota
	RefusalWrapper
)

// Pipeline is one or more units joined by `|` or `|&`, ended by Sep.
type Pipeline struct {
	Units []Unit
	// Pipes[i] is the operator between Units[i] and Units[i+1]: "|" or "|&".
	Pipes []string
	// Sep is the operator that ends the pipeline: "&&", "||", ";", ";;",
	// ";&", ";;&", "&" (a background job), "\n", or "" at the end of input.
	Sep string
	// Bang is true when the pipeline is led by `!`.
	Bang bool
}

// Kind is what a Unit is.
type Kind int

// The unit kinds.
const (
	KindSimple   Kind = iota // a simple command (possibly only assignments or redirects)
	KindSubshell             // ( list )
	KindGroup                // { list; }
	KindCompound             // if, while, until, for, select, case, [[ ]], (( ))
	KindShell                // a shell run with -c: bash -c 'script'
)

func (k Kind) String() string {
	switch k {
	case KindSimple:
		return "simple"
	case KindSubshell:
		return "subshell"
	case KindGroup:
		return "group"
	case KindCompound:
		return "compound"
	case KindShell:
		return "shell"
	}
	return "unknown"
}

// Unit is one element of a pipeline.
type Unit struct {
	Kind Kind
	// DeclaredScript marks an operator-declared script wrapper. Script is
	// the literal script; Cwd is its declared directory, inherited by its
	// body. Relative directories remain relative to the caller's cwd.
	DeclaredScript bool
	Script         string
	Cwd            string
	// CwdUnknown means a script wrapper did not declare its working directory.
	// Its body inherits this uncertainty, even after a relative cd.
	CwdUnknown bool
	// Keyword names a compound: "if", "while", "until", "for", "select",
	// "case", "[[" or "((".
	Keyword string
	// Env holds leading NAME=value assignments (and those a peeled sudo or
	// env carried).
	Env []Assign
	// Wrappers are the peeled wrapper commands, outermost first: "sudo",
	// "env", "command", "exec", "nohup", "time", "nice", "timeout", "rtk".
	Wrappers []string
	// Words is the argv that runs, after Env and Wrappers. For KindShell it
	// is the shell's own argv (`bash -c script …`); for a for, select or
	// case compound the loop variable and list, or the case subject; for
	// `[[` the test's words.
	Words []Word
	// Redirects in source order, those after a compound's closer included.
	Redirects []Redirect
	// Body is the nested list of a subshell, group, compound or -c shell.
	Body *Command
	// Subst are the command and process substitutions found in Words, Env
	// values and redirect targets, in source order.
	Subst []*Command
}

// Name is the base name of the command that runs (Words[0]), or "" when the
// unit has no words.
func (u *Unit) Name() string {
	if len(u.Words) == 0 {
		return ""
	}
	return path.Base(u.Words[0].Text)
}

// Args is Words' literal texts.
func (u *Unit) Args() []string {
	out := make([]string, len(u.Words))
	for i, w := range u.Words {
		out[i] = w.Text
	}
	return out
}

// HasWrapper reports whether name was peeled from the unit.
func (u *Unit) HasWrapper(name string) bool {
	for _, w := range u.Wrappers {
		if w == name {
			return true
		}
	}
	return false
}

// Word is one shell word after quote removal.
type Word struct {
	// Text is the quote-removed literal; an expansion keeps its source text.
	Text string
	// Quoted is true when any part of the word was quoted or escaped.
	Quoted bool
	// Dynamic is true when the word carries a parameter, command or
	// arithmetic expansion, so the shell would compute its value.
	Dynamic bool
	// Glob is true when the word carries an unquoted glob or brace pattern.
	Glob bool
}

// Literal reports whether the shell would pass Text unchanged.
func (w Word) Literal() bool { return !w.Dynamic && !w.Glob }

// Assign is one NAME=value (or NAME+=value) prefix.
type Assign struct {
	Name   string
	Value  Word
	Append bool
}

// Redirect is one redirection.
type Redirect struct {
	// Fd is the explicit file descriptor (2 in `2>&1`), or -1.
	Fd int
	// Op is the operator: "<", ">", ">>", ">|", "<>", "<<", "<<-", "<<<",
	// ">&", "<&", "&>", "&>>".
	Op string
	// Target is the file, descriptor or here-string word; for a heredoc,
	// the delimiter word.
	Target Word
	// Heredoc is set for "<<" and "<<-".
	Heredoc *Heredoc
}

// Heredoc is a heredoc's delimiter and body.
type Heredoc struct {
	Delim string
	// Quoted is true when the delimiter was quoted, so the body is literal.
	Quoted bool
	// StripTabs is true for "<<-".
	StripTabs bool
	// Body is the text between the operator's line and the delimiter line.
	Body string
	// Dynamic is true when an unquoted body carries an expansion.
	Dynamic bool
}

// Units returns every unit in source order, pre-order: a subshell, group,
// compound or -c shell comes before the units of its Body. Command and
// process substitutions are not walked (they are in Unit.Subst).
func (c *Command) Units() []*Unit {
	var out []*Unit
	var walk func(c *Command)
	walk = func(c *Command) {
		if c == nil {
			return
		}
		for pi := range c.Pipelines {
			p := &c.Pipelines[pi]
			for ui := range p.Units {
				u := &p.Units[ui]
				out = append(out, u)
				walk(u.Body)
			}
		}
	}
	walk(c)
	return out
}

// ReadoutPrefix is the literal value of a FATHOM_READOUT= assignment that
// prefixes a top-level unit (the last one when several do), or "".
func (c *Command) ReadoutPrefix() string {
	v := ""
	for _, p := range c.Pipelines {
		for _, u := range p.Units {
			for _, a := range u.Env {
				if a.Name == "FATHOM_READOUT" && !a.Append && a.Value.Literal() {
					v = a.Value.Text
				}
			}
		}
	}
	return v
}

// Rtk reports whether any unit runs RTK itself or behind an `rtk ` prefix.
func (c *Command) Rtk() bool {
	for _, u := range c.Units() {
		if u.Name() == "rtk" || u.HasWrapper("rtk") {
			return true
		}
	}
	return false
}
