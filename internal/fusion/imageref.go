package fusion

// imageref.go — LB-X-docker-image-ref (owner decision D7, docs/design/
// language-breadth.md §6): a Dockerfile's FROM <image> and COPY
// --from=<image>, a compose service's image: and a Kubernetes container's
// image: resolved to the in-repo Dockerfile that builds that image.
//
// The image-to-Dockerfile evidence lives in other formats, so the join is an
// image-tag index built in memory per doc-xref pass (no table, no cache) from
// three build sites: a compose service carrying both image: and build:, a
// docker-bake target (tags, context, dockerfile), and a literal `docker build`
// or `docker buildx build` with -t in a shell script, a Makefile recipe or a CI
// run step. A tag is joined only as written: one holding an unexpanded ${X},
// $(X) or $X is never joined (counted in TemplatedBuilds), and a tag two build
// sites build from two Dockerfiles is ambiguous and joins nothing (counted in
// Ambiguous). A Dockerfile reaches the resolver as a DocFile synthesized from
// the dispatched dockerfile tags dialect's own nodes (DockerfileDocFile), so
// both ends of an edge are symbol keys the store holds.

import (
	"bytes"
	"encoding/json"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ImageBuild is one build site: Tag (as written) is built from the first of
// Dockerfiles (repo-relative candidates, in preference order: a shell build's
// working directory is not known, so its paths are tried from the script's
// directory and from the repository root) that is a Dockerfile of the set.
// Templated: the tag, the context or the Dockerfile holds a variable, so the
// site builds no tag the index can name.
type ImageBuild struct {
	Tag         string
	Dockerfiles []string
	Line        int
	Templated   bool
}

// Edges a compose-shaped doc dialect writes for a build (the docker-compose
// dialect's doc.json): imageEdge is an image: reference (strategy image-ref),
// buildEdge a build context (build:, or build.context), and
// buildDockerfileEdge the Dockerfile named relative to that context
// (build.dockerfile); both build edges are strategy repo-path.
const (
	imageEdge           = "runs"
	buildEdge           = "build"
	buildDockerfileEdge = "build_dockerfile"
)

// NormalizeImageRef is an image reference as Docker reads it: docker.io/
// library/ (and index.docker.io/, and a bare library/) and docker.io/
// dropped, a missing tag read as :latest, a digest kept. ok is false for an
// empty reference, an empty tag, or one holding an unexpanded variable ($).
func NormalizeImageRef(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "$") || strings.ContainsAny(ref, " \t") {
		return "", false
	}
	name, digest := ref, ""
	if i := strings.IndexByte(ref, '@'); i >= 0 {
		name, digest = ref[:i], ref[i:]
	}
	for _, p := range []string{"docker.io/library/", "index.docker.io/library/", "registry-1.docker.io/library/", "docker.io/", "index.docker.io/", "registry-1.docker.io/"} {
		if strings.HasPrefix(name, p) {
			name = strings.TrimPrefix(name, p)
			break
		}
	}
	name = strings.TrimPrefix(name, "library/")
	last := name[strings.LastIndexByte(name, '/')+1:]
	if i := strings.IndexByte(last, ':'); i >= 0 {
		if i == len(last)-1 {
			return "", false
		}
	} else if digest == "" {
		name += ":latest"
	}
	if name == "" || strings.HasPrefix(name, ":") {
		return "", false
	}
	return name + digest, true
}

// templated: a value holding an unexpanded variable (${X}, $(X), $X, a CI
// expression ${{ }}): never one value, so never joined.
func templated(s string) bool { return strings.Contains(s, "$") }

// ImageIndex maps a normalized image tag to the Dockerfile that builds it.
type ImageIndex struct {
	byTag map[string]map[string]bool
	// Sites is every build site read; TemplatedBuilds the ones whose tag
	// (or context or Dockerfile) holds a variable, skipped; Ambiguous the
	// tags two Dockerfiles build, which join nothing.
	Sites, TemplatedBuilds, Ambiguous int
}

