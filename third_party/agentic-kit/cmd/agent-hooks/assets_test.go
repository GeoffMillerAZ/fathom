package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestHookTemplatesAreValidJSONForAnyPath(t *testing.T) {
	for _, host := range []string{hostClaude, hostCodex} {
		for _, bin := range []string{"/usr/local/bin/agent-hooks", `/Users/a b/it's "here"\bin/agent-hooks`} {
			out, err := hookTemplate(host, bin)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Hooks map[string][]struct {
					Matcher string `json:"matcher"`
					Hooks   []struct {
						Type    string `json:"type"`
						Command string `json:"command"`
						Timeout int    `json:"timeout"`
					} `json:"hooks"`
				} `json:"hooks"`
			}
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("%s template with %q is not JSON: %v", host, bin, err)
			}
			for event, groups := range doc.Hooks {
				for _, g := range groups {
					for _, h := range g.Hooks {
						if h.Type != "command" || h.Timeout <= 0 || firstField(h.Command) != bin || !strings.HasSuffix(h.Command, "hook --host "+host) {
							t.Errorf("%s %s handler = %+v", host, event, h)
						}
					}
				}
			}
		}
	}
}

func TestPortableHomeBinary(t *testing.T) {
	out, err := hookTemplate(hostCodex, "$HOME/.local/bin/agent-hooks")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"command": "\"$HOME\"/.local/bin/agent-hooks hook --host codex"`) {
		t.Fatalf("portable command not rendered as \"$HOME\"/...:\n%s", out)
	}
	ta := newTestApp(t)
	if got := ta.expandHome(firstField(`"$HOME"/.local/bin/agent-hooks hook --host codex`)); got != ta.home+"/.local/bin/agent-hooks" {
		t.Fatalf("doctor resolves the portable command to %q", got)
	}
	if code := ta.run([]string{"print-hooks", "--host", hostClaude, "--bin", "$HOME/.local/bin/agent-hooks"}); code != 0 {
		t.Fatalf("print-hooks rejected a $HOME path: %s", ta.errOut.String())
	}
}

func TestPrintHooksRequiresAbsoluteBin(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.run([]string{"print-hooks", "--host", hostClaude, "--bin", "relative/agent-hooks"}); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if code := ta.run([]string{"print-hooks", "--host", hostCodex, "--bin", "/opt/agent-hooks"}); code != 0 || !strings.Contains(ta.out.String(), "/opt/agent-hooks hook --host codex") {
		t.Fatalf("exit = %d, out = %s", code, ta.out.String())
	}
}

// TestAgentDefinitionsMatchAcrossHosts keeps the Claude Code and Codex
// versions of each agent saying exactly the same thing.
func TestAgentDefinitionsMatchAcrossHosts(t *testing.T) {
	front := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n\n(.*)$`)
	toml := regexp.MustCompile(`(?s)name = "([^"]+)"\ndescription = "([^"]+)"\ndeveloper_instructions = '''\n(.*)'''\n$`)
	claude, codex := map[string][3]string{}, map[string][3]string{}
	for _, ag := range agentAssets() {
		text := string(ag.Content)
		switch ag.Host {
		case hostClaude:
			m := front.FindStringSubmatch(text)
			if m == nil {
				t.Fatalf("%s: no frontmatter", ag.Name)
			}
			meta := map[string]string{}
			for _, line := range strings.Split(m[1], "\n") {
				k, v, _ := strings.Cut(line, ": ")
				meta[k] = v
			}
			if meta["name"]+".md" != ag.Name || meta["model"] == "" {
				t.Errorf("%s: name %q / model %q", ag.Name, meta["name"], meta["model"])
			}
			claude[meta["name"]] = [3]string{meta["name"], meta["description"], m[2]}
		case hostCodex:
			m := toml.FindStringSubmatch(text)
			if m == nil {
				t.Fatalf("%s: unexpected TOML layout", ag.Name)
			}
			if m[1]+".toml" != ag.Name || strings.Contains(text, "\nmodel") {
				t.Errorf("%s: name %q, or it pins a model (the spawn call must decide)", ag.Name, m[1])
			}
			codex[m[1]] = [3]string{m[1], m[2], m[3]}
		}
	}
	if len(claude) == 0 || len(claude) != len(codex) {
		t.Fatalf("claude agents %d, codex agents %d", len(claude), len(codex))
	}
	for name, c := range claude {
		if codex[name] != c {
			t.Errorf("%s differs between hosts", name)
		}
	}
}
