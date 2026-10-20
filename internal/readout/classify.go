package readout

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// The compound shapes a rule may accept (design §7).
const (
	CompoundSimple      = "simple"
	CompoundDisplayTail = "display_tail"
	CompoundAfterQuiet  = "after_quiet"
	compoundList        = "list" // any other list: no rule accepts it
)

// shape is what the classifier reads off a lexed command.
type shape struct {
	unit     *shellcmd.Unit // the producer whose stdout the agent sees
	compound string
	// machine names why the output is machine-bound (ADR-071 decision
	// 3e), "" when it is not.
	machine string
	// merged is true when stderr joins stdout (2>&1, |&).
	merged bool
	// declared: the caller's test-run matcher (TR-1) says the command runs
	// a declared test command (Input.DeclaredTest).
	declared bool
	// quiet are the leading `&&`-joined quiet segments read past before the
	// command was shaped (stripQuietPrefix, RO-28); their own output lines
	// lead stdout and are kept verbatim (quietOutput).
	quiet []shellcmd.Pipeline
}

// QuietHeads is the quiet list as a reading names it, in the order isQuiet
// tests it; the user's [readouts] quiet_prefixes follow it.
func QuietHeads() []string {
	return []string{"cd", "export", "mkdir -p", "sleep N", "(assignment)", "set", "source", ".", "tccutil reset"}
}

// IsQuiet reports whether a unit is a quiet segment's command (QuietHeads,
// then extra): a simple command with no redirect and no wrapper that prints
// nothing on success but what quietOutput recognizes. extra are the user's
// [readouts] quiet_prefixes, each a command name or its leading words.
func IsQuiet(u *shellcmd.Unit, extra []string) bool {
	return isQuiet(u, extra)
}

func isQuiet(u *shellcmd.Unit, extra []string) bool {
	if u.Kind != shellcmd.KindSimple || len(u.Redirects) > 0 || len(u.Wrappers) > 0 {
		return false
	}
	if len(u.Words) == 0 {
		return len(u.Env) > 0 // an assignment alone
	}
	args := u.Args()
	switch u.Name() {
	case "cd", "export", "source", ".":
		return true
	case "mkdir":
		return hasOption(args[1:], 'p', "--parents")
	case "sleep":
		if len(args) != 2 {
			return false
		}
		_, err := strconv.ParseFloat(args[1], 64)
		return err == nil
	case "set":
		// set alone prints every variable; with an option it prints nothing.
		return len(args) > 1 && (strings.HasPrefix(args[1], "-") || strings.HasPrefix(args[1], "+"))
	case "tccutil":
		return len(args) > 1 && args[1] == "reset"
	}
	for _, q := range extra {
		ws := strings.Fields(q)
		if len(ws) == 0 || len(ws) > len(args) || u.Name() != ws[0] {
			continue
		}
		if slices.Equal(args[1:len(ws)], ws[1:]) {
			return true
		}
	}
	return false
}

// hasOption reports whether args carry the short option ch, alone or in a
// cluster, or the long one ("" for none), before `--`.
func hasOption(args []string, ch rune, long string) bool {
	for _, a := range args {
		switch {
		case a == "--":
			return false
		case long != "" && a == long:
			return true
		case len(a) > 1 && a[0] == '-' && a[1] != '-' && strings.ContainsRune(a[1:], ch):
			return true
		}
	}
	return false
}

// stripQuietPrefix splits a readable command at its leading quiet segments
// (RO-28): each one quiet unit (isQuiet), not negated, joined to the next by
// `&&`; the last segment is never stripped. A `;`, `||` or newline, a
// non-quiet segment, a redirect or a wrapper ends the prefix. quiet is nil
// when nothing is stripped; rest is the command read from there on.
func stripQuietPrefix(c *shellcmd.Command, extra []string) (quiet, rest []shellcmd.Pipeline) {
	ps := c.Pipelines
	if c.Unreadable {
		return nil, ps
	}
	n := 0
	for n < len(ps)-1 {
		p := &ps[n]
		if len(p.Units) != 1 || p.Bang || p.Sep != "&&" || !isQuiet(&p.Units[0], extra) {
			break
		}
		n++
	}
	if n == 0 {
		return nil, ps
	}
	return ps[:n], ps[n:]
}

// xtraceLine is a set -x trace: one or more `+`, then a space.
var xtraceLine = regexp.MustCompile(`^\++ `)