// BuildImageIndex reads every build site of the set — each file's Builds,
// and the compose-shaped image: + build: pairs in its CrossFile — and keeps
// the ones whose Dockerfile is a file of the set with a root entity.
func BuildImageIndex(files []DocFile) *ImageIndex {
	known := map[string]bool{}
	for _, f := range files {
		if len(f.Result.Roots) > 0 {
			known[f.Path] = true
		}
	}
	idx := &ImageIndex{byTag: map[string]map[string]bool{}}
	for _, f := range files {
		sites := append(composeBuilds(f), f.Builds...)
		for _, b := range sites {
			idx.Sites++
			if b.Templated || templated(b.Tag) {
				idx.TemplatedBuilds++
				continue
			}
			tag, ok := NormalizeImageRef(b.Tag)
			if !ok {
				continue
			}
			for _, d := range b.Dockerfiles {
				if known[d] {
					if idx.byTag[tag] == nil {
						idx.byTag[tag] = map[string]bool{}
					}
					idx.byTag[tag][d] = true
					break
				}
			}
		}
	}
	for _, ds := range idx.byTag {
		if len(ds) > 1 {
			idx.Ambiguous++
		}
	}
	return idx
}

// Lookup is the one Dockerfile that builds ref; false when none or several
// do, or ref is templated.
func (x *ImageIndex) Lookup(ref string) (string, bool) {
	tag, ok := NormalizeImageRef(ref)
	if !ok {
		return "", false
	}
	ds := x.byTag[tag]
	if len(ds) != 1 {
		return "", false
	}
	for d := range ds {
		return d, true
	}
	return "", false
}

// composeBuild is one origin's build: its context as written (hasCtx false:
// none written, the referring file's directory) and its Dockerfile.
type composeBuild struct {
	ctx, dockerfile string
	hasCtx, has     bool
	line            int
}

// composeBuildsOf groups a file's build and build_dockerfile references by
// their origin entity.
func composeBuildsOf(f DocFile) map[string]*composeBuild {
	out := map[string]*composeBuild{}
	for _, c := range f.Result.CrossFile {
		if c.Strategy != "repo-path" || (c.Edge != buildEdge && c.Edge != buildDockerfileEdge) {
			continue
		}
		b := out[c.From]
		if b == nil {
			b = &composeBuild{line: c.Line}
			out[c.From] = b
		}
		b.has = true
		if c.Edge == buildEdge {
			b.ctx, b.hasCtx = c.Raw, true
		} else {
			b.dockerfile = c.Raw
		}
	}
	return out
}

// file is the Dockerfile a build names, repo-relative (Docker's rule: the
// context relative to the referring file, the Dockerfile relative to the
// context, "Dockerfile" when none is named); false for a remote or
// templated context or one leaving the repository.
func (b composeBuild) file(referrer string) (string, bool) {
	ctx := "."
	if b.hasCtx {
		ctx = b.ctx
	}
	df := b.dockerfile
	if df == "" {
		df = "Dockerfile"
	}
	if templated(ctx+df) || strings.Contains(ctx, "://") || strings.HasPrefix(ctx, "git@") || path.IsAbs(df) {
		return "", false
	}
	return inRepo(path.Join(path.Dir(referrer), ctx, df))
}

