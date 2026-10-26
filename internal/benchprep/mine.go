package benchprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

// TestRunner runs in checkout at commit, returning package-qualified test IDs
// with pass, fail or skip outcomes. An empty selection means the entire suite.
// A failing test is an outcome, while an infrastructure failure is an error.
type TestRunner func(ctx context.Context, checkout, commit string, selection []string) (map[string]string, error)

// FixCommit is a mined receipt, not a second scenario schema. BaseCommit is the
// pre-fix parent: applying TestPatch supplies the regression tests without the
// fix in its history. GoldPatch redoes the fix. RevertPatch is its inverse.
type FixCommit struct {
	Commit      string   `json:"commit"`
	Parent      string   `json:"parent"`
	CreatedAt   string   `json:"created_at"`
	Shape       string   `json:"shape"`
	BaseCommit  string   `json:"base_commit"`
	GoldPatch   string   `json:"gold_patch"`
	RevertPatch string   `json:"revert_patch"`
	TestPatch   string   `json:"test_patch"`
	FailToPass  []string `json:"fail_to_pass"`
	PassToPass  []string `json:"pass_to_pass"`
}

// MineFixCommits walks all non-merge commits in an operator-prepared full clone
// under workspace/scenariobench/subjects. It never checks out or edits the clone:
// each candidate and its parent run in detached worktrees under sibling mine-tmp/.
// Build-only parent failures are refused: they prove no named test failed.
// The default runner supports Go; injected runners share its outcome contract.
// Results and test IDs are sorted. No network or model client is used here.
func MineFixCommits(ctx context.Context, clone string, runner TestRunner) ([]FixCommit, error) {
	clone, err := filepath.Abs(clone)
	if err != nil {
		return nil, err
	}
	shallow, err := mineGitOutput(ctx, clone, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(shallow) != "false" {
		return nil, fmt.Errorf("mining requires a full clone, not a shallow repository")
	}
	if _, err := mineGitOutput(ctx, clone, "worktree", "prune", "--expire=now"); err != nil {
		return nil, err
	}
	history, err := mineGitOutput(ctx, clone, "rev-list", "--all", "--no-merges", "--parents")
	if err != nil {
		return nil, err
	}
	if runner == nil {
		runner = GoTestRunner
	}
	var results []FixCommit
	for _, line := range strings.Split(strings.TrimSpace(history), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 { // A root has no parent to prove.
			continue
		}
		receipt, err := mineCandidate(ctx, clone, fields[0], fields[1], runner)
		if err != nil {
			return nil, fmt.Errorf("mining %s: %w", fields[0], err)
		}
		if receipt != nil {
			results = append(results, *receipt)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Commit < results[j].Commit })
	return results, nil
}

func mineCandidate(ctx context.Context, clone, commit, parent string, runner TestRunner) (result *FixCommit, err error) {
	paths, err := mineGitOutput(ctx, clone, "diff", "--name-only", "-z", parent, commit, "--", "*_test.go")
	if err != nil || paths == "" {
		return nil, err
	}
	selection, err := touchedGoTests(ctx, clone, parent, commit, paths)
	if err != nil || len(selection) == 0 {
		return nil, err
	}
	tests, err := mineGitOutput(ctx, clone, "diff", "--binary", parent, commit, "--", "*_test.go", "testdata/**", "**/testdata/**")
	if err != nil {
		return nil, err
	}
	code, err := mineGitOutput(ctx, clone, "diff", "--binary", parent, commit, "--", ".", ":(exclude)*_test.go", ":(exclude)testdata/**", ":(exclude)**/testdata/**")
	if err != nil || code == "" {
		return nil, err
	}
	reverse, err := mineGitOutput(ctx, clone, "diff", "--binary", commit, parent, "--", ".", ":(exclude)*_test.go", ":(exclude)testdata/**", ":(exclude)**/testdata/**")
	if err != nil {
		return nil, err
	}
	created, err := mineGitOutput(ctx, clone, "show", "-s", "--format=%aI", commit)
	if err != nil {
		return nil, err
	}
	tmpRoot := filepath.Join(filepath.Dir(filepath.Dir(clone)), "mine-tmp")
	if err := os.MkdirAll(tmpRoot, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(tmpRoot, ".mine-")
	if err != nil {
		return nil, err
	}
	var worktrees []string
	defer func() {
		// Cancellation must not strand a registered worktree.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		for _, path := range worktrees {
			if _, cleanupErr := mineGitOutput(cleanupCtx, clone, "worktree", "remove", "--force", path); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("cleaning mining worktree: %w", cleanupErr))
			}
		}
		if cleanupErr := os.RemoveAll(tmp); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	checkout := func(name, revision string) (string, error) {
		path := filepath.Join(tmp, name)
		if _, err := mineGitOutput(ctx, clone, "worktree", "add", "--detach", path, revision); err != nil {
			return "", err
		}
		worktrees = append(worktrees, path)
		return path, nil
	}
	after, err := checkout("commit", commit)
	if err != nil {
		return nil, err
	}
	suite, err := runner(ctx, after, commit, nil)
	if err != nil {
		return nil, err
	}
	if len(suite) == 0 {
		return nil, nil
	}
	for _, outcome := range suite {
		if outcome != "pass" && outcome != "skip" {
			return nil, nil
		}
	}
	before, err := checkout("parent", parent)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--binary", "-")
	cmd.Dir, cmd.Stdin = before, strings.NewReader(tests)
	if out, applyErr := cmd.CombinedOutput(); applyErr != nil {
		return nil, fmt.Errorf("applying regression tests at parent: %w: %s", applyErr, out)
	}
	// The parent suite establishes pass-to-pass evidence as well as the
	// touched tests' failures, rather than assuming a commit-only pass.
	baseline, err := runner(ctx, before, parent, nil)
	if err != nil {
		return nil, err
	}
	var failToPass, passToPass []string
	for id, outcome := range baseline {
		if outcome == "fail" && selectedTest(id, selection) && suite[id] == "pass" {
			failToPass = append(failToPass, id)
		}
	}
	if len(failToPass) == 0 {
		return nil, nil
	}
	sort.Strings(failToPass)
	for id, outcome := range suite {
		if outcome == "pass" && baseline[id] == "pass" {
			passToPass = append(passToPass, id)
		}
	}
	sort.Strings(passToPass)
	return &FixCommit{Commit: commit, Parent: parent, CreatedAt: strings.TrimSpace(created), Shape: "revert-and-redo",
		BaseCommit: parent, GoldPatch: code, RevertPatch: reverse, TestPatch: tests,
		FailToPass: failToPass, PassToPass: passToPass}, nil
}

