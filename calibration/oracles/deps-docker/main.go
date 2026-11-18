// deps-docker-oracle uses BuildKit's parser and instruction types. It never
// invokes the frontend, solver, daemon, or any instruction in a Dockerfile.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/moby/buildkit/util/system"
	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
	fscopy "github.com/tonistiigi/fsutil/copy"
)

type edge struct {
	Kind, Raw, Resolution, To string
	Line                      int
}

type row struct {
	Path   string
	Parsed bool
	Edges  []edge
	Gaps   []string
}

type dockerTruth struct {
	root *os.Root
	name string
	row
	ignore      *patternmatcher.PatternMatcher
	rootContext bool
}

func main() {
	root := flag.String("root", "", "read-only context checkout")
	version := flag.Bool("version", false, "print the linked BuildKit version and checksum")
	flag.Parse()
	if *version {
		info, _ := debug.ReadBuildInfo()
		for _, dep := range info.Deps {
			if dep.Path == "github.com/moby/buildkit" && dep.Replace == nil {
				fmt.Printf("%s@%s %s %s\n", dep.Path, dep.Version, dep.Sum, info.GoVersion)
				return
			}
		}
		fail(errors.New("BuildKit version unavailable"))
	}
	if *root == "" || flag.NArg() == 0 {
		fail(errors.New("expected -root and one or more Dockerfile paths"))
	}
	view, err := os.OpenRoot(*root)
	if err != nil {
		fail(err)
	}
	defer view.Close()

	out := make([]row, 0, flag.NArg())
	for _, name := range flag.Args() {
		if !fs.ValidPath(name) || name == "." {
			fail(errors.New("invalid relative Dockerfile path"))
		}
		f, err := view.Open(name)
		if err != nil {
			fail(err)
		}
		truth := dockerTruth{root: view, name: *root, row: row{Path: name, Edges: []edge{}, Gaps: []string{}}, rootContext: path.Dir(name) == "."}
		truth.read(f)
		f.Close()
		slices.Sort(truth.Gaps)
		truth.Gaps = slices.Compact(truth.Gaps)
		out = append(out, truth.row)
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func (d *dockerTruth) gap(reason string) { d.Gaps = append(d.Gaps, reason) }

func (d *dockerTruth) read(f *os.File) {
	native, err := parser.Parse(f)
	if err != nil {
		d.gap("parser")
		return
	}
	stages, _, err := instructions.Parse(native.AST, nil)
	if err != nil || len(stages) == 0 {
		d.gap("instruction")
		return
	}
	d.Parsed = true
	var dp parser.DirectiveParser
	// Parse the leading frontend directive independently of the product.
	// Native instruction evidence remains useful for partial rows.
	data, err := d.root.ReadFile(d.Path)
	if err != nil {
		fail(err)
	}
	directives, err := dp.ParseAll([]byte(strings.TrimPrefix(string(data), "\ufeff")))
	if err == nil {
		for _, directive := range directives {
			if directive.Name == "syntax" {
				d.gap("explicit-frontend")
			}
		}
	}
	d.loadIgnore()
	lex := shell.NewLex(native.EscapeToken)
	lex.SkipUnsetEnv = true
	env := shell.EnvsFromSlice(nil)

	aliases := make(map[string]int)
	for i, stage := range stages {
		if stage.BaseName != "scratch" {
			if _, found := aliases[strings.ToLower(stage.BaseName)]; !found {
				// An image can contribute inherited ONBUILD and named-context
				// bindings. Neither follows from the parsed reference.
				d.gap("base-image-config")
			}
		}
		if strings.Contains(stage.BaseName+stage.Platform, "$") {
			d.gap("computed-image-or-platform")
		}
		if stage.Platform != "" && !strings.HasPrefix(stage.Platform, "linux/") {
			d.gap("platform")
		}
		for _, cmd := range stage.Commands {
			line := cmd.Location()[0].Start.Line
			switch cmd := cmd.(type) {
			case *instructions.CopyCommand:
				if cmd.From != "" {
					d.checkStage(aliases, i, cmd.From)
				} else if cmd.Parents {
					d.gap("copy-parents")
				} else {
					d.sources(lex, env, cmd.SourcePaths, cmd.ExcludePatterns, false, line)
				}
			case *instructions.AddCommand:
				d.sources(lex, env, cmd.SourcePaths, cmd.ExcludePatterns, true, line)
			case *instructions.RunCommand:
				d.mounts(cmd, aliases, i, lex, env, line)
			case *instructions.OnbuildCommand:
				d.gap("onbuild")
			}
		}
		if stage.Name != "" {
			aliases[stage.Name] = i
		}
	}
}

func (d *dockerTruth) checkStage(aliases map[string]int, current int, from string) {
	if index, err := strconv.Atoi(from); err == nil {
		if index < 0 || index >= current {
			d.gap("stage-index")
		}
	} else if _, found := aliases[strings.ToLower(from)]; !found {
		d.gap("stage-image-or-context")
	}
}

func (d *dockerTruth) loadIgnore() {
	for _, name := range []string{d.Path + ".dockerignore", ".dockerignore"} {
		if name == ".dockerignore" && !d.rootContext {
			break
		}
		info, err := d.root.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			d.gap("ignore-input")
			break
		}
		f, err := d.root.Open(name)
		if err != nil {
			fail(err)
		}
		patterns, err := ignorefile.ReadAll(f)
		f.Close()
		if err == nil {
			d.ignore, err = patternmatcher.New(patterns)
		}
		if err != nil {
			d.gap("ignore-pattern")
		}
		d.Edges = append(d.Edges, edge{Kind: "config-ref", Raw: name, Resolution: "file", To: name})
		break
	}
}

func (d *dockerTruth) sources(lex *shell.Lex, env shell.EnvGetter, sources, excludes []string, add bool, line int) {
	matcher, err := patternmatcher.New(excludes)
	if err != nil {
		d.gap("exclude-pattern")
		return
	}
	for _, raw := range sources {
		if strings.Contains(raw, "$") {
			d.gap("computed-source")
			continue
		}
		src, _, err := lex.ProcessWord(raw, env)
		if err != nil || src == "" {
			d.gap("source-operand")
			continue
		}
		if add && (strings.Contains(src, "://") || strings.Contains(src, "@") && strings.Contains(src, ":")) {
			d.gap("remote-add")
			continue
		}
		d.local(src, src, matcher, line, true)
	}
}

func (d *dockerTruth) mounts(cmd *instructions.RunCommand, stages map[string]int, i int, lex *shell.Lex, env shell.EnvGetter, line int) {
	if err := cmd.Expand(func(raw string) (string, error) {
		if strings.Contains(raw, "$") {
			return "", errors.New("computed mount")
		}
		word, _, err := lex.ProcessWord(raw, env)
		return word, err
	}); err != nil {
		d.gap("computed-or-invalid-mount")
		return
	}
	for _, mount := range instructions.GetMounts(cmd) {
		switch {
		case mount.Type == instructions.MountTypeSecret || mount.Type == instructions.MountTypeSSH:
			d.gap("mount-external")
		case mount.From != "":
			d.checkStage(stages, i, mount.From)
		case mount.Type == instructions.MountTypeBind:
			src := mount.Source
			if src == "" {
				src = "."
			}
			d.local(src, src, nil, line, false)
		}
	}
}

func (d *dockerTruth) local(source, raw string, excludes *patternmatcher.PatternMatcher, line int, allowGlob bool) {
	if !d.rootContext {
		d.gap("context")
		return
	}
	src, err := system.NormalizePath("/", source, "linux", false)
	if err != nil {
		d.gap("source-operand")
		return
	}
	matches := []string{src}
	if allowGlob {
		matches, err = fscopy.ResolveWildcards(d.name, src, false)
		if err != nil || len(matches) == 0 {
			d.gap("source-glob")
			return
		}
	}
	for _, match := range matches {
		name := strings.TrimPrefix(match, "/")
		if name == "" {
			name = "."
		}
		d.members(name, raw, excludes, line)
	}
}

func (d *dockerTruth) members(name, raw string, excludes *patternmatcher.PatternMatcher, line int) {
	before := len(d.Edges)
	err := fs.WalkDir(d.root.FS(), name, func(member string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if member == ".git" || strings.HasPrefix(member, ".git/") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel := path.Base(member)
		if member != name {
			rel = strings.TrimPrefix(member, name+"/")
		}
		ignored, err := nativeExcluded(d.ignore, member)
		if err != nil {
			return err
		}
		excluded, err := nativeExcluded(excludes, rel)
		if err != nil {
			return err
		}
		if ignored || excluded && (member != name || !entry.IsDir()) {
			return nil
		}
		if entry.IsDir() {
			children, err := fs.ReadDir(d.root.FS(), member)
			if err != nil {
				return err
			}
			if len(children) == 0 {
				d.Edges = append(d.Edges, edge{"build-input", raw, "directory", member, line})
			}
		} else if entry.Type().IsRegular() {
			d.Edges = append(d.Edges, edge{"build-input", raw, "file", member, line})
		} else {
			d.gap("source-non-regular")
		}
		return nil
	})
	if err != nil || len(d.Edges) == before {
		d.gap("source-unavailable")
	}
}

func nativeExcluded(matcher *patternmatcher.PatternMatcher, name string) (bool, error) {
	if matcher == nil || name == "." {
		return false, nil
	}
	return matcher.MatchesOrParentMatches(name)
}