// inRepo cleans a repo-relative path; false when it leaves the repository.
func inRepo(p string) (string, bool) {
	p = path.Clean(strings.TrimPrefix(p, "/"))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// composeBuilds is a compose-shaped file's build sites: every origin carrying
// both an image: reference and a build.
func composeBuilds(f DocFile) []ImageBuild {
	builds := composeBuildsOf(f)
	var out []ImageBuild
	for _, c := range f.Result.CrossFile {
		if c.Strategy != "image-ref" || c.Edge != imageEdge {
			continue
		}
		b := builds[c.From]
		if b == nil {
			continue
		}
		site := ImageBuild{Tag: c.Raw, Line: c.Line, Templated: templated(b.ctx + b.dockerfile)}
		if df, ok := b.file(f.Path); ok {
			site.Dockerfiles = []string{df}
		}
		out = append(out, site)
	}
	return out
}

// ── Dockerfiles ──────────────────────────────────────────────────────────────

// DockerfileDocFile is a Dockerfile as the doc-xref resolver reads it,
// synthesized from the dockerfile tags dialect's nodes for the file (the
// nodes ingest stores): its stages are the entities, its last stage the root
// (the image `docker build` produces), and each FROM and COPY --from naming
// an image — not an earlier stage, not a stage index, not scratch — an
// image-ref cross-file reference from the stage it is written in.
func DockerfileDocFile(relPath string, src []byte, nodes []core.SymbolNode) DocFile {
	var stages []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindDockerStage {
			stages = append(stages, n)
		}
	}
	sort.SliceStable(stages, func(i, j int) bool { return stages[i].Line < stages[j].Line })
	df := DocFile{Path: relPath, Result: extractor.DocResult{Nodes: stages}}
	if len(stages) == 0 {
		return df
	}
	df.Result.Roots = []string{stages[len(stages)-1].Name}
	stageAt := func(line int) (core.SymbolNode, bool) {
		var hit core.SymbolNode
		ok := false
		for _, s := range stages {
			if s.Line <= line {
				hit, ok = s, true
			}
		}
		return hit, ok
	}
	aliases := map[string]bool{}
	for _, in := range dockerfileInstructions(src) {
		switch in.cmd {
		case "FROM":
			image, alias := fromImage(in.args)
			s, ok := stageAt(in.line)
			if ok && s.Line == in.line && image != "" && !aliases[strings.ToLower(image)] && !strings.EqualFold(image, "scratch") {
				df.Result.CrossFile = append(df.Result.CrossFile, extractor.DocCrossFileRef{From: s.Name, Edge: "from", Raw: image, Target: image, Strategy: "image-ref", Line: in.line})
			}
			if alias != "" {
				aliases[strings.ToLower(alias)] = true
			}
		case "COPY", "ADD":
			from := copyFrom(in.args)
			if from == "" || aliases[strings.ToLower(from)] {
				continue
			}
			if _, err := strconv.Atoi(from); err == nil {
				continue
			}
			if s, ok := stageAt(in.line); ok {
				df.Result.CrossFile = append(df.Result.CrossFile, extractor.DocCrossFileRef{From: s.Name, Edge: "copy_from", Raw: from, Target: from, Strategy: "image-ref", Line: in.line})
			}
		}
	}
	return df
}

// fromImage is a FROM instruction's image and alias (flags skipped).
func fromImage(args []string) (image, alias string) {
	var words []string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			words = append(words, a)
		}
	}
	if len(words) == 0 {
		return "", ""
	}
	if len(words) >= 3 && strings.EqualFold(words[1], "as") {
		alias = words[2]
	}
	return words[0], alias
}

// copyFrom is a COPY's --from value ("" when it has none).
func copyFrom(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "--from=") {
			return strings.Trim(strings.TrimPrefix(a, "--from="), `"'`)
		}
		if !strings.HasPrefix(a, "--") {
			break
		}
	}
	return ""
}

// dockerInstr is one Dockerfile instruction: its first line, its upper-cased
// command and its whitespace-split arguments.
type dockerInstr struct {
	line int
	cmd  string
	args []string
}

