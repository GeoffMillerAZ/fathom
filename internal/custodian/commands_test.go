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
		"```text", // 8
		"task list for the release: none of this is a command", // 9
		"```",                      // 10
		"```",                      // 11: an unlabelled fence is a command line
		"task --list && task nope", // 12
		"```",                      // 13
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
	// An include without `dir:` runs in the ROOT, so its script path is
	// root-relative (TestCommandsResolveFromTheirWorkingDirectory holds the
	// include-relative mistake).
	writeTestFile(t, filepath.Join(root, "ui", "Taskfile.yml"), "version: '3'\ntasks:\n  css:\n    cmds: [bash scripts/ci.sh]\n")
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
		CorpusAgentConfig: {"CLAUDE.md:2 → task deploy (no such target)", "CLAUDE.md:12 → task nope (no such target)"},
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
		"deps: [test, gone]", "deps: [test, nope]",
		"      - task ui:nope\n", "",
		"  missing: ./missing/Taskfile.yml\n", "",
		"  test:\n", "  deploy:\n    cmds: [echo ship]\n  nope:\n    cmds: [echo named]\n  test:\n",
	).Replace(string(tf))
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), mended)
	fs = Audit(root, []Concern{chain(CorpusAgentConfig), chain(CorpusLefthook), chain(CorpusTaskfile)})
	for _, f := range fs {
		if f.Status != StatusHolds {
			t.Errorf("%s: mended chain must hold, got %+v", f.Concern.Check.Where, f)
		}
	}
	if !strings.Contains(fs[0].Detail, "4 command reference(s) resolve") {
		t.Errorf("agent-config: validate, deploy, the sh-fenced ./scripts/ci.sh and the unlabelled fence's nope resolve (prose, a text fence and the bare pointer are not commands): %q", fs[0].Detail)
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

// TestTaskfileIncludeCallsResolveInTheirNamespace: inside an included
// Taskfile, a `task:` call and a `deps:` entry resolve in that include's
// namespace (Task runs `sub:build` for `deps: [build]` in sub/), a leading
// ':' names the root's, and a flattened include shares its parent's
// namespace. Both directions of the root-namespace mistake are pinned: a
// correct intra-include call is never reported dead, and an include
// calling a name only the root defines — which Task fails with `Task
// "sub:lint" does not exist` — never holds.
func TestTaskfileIncludeCallsResolveInTheirNamespace(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), strings.Join([]string{
		"version: '3'",
		"includes:",
		"  sub: ./sub",
		"  flat:",
		"    taskfile: ./flat",
		"    flatten: true",
		"tasks:",
		"  validate:",
		"    cmds:",
		"      - task: sub:check",
		"  lint:",
		"    cmds: [echo lint]",
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "sub", "Taskfile.yml"), strings.Join([]string{
		"version: '3'",        // 1
		"tasks:",              // 2
		"  build:",            // 3
		"    cmds: [echo b]",  // 4
		"  check:",            // 5
		"    deps: [build]",   // 6: sub:build
		"    cmds:",           // 7
		"      - task: build", // 8: sub:build
		"      - task: :lint", // 9: the root's lint
		"      - task: lint",  // 10: sub:lint — does not exist
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "flat", "Taskfile.yml"), strings.Join([]string{
		"version: '3'",        // 1
		"tasks:",              // 2
		"  flatcheck:",        // 3
		"    cmds:",           // 4
		"      - task: lint",  // 5: flattened — the root's lint
		"      - task: build", // 6: the root has no build
		"",
	}, "\n"))
	c := Concern{ID: "tf", Want: "every call resolves", Check: Check{Kind: KindCommandsResolve, Where: CorpusTaskfile}}
	f := Audit(root, []Concern{c})[0]
	want := []string{
		"flat/Taskfile.yml:6 → task build (no such target)",
		`sub/Taskfile.yml:10 → task lint (no such target sub:lint — a call inside an include resolves in its namespace; the root's is :lint)`,
	}
	if f.Status != StatusViolated || !reflect.DeepEqual(f.Evidence, want) {
		t.Fatalf("got %s %q\nevidence %q\nwant %q", f.Status, f.Detail, f.Evidence, want)
	}
	if !strings.Contains(f.Detail, "2 of 7 command reference(s)") {
		t.Errorf("sub:check, sub:build twice, :lint and the flattened lint resolve beside the two dead calls: %q", f.Detail)
	}

	// Mend both the way Task would accept them.
	writeTestFile(t, filepath.Join(root, "sub", "Taskfile.yml"), strings.Replace(mustRead(t, filepath.Join(root, "sub", "Taskfile.yml")), "      - task: lint\n", "      - task: :lint\n", 1))
	writeTestFile(t, filepath.Join(root, "flat", "Taskfile.yml"), strings.Replace(mustRead(t, filepath.Join(root, "flat", "Taskfile.yml")), "      - task: build\n", "      - task: sub:build\n", 1))
	if f := Audit(root, []Concern{c})[0]; f.Status != StatusHolds {
		t.Fatalf("mended calls must hold: %+v", f)
	}
}

// TestCommandsResolveFromTheirWorkingDirectory: a script path and a
// `task X` resolve from the directory the command actually runs in — a
// Taskfile task's `dir:`, an include's `dir:` (an include without one runs
// in its includer's directory, NOT its own), lefthook's `root:`, a GitHub
// step's `working-directory:` or its job's defaults, and for a nested
// CLAUDE.md the Taskfile Task finds from that directory. A directory this
// audit cannot model (templated, a CI key it does not read) makes the ref
// not checkable, never dead and never resolved.
func TestCommandsResolveFromTheirWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "scripts", "ci.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(root, "web", "scripts", "build.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(root, "ui", "scripts", "ui.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), strings.Join([]string{
		"version: '3'",                     // 1
		"includes:",                        // 2
		"  ui: ./ui",                       // 3: runs in the root
		"  ui2:",                           // 4
		"    taskfile: ./ui2",              // 5
		"    dir: ./ui2",                   // 6: runs in ui2/
		"tasks:",                           // 7
		"  build:",                         // 8
		"    dir: web",                     // 9
		"    cmds:",                        // 10
		"      - ./scripts/build.sh",       // 11: web/scripts/build.sh
		"      - bash scripts/ci.sh",       // 12: web/scripts/ci.sh does not exist
		"  templated:",                     // 13
		"    dir: '{{.USER_WORKING_DIR}}'", // 14
		"    cmds: [bash scripts/any.sh]",  // 15: not checkable
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "ui", "Taskfile.yml"), strings.Join([]string{
		"version: '3'",                      // 1
		"tasks:",                            // 2
		"  css:",                            // 3
		"    cmds: [bash ../scripts/ci.sh]", // 4: from the root this leaves the repo
		"  js:",                             // 5
		"    cmds: [bash scripts/ui.sh]",    // 6: ui/scripts/ui.sh, but it runs in the root
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "ui2", "Taskfile.yml"), "version: '3'\ntasks:\n  css:\n    cmds: [bash ../scripts/ci.sh]\n")
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), strings.Join([]string{
		"pre-commit:",                   // 1
		"  commands:",                   // 2
		"    build:",                    // 3
		"      root: \"web/\"",          // 4
		"      run: ./scripts/build.sh", // 5: web/scripts/build.sh
		"    ci:",                       // 6
		"      root: web",               // 7
		"      run: bash scripts/ci.sh", // 8: web/scripts/ci.sh does not exist
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, ".github", "workflows", "ci.yml"), strings.Join([]string{
		"on: push",                        // 1
		"defaults:",                       // 2
		"  run:",                          // 3
		"    working-directory: web",      // 4
		"jobs:",                           // 5
		"  test:",                         // 6
		"    runs-on: ubuntu-latest",      // 7
		"    steps:",                      // 8
		"      - run: ./scripts/build.sh", // 9: web/ from the workflow default
		"      - run: bash scripts/ci.sh", // 10: web/scripts/ci.sh does not exist
		"        working-directory: .",    // 11
		"      - run: bash scripts/ci.sh", // 12: web/ again — dead
		"      - run: bash scripts/ci.sh", // 13
		"        working-directory: ${{ github.workspace }}/x", // 14: not checkable
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "web", "Taskfile.yml"), "version: '3'\ntasks:\n  serve:\n    cmds: [echo serve]\n")
	writeTestFile(t, filepath.Join(root, "web", "CLAUDE.md"), "# web\nRun `task serve` here; `task nothing` is gone.\n")

	chain := func(where string) Concern {
		return Concern{ID: where, Want: "resolves", Check: Check{Kind: KindCommandsResolve, Where: where}}
	}
	fs := Audit(root, []Concern{chain(CorpusTaskfile), chain(CorpusLefthook), chain(CorpusCI), chain(CorpusAgentConfig)})
	want := map[string][]string{
		CorpusTaskfile: {
			"Taskfile.yml:12 → scripts/ci.sh (no such file — it resolves from the repo root, the file's own directory, but the command runs in web/)",
			"ui/Taskfile.yml:4 → ../scripts/ci.sh (leaves the repo from the repo root — it resolves from ui/, the file's own directory, but the command runs in the repo root)",
			"ui/Taskfile.yml:6 → scripts/ui.sh (no such file — it resolves from ui/, the file's own directory, but the command runs in the repo root)",
		},
		CorpusLefthook:    {"lefthook.yml:8 → scripts/ci.sh (no such file — it resolves from the repo root, the file's own directory, but the command runs in web/)"},
		CorpusCI:          {".github/workflows/ci.yml:12 → scripts/ci.sh (no such file in web/)"},
		CorpusAgentConfig: {"web/CLAUDE.md:2 → task nothing (no such target)"},
	}
	for _, f := range fs {
		w := f.Concern.Check.Where
		if f.Status != StatusViolated || !reflect.DeepEqual(f.Evidence, want[w]) {
			t.Errorf("%s: got %s %q\n evidence %q\n want     %q", w, f.Status, f.Detail, f.Evidence, want[w])
		}
	}
	// config propose writes a moved script's path from where the command
	// runs: web/ for the hook, the root for the include that runs there.
	props, err := Propose(root, fs[:2])
	if err != nil {
		t.Fatal(err)
	}
	newLines := map[string]bool{}
	for _, p := range props {
		for _, e := range p.Edits {
			newLines[e.File+": "+strings.TrimSpace(e.New)] = true
		}
	}
	for _, w := range []string{
		"Taskfile.yml: - bash ../scripts/ci.sh",
		"ui/Taskfile.yml: cmds: [bash scripts/ci.sh]",
		"ui/Taskfile.yml: cmds: [bash ui/scripts/ui.sh]",
		"lefthook.yml: run: bash ../scripts/ci.sh",
	} {
		if !newLines[w] {
			t.Errorf("want the edit %q among %v", w, newLines)
		}
	}

	for _, tc := range []struct {
		i      int
		detail string
	}{
		{0, "3 of 5 command reference(s) do not resolve across 3 Taskfile(s); 1 not checkable"}, // build.sh and ui2's ../scripts/ci.sh resolve
		{1, "1 of 2 command reference(s)"},
		{2, "1 of 3 command reference(s) do not resolve across 1 CI config(s); 1 not checkable"},
		{3, "1 of 2 command reference(s)"}, // `task serve` resolves in web/Taskfile.yml
	} {
		if !strings.Contains(fs[tc.i].Detail, tc.detail) {
			t.Errorf("%s detail %q, want it to contain %q", fs[tc.i].Concern.Check.Where, fs[tc.i].Detail, tc.detail)
		}
	}
}

