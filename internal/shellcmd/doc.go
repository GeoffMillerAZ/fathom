// Package shellcmd is the one command lexer every Bash consumer reads (RO-3,
// docs/plans/readouts.md; design docs/design/readouts.md §2).
//
// ParseCommand reads a Bash command line and returns its structure: the
// top-level pipelines in source order (Command.Pipelines), each ended by its
// separator (`&&`, `||`, `;`, `;;`, `;&`, `;;&`, `&`, a newline, or "" at the
// end), each made of units joined by `|` or `|&`. A Unit is a simple command
// (env prefixes, peeled wrappers, words, redirects), a subshell `( … )`, a
// group `{ …; }`, a compound command (`if`, `while`, `until`, `for`,
// `select`, `case`, `[[ … ]]`, `(( … ))`) or a shell run with `-c`
// (`bash -c '…'`, whose script is lexed into Unit.Body). Quotes, escapes,
// comments, line continuations, redirects with file descriptors, heredocs
// (bodies read), here-strings, `$(…)`, backticks, `$((…))`, `${…}`, process
// substitution and background jobs are recognised; command and process
// substitutions are lexed into Unit.Subst. Wrappers (`sudo`, `env`,
// `command`, `exec`, `nohup`, `time`, `nice`, `timeout` and an `rtk ` prefix)
// are peeled into Unit.Wrappers so Unit.Words is the command that runs, and
// leading `NAME=value` words are Unit.Env, which is where a `FATHOM_READOUT=`
// prefix is read (Command.ReadoutPrefix). Command.Units walks every unit,
// nested bodies included, in source order.
// Optional Wrapper declarations extend argv prefixes and literal script
// arguments. Declared scripts have KindShell, DeclaredScript and a Body;
// their Cwd is inherited by nested units. Two declared script levels are
// allowed. Transparent projects standalone declared scripts for classifiers
// while preserving opaque pipeline boundaries. Unwrap exposes the same
// operation to consumers with an already lexed command.
//
// The lexer classifies and never re-emits: nothing here renders a Command
// back to shell text, because readouts never rewrite a command (ADR-071
// decision 2). A Word's Text is its quote-removed literal; an expansion in it
// stays as its source text and sets Word.Dynamic, so a consumer never trusts
// a value the shell would compute.
//
// What it cannot read is unreadable, never guessed: an unterminated quote,
// substitution or heredoc, an unbalanced compound, a syntax error, a command
// name that is not literal, `bash -c` with a script that is not literal,
// `eval`, `env -S`, a function definition, a `case` inside a command
// substitution, a NUL byte, or nesting deeper than 16. Command.Unreadable is
// then true, Command.Reason names the first cause, and Command.Pipelines is
// nil. The lexer applies no default for it: each consumer maps unreadable
// itself (readouts to raw, the bash-write detector to doubt, the test-run
// matcher to no match, the discovery observer to a call counted unreadable;
// RO-4).
//
// ParseCommand is pure (no I/O, no clock) and deterministic: the same text
// and wrapper declarations give the same Command.
package shellcmd
