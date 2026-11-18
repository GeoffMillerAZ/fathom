package boundaries

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type glob struct {
	ecosystem string
	parts     []*regexp.Regexp // a nil part is **
}

const classBody = `\[\^?[A-Za-z0-9._@+~ -]+\]`

var (
	pathPart     = regexp.MustCompile(`^([A-Za-z0-9._@+~ *?-]|` + classBody + `)+$`)
	externalPart = regexp.MustCompile(`^([A-Za-z0-9._+~*?-]|\[\^?[A-Za-z0-9._+~-]+\])+$`)
	registryPort = regexp.MustCompile(`^[0-9]+$`)
)

func compileGlobs(patterns []string, targets bool) ([]glob, error) {
	if len(patterns) == 0 {
		return nil, errors.New("expected at least one glob")
	}
	out := make([]glob, 0, len(patterns))
	for i, pattern := range patterns {
		g, ok := compileGlob(pattern, targets)
		if !ok {
			return nil, fmt.Errorf("invalid glob at position %d", i+1)
		}
		out = append(out, g)
	}
	return out, nil
}

func compileGlob(pattern string, targets bool) (glob, bool) {
	var g glob
	if len(pattern) > 512 {
		return g, false
	}
	partShape := pathPart
	if ecosystem, name, ok := strings.Cut(pattern, ":"); ok {
		if !targets || ecosystem == "repo" || !ruleID.MatchString(ecosystem) {
			return g, false
		}
		g.ecosystem, pattern, partShape = ecosystem, name, externalPart
	}
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if part == "**" {
			g.parts = append(g.parts, nil)
			continue
		}
		base, port, hasPort := strings.Cut(part, ":")
		if hasPort && (g.ecosystem == "" || i != 0 || len(parts) == 1 || !registryPort.MatchString(port)) {
			return g, false
		}
		if g.ecosystem != "" && i == 0 {
			base = strings.TrimPrefix(base, "@")
		}
		root := part == "." && targets && g.ecosystem == "" && len(parts) == 1
		if !partShape.MatchString(base) || (!root && (base == "." || base == "..")) || strings.Contains(base, "**") {
			return g, false
		}
		re, err := compilePart(part)
		if err != nil {
			return g, false
		}
		g.parts = append(g.parts, re)
	}
	return g, true
}

func compilePart(part string) (*regexp.Regexp, error) {
	var expr strings.Builder
	expr.WriteString("^")
	for len(part) != 0 {
		switch part[0] {
		case '*':
			expr.WriteString(".*")
		case '?':
			expr.WriteString(".")
		case '[':
			end := strings.IndexByte(part, ']') + 1
			expr.WriteString(part[:end])
			part = part[end:]
			continue
		default:
			expr.WriteString(regexp.QuoteMeta(part[:1]))
		}
		part = part[1:]
	}
	expr.WriteString("$")
	return regexp.Compile(expr.String())
}

func matches(globs []glob, ecosystem, name string) bool {
	parts := strings.Split(name, "/")
	return slices.ContainsFunc(globs, func(g glob) bool {
		return g.ecosystem == ecosystem && g.match(parts)
	})
}

// A whole ** component eats zero or more components. On a mismatch, retry
// after letting the last ** consume one more component.
func (g glob) match(name []string) bool {
	pat, pos, star, retry := 0, 0, -1, 0
	for pos < len(name) {
		switch {
		case pat < len(g.parts) && g.parts[pat] == nil:
			star, retry = pat, pos
			pat++
		case pat < len(g.parts) && g.parts[pat].MatchString(name[pos]):
			pat++
			pos++
		case star >= 0:
			retry++
			pos, pat = retry, star+1
		default:
			return false
		}
	}
	for pat < len(g.parts) && g.parts[pat] == nil {
		pat++
	}
	return pat == len(g.parts)
}