// dockerfileInstructions splits a Dockerfile into instructions the way
// BuildKit's parser does for what FROM and COPY need: the escape directive
// (`# escape=` before the first instruction) or `\` continues a line, a
// comment line inside a continuation is dropped, and a heredoc body
// (`<<EOF` … `EOF`) is skipped.
func dockerfileInstructions(src []byte) []dockerInstr {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	escape := "\\"
	var out []dockerInstr
	var cur strings.Builder
	start := 0
	directives := true
	var heredocs []string
	for i, raw := range lines {
		ln := i + 1
		if len(heredocs) > 0 {
			if strings.TrimSpace(raw) == heredocs[0] {
				heredocs = heredocs[1:]
			}
			continue
		}
		t := strings.TrimSpace(raw)
		if cur.Len() == 0 {
			if t == "" {
				continue
			}
			if strings.HasPrefix(t, "#") {
				if directives {
					if d := strings.TrimSpace(strings.TrimPrefix(t, "#")); strings.HasPrefix(strings.ToLower(d), "escape=") {
						if e := strings.TrimSpace(d[len("escape="):]); e == "`" || e == "\\" {
							escape = e
						}
					}
				}
				continue
			}
			directives = false
			start = ln
		} else if strings.HasPrefix(t, "#") || t == "" {
			continue
		}
		if strings.HasSuffix(t, escape) {
			cur.WriteString(strings.TrimSuffix(t, escape))
			cur.WriteByte(' ')
			continue
		}
		cur.WriteString(t)
		f := strings.Fields(cur.String())
		cur.Reset()
		if len(f) == 0 {
			continue
		}
		in := dockerInstr{line: start, cmd: strings.ToUpper(f[0]), args: f[1:]}
		out = append(out, in)
		for _, a := range in.args {
			if strings.HasPrefix(a, "<<") {
				w := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(a, "<<"), "-"), `"'`)
				if w != "" {
					heredocs = append(heredocs, w)
				}
			}
		}
	}
	return out
}

// ── docker-bake ──────────────────────────────────────────────────────────────

// isBakeFile: a docker-bake file buildx reads by default (docker-bake.hcl,
// docker-bake.json and their .override forms) or another docker-bake*.hcl/json.
func isBakeFile(p string) bool {
	b := path.Base(p)
	return strings.HasPrefix(b, "docker-bake") && (strings.HasSuffix(b, ".hcl") || strings.HasSuffix(b, ".json"))
}

// bakeTarget is one target's build attributes as written.
type bakeTarget struct {
	context, dockerfile       string
	hasCtx, hasDockerfile     bool
	tags                      []string
	tagLines                  []int
	templatedTags             int
	inherits                  []string
	line                      int
	templatedCtx, templatedDF bool
}

// BakeBuilds is a docker-bake file's build sites: every target's tags, built
// from its context (relative to the bake file) and its dockerfile (relative
// to the context), each inherited from the targets `inherits` names when the
// target writes none. A value that is not a literal (a variable, a function)
// is templated.
func BakeBuilds(relPath string, src []byte) []ImageBuild {
	var targets map[string]*bakeTarget
	if strings.HasSuffix(relPath, ".json") {
		targets = bakeJSON(src)
	} else {
		targets = bakeHCL(relPath, src)
	}
	names := make([]string, 0, len(targets))
	for n := range targets {
		names = append(names, n)
	}
	sort.Strings(names)
	var resolve func(name string, seen map[string]bool) (ctx, df string, hasCtx, hasDF, bad bool)
	resolve = func(name string, seen map[string]bool) (string, string, bool, bool, bool) {
		t := targets[name]
		if t == nil || seen[name] {
			return "", "", false, false, false
		}
		seen[name] = true
		ctx, df, hasCtx, hasDF, bad := t.context, t.dockerfile, t.hasCtx, t.hasDockerfile, t.templatedCtx || t.templatedDF
		for _, parent := range t.inherits {
			pc, pd, phc, phd, pb := resolve(parent, seen)
			if !hasCtx && phc {
				ctx, hasCtx = pc, true
				bad = bad || pb
			}
			if !hasDF && phd {
				df, hasDF = pd, true
				bad = bad || pb
			}
		}
		return ctx, df, hasCtx, hasDF, bad
	}
	var out []ImageBuild
	for _, n := range names {
		t := targets[n]
		ctx, df, hasCtx, _, bad := resolve(n, map[string]bool{})
		b := composeBuild{ctx: ctx, dockerfile: df, hasCtx: hasCtx}
		bad = bad || templated(ctx+df)
		var dfs []string
		if f, ok := b.file(relPath); ok && !bad {
			dfs = []string{f}
		}
		for i, tag := range t.tags {
			out = append(out, ImageBuild{Tag: tag, Dockerfiles: dfs, Line: t.tagLines[i], Templated: bad})
		}
		for i := 0; i < t.templatedTags; i++ {
			out = append(out, ImageBuild{Line: t.line, Templated: true})
		}
	}
	return out
}

