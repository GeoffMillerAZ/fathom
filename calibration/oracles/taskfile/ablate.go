package main

// ablate.go: tier-A references by ablation (-ablate), the kustomize oracle's
// rule. go-task lists nothing of what a task depends on, but `task --dry
// --force <task>` evaluates a task's run plan offline, its deps and its `task:`
// calls in order, and prints `task: [<name>] <command>` for each command of
// every task it would run. A written reference T -> D is attested when cutting
// it (every deps entry and every `task:` call or deferred call of T that names
// D) from a scratch copy changes the set of tasks the dry run names, or makes
// the tool refuse the run.
//
// A task that `requires` variables is refused without them, so every variable
// any task of the file requires is passed on the command line with a
// placeholder (example.com, or an enum's first value): a value names no task.
//
// --dry still evaluates a task's variables, preconditions and `if:` conditions,
// and those run shell (a dynamic variable's `sh:`, go-task/task's release task
// runs `go run ./cmd/release`): the scratch copy has every `sh:` and `if:` value
// replaced by `true` first (neutralize), so no subject command runs and nothing
// needs the network. The neutralized copy is the baseline every cut is compared
// against, and the comparison is the set of task names, not the commands, which
// carry templated values such as a build date.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// neutralizeExpr replaces every sh: and if: value with `true`.
const neutralizeExpr = `(.. | select(tag == "!!map" and has("sh")) | .sh) |= "true" | (.. | select(tag == "!!map" and has("if")) | .if) |= "true"`

// cutExpr removes, from task F, every deps entry and every task: or deferred
// task: call that names T (F and T come from the environment).
const cutExpr = `with(.tasks[strenv(F)] | select(tag == "!!map" and has("deps")) | .deps; . = [.[] | select((tag == "!!str" and . != strenv(T)) or (tag == "!!map" and (.task // "") != strenv(T)))]) | ` +
	`with(.tasks[strenv(F)] | select(tag == "!!map" and has("cmds")) | .cmds; . = [.[] | select(tag != "!!map" or ((.task // "") != strenv(T) and ((.defer | select(tag == "!!map") | .task) // "") != strenv(T)))]) | ` +
	`with(.tasks[strenv(F)] | select(tag == "!!seq"); . = [.[] | select(tag != "!!map" or ((.task // "") != strenv(T) and ((.defer | select(tag == "!!map") | .task) // "") != strenv(T)))])`

// neutralize rewrites a Taskfile in place with every sh: and if: set to true.
func (r runner) neutralize(ctx context.Context, file string) error {
	src, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	out, se, err := r.run(ctx, filepath.Dir(file), src, r.yq, neutralizeExpr, "-")
	if err != nil {
		return fmt.Errorf("yq neutralize %s: %v: %s", file, err, firstLine(se))
	}
	return os.WriteFile(file, out, 0o644)
}

var (
	dryLineRE = regexp.MustCompile(`(?m)^task: \[([^\]]+)\] `)
	ansiRE    = regexp.MustCompile("\x1b\\[[0-9;]*m")
)

// dryNames are the task names a dry run prints a command for (colour codes,
// which the tool writes under CI=true, are dropped first).
func dryNames(out []byte) map[string]bool {
	names := map[string]bool{}
	for _, m := range dryLineRE.FindAllSubmatch(ansiRE.ReplaceAll(out, nil), -1) {
		names[string(m[1])] = true
	}
	return names
}

type dryAnswer struct {
	names map[string]bool
	err   string
}

type ablator struct {
	r    runner
	base map[string]dryAnswer // file + task -> the uncut answer
	vars map[string][]string  // file -> the placeholders for its required variables
	runs int
}

// requiredVars are NAME=value placeholders for every variable a task of the
// decoded tasks map requires (a string, or a map with a name and an enum).
func requiredVars(tasks []byte) []string {
	var m map[string]any
	if json.Unmarshal(tasks, &m) != nil {
		return nil
	}
	seen := map[string]string{}
	for _, t := range m {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		req, _ := tm["requires"].(map[string]any)
		vs, _ := req["vars"].([]any)
		for _, v := range vs {
			switch x := v.(type) {
			case string:
				if _, ok := seen[x]; !ok {
					seen[x] = "example.com"
				}
			case map[string]any:
				name, _ := x["name"].(string)
				if name == "" {
					continue
				}
				val := "example.com"
				if enum, ok := x["enum"].([]any); ok && len(enum) > 0 {
					val = fmt.Sprint(enum[0])
				}
				seen[name] = val
			}
		}
	}
	var out []string
	for k, v := range seen {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

func newAblator(r runner) *ablator {
	return &ablator{r: r, base: map[string]dryAnswer{}, vars: map[string][]string{}}
}

func (a *ablator) dry(ctx context.Context, dir, file, task string) dryAnswer {
	a.runs++
	vars, ok := a.vars[file]
	if !ok {
		if src, err := os.ReadFile(file); err == nil {
			if tasks, _, err := a.r.run(ctx, dir, src, a.r.yq, "-o=json", ".tasks // {}", "-"); err == nil {
				vars = requiredVars(tasks)
			}
		}
		a.vars[file] = vars
	}
	args := append([]string{"--dry", "--force", "--silent=false", "--color=false", "--taskfile", file, task}, vars...)
	so, se, err := a.r.run(ctx, dir, nil, a.r.task, args...)
	ans := dryAnswer{names: dryNames(append(append([]byte{}, so...), se...))}
	if err != nil {
		ans.err = strings.TrimSpace(string(se))
		if ans.err == "" {
			ans.err = err.Error()
		}
	}
	return ans
}

// attest cuts from -> to in file (a neutralized copy), runs the dry run, and
// restores the file. It reports whether the answer changed.
func (a *ablator) attest(ctx context.Context, dir, file, from, to string) (bool, error) {
	key := file + "\x00" + from
	base, ok := a.base[key]
	if !ok {
		base = a.dry(ctx, dir, file, from)
		a.base[key] = base
	}
	orig, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}
	env := append(append([]string{}, a.r.env...), "F="+from, "T="+to)
	cr := a.r
	cr.env = env
	cut, se, err := cr.run(ctx, dir, orig, a.r.yq, cutExpr, "-")
	if err != nil {
		return false, fmt.Errorf("yq cut %s %s -> %s: %v: %s", file, from, to, err, firstLine(se))
	}
	if bytes.Equal(cut, orig) {
		return false, nil // nothing to cut: the reference is not written in this form
	}
	if err := os.WriteFile(file, cut, 0o644); err != nil {
		return false, err
	}
	got := a.dry(ctx, dir, file, from)
	if err := os.WriteFile(file, orig, 0o644); err != nil {
		return false, err
	}
	if (got.err == "") != (base.err == "") {
		return true, nil
	}
	if len(got.names) != len(base.names) {
		return true, nil
	}
	for n := range base.names {
		if !got.names[n] {
			return true, nil
		}
	}
	return false, nil
}