func selectedTest(id string, selection []string) bool {
	for _, selected := range selection {
		if id == selected || strings.HasPrefix(id, selected+"/") {
			return true
		}
	}
	return false
}

func touchedGoTests(ctx context.Context, clone, parent, commit, paths string) ([]string, error) {
	names := map[string]bool{}
	for _, file := range strings.Split(strings.TrimSuffix(paths, "\x00"), "\x00") {
		after, err := mineGitOutput(ctx, clone, "show", commit+":"+file)
		if err != nil { // A deleted test cannot demonstrate fail-to-pass.
			continue
		}
		before, _ := mineGitOutput(ctx, clone, "show", parent+":"+file)
		newFuncs, err := goTestBodies(file, after)
		if err != nil {
			return nil, err
		}
		oldFuncs, err := goTestBodies(file, before)
		if err != nil {
			return nil, err
		}
		// Each test belongs to its nearest module, including repositories with
		// no root module. Package IDs must agree with go test's JSON events.
		dir := path.Dir(file)
		var modulePath, moduleDir string
		for {
			module, err := mineGitOutput(ctx, clone, "show", commit+":"+path.Join(dir, "go.mod"))
			if err == nil {
				modulePath, moduleDir = modfile.ModulePath([]byte(module)), dir
				break
			}
			if dir == "." {
				break
			}
			dir = path.Dir(dir)
		}
		if modulePath == "" {
			return nil, fmt.Errorf("Go mining requires a module path for %s", file)
		}
		rel, err := filepath.Rel(moduleDir, path.Dir(file))
		if err != nil {
			return nil, err
		}
		for name, body := range newFuncs {
			if oldFuncs[name] != body {
				names[path.Join(modulePath, filepath.ToSlash(rel))+"."+name] = true
			}
		}
	}
	var result []string
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func goTestBodies(path, source string) (map[string]string, error) {
	result := map[string]string{}
	if source == "" {
		return result, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, err
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || fn.Name.Name == "TestMain" || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		result[fn.Name.Name] = source[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset]
	}
	return result, nil
}

func mineGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}
	return string(out), nil
}

