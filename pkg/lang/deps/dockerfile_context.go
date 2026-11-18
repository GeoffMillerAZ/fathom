package deps

import (
	"bufio"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

const maxDockerContextVisits = 4096

type dockerPattern struct {
	expression *regexp.Regexp
	include    bool
}

var errDockerPattern = errors.New("unsupported Docker path pattern")

func compileDockerPattern(raw string) (dockerPattern, error) {
	raw = strings.TrimSpace(raw)
	pattern := dockerPattern{include: strings.HasPrefix(raw, "!")}
	if pattern.include {
		raw = strings.TrimSpace(raw[1:])
	}
	// Escaped characters and character classes may change directory
	// traversal. Disclose them instead of applying a partial ignore list.
	if raw == "" || strings.ContainsAny(raw, "\\[\x00") {
		return pattern, errDockerPattern
	}
	raw = strings.TrimPrefix(path.Clean(raw), "/")
	if raw == "." || raw == "" {
		return pattern, nil
	}
	var expr strings.Builder
	expr.WriteByte('^')
	for raw != "" {
		switch {
		case strings.HasPrefix(raw, "**/"):
			expr.WriteString("(?:.*/)?")
			raw = raw[3:]
		case strings.HasPrefix(raw, "**"):
			expr.WriteString(".*")
			raw = raw[2:]
		case raw[0] == '*':
			expr.WriteString("[^/]*")
			raw = raw[1:]
		case raw[0] == '?':
			expr.WriteString("[^/]")
			raw = raw[1:]
		default:
			n := strings.IndexAny(raw, "*?")
			if n < 0 {
				n = len(raw)
			}
			expr.WriteString(regexp.QuoteMeta(raw[:n]))
			raw = raw[n:]
		}
	}
	expr.WriteByte('$')
	re, err := regexp.Compile(expr.String())
	pattern.expression = re
	return pattern, err
}

func dockerIgnored(patterns []dockerPattern, name string) (ignored bool) {
	for _, pattern := range patterns {
		if pattern.expression == nil {
			continue
		}
		for candidate := name; candidate != "."; candidate = path.Dir(candidate) {
			if pattern.expression.MatchString(candidate) {
				ignored = !pattern.include
				break
			}
		}
	}
	return ignored
}

func (d *dockerRead) readIgnore() {
	specific := d.out.Path + ".dockerignore"
	if d.ignoreFile(specific) {
		return
	}
	if path.Dir(d.out.Path) == "." {
		d.ignoreFile(".dockerignore")
	}
}

// A Dockerfile-specific ignore file takes precedence, including when it is
// empty. An uninspectable specific file also prevents using the default.
func (d *dockerRead) ignoreFile(name string) bool {
	info, err := d.repo.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err == nil && info.Mode().IsRegular() {
		d.edge(ConfigRef, name, name, File, "", 1)
	}
	src, code := readSource(d.repo, name)
	if code != "" {
		d.unknown(code, name, 1, ConfigFile)
		d.blocked = true
		return true
	}
	scan := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(src), "\ufeff")))
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		pattern, err := compileDockerPattern(line)
		if err != nil || len(d.ignore) >= maxDockerContextVisits {
			code := UnsupportedConstruct
			if err == nil {
				code = ReadLimit
			}
			d.unknown(code, name, 1, ConfigFile)
			d.blocked = true
			return true
		}
		d.ignore = append(d.ignore, pattern)
	}
	if scan.Err() != nil {
		d.unknown(ReadLimit, name, 1, ConfigFile)
		d.blocked = true
	}
	return true
}