// TestCommandsResolveUnstattableScriptIsUnableToCheck is I38 for the
// script half: a script that exists but cannot be stat'd (EACCES, or the
// macOS TCC wedge's EPERM) is unable-to-check naming the error, never
// "no such file".
func TestCommandsResolveUnstattableScriptIsUnableToCheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads through a 000 directory")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), "pre-push:\n  commands:\n    gate:\n      run: bash scripts/ci.sh && fathom verify\n")
	writeTestFile(t, filepath.Join(root, "scripts", "ci.sh"), "#!/bin/sh\n")
	dir := filepath.Join(root, "scripts")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	f := Audit(root, []Concern{{ID: "lh", Want: "resolves", Check: Check{Kind: KindCommandsResolve, Where: CorpusLefthook}}})[0]
	if f.Status != StatusUnable || !strings.Contains(f.Detail, "permission denied") || !strings.Contains(f.Detail, "scripts/ci.sh") {
		t.Fatalf("an unstattable script is unable-to-check naming the path and errno, never absent: %+v", f)
	}
}

// TestFoldedScalarRefsKeepTheirSourceLine: a folded block (`run: >`)
// joins its source lines into one, so a ref is mapped back to the source
// line that carries it — the evidence names line 6, not the block's first
// line, and config propose can mend it in place.
func TestFoldedScalarRefsKeepTheirSourceLine(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), strings.Join([]string{
		"pre-push:",                        // 1
		"  commands:",                      // 2
		"    gate:",                        // 3
		"      run: >",                     // 4
		"        echo start &&",            // 5
		"        bash scripts/check.sh &&", // 6
		"        fathom verify",            // 7
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "scripts", "ci", "check.sh"), "#!/bin/sh\n")
	c := Concern{ID: "lh", Want: "resolves", Check: Check{Kind: KindCommandsResolve, Where: CorpusLefthook}}
	fs := Audit(root, []Concern{c})
	if want := []string{"lefthook.yml:6 → scripts/check.sh (no such file)"}; !reflect.DeepEqual(fs[0].Evidence, want) {
		t.Fatalf("evidence %q, want %q", fs[0].Evidence, want)
	}
	props, err := Propose(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 || len(props[0].Edits) != 1 || props[0].Edits[0].New != "        bash scripts/ci/check.sh &&" {
		t.Fatalf("the folded line is mended in place: %+v", props)
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestTaskfileDirVariablesResolve: a task's `dir: '{{.TASKFILE_DIR}}'` —
// the usual fix for an include-relative path — and `{{.ROOT_DIR}}` are
// Task's own absolute paths (the file's directory, the root Taskfile's),
// so the audit models them exactly; any other template, and a templated
// include `dir:` (where Task expands TASKFILE_DIR to nothing), stay not
// checkable.
func TestTaskfileDirVariablesResolve(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "scripts", "ci.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(root, "ui", "scripts", "ui.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), "version: '3'\nincludes:\n  ui: ./ui\n  lib:\n    taskfile: ./lib\n    dir: '{{.TASKFILE_DIR}}/lib'\n")
	writeTestFile(t, filepath.Join(root, "ui", "Taskfile.yml"), strings.Join([]string{
		"version: '3'",
		"tasks:",
		"  here:",
		"    dir: '{{.TASKFILE_DIR}}'",
		"    cmds: [bash scripts/ui.sh]", // 5: ui/scripts/ui.sh
		"  root:",
		"    dir: '{{ .ROOT_DIR }}/scripts'",
		"    cmds: [bash ./ci.sh]", // 8: scripts/ci.sh
		"  gone:",
		"    dir: '{{.TASKFILE_DIR}}'",
		"    cmds: [bash scripts/gone.sh]", // 11: dead in ui/
		"  user:",
		"    dir: '{{.USER_WORKING_DIR}}'",
		"    cmds: [bash scripts/any.sh]", // 14: not checkable
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(root, "lib", "Taskfile.yml"), "version: '3'\ntasks:\n  x:\n    cmds: [bash scripts/any.sh]\n")
	f := Audit(root, []Concern{{ID: "tf", Want: "resolves", Check: Check{Kind: KindCommandsResolve, Where: CorpusTaskfile}}})[0]
	want := []string{"ui/Taskfile.yml:11 → scripts/gone.sh (no such file in ui/)"}
	if !reflect.DeepEqual(f.Evidence, want) || !strings.Contains(f.Detail, "1 of 3 command reference(s)") || !strings.Contains(f.Detail, "2 not checkable") {
		t.Fatalf("got %s %q %q, want evidence %q with 2 resolved and 2 not checkable", f.Status, f.Detail, f.Evidence, want)
	}
}