// GoTestRunner runs uncached, offline go test JSON and preserves package identity
// in test IDs. Package-level failures also get an outcome, so a build failure or
// a failing TestMain cannot look like a green suite. commit is receipt metadata;
// the caller owns preparing checkout (including its regression test overlay).
func GoTestRunner(ctx context.Context, checkout, commit string, selection []string) (map[string]string, error) {
	modules, err := goModules(checkout)
	if err != nil {
		return nil, err
	}
	results := map[string]string{}
	groups := map[string][]string{}
	for _, id := range selection {
		pkg, name, err := splitGoTestID(id, modules)
		if err != nil {
			return nil, err
		}
		groups[pkg] = append(groups[pkg], name)
	}
	var packages []string
	for pkg := range groups {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	run := func(dir, pkg string, names []string) error {
		args := []string{"test", "-json", "-count=1"}
		if len(names) != 0 {
			patterns := make([]string, len(names))
			for i, name := range names {
				segments := strings.Split(name, "/")
				for j, segment := range segments {
					segments[j] = "^" + regexp.QuoteMeta(segment) + "$"
				}
				patterns[i] = strings.Join(segments, "/")
			}
			args = append(args, "-run", strings.Join(patterns, "|"))
		}
		args = append(args, pkg)
		outcomes, err := runGoTests(ctx, dir, commit, args, len(names) == 0)
		if err != nil {
			return err
		}
		for id, outcome := range outcomes {
			results[id] = outcome
		}
		return nil
	}
	if len(selection) == 0 {
		for _, module := range modules {
			if err := run(module.dir, "./...", nil); err != nil {
				return nil, err
			}
		}
	} else {
		for _, pkg := range packages {
			var dir, owner string
			for _, module := range modules {
				if (pkg == module.path || strings.HasPrefix(pkg, module.path+"/")) && len(module.path) > len(owner) {
					dir, owner = module.dir, module.path
				}
			}
			if dir == "" {
				return nil, fmt.Errorf("no module owns selected package %q", pkg)
			}
			names := groups[pkg]
			sort.Strings(names)
			// Go splits -run on slashes before parsing its regex. Subtest
			// selections run separately to avoid cross products of segments.
			var top []string
			for _, name := range names {
				if strings.Contains(name, "/") {
					if err := run(dir, pkg, []string{name}); err != nil {
						return nil, err
					}
				} else {
					top = append(top, name)
				}
			}
			if len(top) > 0 {
				if err := run(dir, pkg, top); err != nil {
					return nil, err
				}
			}
		}
	}
	return results, nil
}

type goModule struct{ dir, path string }

func splitGoTestID(id string, modules []goModule) (string, string, error) {
	match := regexp.MustCompile(`^(.+?)\.((?:Test|Example|Fuzz)[^/.]*)(/.*)?$`).FindStringSubmatch(id)
	if match == nil {
		return "", "", fmt.Errorf("invalid Go test ID %q", id)
	}
	// Package directories may themselves contain .Test, .Example or .Fuzz.
	// Prefer the longest package prefix that exists in the selected checkout,
	// so a test-like subtest name cannot invent a nonexistent package.
	boundaries := regexp.MustCompile(`\.(?:Test|Example|Fuzz)`).FindAllStringIndex(id, -1)
	namePattern := regexp.MustCompile(`^(?:Test|Example|Fuzz)[^/.]*(/.*)?$`)
	for i := len(boundaries) - 1; i >= 0; i-- {
		pkg, name := id[:boundaries[i][0]], id[boundaries[i][0]+1:]
		if !namePattern.MatchString(name) {
			continue
		}
		var dir, owner string
		for _, module := range modules {
			if (pkg == module.path || strings.HasPrefix(pkg, module.path+"/")) && len(module.path) > len(owner) {
				owner = module.path
				dir = filepath.Join(module.dir, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(pkg, owner), "/")))
			}
		}
		if dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return pkg, name, nil
			}
		}
	}
	return match[1], match[2] + match[3], nil
}

func goModules(checkout string) ([]goModule, error) {
	var modules []goModule
	err := filepath.WalkDir(checkout, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if file != checkout && (strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || entry.Name() == "vendor" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		modulePath := modfile.ModulePath(data)
		if modulePath == "" {
			return fmt.Errorf("Go mining requires a module path in %s", file)
		}
		modules = append(modules, goModule{filepath.Dir(file), modulePath})
		return nil
	})
	if err == nil && len(modules) == 0 {
		err = fmt.Errorf("Go mining requires at least one module")
	}
	return modules, err
}

func runGoTests(ctx context.Context, checkout, commit string, args []string, allowEmpty bool) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = checkout
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=readonly")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	results := map[string]string{}
	failed := false
	for {
		var event struct{ Action, Package, Test string }
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decoding go test at %s: %w", commit, err)
		}
		if event.Action != "pass" && event.Action != "fail" && event.Action != "skip" {
			continue
		}
		if event.Test != "" {
			results[event.Package+"."+event.Test] = event.Action
		} else if event.Action == "fail" {
			results[event.Package] = "fail"
		}
		failed = failed || event.Action == "fail"
	}
	if runErr != nil && !failed && !(allowEmpty && strings.Contains(stderr.String(), "no packages to test")) {
		return nil, fmt.Errorf("go test at %s: %w: %s", commit, runErr, stderr.String())
	}
	return results, nil
}
