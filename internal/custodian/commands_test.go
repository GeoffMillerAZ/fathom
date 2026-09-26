package custodian

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// chainRepo writes a repo whose gate chain — docs → lefthook → Taskfile →
// scripts — has one broken link per corpus, and the prose, comments,
// quoted strings and descriptions that a grep would misread as commands.
func chainRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), strings.Join([]string{
		"# router", // 1
		"Run `task validate` before pushing; `task deploy` ships.", // 2
		"This task never ingests; neither does `task`.",            // 3
		"```sh",                    // 4
		"./scripts/ci.sh --strict", // 5
		"```",                      // 6
		"A pointer, not a command: `scripts/gone.sh`.", // 7
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), strings.Join([]string{
		"# task nope — a comment names nothing", // 1
		"pre-commit:",                     // 2
		"  commands:",                     // 3
		"    fmt:",                        // 4
		"      run: |",                    // 5
		"        [ -z \"$x\" ] && exit 0", // 6
		"        bash scripts/fmt.sh {staged_files}", // 7
		"pre-push:",                // 8
		"  commands:",              // 9
		"    gate:",                // 10
		"      run: task validate", // 11
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), strings.Join([]string{
		"version: '3'", // 1
		"includes:",    // 2
		"  ui: ./ui",   // 3
		"  remote: https://example.invalid/Taskfile.yml", // 4
		"  missing: ./missing/Taskfile.yml",              // 5
		"tasks:",                                         // 6
		"  validate:",                                    // 7
		"    desc: this task never ingests anything",     // 8
		"    deps: [test, gone]",                         // 9
		"    cmds:",                                      // 10
		"      - task: test",                             // 11
		"      - bash scripts/ci.sh",                     // 12
		"      - echo \"no task the corpus runs\"",       // 13
		"      - task ui:css",                            // 14
		"      - task ui:nope",                           // 15
		"      - task remote:anything",                   // 16
		"      - task --list",                            // 17
		"  test:",                                        // 18
		"    cmds: [go test ./...]",                      // 19
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "ui", "Taskfile.yml"), "version: '3'\ntasks:\n  css:\n    cmds: [bash ../scripts/ci.sh]\n")
	writeTestFile(t, filepath.Join(root, "scripts", "ci.sh"), "#!/bin/sh\n")
	return root
}

// TestChainChecks_DocsLefthookTaskfileScripts is CC-12's chain contract:
// commands_resolve, one kind over three corpora, finds exactly the broken
// link in each — a documented target that is gone, a hook's missing
// script, a Taskfile dep / namespaced call / include that is gone — with
// file:line evidence, and reads nothing from prose, comments, quoted
// strings or `desc:`. Mending the links turns every one to holds.
func TestChainChecks_DocsLefthookTaskfileScripts(t *testing.T) {
	root := chainRepo(t)
	chain := func(where string) Concern {
		return Concern{ID: where + "_chain", Want: "every command resolves", Check: Check{Kind: KindCommandsResolve, Where: where}}
	}
	fs := Audit(root, []Concern{chain(CorpusAgentConfig), chain(CorpusLefthook), chain(CorpusTaskfile), chain(CorpusCI)})

	wantEvidence := map[string][]string{
		CorpusAgentConfig: {"CLAUDE.md:2 → task deploy (no such target)"},
		CorpusLefthook:    {"lefthook.yml:7 → scripts/fmt.sh (no such file)"},
		CorpusTaskfile: {
			"Taskfile.yml:9 → task gone (no such target)",
			`Taskfile.yml:15 → task ui:nope (no such target)`,
			`Taskfile.yml → Taskfile.yml includes "missing" → ./missing/Taskfile.yml, which does not exist`,
		},
	}
	for i, where := range []string{CorpusAgentConfig, CorpusLefthook, CorpusTaskfile} {
		f := fs[i]
		if f.Status != StatusViolated {
			t.Fatalf("%s: want violated, got %+v", where, f)
		}
		if !reflect.DeepEqual(f.Evidence, wantEvidence[where]) {
			t.Errorf("%s evidence =\n  %q\nwant\n  %q", where, f.Evidence, wantEvidence[where])
		}
	}
	if !strings.Contains(fs[2].Detail, "1 not checkable") {
		t.Errorf("the remote-include ref must be counted as not checkable, never guessed: %q", fs[2].Detail)
	}
	// A repo with no CI config: the corpus is named empty, not a vacuous pass.
	if fs[3].Status != StatusUnable || !strings.Contains(fs[3].Detail, "resolved no files") {
		t.Errorf("an empty corpus must be unable-to-check naming it, got %+v", fs[3])
	}

	// Mend every link.
	writeTestFile(t, filepath.Join(root, "scripts", "fmt.sh"), "#!/bin/sh\n")
	tf, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	mended := strings.NewReplacer(
		"deps: [test, gone]", "deps: [test]",
		"      - task ui:nope\n", "",
		"  missing: ./missing/Taskfile.yml\n", "",
		"  test:\n", "  deploy:\n    cmds: [echo ship]\n  test:\n",
	).Replace(string(tf))
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), mended)
	fs = Audit(root, []Concern{chain(CorpusAgentConfig), chain(CorpusLefthook), chain(CorpusTaskfile)})
	for _, f := range fs {
		if f.Status != StatusHolds {
			t.Errorf("%s: mended chain must hold, got %+v", f.Concern.Check.Where, f)
		}
	}
	if !strings.Contains(fs[0].Detail, "3 command reference(s) resolve") {
		t.Errorf("agent-config: validate, deploy and the fenced ./scripts/ci.sh resolve (prose and the bare pointer are not commands): %q", fs[0].Detail)
	}
}