// quietOutput splits stdout after the quiet segments' own lines: the
// leading run of lines one of them prints on success. A quiet head prints
// nothing but these: an xtrace line once a `set` turns on -x (in stdout
// only when stderr joins it), a directory `mkdir -v` created (the GNU and
// BSD forms), the directory `cd -` or a CDPATH hit echoes, and tccutil
// reset's line. Every other line is the producer's.
func quietOutput(quiet []shellcmd.Pipeline, stdout string) (head, rest string) {
	if len(quiet) == 0 {
		return "", stdout
	}
	xtrace, tcc := false, false
	var mkdirs, cds []string
	for i := range quiet {
		u := &quiet[i].Units[0]
		args := u.Args()
		switch u.Name() {
		case "set":
			for j, a := range args[1:] {
				if hasOption([]string{a}, 'x', "") || (a == "-o" && j+2 < len(args) && args[j+2] == "xtrace") {
					xtrace = true
				}
			}
		case "mkdir":
			if hasOption(args[1:], 'v', "--verbose") {
				for _, a := range args[1:] {
					if !strings.HasPrefix(a, "-") {
						mkdirs = append(mkdirs, strings.TrimSuffix(a, "/"))
					}
				}
			}
		case "cd":
			if len(args) > 1 {
				cds = append(cds, strings.TrimSuffix(args[len(args)-1], "/"))
			}
		case "tccutil":
			tcc = true
		}
	}
	quietLine := func(l string) bool {
		if xtrace && xtraceLine.MatchString(l) {
			return true
		}
		if tcc && strings.HasPrefix(l, "Successfully reset") {
			return true
		}
		for _, d := range cds {
			// cd - echoes the directory; a CDPATH hit echoes where it went.
			if strings.HasPrefix(l, "/") && (d == "-" || (!strings.HasPrefix(d, "/") && !strings.HasPrefix(d, ".") && strings.HasSuffix(l, "/"+d))) {
				return true
			}
		}
		for _, d := range mkdirs {
			if strings.HasPrefix(l, "mkdir: created directory ") || l == d || strings.HasPrefix(d, l+"/") {
				return true
			}
		}
		return false
	}
	at := 0
	for at < len(stdout) {
		line, next := stdout[at:], len(stdout)
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line, next = line[:end], at+end+1
		}
		if !quietLine(line) {
			break
		}
		at = next
	}
	return stdout[:at], stdout[at:]
}

// displayOnly reports whether a piped-into unit only shows its input: head,
// tail without follow, or cat, with no file operand.
func displayOnly(u *shellcmd.Unit) bool {
	if u.Kind != shellcmd.KindSimple || len(u.Redirects) > 0 || len(u.Wrappers) > 0 || len(u.Env) > 0 {
		return false
	}
	name := u.Name()
	switch name {
	case "head", "tail", "cat":
	default:
		return false
	}
	args := u.Args()[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case name == "tail" && (a == "-f" || a == "-F" || strings.HasPrefix(a, "--follow") || (len(a) > 1 && a[0] == '-' && a[1] != '-' && strings.ContainsAny(a, "fF"))):
			return false
		case (name == "head" || name == "tail") && (a == "-n" || a == "-c"):
			i++
		case strings.HasPrefix(a, "-") && a != "-":
		default:
			return false
		}
	}
	return true
}

// machineFlags are machine-bound for every command; grepMachineFlags for
// the grep family only, where -l means files and -c counts (ls -l is a
// listing, not machine output).
var (
	machineFlags     = []string{"--porcelain", "-z", "--json", "-json", "--name-only", "--null-data"}
	grepFamily       = map[string]bool{"grep": true, "egrep": true, "fgrep": true, "rg": true}
	grepMachineShort = "clLoZ"
	grepMachineLong  = []string{"--count", "--files-with-matches", "--files-without-match", "--only-matching", "--null", "--json"}
)

func hasMachineFlag(u *shellcmd.Unit) string {
	args := u.Args()
	grep := grepFamily[u.Name()] || (u.Name() == "git" && gitSub(u) == "grep")
	for _, a := range args[1:] {
		if a == "--" {
			break
		}
		for _, f := range machineFlags {
			if a == f || strings.HasPrefix(a, f+"=") {
				return f
			}
		}
		if !grep || !strings.HasPrefix(a, "-") {
			continue
		}
		if strings.HasPrefix(a, "--") {
			for _, f := range grepMachineLong {
				if a == f || strings.HasPrefix(a, f+"=") {
					return f
				}
			}
			continue
		}
		for _, ch := range a[1:] {
			if strings.ContainsRune("ABCefm", ch) {
				break // the rest of the cluster is that option's argument
			}
			if strings.ContainsRune(grepMachineShort, ch) {
				return "-" + string(ch)
			}
		}
	}
	return ""
}