func (d *dockerRead) local(raw string, excludes []dockerPattern, line int) {
	if path.Dir(d.out.Path) != "." {
		d.unknown(MissingConfig, raw, line, ConfigFile, Declaration)
		return
	}
	if d.blocked {
		d.unknown(MissingConfig, raw, line, ConfigFile)
		return
	}
	if strings.ContainsAny(raw, "\\:\x00\n\r\t") {
		d.unknown(UnsupportedConstruct, "", line, Toolchain)
		return
	}
	if d.resolver != nil {
		d.out.resolveExtension(d.repo, d.resolver, raw, line)
		return
	}
	// BuildKit normalizes sources against the context root, even when they
	// begin with a slash or parent traversal.
	name := strings.TrimPrefix(path.Clean("/"+raw), "/")
	if name == "" {
		name = "."
	}
	start := len(d.out.Edges)
	visits := d.visits
	glob := strings.ContainsAny(name, "*?[")
	if glob {
		if _, err := path.Match(name, ""); err != nil {
			d.unknown(UnboundedGlob, raw, line, Toolchain)
			return
		}
		d.glob(".", strings.Split(name, "/"), raw, excludes, line)
	} else {
		d.includeSource(name, raw, excludes, line)
	}
	if d.visits > maxDockerContextVisits {
		d.out.Edges = d.out.Edges[:start]
		if visits <= maxDockerContextVisits {
			d.unknown(ReadLimit, raw, line, Observation)
		}
	} else if len(d.out.Edges) == start {
		code := MissingConfig
		if glob {
			code = UnboundedGlob
		}
		d.unknown(code, raw, line, Toolchain, ConfigFile)
	}
}

func (d *dockerRead) visit() bool {
	if d.ctx.Err() != nil || d.visits > maxDockerContextVisits {
		return false
	}
	d.visits++
	return d.visits <= maxDockerContextVisits
}

func (d *dockerRead) glob(dir string, parts []string, raw string, excludes []dockerPattern, line int) {
	if !d.visit() {
		return
	}
	part := parts[0]
	if !strings.ContainsAny(part, "*?[") {
		name := path.Join(dir, part)
		if len(parts) == 1 {
			d.includeSource(name, raw, excludes, line)
		} else {
			d.glob(name, parts[1:], raw, excludes, line)
		}
		return
	}
	entries, err := fs.ReadDir(d.repo, dir)
	if err != nil {
		d.unknown(UnsupportedConstruct, raw, line, Toolchain)
		return
	}
	for _, entry := range entries {
		if !d.visit() {
			return
		}
		matched, _ := path.Match(part, entry.Name())
		if !matched {
			continue
		}
		name := path.Join(dir, entry.Name())
		if len(parts) == 1 {
			d.includeSource(name, raw, excludes, line)
		} else if entry.IsDir() {
			d.glob(name, parts[1:], raw, excludes, line)
		} else if entry.Type()&fs.ModeSymlink != 0 && !dockerIgnored(d.ignore, name) {
			d.unknown(UnsupportedConstruct, raw, line, Toolchain)
		}
	}
}

func (d *dockerRead) includeSource(name, raw string, excludes []dockerPattern, line int) {
	if !d.visit() {
		return
	}
	info, err := d.repo.Stat(name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) && !dockerIgnored(d.ignore, name) {
			d.unknown(UnsupportedConstruct, raw, line, Toolchain)
		}
		return
	}
	if info.IsDir() {
		d.walkSource(name, name, raw, excludes, line)
	} else if !dockerIgnored(d.ignore, name) && !dockerIgnored(excludes, path.Base(name)) {
		d.edge(BuildInput, raw, name, File, "", line)
	}
}

// Local directories contribute surviving members. Excludes are relative to
// the copied source root, whereas dockerignore patterns use the context root.
func (d *dockerRead) walkSource(root, name, raw string, excludes []dockerPattern, line int) {
	if !d.visit() {
		return
	}
	entries, err := fs.ReadDir(d.repo, name)
	if err != nil {
		d.unknown(UnsupportedConstruct, raw, line, Toolchain)
		return
	}
	if len(entries) == 0 && !dockerIgnored(d.ignore, name) {
		rel := dockerSourceRelative(root, name)
		if rel == "" || !dockerIgnored(excludes, rel) {
			d.edge(BuildInput, raw, name, Directory, "", line)
		}
	}
	for _, entry := range entries {
		if !d.visit() {
			return
		}
		member := path.Join(name, entry.Name())
		rel := dockerSourceRelative(root, member)
		if entry.IsDir() && (validPath(member) || member == ".") {
			d.walkSource(root, member, raw, excludes, line)
			continue
		}
		if dockerIgnored(d.ignore, member) || dockerIgnored(excludes, rel) {
			continue
		}
		if entry.Type().IsRegular() {
			d.edge(BuildInput, raw, member, File, "", line)
		} else {
			d.unknown(UnsupportedConstruct, raw, line, Toolchain)
		}
	}
}

func dockerSourceRelative(root, name string) string {
	if root == name {
		return "."
	}
	if root == "." {
		return name
	}
	return strings.TrimPrefix(name, root+"/")
}