// TestShellRefsReadsOnlyCommands pins the precision rules one line at a
// time: command position only, quotes blanked, comments cut, flags and
// templates uncheckable rather than guessed.
func TestShellRefsReadsOnlyCommands(t *testing.T) {
	for _, tc := range []struct {
		line    string
		want    []string
		unknown int
	}{
		{"task validate", []string{"task validate"}, 0},
		{"FOO=1 task build -- extra", []string{"task build"}, 0},
		{"cd x && task ui:css || true", []string{"task ui:css"}, 0},
		{`echo "run task validate first"`, nil, 0},
		{"echo ok # task nope", nil, 0},
		{"task --list", nil, 0},
		{"task -d sub build", nil, 1},
		{"task {{.TARGET}}", nil, 1},
		{"bash scripts/ci.sh --strict", []string{"scripts/ci.sh"}, 0},
		{"bash -n scripts/ci.sh", []string{"scripts/ci.sh"}, 0},
		{"./tools/gen.py > out", []string{"./tools/gen.py"}, 0},
		{"bash $DIR/run.sh", nil, 1},
		{"go test ./...", nil, 0},
		{"if true; then task deploy; fi", []string{"task deploy"}, 0},
	} {
		refs, unk := shellRefs(tc.line, true)
		var got []string
		for _, r := range refs {
			if r.Script {
				got = append(got, r.Name)
			} else {
				got = append(got, "task "+r.Name)
			}
		}
		if !reflect.DeepEqual(got, tc.want) || unk != tc.unknown {
			t.Errorf("%q → %v (unknown %d), want %v (unknown %d)", tc.line, got, unk, tc.want, tc.unknown)
		}
	}
}

// TestRequiredPattern: at least one line must match; an empty corpus is a
// violation (nothing there does the required thing), never a vacuous
// pass; the matching lines are the evidence.
func TestRequiredPattern(t *testing.T) {
	root := t.TempDir()
	c := Concern{ID: "gate_verify", Want: "the hook runs verify", Check: Check{Kind: KindRequiredPattern, Pattern: `fathom\s+verify\b`, Where: CorpusLefthook}}

	fs := Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated || len(fs[0].Files) != 0 || !strings.Contains(fs[0].Detail, "no lefthook file(s) at all") {
		t.Fatalf("no lefthook config: want a named violation, got %+v", fs[0])
	}
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), "pre-push:\n  commands:\n    lint:\n      run: golangci-lint run\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated || !strings.Contains(fs[0].Detail, "no line matches") {
		t.Fatalf("no matching line: %+v", fs[0])
	}
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), "pre-push:\n  commands:\n    gate:\n      run: bin/fathom verify --service s\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusHolds || !reflect.DeepEqual(fs[0].Evidence, []string{"lefthook.yml:4"}) {
		t.Fatalf("match must hold with the line as evidence: %+v", fs[0])
	}
}

