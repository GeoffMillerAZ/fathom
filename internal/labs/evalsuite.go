package labs

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// EvalLabCard projects the LAB-1 #Lab fields used by the fork renderer.
// The registry loader owns validation of the full card.
type EvalLabCard struct {
	ID      string
	Surface []string
}

// LabEvalCase projects #Case with a fork-specific input payload. The case
// loader owns provenance, split and full #Case validation.
type LabEvalCase struct {
	ID    string
	Lab   string
	Input LabEvalInput
}

type LabEvalInput struct {
	Prompt     string
	PrefixFile string
	Graders    []LabEvalGrader
}

// LabEvalGrader admits only the three documented free grader types below.
// Min and Max are pointers so an explicit zero survives serialization.
type LabEvalGrader struct {
	Name, Type            string
	Tool, InputMatch      string
	Min, Max              *int
	Before, After         string
	Pattern, Flags, Match string
	Target                string
}

// LabEvalOptions describes local rendering only. Root is the project root;
// commands are argv vectors to run from Root, never shell strings. RenderedPlugin
// must be the output of cmd/fathom's existing plugin renderer, which package
// internal/labs cannot import. Helped requires a caller-owned scratch daemon;
// ScratchMCPURL configures both copies but does not start or verify that daemon.
type LabEvalOptions struct {
	Root, Run, RenderedPlugin, Feature string
	MaxTurns                           int
	AllowedTools                       []string
	Helped                             bool
	ScratchMCPURL                      string
}

type LabEvalCommand struct {
	Arm, Directory string
	Args           []string
}