// classifyCommand reads the shape of a lexed command: past its leading
// `&&`-joined quiet segments (stripQuietPrefix, RO-28), by what remains.
func classifyCommand(c *shellcmd.Command, quiet []string) shape {
	ps := c.Pipelines
	if len(ps) == 0 {
		return shape{machine: "empty command"}
	}
	for _, p := range ps {
		if p.Sep == "&" {
			return shape{machine: "background job"}
		}
	}
	pre, ps := stripQuietPrefix(c, quiet)
	last := len(ps) - 1
	sh := shape{compound: CompoundSimple, quiet: pre}
	if last > 0 {
		sh.compound = CompoundAfterQuiet
		for _, p := range ps[:last] {
			if len(p.Units) != 1 || p.Bang || !(p.Sep == "&&" || p.Sep == ";" || p.Sep == "\n") || !isQuiet(&p.Units[0], quiet) {
				sh.compound = compoundList
				break
			}
		}
	}
	p := ps[last]
	u := &p.Units[0]
	sh.unit = u
	switch {
	case p.Bang:
		sh.machine = "negated pipeline"
	case u.Kind == shellcmd.KindSubshell:
		sh.machine = "subshell"
	case u.Kind == shellcmd.KindShell:
		sh.machine = "shell -c"
	case u.Kind != shellcmd.KindSimple:
		sh.machine = "compound command"
	case u.HasWrapper("sudo"):
		sh.machine = "sudo"
	case len(u.Words) == 0:
		sh.machine = "no command"
	}
	if sh.machine != "" {
		return sh
	}
	for _, r := range u.Redirects {
		switch {
		case r.Heredoc != nil || r.Op == "<<<":
			sh.machine = "heredoc"
		case r.Op == ">&" && r.Fd == 2 && r.Target.Text == "1":
			sh.merged = true
		default:
			sh.machine = "redirect"
		}
		if sh.machine != "" {
			return sh
		}
	}
	if f := hasMachineFlag(u); f != "" {
		sh.machine = "flag " + f
		return sh
	}
	if len(p.Units) > 1 {
		if sh.compound != CompoundSimple {
			sh.compound = compoundList
		} else {
			sh.compound = CompoundDisplayTail
		}
		for i := 1; i < len(p.Units); i++ {
			if !displayOnly(&p.Units[i]) {
				sh.machine = "pipe into " + p.Units[i].Name()
				return sh
			}
		}
		for _, op := range p.Pipes {
			if op == "|&" {
				sh.merged = true
			}
		}
	}
	return sh
}

// gitSub is git's subcommand, past its global options.
func gitSub(u *shellcmd.Unit) string {
	args := u.Args()
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree" || a == "--namespace":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			return a
		}
	}
	return ""
}

