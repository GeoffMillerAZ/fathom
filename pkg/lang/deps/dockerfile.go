package deps

import (
	"context"
	"encoding/csv"
	"regexp"
	"strconv"
	"strings"
)

type dockerRead struct {
	ctx      context.Context
	repo     repositoryFS
	out      *result
	resolver Resolver
	stages   int
	aliases  map[string]int
	ignore   []dockerPattern
	blocked  bool
	visits   int
}

var (
	dockerfileName = regexp.MustCompile(`(?i)^(?:(?:docker|container)file(?:\.[^/]*)?|[^/]+\.(?:docker|container)file)$`)
	dockerAlias    = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
	dockerDigest   = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	dockerImage    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(?::[0-9]+)?(?:/[a-z0-9][a-z0-9._-]*)*$`)
	dockerTag      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
)

func readDockerfileDependencies(ctx context.Context, repo repositoryFS, src []byte, resolver Resolver, out *result) {
	d := dockerRead{ctx: ctx, repo: repo, out: out, resolver: resolver, aliases: make(map[string]int)}
	d.readIgnore()
	parser := newDockerParser(src)
	for ctx.Err() == nil && parser.next() {
		in := parser.instruction
		if in.name == "FROM" {
			d.from(in)
			continue
		}
		if d.stages == 0 && in.name != "ARG" {
			d.unknown(ParseError, "", in.line, Toolchain)
		}
		switch in.name {
		case "COPY", "ADD":
			d.copy(in)
		case "RUN":
			d.run(in)
		case "ARG", "CMD", "ENTRYPOINT", "ENV", "EXPOSE", "HEALTHCHECK", "LABEL",
			"MAINTAINER", "SHELL", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR":
			if in.args == "" || len(in.flags) != 0 && in.name != "HEALTHCHECK" {
				d.unknown(ParseError, "", in.line, Toolchain)
			}
		default:
			// ONBUILD children have deferred instructions. Future instructions
			// might also introduce a new source kind.
			d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
		}
	}
	if parser.failed != 0 {
		d.unknown(ParseError, "", parser.failed, Toolchain)
	} else if d.stages == 0 {
		d.unknown(ParseError, "", 1, Toolchain)
	}
	if parser.frontend != "" {
		d.image(ConfigRef, parser.frontend, parser.frontendLine)
		// The explicit frontend can have a different instruction set.
		d.unknown(UnsupportedConstruct, "", parser.frontendLine, Toolchain)
	}
}

func (d *dockerRead) unknown(code UnknownCode, raw string, line int, by ...DecidableBy) {
	d.out.addUnknown(Unknown{Code: code, Raw: raw, Line: line, DecidableBy: by})
}

func (d *dockerRead) edge(kind Kind, raw, to string, res Resolution, version string, line int) {
	d.out.addEdge(Edge{Kind: kind, Raw: raw, To: to, Resolution: res, Version: version, Line: line, Provenance: Builtin})
}

func (d *dockerRead) from(in dockerInstruction) {
	words := strings.Fields(in.args)
	if len(words) != 1 && (len(words) != 3 || !strings.EqualFold(words[1], "as")) {
		d.unknown(ParseError, "", in.line, Toolchain)
		return
	}
	name := ""
	if len(words) == 3 {
		name = strings.ToLower(words[2])
		if !dockerAlias.MatchString(name) || name == "scratch" || d.stageNamed(name) {
			d.unknown(ParseError, "", in.line, Toolchain)
			return
		}
	}
	for _, flag := range in.flags {
		if flag.key != "platform" || flag.value == "" {
			d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
		} else if strings.Contains(flag.value, "$") {
			d.unknown(ComputedPath, flag.value, in.line, ConfigFile)
		} else if !strings.HasPrefix(flag.value, "linux/") {
			d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
			d.blocked = true
		}
	}
	base := words[0]
	if strings.Contains(base, "$") {
		d.unknown(ComputedPath, base, in.line, ConfigFile)
	} else if base != "scratch" && !d.stageNamed(base) {
		d.image(BuildInput, base, in.line)
		// An image's inherited ONBUILD commands and a named context override
		// cannot be read from the Dockerfile bytes.
		d.unknown(UnsupportedConstruct, "", in.line, Toolchain, ConfigFile)
	}
	if name != "" {
		d.aliases[name] = d.stages
	}
	d.stages++
}

func (d *dockerRead) stageNamed(name string) bool {
	_, ok := d.aliases[strings.ToLower(name)]
	return ok
}

func (d *dockerRead) image(kind Kind, raw string, line int) {
	if strings.Contains(raw, "$") {
		d.unknown(ComputedPath, raw, line, ConfigFile)
		return
	}
	name, digest, hasDigest := strings.Cut(raw, "@")
	if hasDigest && !dockerDigest.MatchString(digest) {
		d.unknown(UnsupportedConstruct, "", line, Toolchain)
		return
	}
	if colon := strings.LastIndexByte(name, ':'); colon > strings.LastIndexByte(name, '/') {
		if !dockerTag.MatchString(name[colon+1:]) {
			d.unknown(UnsupportedConstruct, "", line, Toolchain)
			return
		}
		name = name[:colon]
	}
	if !dockerImage.MatchString(name) {
		d.unknown(UnsupportedConstruct, "", line, Toolchain)
		return
	}
	d.edge(kind, raw, "docker:"+name, External, digest, line)
}

func (d *dockerRead) otherSource(name string, line int) {
	if strings.Contains(name, "$") {
		d.unknown(ComputedPath, name, line, ConfigFile)
		return
	}
	if n, err := strconv.Atoi(name); err == nil {
		if n < 0 || n >= d.stages-1 {
			d.unknown(UnresolvedAlias, name, line, Toolchain)
		}
		return
	}
	if i, ok := d.aliases[strings.ToLower(name)]; ok {
		if i == d.stages-1 {
			d.unknown(UnresolvedAlias, name, line, Toolchain)
		}
		return
	}
	// A name outside the already known stages may be a forward stage, an
	// image or a named context. Only a pinned image spelling merits an edge.
	if strings.Contains(name, "@") {
		d.image(BuildInput, name, line)
	}
	d.unknown(UnresolvedAlias, name, line, ConfigFile, Toolchain)
}

func (d *dockerRead) copy(in dockerInstruction) {
	sources, ok := dockerCopyWords(in.args, in.escape)
	if !ok || len(sources) < 2 {
		d.unknown(ParseError, "", in.line, Toolchain)
		return
	}
	var from string
	var excludes []dockerPattern
	for _, flag := range in.flags {
		switch flag.key {
		case "chown", "chmod", "link":
		case "keep-git-dir", "checksum", "unpack":
			if in.name != "ADD" {
				d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
			}
		case "from":
			if in.name != "COPY" || flag.value == "" || from != "" {
				d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
				return
			}
			from = flag.value
		case "exclude":
			if strings.Contains(flag.value, "$") {
				d.unknown(ComputedPath, flag.value, in.line, ConfigFile)
				return
			}
			pattern, err := compileDockerPattern(flag.value)
			if err != nil {
				d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
				return
			}
			excludes = append(excludes, pattern)
		default:
			// --parents changes how source paths and exclusions are rooted.
			d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
			return
		}
	}
	if _, ok := dockerHeredoc(sources[len(sources)-1]); ok {
		d.unknown(ParseError, "", in.line, Toolchain)
		return
	}
	if from != "" {
		d.otherSource(from, in.line)
	}
	for _, source := range sources[:len(sources)-1] {
		if _, ok := dockerHeredoc(source); ok {
			continue
		}
		if strings.Contains(source, "$") {
			d.unknown(ComputedPath, source, in.line, ConfigFile)
		} else if from == "" {
			if in.name == "ADD" && dockerRemote(source) {
				// Suppress URL userinfo, host and payload. Native ADD can also
				// unpack content whose dependencies are opaque to this view.
				d.unknown(ExternalUnpinned, "", in.line, Lockfile, ConfigFile)
			} else {
				d.local(source, excludes, in.line)
			}
		}
	}
}

func dockerRemote(source string) bool {
	return strings.Contains(source, "://") || strings.Contains(source, "@") && strings.Contains(source, ":")
}

func (d *dockerRead) run(in dockerInstruction) {
	for _, flag := range in.flags {
		switch flag.key {
		case "mount":
			d.mount(flag.value, in.line)
		case "network", "security":
		default:
			d.unknown(UnsupportedConstruct, "", in.line, Toolchain)
		}
	}
	if in.args == "" {
		d.unknown(ParseError, "", in.line, Toolchain)
	}
}

func (d *dockerRead) mount(raw string, line int) {
	if strings.Contains(raw, "$") {
		d.unknown(ComputedPath, raw, line, ConfigFile)
		return
	}
	fields, err := csv.NewReader(strings.NewReader(raw)).Read()
	if err != nil {
		d.unknown(ParseError, "", line, Toolchain)
		return
	}
	kind, source, from := "bind", ".", ""
	for _, field := range fields {
		key, value, hasValue := strings.Cut(field, "=")
		switch strings.ToLower(key) {
		case "type":
			kind = strings.ToLower(value)
		case "source", "src":
			if hasValue && value != "" {
				source = value
			}
		case "from":
			from = value
		case "target", "dst", "destination", "readonly", "ro", "readwrite", "rw", "required",
			"id", "sharing", "size", "mode", "uid", "gid", "env":
		default:
			d.unknown(UnsupportedConstruct, "", line, Toolchain)
		}
	}
	switch kind {
	case "bind":
		if from != "" {
			d.otherSource(from, line)
		} else if strings.ContainsAny(source, "*?[") {
			d.unknown(UnboundedGlob, source, line, Toolchain)
		} else {
			d.local(source, nil, line)
		}
	case "cache", "tmpfs":
		if from != "" {
			d.otherSource(from, line)
		}
	case "secret", "ssh":
		d.unknown(UnsupportedConstruct, "", line, Declaration, Observation)
	default:
		d.unknown(UnsupportedConstruct, "", line, Toolchain)
	}
}
