package main

// Static wrapper inspection only. Neither shell scripts nor task runners run.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

const doctorWrapperBytes = 64 << 10
const doctorWrapperLevels = 2

type doctorWiringEvidence struct {
	found, indirect, unknown bool
	missingScript            string
	file                     string
	// opaque names the last project file the walk followed into that is not
	// a shell script (an interpreter's script): it may hold the step, but
	// this reader cannot read it.
	opaque string
}

type doctorWrapperReader struct {
	ctx       context.Context
	root      string
	remaining int
	matches   func([]string) bool
	// cache holds each file once: a file read again (a Taskfile per `task X`)
	// is never charged twice. cut is set when the byte budget stopped a read.
	cache map[string][]byte
	cut   bool
	// A cut hides commands only if its target is never walked elsewhere.
	// Keep cuts in traversal order and label them while unwinding.
	walked    map[string]bool
	depthCuts []doctorWrapperDepthCut
}

type doctorWrapperDepthCut struct {
	target, at string
}

func (r *doctorWrapperReader) markWalked(target string) {
	if r.walked == nil {
		r.walked = map[string]bool{}
	}
	r.walked[r.walkKey(target)] = true
}

// walkKey is the key a script is walked under: its resolved repository path,
// so a symlink to a walked script (or the hook file reached by another name)
// is the same script. A target that does not resolve inside the repository,
// or is not a file path (a task label), keeps its own spelling.
func (r *doctorWrapperReader) walkKey(target string) string {
	if r.root == "" || filepath.IsAbs(target) || strings.ContainsAny(target, " ()") {
		return target
	}
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		return target
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, target))
	if err != nil {
		return target
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return target
	}
	return rel
}

func (r *doctorWrapperReader) labelDepthCuts(start int, at string) {
	for i := start; i < len(r.depthCuts); i++ {
		if r.depthCuts[i].at == "" {
			r.depthCuts[i].at = at
		}
	}
}

// Decide only after the walk: a shallower visit can follow a cut.
func (r *doctorWrapperReader) depthCutAt() string {
	for _, cut := range r.depthCuts {
		if !r.walked[r.walkKey(cut.target)] {
			return cut.at
		}
	}
	return ""
}

func doctorCommandEvidence(ctx context.Context, root, command, sub string) doctorWiringEvidence {
	r := doctorWrapperReader{ctx: ctx, root: root, remaining: doctorWrapperBytes}
	return r.command(command, sub, 0)
}

