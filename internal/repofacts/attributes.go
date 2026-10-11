package repofacts

import (
	"errors"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// The attribute basis (RF-3): what a repository declares about its own
// files through .gitattributes' linguist-generated, linguist-vendored and
// linguist-documentation, root and nested, read with git's rules: a nested
// file's lines override its ancestors', a later line overrides an earlier
// one, `attr`/`attr=true` sets, `-attr`/`attr=false` clears and `!attr`
// unspecifies. A cleared attribute is a declaration too: the role it names
// is then refused at every weaker basis (a `-linguist-vendored` tree is not
// vendored by Linguist's data or the convention).

// attrState is one attribute's value for one path.
type attrState int8

const (
	attrUnset attrState = iota
	attrTrue
	attrFalse
)

// Attributes are the linguist attributes .gitattributes assigns one path.
// The zero value declares nothing.
type Attributes struct {
	Generated     attrState
	Vendored      attrState
	Documentation attrState
	// Unread names a .gitattributes on the path's chain that exists and
	// could not be read: the answer then omits its declarations (I38).
	Unread []string
}

// role is the attribute basis' answer: vendored, then generated, then
// documentation, the first set true.
func (a Attributes) role() (RoleName, bool) {
	switch {
	case a.Vendored == attrTrue:
		return Vendored, true
	case a.Generated == attrTrue:
		return Generated, true
	case a.Documentation == attrTrue:
		return Docs, true
	}
	return "", false
}

// allows reports whether role r survives the attributes: false only when
// the repository cleared the attribute that names r.
func (a Attributes) allows(r RoleName) bool {
	switch r {
	case Vendored:
		return a.Vendored != attrFalse
	case Generated:
		return a.Generated != attrFalse
	case Docs:
		return a.Documentation != attrFalse
	}
	return true
}

// attrLine is one .gitattributes line that names a linguist attribute.
type attrLine struct {
	re    *regexp.Regexp
	attrs map[string]attrState // "generated", "vendored", "documentation"
}

// ReadAttributes returns the linguist attributes the .gitattributes files
// on rel's ancestor chain in fsys (the repository root's first, the
// nearest last) assign rel. A missing file declares nothing; a file that
// exists and cannot be read is named in Unread.
func ReadAttributes(fsys fs.FS, rel string) Attributes {
	rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
	var out Attributes
	dirs := []string{""}
	segs := strings.Split(rel, "/")
	for i := 1; i < len(segs); i++ {
		dirs = append(dirs, strings.Join(segs[:i], "/"))
	}
	for _, dir := range dirs {
		name := path.Join(dir, ".gitattributes")
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				out.Unread = append(out.Unread, name)
			}
			continue
		}
		sub := rel
		if dir != "" {
			sub = strings.TrimPrefix(rel, dir+"/")
		}
		for _, l := range parseGitattributes(b) {
			if !l.re.MatchString(sub) {
				continue
			}
			for k, v := range l.attrs {
				switch k {
				case "generated":
					out.Generated = v
				case "vendored":
					out.Vendored = v
				case "documentation":
					out.Documentation = v
				}
			}
		}
	}
	return out
}

// parseGitattributes parses one .gitattributes file's lines that name a
// linguist-generated, -vendored or -documentation attribute, in file
// order. A line whose pattern git would not match against a file (a
// trailing-slash directory pattern, a negation) is skipped.
func parseGitattributes(b []byte) []attrLine {
	var out []attrLine
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		pat := fields[0]
		if strings.HasPrefix(pat, "!") || strings.HasSuffix(pat, "/") {
			continue
		}
		attrs := map[string]attrState{}
		for _, f := range fields[1:] {
			name, state, ok := linguistAttr(f)
			if ok {
				attrs[name] = state
			}
		}
		if len(attrs) == 0 {
			continue
		}
		re, err := regexp.Compile(wildmatchRegexp(pat))
		if err != nil {
			continue
		}
		out = append(out, attrLine{re: re, attrs: attrs})
	}
	return out
}

// linguistAttr reads one attribute token: linguist-x, linguist-x=true,
// -linguist-x, linguist-x=false, !linguist-x (unspecified).
func linguistAttr(tok string) (string, attrState, bool) {
	state := attrTrue
	switch {
	case strings.HasPrefix(tok, "-"):
		tok, state = tok[1:], attrFalse
	case strings.HasPrefix(tok, "!"):
		tok, state = tok[1:], attrUnset
	}
	name, val, hasVal := strings.Cut(tok, "=")
	if hasVal {
		switch strings.ToLower(val) {
		case "true", "1":
			state = attrTrue
		case "false", "0":
			state = attrFalse
		default:
			return "", 0, false
		}
	}
	switch name {
	case "linguist-generated":
		return "generated", state, true
	case "linguist-vendored":
		return "vendored", state, true
	case "linguist-documentation":
		return "documentation", state, true
	}
	return "", 0, false
}

// wildmatchRegexp translates a gitattributes pattern (gitignore's
// wildmatch rules) into an anchored regexp over a path relative to the
// .gitattributes file's directory: a pattern with no slash matches the
// base name at any depth, a pattern with one is anchored there; `*` and
// `?` never cross a slash, `**/`, `/**` and `/**/` cross any number.
func wildmatchRegexp(pat string) string {
	anchored := strings.Contains(strings.TrimSuffix(pat, "/"), "/")
	pat = strings.TrimPrefix(pat, "/")
	var b strings.Builder
	b.WriteString("^")
	if !anchored {
		b.WriteString("(?:.*/)?")
	}
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch {
		case strings.HasPrefix(pat[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(pat[i:], "/**") && i+3 == len(pat):
			b.WriteString("/.*")
			i += 2
		case strings.HasPrefix(pat[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '[':
			j := strings.IndexByte(pat[i+1:], ']')
			if j < 0 {
				b.WriteString(`\[`)
				continue
			}
			class := pat[i+1 : i+1+j]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			i += j + 1
		case c == '\\' && i+1 < len(pat):
			i++
			b.WriteString(regexp.QuoteMeta(string(pat[i])))
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String()
}