// RenderLabEvalSuite writes two plugin copies and returns commands without
// executing an eval. Claude Code 2.1.310 plugin eval --help and its embedded
// quick reference pin the output format. history_file path semantics and its
// transcript format are NOT documented; the prefix is copied opaquely beside
// case.yaml. No grader weight or threshold-based gate is inferred.
func RenderLabEvalSuite(card EvalLabCard, cases []LabEvalCase, opts LabEvalOptions) ([]LabEvalCommand, error) {
	if !labLinkPattern.MatchString(card.ID + "/case") {
		return nil, fmt.Errorf("invalid lab id %q", card.ID)
	}
	if !evalName.MatchString(opts.Run) || opts.Run == "." || opts.Run == ".." {
		return nil, fmt.Errorf("invalid run id %q", opts.Run)
	}
	if opts.MaxTurns < 1 || opts.MaxTurns > 10 {
		return nil, fmt.Errorf("max turns must be between 1 and 10")
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no cases")
	}
	if !slices.Contains(card.Surface, opts.Feature) {
		return nil, fmt.Errorf("feature %q is not in lab surface", opts.Feature)
	}
	kind, feature, ok := strings.Cut(opts.Feature, ":")
	if !ok || (kind != "mcp" && kind != "hook" && kind != "skill") || strings.TrimSpace(feature) == "" {
		return nil, fmt.Errorf("unsupported surface %q", opts.Feature)
	}
	if kind != "hook" && !evalName.MatchString(feature) {
		return nil, fmt.Errorf("invalid surface %q", opts.Feature)
	}
	if opts.Helped {
		u, err := url.Parse(opts.ScratchMCPURL)
		if err != nil || u.Scheme != "http" || !net.ParseIP(u.Hostname()).IsLoopback() || u.Port() == "" || u.Port() == "7575" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("helped suite requires a scratch loopback HTTP MCP URL with a non-managed port")
		}
	} else if opts.ScratchMCPURL != "" {
		return nil, fmt.Errorf("scratch MCP URL requires a helped suite")
	}
	manifest, err := os.ReadFile(filepath.Join(opts.RenderedPlugin, ".claude-plugin", "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("rendered plugin manifest: %w", err)
	}
	var plugin struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(manifest, &plugin); err != nil || !evalName.MatchString(plugin.Name) {
		return nil, fmt.Errorf("rendered plugin manifest requires a plugin name")
	}
	// MCP grants must be exact names: a wildcard could re-enable the minus verb.
	tools := append([]string(nil), opts.AllowedTools...)
	for _, tool := range tools {
		if !evalGrant.MatchString(tool) || (strings.HasPrefix(tool, "mcp__") && strings.ContainsAny(tool, "(*")) {
			return nil, fmt.Errorf("allow-tools requires exact MCP names or a named built-in grant: %q", tool)
		}
	}
	targetTool := ""
	if kind == "mcp" {
		targetTool = "mcp__plugin_" + plugin.Name + "_fathom__" + feature
		tools = append(tools, targetTool)
	}
	sort.Strings(tools)
	tools = slices.Compact(tools)
	ordered := append([]LabEvalCase(nil), cases...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	prefixes := map[string][]byte{}
	for i, c := range ordered {
		if !evalName.MatchString(c.ID) || c.ID == "." || c.ID == ".." {
			return nil, fmt.Errorf("invalid case id %q", c.ID)
		}
		if i > 0 && ordered[i-1].ID == c.ID {
			return nil, fmt.Errorf("duplicate case %q", c.ID)
		}
		if c.Lab != card.ID {
			return nil, fmt.Errorf("case %s belongs to lab %q, want %s", c.ID, c.Lab, card.ID)
		}
		if strings.TrimSpace(c.Input.Prompt) == "" || len(c.Input.Graders) == 0 {
			return nil, fmt.Errorf("case %s requires prompt and graders", c.ID)
		}
		graders := append([]LabEvalGrader(nil), c.Input.Graders...)
		sort.Slice(graders, func(i, j int) bool { return graders[i].Name < graders[j].Name })
		for j, g := range graders {
			if !evalName.MatchString(g.Name) || g.Name == "." || g.Name == ".." {
				return nil, fmt.Errorf("case %s: invalid grader name %q", c.ID, g.Name)
			}
			if j > 0 && graders[j-1].Name == g.Name {
				return nil, fmt.Errorf("case %s: duplicate grader %q", c.ID, g.Name)
			}
			if _, err := evalGraderFields(g); err != nil {
				return nil, fmt.Errorf("case %s: %s: %w", c.ID, g.Name, err)
			}
		}
		ordered[i].Input.Graders = graders
		if c.Input.PrefixFile != "" {
			prefix, err := os.ReadFile(c.Input.PrefixFile)
			if err != nil {
				return nil, fmt.Errorf("case %s prefix: %w", c.ID, err)
			}
			prefixes[c.ID] = prefix
		}
	}
	runRel := filepath.Join("workspace", "labs", "runs", opts.Run)
	runPath := filepath.Join(opts.Root, runRel)
	sourceAbs, err := filepath.Abs(opts.RenderedPlugin)
	if err != nil {
		return nil, err
	}
	runAbs, err := filepath.Abs(runPath)
	if err != nil {
		return nil, err
	}
	if rel, err := filepath.Rel(sourceAbs, runAbs); err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return nil, fmt.Errorf("rendered plugin must not contain the output run")
	}
	// Never overwrite a previous run or follow output symlinks.
	parent := opts.Root
	for _, part := range []string{"workspace", "labs", "runs"} {
		parent = filepath.Join(parent, part)
		if err := os.Mkdir(parent, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
		info, err := os.Lstat(parent)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("run parent is not a directory: %s", part)
		}
	}
	if err := os.Mkdir(runPath, 0700); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(runPath)
		}
	}()
	var commands []LabEvalCommand
	for _, arm := range []string{"minus", "with"} {
		rel := filepath.Join(runRel, arm)
		dir := filepath.Join(opts.Root, rel)
		if err := copyEvalPlugin(opts.RenderedPlugin, dir); err != nil {
			return nil, err
		}
		if opts.Helped {
			for _, name := range []string{".mcp.json", "mcp.json"} {
				if err := evalScratchEndpoint(filepath.Join(dir, name), opts.ScratchMCPURL); err != nil {
					return nil, err
				}
			}
		}
		grants := append([]string(nil), tools...)
		if arm == "minus" {
			switch kind {
			case "mcp":
				grants = slices.DeleteFunc(grants, func(t string) bool { return t == targetTool })
			case "skill":
				path := filepath.Join(dir, "skills", feature)
				info, err := os.Stat(filepath.Join(path, "SKILL.md"))
				if err != nil || !info.Mode().IsRegular() {
					return nil, fmt.Errorf("skill %q not present in rendered plugin", feature)
				}
				if err := os.RemoveAll(path); err != nil {
					return nil, err
				}
			case "hook":
				if err := removeEvalHook(filepath.Join(dir, "hooks", "hooks.json"), feature); err != nil {
					return nil, err
				}
			}
		}
		for _, c := range ordered {
			base := filepath.Join(dir, "evals", c.ID)
			prompt, err := evalMarkdown(map[string]any{"name": c.ID, "runs": 3, "max_turns": opts.MaxTurns}, c.Input.Prompt)
			if err != nil {
				return nil, err
			}
			if err := evalWrite(filepath.Join(base, "prompt.md"), prompt); err != nil {
				return nil, err
			}
			context := map[string]any{"schema_version": "1.1", "name": c.ID}
			if prefix, ok := prefixes[c.ID]; ok {
				context["context"] = map[string]any{"history_file": "history.prefix"}
				if err := evalWrite(filepath.Join(base, "history.prefix"), prefix); err != nil {
					return nil, err
				}
			}
			b, err := yaml.Marshal(context)
			if err != nil {
				return nil, err
			}
			if err := evalWrite(filepath.Join(base, "case.yaml"), b); err != nil {
				return nil, err
			}
			for _, g := range c.Input.Graders {
				fields, _ := evalGraderFields(g)
				b, err := evalMarkdown(fields, "")
				if err != nil {
					return nil, err
				}
				if err := evalWrite(filepath.Join(base, "graders", g.Name+".md"), b); err != nil {
					return nil, err
				}
			}
		}
		rel = filepath.ToSlash(rel)
		args := []string{"claude", "plugin", "eval", "./" + rel, "--eval-dir", "evals", "--no-publish", "--ablation", "none", "--runs", "3"}
		// One flag per grant: the CLI's help does not say whether --allow-tools
		// is variadic, and a repeated flag reads the same either way.
		for _, grant := range grants {
			args = append(args, "--allow-tools", grant)
		}
		if opts.Helped {
			args = append(args, "--mocks", "off")
		}
		commands = append(commands, LabEvalCommand{Arm: arm, Directory: rel, Args: args})
	}
	complete = true
	return commands, nil
}

