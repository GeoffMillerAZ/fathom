package labs

import (
	"encoding/json"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const evalTool = "mcp__plugin_fathom_fathom__find_symbol"

func evalFixture(t *testing.T) (EvalLabCard, []LabEvalCase, LabEvalOptions) {
	t.Helper()
	root := t.TempDir()
	rendered := filepath.Join(root, "rendered")
	writeLabObservation(t, rendered, ".claude-plugin/plugin.json", `{"name":"fathom"}`)
	for _, name := range []string{".mcp.json", "mcp.json"} {
		writeLabObservation(t, rendered, name, `{"mcpServers":{"fathom":{"type":"http","url":"http://127.0.0.1:19555/rpc/agent","headers":{"keep":"yes"}}}}`)
	}
	writeLabObservation(t, rendered, "skills/doctor/SKILL.md", "doctor\n")
	writeLabObservation(t, rendered, "skills/other/SKILL.md", "other\n")
	writeLabObservation(t, rendered, "hooks/hooks.json", `{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"command -v fathom >/dev/null 2>&1 || exit 0; fathom edit-hook --pre","timeout":10},{"type":"command","command":"command -v fathom >/dev/null 2>&1 || exit 0; fathom edit-hook","timeout":20}]}],"Stop":[{"hooks":[{"type":"command","command":"command -v fathom >/dev/null 2>&1 || exit 0; fathom stop-hook"}]}]}}`)
	writeLabObservation(t, rendered, "evals/stale/prompt.md", "must not copy")
	prefix := filepath.Join(root, "private-prefix")
	if err := os.WriteFile(prefix, []byte("opaque prefix\n"), 0600); err != nil {
		t.Fatal(err)
	}
	zero, one := 0, 1
	card := EvalLabCard{ID: "lab-find-symbol", Surface: []string{"mcp:find_symbol", "hook:edit-hook --pre", "skill:doctor"}}
	cases := []LabEvalCase{
		{ID: "z-negative", Lab: card.ID, Input: LabEvalInput{Prompt: "Find literal config", Graders: []LabEvalGrader{{Name: "quiet", Type: "tool_used", Tool: evalTool, Min: &zero, Max: &zero}}}},
		{ID: "a-positive", Lab: card.ID, Input: LabEvalInput{Prompt: "Find Widget\nKeep it concise.", PrefixFile: prefix, Graders: []LabEvalGrader{
			{Name: "z-order", Type: "tool_order", Before: evalTool, After: "Read"},
			{Name: "a-used", Type: "tool_used", Tool: evalTool, InputMatch: "Widget", Min: &one, Max: &one},
			{Name: "m-answer", Type: "regex", Pattern: "Widget", Flags: "i", Match: "contains", Target: "last_message"},
		}}},
	}
	return card, cases, LabEvalOptions{Root: root, Run: "fixture", RenderedPlugin: rendered, Feature: "mcp:find_symbol", MaxTurns: 4, AllowedTools: []string{"Read", "Bash(git:*)", "Read"}}
}

func evalRead(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func evalYAML(t *testing.T, text string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := yaml.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func evalSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = evalRead(t, p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRenderLabEvalSuiteFilesAndCommands(t *testing.T) {
	card, cases, opts := evalFixture(t)
	source := evalSnapshot(t, opts.RenderedPlugin)
	got, err := RenderLabEvalSuite(card, cases, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []LabEvalCommand{}
	for _, arm := range []string{"minus", "with"} {
		dir := "workspace/labs/runs/fixture/" + arm
		args := []string{"claude", "plugin", "eval", "./" + dir, "--eval-dir", "evals", "--no-publish", "--ablation", "none", "--runs", "3", "--allow-tools", "Bash(git:*)", "--allow-tools", "Read"}
		if arm == "with" {
			args = append(args, "--allow-tools", evalTool)
		}
		want = append(want, LabEvalCommand{Arm: arm, Directory: dir, Args: args})
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands:\n%#v\nwant %#v", got, want)
	}
	for _, cmd := range got {
		base := filepath.Join(opts.Root, cmd.Directory)
		if _, err := os.Stat(filepath.Join(base, "evals/stale")); !os.IsNotExist(err) {
			t.Fatal("copied stale suite")
		}
		prompt := evalRead(t, filepath.Join(base, "evals/a-positive/prompt.md"))
		parts := strings.SplitN(prompt, "---\n", 3)
		if len(parts) != 3 {
			t.Fatalf("frontmatter: %q", prompt)
		}
		meta := evalYAML(t, parts[1])
		if !reflect.DeepEqual(meta, map[string]any{"name": "a-positive", "runs": 3, "max_turns": 4}) || parts[2] != "Find Widget\nKeep it concise.\n" {
			t.Fatalf("prompt: %q", prompt)
		}
		context := evalYAML(t, evalRead(t, filepath.Join(base, "evals/a-positive/case.yaml")))
		if !reflect.DeepEqual(context, map[string]any{"schema_version": "1.1", "name": "a-positive", "context": map[string]any{"history_file": "history.prefix"}}) {
			t.Fatalf("context: %#v", context)
		}
		if evalRead(t, filepath.Join(base, "evals/a-positive/history.prefix")) != "opaque prefix\n" {
			t.Fatal("prefix changed")
		}
		for name, expected := range map[string]map[string]any{
			"a-used":   {"type": "tool_used", "tool": evalTool, "input_match": "Widget", "min": 1, "max": 1},
			"m-answer": {"type": "regex", "pattern": "Widget", "flags": "i", "match": "contains", "target": "last_message"},
			"z-order":  {"type": "tool_order", "before": evalTool, "after": "Read"},
		} {
			text := evalRead(t, filepath.Join(base, "evals/a-positive/graders/"+name+".md"))
			if !reflect.DeepEqual(evalYAML(t, strings.Split(text, "---\n")[1]), expected) {
				t.Fatalf("grader %s: %s", name, text)
			}
		}
		quiet := evalRead(t, filepath.Join(base, "evals/z-negative/graders/quiet.md"))
		if !reflect.DeepEqual(evalYAML(t, strings.Split(quiet, "---\n")[1]), map[string]any{"type": "tool_used", "tool": evalTool, "min": 0, "max": 0}) {
			t.Fatalf("must-not: %s", quiet)
		}
		for _, text := range evalSnapshot(t, base) {
			if strings.Contains(text, opts.Root) {
				t.Fatal("absolute path leaked")
			}
		}
	}
	if !reflect.DeepEqual(source, evalSnapshot(t, opts.RenderedPlugin)) {
		t.Fatal("modified renderer input")
	}
	// Reordering cases and graders must not change the tree or commands.
	first := evalSnapshot(t, filepath.Join(opts.Root, "workspace"))
	cases[0], cases[1] = cases[1], cases[0]
	cases[0].Input.Graders[0], cases[0].Input.Graders[2] = cases[0].Input.Graders[2], cases[0].Input.Graders[0]
	opts.Root = t.TempDir()
	again, err := RenderLabEvalSuite(card, cases, opts)
	if err != nil || !reflect.DeepEqual(got, again) || !reflect.DeepEqual(first, evalSnapshot(t, filepath.Join(opts.Root, "workspace"))) {
		t.Fatalf("not deterministic: %v", err)
	}
}

func TestRenderLabEvalSuiteFeatureRemoval(t *testing.T) {
	for _, feature := range []string{"skill:doctor", "hook:edit-hook --pre"} {
		t.Run(feature, func(t *testing.T) {
			card, cases, opts := evalFixture(t)
			opts.Feature = feature
			commands, err := RenderLabEvalSuite(card, cases, opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(commands) != 2 {
				t.Fatalf("commands: %#v", commands)
			}
			plus := filepath.Join(opts.Root, commands[1].Directory)
			minus := filepath.Join(opts.Root, commands[0].Directory)
			if feature == "skill:doctor" {
				if _, err := os.Stat(filepath.Join(minus, "skills/doctor")); !os.IsNotExist(err) {
					t.Fatal("skill retained")
				}
				if evalRead(t, filepath.Join(plus, "skills/doctor/SKILL.md")) != "doctor\n" || evalRead(t, filepath.Join(minus, "skills/other/SKILL.md")) != "other\n" {
					t.Fatal("wrong skill removed")
				}
			} else {
				var hooks map[string]any
				if err := json.Unmarshal([]byte(evalRead(t, filepath.Join(minus, "hooks/hooks.json"))), &hooks); err != nil {
					t.Fatal(err)
				}
				text, _ := json.Marshal(hooks)
				if strings.Contains(string(text), "edit-hook --pre") || !strings.Contains(string(text), `fathom edit-hook"`) || !strings.Contains(string(text), "stop-hook") || !strings.Contains(string(text), `"timeout":20`) {
					t.Fatalf("wrong hook removal: %s", text)
				}
			}
		})
	}
}

func TestRenderLabEvalSuiteHelpedScratch(t *testing.T) {
	card, cases, opts := evalFixture(t)
	opts.Helped = true
	opts.ScratchMCPURL = "http://127.0.0.1:19757/rpc/agent"
	commands, err := RenderLabEvalSuite(card, cases, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 {
		t.Fatal("no paired commands")
	}
	for _, c := range commands {
		if !reflect.DeepEqual(c.Args[len(c.Args)-2:], []string{"--mocks", "off"}) {
			t.Fatalf("mock flags: %v", c.Args)
		}
		for _, name := range []string{".mcp.json", "mcp.json"} {
			text := evalRead(t, filepath.Join(opts.Root, c.Directory, name))
			if !strings.Contains(text, opts.ScratchMCPURL) || strings.Contains(text, ":19555/") || !strings.Contains(text, `"keep": "yes"`) {
				t.Fatalf("scratch endpoint: %s", text)
			}
		}
	}
}

func TestRenderLabEvalSuiteRefusals(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*EvalLabCard, []LabEvalCase, *LabEvalOptions)
	}{
		{"llm", "a-used: grader llm", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].Input.Graders[1].Type = "llm" }},
		{"baseline", "grader baseline", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].Input.Graders[1].Type = "baseline" }},
		{"unknown-grader", "file_exists", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].Input.Graders[1].Type = "file_exists" }},
		{"escape-run", "run", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) { o.Run = "../escape" }},
		{"escape-case", "case", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].ID = "../escape" }},
		{"duplicate-case", "duplicate case", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].ID = c[0].ID }},
		{"duplicate-grader", "duplicate grader", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) {
			c[1].Input.Graders[1].Name = c[1].Input.Graders[0].Name
		}},
		{"wrong-lab", "lab", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].Lab = "lab-other" }},
		{"turn-cap", "max turns", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) { o.MaxTurns = 0 }},
		{"feature", "surface", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) { o.Feature = "mcp:unknown" }},
		{"missing-skill", "skill", func(c *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) {
			c.Surface = append(c.Surface, "skill:absent")
			o.Feature = "skill:absent"
		}},
		{"missing-hook", "hook", func(c *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) {
			c.Surface = append(c.Surface, "hook:absent")
			o.Feature = "hook:absent"
		}},
		{"missing-prefix", "prefix", func(_ *EvalLabCard, c []LabEvalCase, _ *LabEvalOptions) { c[1].Input.PrefixFile = "does-not-exist" }},
		{"no-scratch", "scratch", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) { o.Helped = true }},
		{"managed-daemon", "scratch", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) {
			o.Helped = true
			// Construct the managed endpoint solely to test refusal; never dial it.
			o.ScratchMCPURL = "http://" + net.JoinHostPort("127.0.0.1", "7575") + "/rpc/agent"
		}},
		{"remote-daemon", "scratch", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) {
			o.Helped = true
			o.ScratchMCPURL = "http://192.0.2.1:19757/rpc/agent"
		}},
		{"wildcard-grant", "allow-tools", func(_ *EvalLabCard, _ []LabEvalCase, o *LabEvalOptions) { o.AllowedTools = []string{"mcp__*"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			card, cases, opts := evalFixture(t)
			tc.change(&card, cases, &opts)
			_, err := RenderLabEvalSuite(card, cases, opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q error, got %v", tc.want, err)
			}
		})
	}
}

// TestRenderLabEvalSuiteKeepsCommandlessHooks: a prompt, http or agent hook
// has no command, so it is never the removed feature and must not fail the
// render.
func TestRenderLabEvalSuiteKeepsCommandlessHooks(t *testing.T) {
	card, cases, opts := evalFixture(t)
	writeLabObservation(t, opts.RenderedPlugin, "hooks/hooks.json", `{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"prompt","prompt":"keep me"},{"type":"command","command":"command -v fathom >/dev/null 2>&1 || exit 0; fathom edit-hook --pre"}]}]}}`)
	opts.Feature = "hook:edit-hook --pre"
	commands, err := RenderLabEvalSuite(card, cases, opts)
	if err != nil {
		t.Fatal(err)
	}
	minus := evalRead(t, filepath.Join(opts.Root, commands[0].Directory, "hooks/hooks.json"))
	if !strings.Contains(minus, "keep me") || strings.Contains(minus, "edit-hook --pre") {
		t.Fatalf("minus copy hooks: %s", minus)
	}
}