// bakeHCL reads the target blocks of an HCL bake file; a file that does not
// parse has none.
func bakeHCL(relPath string, src []byte) map[string]*bakeTarget {
	f, diags := hclsyntax.ParseConfig(src, relPath, hcl.InitialPos)
	if diags.HasErrors() || f == nil {
		return nil
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	literal := func(e hclsyntax.Expression) (string, bool) {
		v, d := e.Value(nil)
		if d.HasErrors() || !v.IsKnown() || v.IsNull() || v.Type() != cty.String {
			return "", false
		}
		return v.AsString(), true
	}
	out := map[string]*bakeTarget{}
	for _, b := range body.Blocks {
		if b.Type != "target" || len(b.Labels) != 1 {
			continue
		}
		t := &bakeTarget{line: b.TypeRange.Start.Line}
		for name, a := range b.Body.Attributes {
			switch name {
			case "context":
				t.hasCtx = true
				if s, ok := literal(a.Expr); ok {
					t.context = s
				} else {
					t.templatedCtx = true
				}
			case "dockerfile":
				t.hasDockerfile = true
				if s, ok := literal(a.Expr); ok {
					t.dockerfile = s
				} else {
					t.templatedDF = true
				}
			case "tags":
				for _, e := range tupleItems(a.Expr) {
					if s, ok := literal(e); ok && !templated(s) {
						t.tags = append(t.tags, s)
						t.tagLines = append(t.tagLines, e.Range().Start.Line)
					} else {
						t.templatedTags++
					}
				}
			case "inherits":
				for _, e := range tupleItems(a.Expr) {
					if s, ok := literal(e); ok {
						t.inherits = append(t.inherits, s)
					}
				}
			}
		}
		out[b.Labels[0]] = t
	}
	return out
}

// tupleItems is a list expression's elements (one element for anything else).
func tupleItems(e hclsyntax.Expression) []hclsyntax.Expression {
	if tc, ok := e.(*hclsyntax.TupleConsExpr); ok {
		return tc.Exprs
	}
	return []hclsyntax.Expression{e}
}

// bakeJSON reads the "target" object of a JSON bake file.
func bakeJSON(src []byte) map[string]*bakeTarget {
	var doc struct {
		Target map[string]struct {
			Context    *string  `json:"context"`
			Dockerfile *string  `json:"dockerfile"`
			Tags       []string `json:"tags"`
			Inherits   []string `json:"inherits"`
		} `json:"target"`
	}
	if err := json.Unmarshal(src, &doc); err != nil {
		return nil
	}
	out := map[string]*bakeTarget{}
	for name, tj := range doc.Target {
		t := &bakeTarget{inherits: tj.Inherits, line: lineOf(src, `"`+name+`"`)}
		if tj.Context != nil {
			t.context, t.hasCtx = *tj.Context, true
			t.templatedCtx = templated(*tj.Context)
		}
		if tj.Dockerfile != nil {
			t.dockerfile, t.hasDockerfile = *tj.Dockerfile, true
			t.templatedDF = templated(*tj.Dockerfile)
		}
		for _, tag := range tj.Tags {
			if templated(tag) {
				t.templatedTags++
				continue
			}
			t.tags = append(t.tags, tag)
			t.tagLines = append(t.tagLines, lineOf(src, `"`+tag+`"`))
		}
		out[name] = t
	}
	return out
}

// lineOf is the 1-based line of needle's first occurrence (1 when absent).
func lineOf(src []byte, needle string) int {
	i := bytes.Index(src, []byte(needle))
	if i < 0 {
		return 1
	}
	return bytes.Count(src[:i], []byte("\n")) + 1
}

// ── shell, Makefile and CI run steps ─────────────────────────────────────────

// isShellBuildSource: a file a literal `docker build` is read from — a shell
// script, a Makefile, or a CI pipeline's YAML (GitHub Actions workflows,
// GitLab CI, CircleCI, Azure Pipelines, Bitbucket, Drone, Buildkite).
func isShellBuildSource(p string) bool {
	b := path.Base(p)
	switch b {
	case "Makefile", "makefile", "GNUmakefile", ".gitlab-ci.yml", "bitbucket-pipelines.yml", ".drone.yml", "azure-pipelines.yml", "azure-pipelines.yaml":
		return true
	}
	switch strings.ToLower(path.Ext(b)) {
	case ".sh", ".bash", ".mk":
		return true
	case ".yml", ".yaml":
		return ciRunDir(p)
	}
	return false
}

// ciRunDir: a YAML file under a CI system's pipeline directory, whose run
// steps execute from the repository root.
func ciRunDir(p string) bool {
	for _, d := range []string{".github/workflows/", ".gitlab/", ".circleci/", ".buildkite/", ".azure-pipelines/"} {
		if strings.HasPrefix(p, d) || strings.Contains(p, "/"+d) {
			return true
		}
	}
	b := path.Base(p)
	return strings.HasSuffix(b, ".gitlab-ci.yml") || strings.HasPrefix(b, "azure-pipelines")
}

// dockerBoolFlags are the build flags that take no value; every other flag
// written without = takes the next word.
var dockerBoolFlags = map[string]bool{
	"--push": true, "--load": true, "--no-cache": true, "--pull": true, "-q": true, "--quiet": true,
	"--rm": true, "--force-rm": true, "--squash": true, "--compress": true, "--disable-content-trust": true,
	"--check": true, "--debug": true,
}

// ShellBuilds is the build sites written as literal `docker build`, `docker
// image build` or `docker buildx build` commands: each -t/--tag, built from
// -f/--file (relative to the working directory) or the context's Dockerfile.
// The working directory is the file's own for a script or a Makefile, tried
// before the repository root, and the repository root for a CI run step,
// tried before the file's directory.
func ShellBuilds(relPath string, src []byte) []ImageBuild {
	dir := path.Dir(relPath)
	bases := []string{dir, "."}
	if ciRunDir(relPath) {
		bases = []string{".", dir}
	}
	var out []ImageBuild
	for _, cmd := range shellCommands(src) {
		tags, file, ctx, ok := dockerBuildArgs(cmd.words)
		if !ok {
			continue
		}
		var dfs []string
		bad := templated(ctx + file)
		seen := map[string]bool{}
		for _, base := range bases {
			var p string
			if file != "" {
				p = path.Join(base, file)
			} else {
				p = path.Join(base, ctx, "Dockerfile")
			}
			if c, ok := inRepo(p); ok && !seen[c] && !bad {
				seen[c] = true
				dfs = append(dfs, c)
			}
		}
		for _, t := range tags {
			out = append(out, ImageBuild{Tag: t, Line: cmd.line, Dockerfiles: dfs, Templated: bad})
		}
	}
	return out
}

// dockerBuildArgs reads one command: its tags, -f file and context, when it is
// a docker build with a context that is a local directory.
func dockerBuildArgs(w []string) (tags []string, file, ctx string, ok bool) {
	// the command word: past a YAML list dash or key (`- run:`), Make's
	// recipe prefixes (@, -, +), sudo/exec/time, a shell keyword and an
	// environment assignment
	i := 0
	for i < len(w) {
		x := strings.TrimLeft(w[i], "@-+")
		if x == "" || strings.HasSuffix(x, ":") || shellPrefixWords[x] || strings.Contains(x, "=") {
			i++
			continue
		}
		break
	}
	if i >= len(w) || path.Base(strings.TrimLeft(w[i], "@-+")) != "docker" {
		return nil, "", "", false
	}
	i++
	switch {
	case i+1 < len(w) && w[i] == "buildx" && w[i+1] == "build", i+1 < len(w) && w[i] == "image" && w[i+1] == "build":
		i += 2
	case i < len(w) && w[i] == "build":
		i++
	default:
		return nil, "", "", false
	}
	for ; i < len(w); i++ {
		a := w[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			if ctx == "" {
				ctx = a
			}
			continue
		}
		name, val, hasVal := strings.Cut(a, "=")
		if !hasVal && !dockerBoolFlags[name] {
			if i+1 < len(w) {
				val = w[i+1]
				i++
			}
		}
		switch name {
		case "-t", "--tag":
			tags = append(tags, val)
		case "-f", "--file":
			file = val
		}
	}
	if ctx == "" || ctx == "-" || strings.Contains(ctx, "://") || strings.HasPrefix(ctx, "git@") || file == "-" || len(tags) == 0 {
		return nil, "", "", false
	}
	return tags, file, ctx, true
}

// shellPrefixWords precede a command without being it.
var shellPrefixWords = map[string]bool{"sudo": true, "exec": true, "time": true, "then": true, "else": true, "do": true, "!": true, "command": true, "nohup": true}

// shellCommand is one simple command's words and its first line.
type shellCommand struct {
	line  int
	words []string
}

// shellCommands splits src into simple commands: a trailing \ continues a
// line, a # starting a word comments out the rest of the line, quotes group,
// and ;, &&, || and | end a command.
func shellCommands(src []byte) []shellCommand {
	var out []shellCommand
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	var logical strings.Builder
	start := 0
	for i, l := range lines {
		if logical.Len() == 0 {
			start = i + 1
		}
		t := strings.TrimRight(l, " \t")
		if strings.HasSuffix(t, "\\") {
			logical.WriteString(strings.TrimSuffix(t, "\\"))
			logical.WriteByte(' ')
			continue
		}
		logical.WriteString(t)
		for _, ws := range splitShell(logical.String()) {
			if len(ws) > 0 {
				out = append(out, shellCommand{line: start, words: ws})
			}
		}
		logical.Reset()
	}
	return out
}

// splitShell tokenizes one logical line into commands of words.
func splitShell(s string) [][]string {
	var cmds [][]string
	var words []string
	var w strings.Builder
	inWord := false
	flush := func() {
		if inWord {
			words = append(words, w.String())
			w.Reset()
			inWord = false
		}
	}
	end := func() {
		flush()
		cmds = append(cmds, words)
		words = nil
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			// a command substitution or a Make variable stays in its word
			depth, j := 0, i+1
			for ; j < len(s); j++ {
				if s[j] == '(' {
					depth++
				} else if s[j] == ')' {
					if depth--; depth == 0 {
						break
					}
				}
			}
			if j >= len(s) {
				j = len(s) - 1
			}
			w.WriteString(s[i : j+1])
			i = j
			inWord = true
		case c == '\'' || c == '"':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				w.WriteString(s[i+1:])
				i = len(s)
			} else {
				w.WriteString(s[i+1 : i+1+j])
				i += j + 1
			}
			inWord = true
		case c == ' ' || c == '\t':
			flush()
		case c == '#' && !inWord:
			end()
			return cmds
		case c == ';' || c == '|' || c == '&' || c == '<' || c == '>' || c == '(' || c == ')' || c == '`':
			end()
			for i+1 < len(s) && (s[i+1] == c || s[i+1] == '&') {
				i++
			}
		default:
			w.WriteByte(c)
			inWord = true
		}
	}
	end()
	return cmds
}

// imageBuildSites is a file's build sites by its kind (bake, or shell-like).
func imageBuildSites(relPath string, src []byte) []ImageBuild {
	if isBakeFile(relPath) {
		return BakeBuilds(relPath, src)
	}
	if isShellBuildSource(relPath) {
		return ShellBuilds(relPath, src)
	}
	return nil
}

// isImageBuildCandidate: a file the image-tag index reads (LB-X-docker-
// image-ref): a Dockerfile (where FROM and COPY --from are written, and
// what a build resolves to), a docker-bake file, and the shell, Makefile and
// CI sources a literal `docker build` is written in. The doc-xref pass admits
// it beside isDocCandidate.
func isImageBuildCandidate(p string) bool {
	b := path.Base(p)
	return b == "Dockerfile" || strings.HasSuffix(strings.ToLower(b), ".dockerfile") || isBakeFile(p) || isShellBuildSource(p)
}