var evalName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var evalGrant = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\([^\r\n]*\))?$`)

func evalGraderFields(g LabEvalGrader) (map[string]any, error) {
	fields := map[string]any{"type": g.Type}
	switch g.Type {
	case "llm", "baseline":
		return nil, fmt.Errorf("grader %s is refused (paid grader)", g.Type)
	case "tool_used":
		if g.Tool == "" {
			return nil, fmt.Errorf("tool_used requires tool")
		}
		min := 1
		if g.Min != nil {
			min = *g.Min
			fields["min"] = min
		}
		if min < 0 || (g.Max != nil && *g.Max < min) {
			return nil, fmt.Errorf("invalid tool_used min/max")
		}
		fields["tool"] = g.Tool
		if g.Max != nil {
			fields["max"] = *g.Max
		}
		if g.InputMatch != "" {
			fields["input_match"] = g.InputMatch
		}
	case "tool_order":
		if g.Before == "" || g.After == "" {
			return nil, fmt.Errorf("tool_order requires before and after")
		}
		fields["before"], fields["after"] = g.Before, g.After
	case "regex":
		if g.Pattern == "" {
			return nil, fmt.Errorf("regex requires pattern")
		}
		fields["pattern"] = g.Pattern
		if g.Flags != "" {
			fields["flags"] = g.Flags
		}
		if g.Match != "" {
			if g.Match != "contains" && g.Match != "not_contains" && !regexp.MustCompile(`^count:[0-9]+$`).MatchString(g.Match) {
				return nil, fmt.Errorf("invalid regex match %q", g.Match)
			}
			fields["match"] = g.Match
		}
		if g.Target != "" {
			if !slices.Contains([]string{"last_message", "trace", "files", "mock_calls"}, g.Target) {
				return nil, fmt.Errorf("unsupported regex target %q", g.Target)
			}
			fields["target"] = g.Target
		}
	default:
		return nil, fmt.Errorf("grader %s is unsupported; use tool_used, tool_order or regex", g.Type)
	}
	return fields, nil
}

func evalMarkdown(fields map[string]any, body string) ([]byte, error) {
	b, err := yaml.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return []byte("---\n" + string(b) + "---\n" + strings.TrimRight(body, "\n") + "\n"), nil
}

func evalWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func copyEvalPlugin(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Replace the suite, never mix it with the renderer input's own cases.
		if rel == "evals" && d.IsDir() {
			return filepath.SkipDir
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("rendered plugin contains symlink: %s", rel)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("rendered plugin contains non-file: %s", rel)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := evalWrite(target, b); err != nil {
			return err
		}
		// Preserve executable scaffold/helper files while keeping prefixes private.
		return os.Chmod(target, 0600|info.Mode().Perm()&0111)
	})
}

func evalScratchEndpoint(path, endpoint string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	servers, ok := obj["mcpServers"].(map[string]any)
	if !ok || len(servers) != 1 {
		return fmt.Errorf("scratch plugin must register only the fathom server")
	}
	server, ok := servers["fathom"].(map[string]any)
	if !ok || (server["type"] != "http" && server["type"] != "streamable-http") {
		return fmt.Errorf("scratch plugin requires an HTTP fathom server")
	}
	server["url"] = endpoint
	b, err = json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return evalWrite(path, append(b, '\n'))
}

func removeEvalHook(path, feature string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("hook: %w", err)
	}
	// Preserve all unrelated group and command fields, including async/timeouts.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	var events map[string][]map[string]json.RawMessage
	if err := json.Unmarshal(obj["hooks"], &events); err != nil {
		return err
	}
	removed := 0
	names := make([]string, 0, len(events))
	for name := range events {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, event := range names {
		var groups []map[string]json.RawMessage
		for _, group := range events[event] {
			var hooks []map[string]json.RawMessage
			if err := json.Unmarshal(group["hooks"], &hooks); err != nil {
				return err
			}
			var kept []map[string]json.RawMessage
			for _, hook := range hooks {
				raw, ok := hook["command"]
				if !ok {
					// A prompt, http or agent hook has no command: it is not this
					// feature, so it stays.
					kept = append(kept, hook)
					continue
				}
				var command string
				if err := json.Unmarshal(raw, &command); err != nil {
					return err
				}
				// Exact command from cmd/fathom hookCommand, not substring matching:
				// removing edit-hook must never also remove edit-hook --pre.
				if command == "command -v fathom >/dev/null 2>&1 || exit 0; fathom "+feature {
					removed++
					continue
				}
				kept = append(kept, hook)
			}
			if len(kept) > 0 {
				group["hooks"], _ = json.Marshal(kept)
				groups = append(groups, group)
			}
		}
		if len(groups) == 0 {
			delete(events, event)
		} else {
			events[event] = groups
		}
	}
	if removed == 0 {
		return fmt.Errorf("hook %q not present in rendered plugin", feature)
	}
	obj["hooks"], err = json.Marshal(events)
	if err != nil {
		return err
	}
	b, err = json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return evalWrite(path, append(b, '\n'))
}