// TestStanzaPresentKind: the three-case detection with file-prefixed
// details (doctor renders them verbatim, CC-13); absent is violated,
// unreadable is unable-to-check (I38).
func TestStanzaPresentKind(t *testing.T) {
	root := t.TempDir()
	c := Concern{ID: "wired", Want: "CLAUDE.md is wired", Check: Check{Kind: KindStanzaPresent, File: "CLAUDE.md"}}
	cases := []struct {
		body   string
		status Status
		detail string
	}{
		{"", StatusViolated, "CLAUDE.md: open "},
		{"# no markers\n", StatusViolated, "CLAUDE.md: never wired (no fathom markers)"},
		{"x\n" + stanzaBeginPrefix + " -->\nbody\n", StatusViolated, "CLAUDE.md: corrupted marker pair (begin=true end=false)"},
		{stanzaEnd + "\n" + stanzaBeginPrefix + " -->\n", StatusViolated, "CLAUDE.md: corrupted marker pair (begin=true end=true)"},
		{"x\n" + stanzaBeginPrefix + " v3 sha256:ab -->\nbody\n" + stanzaEnd + "\n", StatusHolds, "CLAUDE.md: fathom stanza well-formed"},
	}
	for i, tc := range cases {
		if i > 0 {
			writeTestFile(t, filepath.Join(root, "CLAUDE.md"), tc.body)
		}
		f := Audit(root, []Concern{c})[0]
		if f.Status != tc.status || !strings.HasPrefix(f.Detail, tc.detail) {
			t.Errorf("case %d: got %s %q, want %s %q", i, f.Status, f.Detail, tc.status, tc.detail)
		}
	}
	if f := Audit(root, []Concern{c})[0]; f.Status == StatusHolds && f.Evidence != nil {
		t.Errorf("a holding stanza carries no evidence: %+v", f)
	}
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "x\n"+stanzaBeginPrefix+" -->\n")
	if f := Audit(root, []Concern{c})[0]; !reflect.DeepEqual(f.Evidence, []string{"CLAUDE.md:2"}) {
		t.Errorf("a corrupted pair locates the marker it found: %v", f.Evidence)
	}

	if os.Geteuid() != 0 {
		p := filepath.Join(root, "CLAUDE.md")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		if f := Audit(root, []Concern{c})[0]; f.Status != StatusUnable || !strings.Contains(f.Detail, "permission denied") {
			t.Errorf("an unreadable CLAUDE.md is unable-to-check naming the errno, never absent: %+v", f)
		}
	}
}

// TestMaxBytes: absent is a named violation; over budget names both
// numbers; within budget holds.
func TestMaxBytes(t *testing.T) {
	root := t.TempDir()
	c := Concern{ID: "fits", Want: "AGENTS.md fits", Check: Check{Kind: KindMaxBytes, File: "AGENTS.md", Bytes: 8}}
	if f := Audit(root, []Concern{c})[0]; f.Status != StatusViolated || !strings.Contains(f.Detail, "absent") {
		t.Fatalf("absent: %+v", f)
	}
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "0123456789")
	if f := Audit(root, []Concern{c})[0]; f.Status != StatusViolated || f.Detail != "AGENTS.md is 10 bytes (max 8)" {
		t.Fatalf("over budget: %+v", f)
	}
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "01234567")
	if f := Audit(root, []Concern{c})[0]; f.Status != StatusHolds {
		t.Fatalf("at budget must hold: %+v", f)
	}
}
