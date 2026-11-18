package deps

import (
	"bufio"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

type dockerInstruction struct {
	name, args string
	line       int
	escape     byte
	flags      []dockerFlag
}

type dockerFlag struct {
	key, value string
}

type dockerParser struct {
	scanner      *bufio.Scanner
	instruction  dockerInstruction
	line, failed int
	escape       byte
	directives   map[string]bool
	frontend     string
	frontendLine int
}

func newDockerParser(src []byte) *dockerParser {
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(src), "\ufeff")))
	return &dockerParser{scanner: scanner, escape: '\\', directives: make(map[string]bool)}
}

func (p *dockerParser) scan() bool {
	if p.scanner.Scan() {
		p.line++
		return true
	}
	if p.scanner.Err() != nil {
		p.failed = p.line + 1
	}
	return false
}

func (p *dockerParser) next() bool {
	for p.failed == 0 && p.scan() {
		raw := strings.TrimLeftFunc(p.scanner.Text(), unicode.IsSpace)
		if p.directives != nil && p.directive(raw) {
			continue
		}
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		start := p.line
		var text strings.Builder
		for {
			part, continued := dockerContinuation(raw, p.escape)
			text.WriteString(part)
			if !continued {
				break
			}
			for {
				if !p.scan() {
					p.failed = start
					return false
				}
				raw = p.scanner.Text()
				trimmed := strings.TrimSpace(raw)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					break
				}
			}
		}
		in, ok := splitDockerInstruction(text.String(), start, p.escape)
		if !ok || !p.heredocs(in) {
			p.failed = start
			return false
		}
		p.instruction = in
		return true
	}
	return false
}

func dockerContinuation(line string, escape byte) (string, bool) {
	end := strings.TrimRight(line, " \t")
	if len(end) == 0 || end[len(end)-1] != escape || len(end) > 1 && end[len(end)-2] == escape {
		return line, false
	}
	return end[:len(end)-1], true
}

func (p *dockerParser) directive(line string) bool {
	key, value, ok := strings.Cut(strings.TrimPrefix(line, "#"), "=")
	key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
	if !ok || !strings.HasPrefix(line, "#") || value == "" || key != "escape" && key != "syntax" && key != "check" {
		p.directives = nil
		return false
	}
	if p.directives[key] {
		p.failed = p.line
		return true
	}
	p.directives[key] = true
	switch key {
	case "escape":
		if value != "\\" && value != "`" {
			p.failed = p.line
		} else {
			p.escape = value[0]
		}
	case "syntax":
		p.frontend, p.frontendLine = strings.Fields(value)[0], p.line
	}
	return true
}

func splitDockerInstruction(text string, line int, escape byte) (dockerInstruction, bool) {
	rest := strings.TrimSpace(text)
	i := strings.IndexFunc(rest, unicode.IsSpace)
	if i < 0 {
		return dockerInstruction{name: strings.ToUpper(rest), line: line, escape: escape}, true
	}
	in := dockerInstruction{name: strings.ToUpper(rest[:i]), line: line, escape: escape}
	rest = strings.TrimSpace(rest[i:])
	for strings.HasPrefix(rest, "--") {
		word, tail, ok := takeDockerWord(rest, escape, false)
		if !ok {
			return in, false
		}
		rest = strings.TrimSpace(tail)
		if word == "--" {
			break
		}
		key, value, _ := strings.Cut(word[2:], "=")
		in.flags = append(in.flags, dockerFlag{key, value})
	}
	in.args = rest
	return in, true
}

// takeDockerWord handles quoting in builder flags and heredoc markers. It
// keeps quotes when scanning potential heredocs, so a quoted shell string
// containing "<<text" is distinguishable from a heredoc redirect.
func takeDockerWord(text string, escape byte, preserve bool) (word, rest string, ok bool) {
	var out strings.Builder
	var quote byte
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if quote == 0 && (ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n') {
			return out.String(), text[i:], true
		}
		if ch == escape && i+1 < len(text) && (quote != '\'' || !preserve) {
			if preserve {
				out.WriteByte(ch)
			}
			i++
			out.WriteByte(text[i])
			continue
		}
		if ch == quote {
			quote = 0
		} else if quote == 0 && (ch == '\'' || ch == '"') {
			quote = ch
		} else {
			out.WriteByte(ch)
			continue
		}
		if preserve {
			out.WriteByte(ch)
		}
	}
	return out.String(), "", quote == 0
}

func dockerCopyWords(args string, escape byte) ([]string, bool) {
	var words []string
	if strings.HasPrefix(args, "[") && json.Valid([]byte(args)) {
		if json.Unmarshal([]byte(args), &words) != nil {
			return nil, false
		}
		// Unmarshal accepts null as a string element. That is not a Docker
		// operand, and an empty source is invalid in either form.
		for _, word := range words {
			if word == "" {
				return nil, false
			}
		}
		return words, true
	}
	for _, word := range strings.Fields(args) {
		if _, heredoc := dockerHeredoc(word); !heredoc {
			var rest string
			var ok bool
			word, rest, ok = takeDockerWord(word, escape, false)
			if !ok || rest != "" || word == "" {
				return nil, false
			}
		}
		words = append(words, word)
	}
	return words, true
}

var dockerHeredocMarker = regexp.MustCompile(`^[0-9]*<<(-?)\s*([^<]+)$`)

type dockerHere struct {
	end string
	tab bool
}

func dockerHeredoc(word string) (dockerHere, bool) {
	match := dockerHeredocMarker.FindStringSubmatch(word)
	if match == nil {
		return dockerHere{}, false
	}
	end, rest, ok := takeDockerWord(match[2], '\\', false)
	return dockerHere{end, match[1] == "-"}, ok && end != "" && rest == ""
}

func (p *dockerParser) heredocs(in dockerInstruction) bool {
	if !strings.Contains(in.args, "<<") {
		return true
	}
	if in.name == "ONBUILD" {
		var ok bool
		in, ok = splitDockerInstruction(in.args, in.line, in.escape)
		if !ok {
			return false
		}
	}
	switch in.name {
	case "RUN", "COPY", "ADD":
	default:
		return true
	}
	var words []string
	if strings.HasPrefix(in.args, "[") && json.Unmarshal([]byte(in.args), &words) == nil {
		return true
	}
	for tail := in.args; tail != ""; {
		word, rest, ok := takeDockerWord(strings.TrimSpace(tail), '\\', true)
		if !ok {
			return false
		}
		tail = rest
		doc, found := dockerHeredoc(word)
		if !found {
			continue
		}
		terminated := false
		for p.scan() {
			line := p.scanner.Text()
			if doc.tab {
				line = strings.TrimLeft(line, "\t")
			}
			if line == doc.end {
				terminated = true
				break
			}
		}
		if !terminated {
			return false
		}
	}
	return true
}