// A cut target need only resolve to a regular repository file. Do not read
// its contents or charge the byte budget for commands we cannot scan.
func (r *doctorWrapperReader) regularFile(path string) bool {
	if r.ctx.Err() != nil || r.root == "" {
		return false
	}
	rel, err := filepath.Rel(r.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	root, err := os.OpenRoot(r.root)
	if err != nil {
		return false
	}
	defer root.Close()
	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	return err == nil && info.Mode().IsRegular()
}

func (r *doctorWrapperReader) read(path string) ([]byte, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	// Resolve the repository boundary before opening any wrapper. OpenRoot
	// also prevents symlink replacement from escaping that boundary.
	if r.root == "" {
		return nil, fmt.Errorf("wrapper root is unknown")
	}
	rel, err := filepath.Rel(r.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, fmt.Errorf("wrapper is outside the repository")
	}
	if b, ok := r.cache[rel]; ok {
		return b, nil
	}
	root, err := os.OpenRoot(r.root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(r.remaining) {
		r.cut = r.cut || (info.Mode().IsRegular() && info.Size() > int64(r.remaining))
		return nil, fmt.Errorf("wrapper is not a bounded regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(r.remaining)+1))
	r.remaining -= len(b)
	if r.remaining < 0 {
		r.cut = true
		return nil, fmt.Errorf("wrapper exceeds 64 KiB")
	}
	if err == nil {
		err = r.ctx.Err()
	}
	if err == nil {
		if r.cache == nil {
			r.cache = map[string][]byte{}
		}
		r.cache[rel] = b
	}
	return b, err
}

// readExternal reads one bounded regular file outside the repository: the
// shared git directory's pre-push hook of a linked worktree or submodule, or
// a core.hooksPath target outside the repository. Callers pass a path git
// itself reported, never one a wrapper named.
func (r *doctorWrapperReader) readExternal(path string) ([]byte, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(r.remaining) {
		return nil, fmt.Errorf("hook is not a bounded regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(r.remaining)+1))
	r.remaining -= len(b)
	return b, err
}

func (r *doctorWrapperReader) command(command, sub string, depth int) doctorWiringEvidence {
	if r.ctx.Err() != nil {
		return doctorWiringEvidence{unknown: true}
	}
	var unknown bool
	var missingScript, opaque string
	// A shell command list is wiring evidence, never proof the branch fired.
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, part := range strings.FieldsFunc(line, func(c rune) bool { return c == '|' || c == '&' || c == ';' }) {
			words := strings.Fields(part)
			for i := range words {
				words[i] = strings.NewReplacer("\"", "", "'", "").Replace(words[i])
			}
			words = slices.DeleteFunc(words, func(w string) bool {
				w = strings.TrimLeft(w, "0123456789")
				return strings.HasPrefix(w, "<") || strings.HasPrefix(w, ">")
			})
			if len(words) > 1 && words[0] == "command" && words[1] == "-v" {
				continue // template availability guard, not an unresolved wrapper
			}
			for len(words) > 0 && (words[0] == "exec" || words[0] == "command" || words[0] == "env" || strings.Contains(words[0], "=")) {
				words = words[1:]
			}
			if len(words) == 0 {
				continue
			}
			if r.matches != nil && r.matches(words) {
				return doctorWiringEvidence{found: true, indirect: depth > 0}
			}
			bin := filepath.Base(words[0])
			if bin == "fathom" || words[0] == "$FATHOM" || words[0] == "${FATHOM}" {
				want := strings.Fields(sub)
				if r.matches == nil && len(words) > len(want) && strings.Join(words[1:1+len(want)], " ") == sub {
					return doctorWiringEvidence{found: true, indirect: depth > 0}
				}
				continue
			}
			// A foreign executable cannot stand in for a project wrapper,
			// even if its basename happens to be a supported task runner.
			if doctorForeignProgram(r.root, words[0]) {
				continue
			}
			shellScript := false
			if bin == "sh" || bin == "bash" || bin == "zsh" {
				if len(words) < 2 || strings.HasPrefix(words[1], "-") {
					unknown = unknown || depth > 0
					continue
				}
				words = words[1:]
				shellScript = true
				bin = filepath.Base(words[0])
			}
			wrapper := shellScript || strings.Contains(words[0], "/") || strings.HasSuffix(bin, ".sh")
			if script := r.opaqueScript(words); script != "" {
				// Only a script run as "...verify..." may hold the verify step;
				// any other interpreter script is unrelated work.
				if opaque == "" && strings.Contains(strings.Join(words, " "), "verify") {
					opaque = script
				}
				continue
			}
			if bin != "task" && bin != "lefthook" && !wrapper {
				// Other tools do not stand for fathom. Dynamic shell work is
				// uncertain only inside a project wrapper we actually read.
				if depth > 0 && (bin == "eval" || bin == "source" || bin == ".") {
					unknown = true
				}
				continue
			}
			atDepthLimit := depth >= doctorWrapperLevels
			if atDepthLimit {
				unknown = true
			}
			var ev doctorWiringEvidence
			switch bin {
			case "task", "lefthook":
				if atDepthLimit && bin != "task" {
					continue
				}
				ev = r.task(words, sub, depth+1)
			default:
				p := words[0]
				// Quote removal precedes expansion so "$VAR"/script.sh works too.
				// Only documented project-dir aliases resolve, using the root doctor
				// is diagnosing, never an unrelated ambient environment value.
				for _, name := range []string{"CLAUDE_PROJECT_DIR", "GEMINI_PROJECT_DIR", "CURSOR_PROJECT_DIR"} {
					p = strings.ReplaceAll(p, "${"+name+"}", ".")
					if p == "$"+name || strings.HasPrefix(p, "$"+name+"/") {
						p = "." + strings.TrimPrefix(p, "$"+name)
					}
				}
				if doctorForeignProgram(r.root, p) {
					continue
				}
				if strings.ContainsAny(p, "$`{}") {
					unknown = true
					continue
				}
				if !filepath.IsAbs(p) {
					p = filepath.Join(r.root, p)
				}
				if atDepthLimit {
					// A missing or foreign script hides no readable commands.
					// Resolve the target, but never walk its contents past the limit.
					if r.regularFile(p) {
						rel, _ := filepath.Rel(r.root, p)
						r.depthCuts = append(r.depthCuts, doctorWrapperDepthCut{target: rel})
					}
					continue
				}
				b, err := r.read(p)
				if err != nil {
					if os.IsNotExist(err) {
						if missingScript == "" {
							missingScript = p
						}
					} else {
						unknown = true
					}
					continue
				}
				// A script that was read is settled, even when it is refused below:
				// a later depth cut at the same script hides nothing.
				rel, _ := filepath.Rel(r.root, p)
				r.markWalked(rel)
				// Dynamic shell structure is outside this deliberately small reader.
				text := string(b)
				if strings.Contains(text, "<<") || strings.Contains(text, "$(") || strings.Contains(text, "()") || strings.Contains(text, "`") {
					unknown = true
					continue
				}
				firstCut := len(r.depthCuts)
				ev = r.command(text, sub, depth+1)
				r.labelDepthCuts(firstCut, rel)
				if ev.found && ev.file == "" {
					ev.file, _ = filepath.Rel(r.root, p)
				}
			}
			if ev.found {
				return ev
			}
			unknown = unknown || ev.unknown
			if missingScript == "" {
				missingScript = ev.missingScript
			}
			if opaque == "" {
				opaque = ev.opaque
			}
		}
	}
	return doctorWiringEvidence{unknown: unknown, missingScript: missingScript, opaque: opaque}
}

// opaqueScript returns the repository-relative path of a project script that
// an interpreter (python, node, ruby, perl) or a .py/.js/.rb/.pl executable
// word runs, when that file exists inside the repository. Such a script is
// outside this deliberately small shell reader: it is named, never read.
func (r *doctorWrapperReader) opaqueScript(words []string) string {
	if len(words) == 0 {
		return ""
	}
	candidate := ""
	bin := filepath.Base(words[0])
	switch {
	case strings.HasPrefix(bin, "python") || bin == "node" || bin == "ruby" || bin == "perl":
		for _, w := range words[1:] {
			if !strings.HasPrefix(w, "-") {
				candidate = w
				break
			}
		}
	case slices.Contains([]string{".py", ".js", ".rb", ".pl"}, filepath.Ext(bin)):
		candidate = words[0]
	}
	if candidate == "" || strings.ContainsAny(candidate, "$`{}") || doctorForeignProgram(r.root, candidate) {
		return ""
	}
	p := candidate
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	rel, err := filepath.Rel(r.root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return filepath.ToSlash(rel)
}

func doctorForeignProgram(root, program string) bool {
	if strings.HasPrefix(program, "~/") {
		return true
	}
	if !filepath.IsAbs(program) {
		return false
	}
	rel, err := filepath.Rel(root, program)
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel)
}

func (r *doctorWrapperReader) task(words []string, sub string, depth int) doctorWiringEvidence {
	bin := filepath.Base(words[0])
	var names []string
	var paths []string
	if bin == "task" {
		if len(words) != 2 || strings.HasPrefix(words[1], "-") {
			return doctorWiringEvidence{unknown: true}
		}
		names = []string{words[1]}
		paths = []string{"Taskfile.yml", "Taskfile.yaml"}
	} else {
		if len(words) != 3 || words[1] != "run" {
			return doctorWiringEvidence{unknown: true}
		}
		names = []string{words[2]}
		paths = []string{"lefthook.yml", "lefthook.yaml"}
	}
	var data []byte
	var err error
	proofFile := ""
	for _, path := range paths {
		data, err = r.read(filepath.Join(r.root, path))
		if !os.IsNotExist(err) {
			proofFile = path
			break
		}
	}
	if err != nil {
		return doctorWiringEvidence{unknown: true}
	}
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil {
		return doctorWiringEvidence{unknown: true}
	}
	var commands []string
	uncertain := false
	if bin == "task" {
		tasks, _ := doc["tasks"].(map[string]any)
		entry, ok := tasks[names[0]]
		if !ok {
			return doctorWiringEvidence{unknown: true}
		}
		switch v := entry.(type) {
		case string:
			commands = append(commands, v)
		case map[string]any:
			_, uncertain = v["deps"]
			if _, ok := v["dir"]; ok {
				return doctorWiringEvidence{unknown: true}
			}
			cmds, ok := v["cmds"].([]any)
			if !ok {
				return doctorWiringEvidence{unknown: true}
			}
			for _, cmd := range cmds {
				switch c := cmd.(type) {
				case string:
					commands = append(commands, c)
				case map[string]any:
					if task, ok := c["task"].(string); ok {
						commands = append(commands, "task "+task)
					} else {
						return doctorWiringEvidence{unknown: true}
					}
				default:
					return doctorWiringEvidence{unknown: true}
				}
			}
		default:
			return doctorWiringEvidence{unknown: true}
		}
	} else {
		entry, ok := doc[names[0]].(map[string]any)
		if !ok {
			return doctorWiringEvidence{unknown: true}
		}
		_, scripts := entry["scripts"]
		_, jobs := entry["jobs"]
		uncertain = scripts || jobs
		cmds, ok := entry["commands"].(map[string]any)
		if !ok {
			return doctorWiringEvidence{unknown: true}
		}
		keys := make([]string, 0, len(cmds))
		for k := range cmds {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c, ok := cmds[k].(map[string]any)
			if !ok {
				return doctorWiringEvidence{unknown: true}
			}
			if _, ok := c["root"]; ok {
				return doctorWiringEvidence{unknown: true}
			}
			run, ok := c["run"].(string)
			if !ok {
				return doctorWiringEvidence{unknown: true}
			}
			commands = append(commands, run)
		}
	}
	target := proofFile + " (" + bin + " " + strings.Join(names, " ") + ")"
	if depth > doctorWrapperLevels {
		// Only a task the normal lookup can scan is a resolved cut target.
		r.depthCuts = append(r.depthCuts, doctorWrapperDepthCut{target: target})
		return doctorWiringEvidence{unknown: true}
	}
	r.markWalked(target)
	firstCut := len(r.depthCuts)
	ev := r.command(strings.Join(commands, "\n"), sub, depth)
	r.labelDepthCuts(firstCut, target)
	ev.unknown = ev.unknown || uncertain
	if ev.found && ev.file == "" {
		ev.file = proofFile
	}
	return ev
}

func doctorHookEvidence(ctx context.Context, root string, hooksObj map[string]any, spec hookSpec) doctorWiringEvidence {
	groups, _ := hooksObj[spec.event].([]any)
	var out doctorWiringEvidence
	for _, g := range groups {
		group, _ := g.(map[string]any)
		matcher, _ := group["matcher"].(string)
		if matcher != spec.matcher {
			continue
		}
		hooks, _ := group["hooks"].([]any)
		for _, h := range hooks {
			hook, _ := h.(map[string]any)
			command, _ := hook["command"].(string)
			// Preserve the existing settings marker contract, including
			// template guards and explicit launcher prefixes. A literal
			// invocation of another hook does not make this hook uncertain.
			if strings.Contains(command, spec.marker) {
				return doctorWiringEvidence{found: true}
			}
			if strings.Contains(command, "fathom ") {
				continue
			}
			ev := doctorCommandEvidence(ctx, root, command, spec.sub)
			if ev.found {
				return ev
			}
			out.unknown = out.unknown || ev.unknown
			if out.missingScript == "" {
				out.missingScript = ev.missingScript
			}
		}
	}
	return out
}

// Project-scope MCP entries are valid wiring too. A stdio wrapper is read,
// never launched (including when it names a task runner).
func checkProjectMCPRegistration(ctx context.Context, root string) (doctorCheck, bool) {
	path := filepath.Join(root, ".mcp.json")
	r := doctorWrapperReader{ctx: ctx, root: root, remaining: doctorWrapperBytes}
	data, err := r.read(path)
	if os.IsNotExist(err) {
		return doctorCheck{}, false
	}
	if err != nil {
		return doctorCheck{Name: "mcp_registration", Status: doctorUnknown, Reason: "unknown: project MCP config unreadable or exceeds 64 KiB"}, true
	}
	var doc struct {
		Servers map[string]struct {
			Type    string   `json:"type"`
			URL     string   `json:"url"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doctorCheck{Name: "mcp_registration", Status: doctorUnknown, Reason: "unknown: project MCP config did not parse"}, true
	}
	entry, ok := doc.Servers["fathom"]
	if !ok {
		return doctorCheck{}, false
	}
	if entry.Command != "" {
		return doctorStdioRegistration(ctx, root, path, entry.Command, entry.Args), true
	}
	if registeredAtAgentSurface(entry.URL) {
		return doctorCheck{Name: "mcp_registration", Status: doctorPass, Reason: "configured: project MCP registration at " + entry.URL}, true
	}
	return doctorCheck{Name: "mcp_registration", Status: doctorFail, Reason: "project MCP registration does not point at /rpc/agent", Remedy: "correct the fathom entry in .mcp.json"}, true
}

func doctorStdioRegistration(ctx context.Context, root, path, command string, args []string) doctorCheck {
	row := doctorCheck{Name: "mcp_registration", Status: doctorUnknown, Reason: "unknown: stdio wrapper unreadable, ambiguous, or exceeds two levels/64 KiB in " + path}
	// argv preserves spaces in an executable's path; an ordinary fathom
	// binary is configuration evidence regardless of its argument spelling.
	if filepath.Base(command) == "fathom" {
		row.Status = doctorPass
		row.Reason = "configured: stdio fathom entry in " + path
		return row
	}
	ev := doctorCommandEvidence(ctx, root, strings.Join(append([]string{command}, args...), " "), "serve-mcp")
	if ev.found {
		row.Status = doctorPass
		row.Reason = "configured (indirect): stdio fathom wrapper in " + path
	}
	if ctx.Err() != nil {
		row.Status = doctorTimeout
		row.Reason = "timeout inspecting stdio wrapper in " + path
	}
	return row
}
