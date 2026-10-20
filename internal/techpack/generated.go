package techpack

import (
	"regexp"
	"strings"
)

// Match reports whether the repository-relative slash path p is this
// declaration's output, and the declared sources it is generated from.
// The output is a glob (`*` within a segment, `**` across segments, `?` one
// character) with two placeholders: `{dir}` binds a directory (the root
// binds "."; a leading `{dir}/` may match nothing) and `{name}` a part of
// one segment. Each placeholder binds once and then fills the sources
// (docs/design/fathom-dir.md section 7.1). A placeholder bound to two
// different values by one output is no match.
func (g Generated) Match(p string) (sources []string, ok bool) {
	re, names := compileGlob(g.Output, true)
	if re == nil {
		return nil, false
	}
	m := re.FindStringSubmatch(p)
	if m == nil {
		return nil, false
	}
	bound := map[string]string{}
	for i, name := range names {
		if name == "" {
			continue
		}
		v := m[i+1]
		if name == "dir" && v == "" {
			v = "."
		}
		if prev, seen := bound[name]; seen && prev != v {
			return nil, false
		}
		bound[name] = v
	}
	out := make([]string, 0, len(g.Source))
	for _, s := range g.Source {
		if dir, has := bound["dir"]; has {
			if dir == "." {
				s = strings.ReplaceAll(s, "{dir}/", "")
			}
			s = strings.ReplaceAll(s, "{dir}", dir)
		}
		if name, has := bound["name"]; has {
			s = strings.ReplaceAll(s, "{name}", name)
		}
		out = append(out, s)
	}
	return out, true
}

// IsGlob reports whether a filled source is a pattern rather than a path.
func IsGlob(s string) bool { return strings.ContainsAny(s, "*?") }

// GlobMatch reports whether the slash path p matches pattern (the output
// grammar of Match, without placeholders).
func GlobMatch(pattern, p string) bool {
	re, _ := compileGlob(pattern, false)
	return re != nil && re.MatchString(p)
}

// compileGlob turns a pattern into an anchored regexp. names lists, per
// capture group, the placeholder it binds ("" for none). With placeholders
// false, `{` is literal.
func compileGlob(pattern string, placeholders bool) (*regexp.Regexp, []string) {
	var b strings.Builder
	var names []string
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		rest := pattern[i:]
		switch {
		case placeholders && strings.HasPrefix(rest, "{dir}/") && i == 0:
			b.WriteString("(?:(.+)/)?")
			names = append(names, "dir")
			i += len("{dir}/")
		case placeholders && strings.HasPrefix(rest, "{dir}"):
			b.WriteString("(.+)")
			names = append(names, "dir")
			i += len("{dir}")
		case placeholders && strings.HasPrefix(rest, "{name}"):
			b.WriteString("([^/]+)")
			names = append(names, "name")
			i += len("{name}")
		case strings.HasPrefix(rest, "**/"):
			b.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(rest, "**"):
			b.WriteString(".*")
			i += 2
		case rest[0] == '*':
			b.WriteString("[^/]*")
			i++
		case rest[0] == '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(rest[:1]))
			i++
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, nil
	}
	return re, names
}