// sub is the subcommand a rule's match.sub reads: git's past its global
// options, else the first word that is not a flag.
func sub(u *shellcmd.Unit) string {
	if u.Name() == "git" {
		return gitSub(u)
	}
	for _, a := range u.Args()[1:] {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// flags are the words after the command name that start with `-`, up to `--`.
func flags(u *shellcmd.Unit) []string {
	var out []string
	for _, a := range u.Args()[1:] {
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			out = append(out, a)
		}
	}
	return out
}

// readOperands are a read's file operands, with the line offset of its first
// shown line when known (-1 when not): cat, head, tail and sed -n.
func readOperands(u *shellcmd.Unit) (files []shellcmd.Word, offset int, ok bool) {
	name := u.Name()
	words := u.Words[1:]
	offset = 0
	switch name {
	case "cat", "head", "tail":
		if name == "tail" {
			offset = -1
		}
		for i := 0; i < len(words); i++ {
			a := words[i].Text
			switch {
			case name != "cat" && (a == "-n" || a == "-c"):
				i++
			case strings.HasPrefix(a, "-") && a != "-":
			default:
				files = append(files, words[i])
			}
		}
		return files, offset, true
	case "sed":
		quiet, script := false, ""
		scripted := false
		for i := 0; i < len(words); i++ {
			a := words[i].Text
			switch {
			case a == "-n" || a == "--quiet" || a == "--silent":
				quiet = true
			case a == "-e" || a == "-f":
				scripted = true
				if i+1 < len(words) {
					script = words[i+1].Text
				}
				i++
			case strings.HasPrefix(a, "-i") || strings.HasPrefix(a, "--in-place"):
				return nil, 0, false
			case strings.HasPrefix(a, "-") && a != "-":
				if strings.Contains(a[1:], "n") && !strings.HasPrefix(a, "--") {
					quiet = true
				}
			case !scripted:
				scripted = true
				script = a
			default:
				files = append(files, words[i])
			}
		}
		if !quiet {
			return nil, 0, false
		}
		offset = -1
		if m := sedRange.FindStringSubmatch(script); m != nil {
			n, _ := strconv.Atoi(m[1])
			offset = n - 1
		}
		return files, offset, true
	}
	return nil, 0, false
}

var sedRange = regexp.MustCompile(`^(\d+)(,(\d+|\$))?p$`)

// isSourceFile reports whether fathom's extractors know the file.
func isSourceFile(w shellcmd.Word) bool {
	return w.Literal() && w.Text != "" && w.Text != "-" && connector.LanguageForPath(w.Text) != core.LangUnclassified
}

// streamOf is the closed content table, fixed in code (design §6.1): the
// stdout of a source-file read, grep, rg and git grep matches, ls, and git
// log, show and diff bodies is content; everything else is diagnostic, and
// so is any stdout stderr was merged into.
func streamOf(sh shape) Stream {
	if sh.merged || sh.unit == nil {
		return StreamDiagnostic
	}
	u := sh.unit
	switch u.Name() {
	case "grep", "egrep", "fgrep", "rg", "ls":
		return StreamContent
	case "git":
		switch gitSub(u) {
		case "log", "show", "diff", "grep":
			return StreamContent
		}
	case "cat", "head", "tail", "sed":
		files, _, ok := readOperands(u)
		if !ok || len(files) == 0 {
			return StreamDiagnostic
		}
		for _, f := range files {
			if !isSourceFile(f) {
				return StreamDiagnostic
			}
		}
		return StreamContent
	}
	return StreamDiagnostic
}

// ruleStreams are the stream classes a rule's match can reach, content
// first (the unmerged ones: a merged stderr is diagnostic for every rule).
func ruleStreams(m Match) []Stream {
	content, diag := false, m.DeclaredTest
	for _, a := range m.Argv0 {
		switch a {
		case "grep", "egrep", "fgrep", "rg", "ls":
			content = true
		case "git":
			allContent := len(m.Sub) > 0
			for _, s := range m.Sub {
				switch s {
				case "log", "show", "diff", "grep":
				default:
					allContent = false
				}
			}
			if allContent {
				content = true
			} else {
				content, diag = true, true
			}
		case "cat", "head", "tail", "sed":
			content = true
			diag = diag || !m.WholeRead
		default:
			diag = true
		}
	}
	var out []Stream
	if content {
		out = append(out, StreamContent)
	}
	if diag {
		out = append(out, StreamDiagnostic)
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// matches reports whether a rule's match accepts the shape.
func (m *Match) matches(sh shape) bool {
	u := sh.unit
	declared := m.DeclaredTest && sh.declared
	if u == nil || (!declared && !contains(m.Argv0, u.Name())) {
		return false
	}
	switch m.Compound {
	case "", CompoundSimple:
		if sh.compound != CompoundSimple {
			return false
		}
	case CompoundDisplayTail, CompoundAfterQuiet:
		if sh.compound != CompoundSimple && sh.compound != m.Compound {
			return false
		}
	default:
		return false
	}
	if m.WholeRead {
		if _, ok := wholeRead(u); !ok {
			return false
		}
	}
	if !declared && len(m.Sub) > 0 && !contains(m.Sub, sub(u)) {
		return false
	}
	fl := flags(u)
	has := func(f string) bool {
		for _, x := range fl {
			if x == f || (strings.HasPrefix(f, "-") && strings.HasPrefix(x, f+"=")) {
				return true
			}
		}
		return false
	}
	if !declared && len(m.FlagsAny) > 0 {
		found := false
		for _, f := range m.FlagsAny {
			if has(f) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, f := range m.FlagsNone {
		if has(f) {
			return false
		}
	}
	return true
}
