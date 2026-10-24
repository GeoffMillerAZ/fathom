package shellcmd

import (
	"path"
	"strings"
)

// peel strips wrapper commands off the front of words, outermost first,
// returning the wrapper names, any assignments a wrapper carried, and the
// words that run. A wrapper with nothing after it (`sudo -v`, bare `env`,
// `command -v go`) is the command itself and is not peeled.
func peel(words []token) (wrappers []string, env []Assign, rest []token, err error) {
	rest = words
	for len(rest) > 0 && rest[0].word.Literal() {
		name := path.Base(rest[0].word.Text)
		next, carried, ok, err := peelOne(name, rest)
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok {
			break
		}
		wrappers = append(wrappers, name)
		env = append(env, carried...)
		rest = next
	}
	return wrappers, env, rest, nil
}

// optArgs names, per wrapper, the options that take a separate argument.
var optArgs = map[string]map[string]bool{
	"sudo": {
		"-u": true, "-g": true, "-h": true, "-p": true, "-C": true, "-D": true, "-r": true, "-t": true, "-T": true, "-U": true,
		"--user": true, "--group": true, "--host": true, "--prompt": true, "--close-from": true, "--chdir": true,
		"--role": true, "--type": true, "--command-timeout": true, "--other-user": true,
	},
	"env":     {"-u": true, "-C": true, "-P": true, "--unset": true, "--chdir": true},
	"exec":    {"-a": true},
	"nice":    {"-n": true, "--adjustment": true},
	"timeout": {"-s": true, "-k": true, "--signal": true, "--kill-after": true},
}

// skipOpts skips options from words[1:] for the named wrapper and returns
// the index of the first operand.
func skipOpts(name string, words []token) int {
	i := 1
	for i < len(words) {
		w := words[i].word.Text
		if w == "--" {
			return i + 1
		}
		if len(w) < 2 || w[0] != '-' {
			return i
		}
		if optArgs[name][w] {
			i += 2
			continue
		}
		i++
	}
	return i
}

func peelOne(name string, words []token) (rest []token, env []Assign, ok bool, err error) {
	switch name {
	case "sudo":
		i := skipOpts(name, words)
		for i < len(words) {
			a, isAssign := assignment(&words[i])
			if !isAssign {
				break
			}
			env = append(env, a)
			i++
		}
		if i >= len(words) {
			return nil, nil, false, nil
		}
		return words[i:], env, true, nil
	case "env":
		for _, t := range words[1:] {
			if w := t.word.Text; w == "-S" || strings.HasPrefix(w, "--split-string") || (strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.ContainsRune(w, 'S')) {
				return nil, nil, false, fail("env -S splits a string the lexer cannot read")
			}
			if !strings.HasPrefix(t.word.Text, "-") {
				break
			}
		}
		i := skipOpts(name, words)
		for i < len(words) {
			a, isAssign := assignment(&words[i])
			if !isAssign {
				break
			}
			env = append(env, a)
			i++
		}
		if i >= len(words) {
			return nil, nil, false, nil
		}
		return words[i:], env, true, nil
	case "command":
		if len(words) > 1 && (words[1].word.Text == "-v" || words[1].word.Text == "-V") {
			return nil, nil, false, nil
		}
		i := skipOpts(name, words)
		if i >= len(words) {
			return nil, nil, false, nil
		}
		return words[i:], nil, true, nil
	case "rtk":
		rest, ok := peelRtk(words)
		return rest, nil, ok, nil
	case "exec", "nohup", "time", "nice":
		i := skipOpts(name, words)
		if name == "nohup" {
			i = 1
		}
		if i >= len(words) {
			return nil, nil, false, nil
		}
		return words[i:], nil, true, nil
	case "timeout":
		i := skipOpts(name, words) + 1 // the duration
		if i >= len(words) {
			return nil, nil, false, nil
		}
		return words[i:], nil, true, nil
	}
	return nil, nil, false, nil
}

// peelRtk recognises direct commands and the test, err and summary wrappers.
// RTK's own tools and controls are not executable names. Only its known
// global options are transparent; an unknown option or dynamic command
// stays with RTK rather than guessing which argument would run.
func peelRtk(words []token) ([]token, bool) {
	i := 1
	for i < len(words) && words[i].word.Literal() {
		w := words[i].word.Text
		if w == "--" {
			i++
			break
		}
		if w != "--verbose" && w != "--skip-env" && !(len(w) > 1 && w[0] == '-' && strings.TrimLeft(w[1:], "v") == "") {
			break
		}
		i++
	}
	if i >= len(words) || !words[i].word.Literal() || words[i].word.Text == "" || strings.HasPrefix(words[i].word.Text, "-") {
		return nil, false
	}
	switch words[i].word.Text {
	case "read", "smart", "json", "deps", "env", "log", "ls", "tree", "find", "grep", "diff", "wc",
		"help", "gain", "discover", "learn", "cc-economics", "hook-audit", "init", "config", "rewrite", "verify", "recall", "telemetry", "session", "llm-proxy":
		return nil, false
	case "test", "err", "summary":
		i++
		if i >= len(words) || !words[i].word.Literal() || words[i].word.Text == "" || strings.HasPrefix(words[i].word.Text, "-") {
			return nil, false
		}
	}
	return words[i:], true
}

// shells are the shells whose -c script is lexed.
var shells = map[string]bool{"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true, "mksh": true, "ash": true}

// shellScript lexes the script of `bash -c 'script'` into a body. isShell is
// false for a shell run without -c (a script file, or stdin).
func shellScript(words []Word, depth int) (*Command, bool, error) {
	if !shells[path.Base(words[0].Text)] {
		return nil, false, nil
	}
	cflag := false
	i := 1
	for i < len(words) {
		w := words[i].Text
		if w == "--" {
			i++
			break
		}
		if len(w) < 2 || (w[0] != '-' && w[0] != '+') {
			break
		}
		if strings.HasPrefix(w, "--") {
			if w == "--rcfile" || w == "--init-file" {
				i++
			}
			i++
			continue
		}
		for _, ch := range w[1:] {
			switch ch {
			case 'c':
				cflag = true
			case 'o', 'O':
				i++
			}
		}
		i++
	}
	if !cflag {
		return nil, false, nil
	}
	if i >= len(words) {
		return nil, false, fail("shell -c without a script")
	}
	script := words[i]
	if !script.Literal() {
		return nil, false, fail("shell -c script is not literal")
	}
	body := parseAt(script.Text, depth+1)
	if body.Unreadable {
		return nil, false, fail("in shell -c: " + body.Reason)
	}
	return body, true, nil
}
