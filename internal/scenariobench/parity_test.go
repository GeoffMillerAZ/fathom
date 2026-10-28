package scenariobench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// parityTask is a minimal task for CheckArmParity: it needs only an id, a
// prompt and a turn cap, because parity never validates or runs a task.
func parityTask(id string, maxTurns int) Task {
	return Task{
		ID:       id,
		Prompt:   "prompt of " + id,
		Sessions: Sessions{Count: 1, MaxTurns: maxTurns, Mode: ModeSingle},
	}
}

// parityArm is an unwired arm that leaves the subject's instruction file
// alone, so RenderArm succeeds on any tree.
func parityArm(mut func(*Arm)) Arm {
	a := Arm{Instructions: "plain"}
	if mut != nil {
		mut(&a)
	}
	return a
}

func parityCorpus(arms map[string]Arm, tasks ...Task) Corpus {
	if len(tasks) == 0 {
		tasks = []Task{parityTask("T1", 5)}
	}
	return Corpus{Arms: arms, Tasks: tasks, Criterion: Criterion{Control: "control", Treatment: "full"}}
}

func parityRow(t *testing.T, rep ParityReport, arm string) ArmParity {
	t.Helper()
	for _, a := range rep.Arms {
		if a.Arm == arm {
			return a
		}
	}
	t.Fatalf("no row for arm %q in %+v", arm, rep.Arms)
	return ArmParity{}
}

func TestParityFieldsVocabulary(t *testing.T) {
	want := []string{"model", "notes_file", "prompt", "settings", "tools", "tree", "turn_cap"}
	got := ParityFields()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParityFields = %v, want %v", got, want)
	}
	if !sort.StringsAreSorted(got) {
		t.Fatalf("ParityFields not sorted: %v", got)
	}
	got[0] = "mutated"
	if ParityFields()[0] != "model" {
		t.Fatal("ParityFields must return a fresh copy")
	}
}

func TestCheckArmParity_FieldMapping(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	tests := []struct {
		name  string
		treat func(*Arm)
		want  string // the one parity field that must differ
		base  func(*Arm)
	}{
		{name: "allowed tools", treat: func(a *Arm) { a.AllowedTools = []string{"Read"} }, want: "tools"},
		{name: "disallowed tools", treat: func(a *Arm) { a.DisallowedTools = []string{"Bash"} }, want: "tools"},
		{name: "notes file", treat: func(a *Arm) { a.NotesFile = true }, want: "notes_file"},
		{name: "model override env", treat: func(a *Arm) { a.Env = map[string]string{"ANTHROPIC_MODEL": "m-1"} }, want: "model"},
		{name: "subagent model env", treat: func(a *Arm) { a.Env = map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "m-1"} }, want: "model"},
		{name: "other env", treat: func(a *Arm) { a.Env = map[string]string{"SOME_FLAG": "1"} }, want: "settings"},
		{name: "mcp config", treat: func(a *Arm) { a.MCP = map[string]any{"srv": map[string]any{"command": "x"}} }, want: "settings"},
		{name: "instructions text", treat: func(a *Arm) { a.Instructions = "text"; a.InstructionsText = "route here" }, want: "prompt"},
		{name: "instructions none", treat: func(a *Arm) { a.Instructions = "none" }, want: ""}, // empty tree: no file to remove, so equal
		{name: "setup list", treat: func(a *Arm) { a.Setup = []string{"make build"} }, want: "tree"},
		{name: "setup is data not run", treat: func(a *Arm) { a.Setup = []string{"touch " + marker} }, want: "tree"},
		{name: "setup order", base: func(a *Arm) { a.Setup = []string{"a", "b"} }, treat: func(a *Arm) { a.Setup = []string{"b", "a"} }, want: "tree"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := parityArm(tc.base)
			treat := parityArm(tc.treat)
			c := parityCorpus(map[string]Arm{"control": base, "full": treat})
			rep, err := CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			row := parityRow(t, rep, "full")
			if tc.want == "" {
				if row.Verdict != ParityPass || len(row.Undeclared) != 0 {
					t.Fatalf("row = %+v, want PASS", row)
				}
				return
			}
			if row.Verdict != ParityFail || !reflect.DeepEqual(row.Undeclared, []string{tc.want}) {
				t.Fatalf("row = %+v, want FAIL Undeclared [%s]", row, tc.want)
			}
			// Declaring exactly that field turns the failure into a pass.
			treat.Declares = []string{tc.want}
			c = parityCorpus(map[string]Arm{"control": base, "full": treat})
			rep, err = CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			row = parityRow(t, rep, "full")
			if row.Verdict != ParityPass || len(row.Undeclared) != 0 || !reflect.DeepEqual(row.Declared, []string{tc.want}) {
				t.Fatalf("declared row = %+v, want PASS Declared [%s]", row, tc.want)
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a Setup line was executed: marker stat err = %v", err)
	}
}

func TestCheckArmParity_ToolsUndeclaredThenDeclared(t *testing.T) {
	control := parityArm(nil)
	full := parityArm(func(a *Arm) { a.AllowedTools = []string{"Read", "Grep"} })
	rep, err := CheckArmParity(parityCorpus(map[string]Arm{"control": control, "full": full}), ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Baseline != "control" {
		t.Fatalf("baseline = %q, want the criterion control", rep.Baseline)
	}
	if row := parityRow(t, rep, "control"); row.Verdict != ParityPass {
		t.Fatalf("baseline row = %+v, want PASS against itself", row)
	}
	row := parityRow(t, rep, "full")
	if row.Verdict != ParityFail || !reflect.DeepEqual(row.Undeclared, []string{"tools"}) || len(row.Declared) != 0 {
		t.Fatalf("row = %+v, want FAIL Undeclared [tools]", row)
	}
	full.Declares = []string{"tools"}
	rep, err = CheckArmParity(parityCorpus(map[string]Arm{"control": control, "full": full}), ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	row = parityRow(t, rep, "full")
	if row.Verdict != ParityPass || len(row.Undeclared) != 0 || !reflect.DeepEqual(row.Declared, []string{"tools"}) {
		t.Fatalf("row = %+v, want PASS Declared [tools]", row)
	}
}

func TestCheckArmParity_NotesFileIsNotPrompt(t *testing.T) {
	c := parityCorpus(map[string]Arm{
		"control": parityArm(nil),
		"full":    parityArm(func(a *Arm) { a.NotesFile = true }),
	})
	rep, err := CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	row := parityRow(t, rep, "full")
	if !reflect.DeepEqual(row.Undeclared, []string{"notes_file"}) {
		t.Fatalf("Undeclared = %v, want exactly [notes_file] (SystemAppend and NOTES.md are notes_file's, not prompt's)", row.Undeclared)
	}
}

func TestCheckArmParity_SystemAppendNotExplainedByNotesFileIsPrompt(t *testing.T) {
	a := armView{notes: true, systemAppend: "x"}
	b := armView{notes: true, systemAppend: "y"}
	got := diffViews(a, b)
	if !reflect.DeepEqual(got, []string{"prompt"}) {
		t.Fatalf("diffViews = %v, want [prompt] for a SystemAppend difference with equal NotesFile", got)
	}
	b.notes = false
	got = diffViews(a, b)
	if !reflect.DeepEqual(got, []string{"notes_file"}) {
		t.Fatalf("diffViews = %v, want [notes_file] once NotesFile itself differs", got)
	}
}

func TestCheckArmParity_InstructionFilesAreComparedInTheTree(t *testing.T) {
	tree := t.TempDir()
	writeTestFile(t, filepath.Join(tree, "CLAUDE.md"), "subject rules\n")
	c := parityCorpus(map[string]Arm{
		"control": parityArm(nil),
		"full":    parityArm(func(a *Arm) { a.Instructions = "none" }),
	})
	rep, err := CheckArmParity(c, ParityOptions{Tree: tree, ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	row := parityRow(t, rep, "full")
	if !reflect.DeepEqual(row.Undeclared, []string{"prompt"}) {
		t.Fatalf("removing the subject's CLAUDE.md: Undeclared = %v, want [prompt]", row.Undeclared)
	}
}

func TestCheckArmParity_TurnCapIsTaskLevel(t *testing.T) {
	tasks := []Task{parityTask("T1", 5), parityTask("T2", 25)}
	c := parityCorpus(map[string]Arm{
		"control": parityArm(nil),
		"full":    parityArm(func(a *Arm) { a.AllowedTools = []string{"Read"}; a.Declares = []string{"tools", "turn_cap"} }),
		"other":   parityArm(func(a *Arm) { a.NotesFile = true }),
	}, tasks...)
	rep, err := CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Tasks, []string{"T1", "T2"}) {
		t.Fatalf("Tasks = %v", rep.Tasks)
	}
	for _, row := range rep.Arms {
		for _, f := range append(append([]string{}, row.Undeclared...), row.Declared...) {
			if f == "turn_cap" {
				t.Fatalf("arm %s reports turn_cap: %+v", row.Arm, row)
			}
		}
	}
}

func TestCheckArmParity_TaskSelectionAndDropped(t *testing.T) {
	dropped := parityTask("T2", 5)
	dropped.Disposition = "dropped"
	c := parityCorpus(map[string]Arm{"control": parityArm(nil)}, parityTask("T1", 5), dropped)
	rep, err := CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Tasks, []string{"T1"}) {
		t.Fatalf("Tasks = %v, want the dropped task left out", rep.Tasks)
	}
	rep, err = CheckArmParity(c, ParityOptions{TaskID: "T2", ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Tasks, []string{"T2"}) {
		t.Fatalf("Tasks = %v, want an explicitly named task honoured", rep.Tasks)
	}
	allDropped := parityTask("T1", 5)
	allDropped.Disposition = "dropped"
	if _, err := CheckArmParity(parityCorpus(map[string]Arm{"control": parityArm(nil)}, allDropped), ParityOptions{ScratchDir: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "no tasks") {
		t.Fatalf("all tasks dropped: err = %v, want a no-tasks refusal", err)
	}
}

func TestCheckArmParity_WiredArmIsIncompleteAndWritesNoHooks(t *testing.T) {
	scratch := t.TempDir()
	c := parityCorpus(map[string]Arm{
		"control": parityArm(nil),
		"full":    {Wired: true, Instructions: "stanza"},
	})
	rep, err := CheckArmParity(c, ParityOptions{ScratchDir: scratch})
	if err != nil {
		t.Fatal(err)
	}
	row := parityRow(t, rep, "full")
	if row.Verdict != ParityIncomplete {
		t.Fatalf("verdict = %s, want INCOMPLETE: %+v", row.Verdict, row)
	}
	if want := []string{"notes_file", "prompt", "settings", "tools"}; !reflect.DeepEqual(row.Unrendered, want) {
		t.Fatalf("Unrendered = %v, want %v", row.Unrendered, want)
	}
	if !strings.Contains(row.RenderError, "settings.json") || !strings.Contains(row.RenderError, "fathom init") {
		t.Fatalf("RenderError = %q, want it to name the missing settings.json / fathom init hooks", row.RenderError)
	}
	if row := parityRow(t, rep, "control"); row.Verdict != ParityPass {
		t.Fatalf("baseline row = %+v, want PASS", row)
	}
	// Arms are copied into index-named subdirectories (control=0, full=1);
	// parity must not have stood in a hook stack for the failed render.
	if raw, err := os.ReadFile(filepath.Join(scratch, "1", ".claude", "settings.json")); err == nil && strings.Contains(string(raw), "hooks") {
		t.Fatalf("parity wrote hooks into the copy: %s", raw)
	}

	t.Run("settings without hooks", func(t *testing.T) {
		tree := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tree, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(tree, ".claude", "settings.json"), "{}\n")
		scratch := t.TempDir()
		rep, err := CheckArmParity(c, ParityOptions{Tree: tree, ScratchDir: scratch})
		if err != nil {
			t.Fatal(err)
		}
		row := parityRow(t, rep, "full")
		if row.Verdict != ParityIncomplete || !strings.Contains(row.RenderError, "hooks.SessionStart") {
			t.Fatalf("row = %+v, want INCOMPLETE naming hooks.SessionStart", row)
		}
		raw, err := os.ReadFile(filepath.Join(scratch, "1", ".claude", "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "hooks") {
			t.Fatalf("parity wrote hooks into the copy: %s", raw)
		}
	})
}

func TestCheckArmParity_UndeclaredBeatsIncomplete(t *testing.T) {
	c := parityCorpus(map[string]Arm{
		"control": parityArm(nil),
		"full":    {Wired: true, Instructions: "stanza", Setup: []string{"build"}},
	})
	rep, err := CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	row := parityRow(t, rep, "full")
	if row.Verdict != ParityFail || !reflect.DeepEqual(row.Undeclared, []string{"tree"}) || len(row.Unrendered) != 4 {
		t.Fatalf("row = %+v, want FAIL [tree] with the four unrendered fields still reported", row)
	}
}

func TestCheckArmParity_BaselineRenderFailureMakesEveryArmIncomplete(t *testing.T) {
	c := parityCorpus(map[string]Arm{
		"control": {Wired: true, Instructions: "stanza"},
		"full":    parityArm(nil),
	})
	rep, err := CheckArmParity(c, ParityOptions{ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"control", "full"} {
		row := parityRow(t, rep, name)
		if row.Verdict != ParityIncomplete || len(row.Unrendered) != 4 || row.RenderError == "" {
			t.Fatalf("%s row = %+v, want INCOMPLETE with a render error", name, row)
		}
	}
}

func TestCheckArmParity_Refusals(t *testing.T) {
	base := map[string]Arm{"control": parityArm(nil), "full": parityArm(nil)}
	tests := []struct {
		name string
		c    Corpus
		opts ParityOptions
		want []string
	}{
		{name: "unknown declares entry", c: parityCorpus(map[string]Arm{"control": parityArm(nil), "full": parityArm(func(a *Arm) { a.Declares = []string{"toolz"} })}), want: []string{`"full"`, `"toolz"`}},
		{name: "unknown declares entry on an arm not compared", c: parityCorpus(map[string]Arm{"control": parityArm(nil), "full": parityArm(nil), "idle": parityArm(func(a *Arm) { a.Declares = []string{"bogus"} })}), want: []string{`"idle"`, `"bogus"`}},
		{name: "unknown baseline", c: parityCorpus(base), opts: ParityOptions{Baseline: "nope"}, want: []string{`"nope"`, "control, full"}},
		{name: "no baseline and no control", c: func() Corpus { c := parityCorpus(base); c.Criterion.Control = ""; return c }(), want: []string{"baseline"}},
		{name: "unknown task", c: parityCorpus(base), opts: ParityOptions{TaskID: "T9"}, want: []string{`"T9"`}},
		{name: "unreadable tree", c: parityCorpus(base), opts: ParityOptions{Tree: filepath.Join(t.TempDir(), "absent")}, want: []string{"tree"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.ScratchDir = t.TempDir()
			rep, err := CheckArmParity(tc.c, tc.opts)
			if err == nil {
				t.Fatalf("want a refusal, got report %+v", rep)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q does not name %s", err, w)
				}
			}
			if !strings.HasPrefix(err.Error(), "parity: ") {
				t.Fatalf("error %q lacks the parity: prefix", err)
			}
			if len(rep.Arms) != 0 {
				t.Fatalf("a refusal must return no report, got %+v", rep)
			}
		})
	}
}

func TestCheckArmParity_UnreadableTreeRefuses(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 does not make a directory unreadable")
	}
	tree := t.TempDir()
	locked := filepath.Join(tree, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(locked, "f"), "x")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	c := parityCorpus(map[string]Arm{"control": parityArm(nil)})
	rep, err := CheckArmParity(c, ParityOptions{Tree: tree, ScratchDir: t.TempDir()})
	if err == nil {
		t.Fatalf("an unreadable tree must refuse (never be treated as empty), got %+v", rep)
	}
}

func TestCheckArmParity_Tree(t *testing.T) {
	tree := t.TempDir()
	writeTestFile(t, filepath.Join(tree, "a.txt"), "alpha\n")
	if err := os.MkdirAll(filepath.Join(tree, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(tree, "sub", "b.txt"), "beta\n")
	if err := os.Symlink("a.txt", filepath.Join(tree, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(tree, ".git", "HEAD"), "ref: refs/heads/main\n")

	c := parityCorpus(map[string]Arm{"control": parityArm(nil), "full": parityArm(nil)})
	scratch := t.TempDir()
	rep, err := CheckArmParity(c, ParityOptions{Tree: tree, ScratchDir: scratch})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".git", "link"}; !reflect.DeepEqual(rep.Skipped, want) {
		t.Fatalf("Skipped = %v, want %v", rep.Skipped, want)
	}
	if len(rep.TreeSHA256) != 64 {
		t.Fatalf("TreeSHA256 = %q, want 64 hex chars", rep.TreeSHA256)
	}
	for _, name := range []string{"control", "full"} {
		if row := parityRow(t, rep, name); row.Verdict != ParityPass {
			t.Fatalf("%s row = %+v, want PASS (one tree copied per arm)", name, row)
		}
	}
	// Each arm got its own copy, named by index; skipped entries were not copied.
	for _, idx := range []string{"0", "1"} {
		if raw, err := os.ReadFile(filepath.Join(scratch, idx, "sub", "b.txt")); err != nil || string(raw) != "beta\n" {
			t.Fatalf("copy %s: sub/b.txt = %q, %v", idx, raw, err)
		}
		for _, skipped := range []string{".git", "link"} {
			if _, err := os.Lstat(filepath.Join(scratch, idx, skipped)); !os.IsNotExist(err) {
				t.Fatalf("copy %s carries skipped entry %s (err %v)", idx, skipped, err)
			}
		}
	}

	again, err := CheckArmParity(c, ParityOptions{Tree: tree, ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if again.TreeSHA256 != rep.TreeSHA256 {
		t.Fatalf("TreeSHA256 unstable: %s vs %s", again.TreeSHA256, rep.TreeSHA256)
	}

	// The source tree is untouched: RenderArm wrote only into copies.
	entries, err := os.ReadDir(tree)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{".git", "a.txt", "link", "sub"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("source tree entries = %v, want %v (parity wrote into the source)", names, want)
	}

	writeTestFile(t, filepath.Join(tree, "a.txt"), "alphA\n")
	changed, err := CheckArmParity(c, ParityOptions{Tree: tree, ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if changed.TreeSHA256 == rep.TreeSHA256 {
		t.Fatal("TreeSHA256 did not change when a file byte changed")
	}
}

func TestCheckArmParity_EmptyTreeAndDefaultScratch(t *testing.T) {
	c := parityCorpus(map[string]Arm{"control": parityArm(nil), "full": parityArm(nil)})
	rep, err := CheckArmParity(c, ParityOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TreeSHA256 == "" || len(rep.Skipped) != 0 {
		t.Fatalf("report = %+v", rep)
	}
	// Arms come back sorted by name.
	if len(rep.Arms) != 2 || rep.Arms[0].Arm != "control" || rep.Arms[1].Arm != "full" {
		t.Fatalf("Arms = %+v", rep.Arms)
	}
}

func TestCheckArmParity_NamedBaseline(t *testing.T) {
	c := parityCorpus(map[string]Arm{
		"control": parityArm(nil),
		"full":    parityArm(func(a *Arm) { a.AllowedTools = []string{"Read"} }),
	})
	rep, err := CheckArmParity(c, ParityOptions{Baseline: "full", ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Baseline != "full" {
		t.Fatalf("Baseline = %q", rep.Baseline)
	}
	if row := parityRow(t, rep, "control"); !reflect.DeepEqual(row.Undeclared, []string{"tools"}) {
		t.Fatalf("control row = %+v", row)
	}
}

func TestArmDeclaresMarshalling(t *testing.T) {
	raw, err := json.Marshal(Arm{Instructions: "plain"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "declares") {
		t.Fatalf("an arm without declarations must marshal with no declares key: %s", raw)
	}
	raw, err = json.Marshal(Arm{Instructions: "plain", Declares: []string{"tools"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"declares":["tools"]`) {
		t.Fatalf("declares missing from %s", raw)
	}
	// Declares is the LAST field of Arm.
	typ := reflect.TypeOf(Arm{})
	if last := typ.Field(typ.NumField() - 1); last.Name != "Declares" {
		t.Fatalf("last Arm field = %s, want Declares", last.Name)
	}
}

func TestParityReportJSONShape(t *testing.T) {
	rep := ParityReport{Baseline: "control", Tasks: []string{"T1"}, TreeSHA256: "ab", Arms: []ArmParity{{Arm: "control", Verdict: ParityPass}}}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"baseline", "tasks", "tree_sha256", "arms"} {
		if _, ok := back[key]; !ok {
			t.Fatalf("ParityReport JSON lacks %q: %s", key, raw)
		}
	}
	if _, ok := back["skipped"]; ok {
		t.Fatalf("an empty skipped list must be omitted: %s", raw)
	}
	arm := back["arms"].([]any)[0].(map[string]any)
	for _, key := range []string{"undeclared", "declared", "unrendered", "render_error"} {
		if _, ok := arm[key]; ok {
			t.Fatalf("empty %q must be omitted: %s", key, raw)
		}
	}
	if arm["arm"] != "control" || arm["verdict"] != "PASS" {
		t.Fatalf("arm row = %v", arm)
	}
}

// committedCorpusGoldens are the identities of every committed corpus under
// testdata/scenariobench/*.json BEFORE Arm.Declares existed (recorded from
// CanonicalCorpusSHA256 and armContentHash at the commit that added it). The
// field is omitempty, so none of them may move: a moved hash orphans every
// committed cell keyed under it.
var committedCorpusGoldens = map[string]struct {
	corpus string
	arms   map[string]string
}{
	"findings-loop.json": {"053c36aa40e088a1740fd18849dee52cf3be025651ba4e8f14ba7f87f32d1c00", map[string]string{"control": "ff7273a241fb", "full": "d9b199f0567c"}},
	"guards-adr.json":    {"ebcabaae382a977cea0ee9b051a41e1fa3e719189c31a4bc3abbb07b2927c92e", map[string]string{"control": "58e2504e9a41", "full": "d9b199f0567c"}},
	"guards-fathom.json": {"da6e7a4b9db217b70bf85cee57ef74a34c2f97ded80ee5d3f2aa2ffdf03085cb", map[string]string{"control": "58e2504e9a41", "full": "d9b199f0567c"}},
	"guards-m1.json":     {"eb333938614c5797f5ae0cff52af04c5da28f4970d224c0af96144b1047f5cc6", map[string]string{"control": "58e2504e9a41", "full": "d9b199f0567c"}},
	"lookups-py.json":    {"28bc17813e66a72ea157c820695bf7bf8562d8de0d068bc076fe26be4f2d37df", map[string]string{"full": "1c2ab0972246", "grep": "cc15494d419f"}},
	"lookups-ts.json":    {"a6ada485c1fb4a1e79b640b7750a81836dee038a42bc48351afcfe29a474aef0", map[string]string{"full": "34662de0633f", "grep": "e320c6adffa6"}},
	"lookups-vs.json": {"85fce60114f56d448640e0b2a0a9e6d45537f5d9371e3459193fd97fe8f84097", map[string]string{
		"full": "ebe31d430998", "gopls": "e535a44d8ef8", "graphify": "b12245f1214e", "grep": "80fe955fc016", "serena": "d13524f3c445"}},
	"public-go.json": {"5035b07fc923fe0b6bbc195e44ad0ab81879bdaf23f42ab452c1e802df0d2021", map[string]string{"control": "58e2504e9a41", "full": "7266ea14d845"}},
	"reuse-go.json":  {"aa0abed7f499771b70e71beb01669b68d475fd20f554d331c7d4594e59a605b5", map[string]string{"control": "5e6f6f943ba8", "full": "c03be0690fdc"}},
}

func TestCommittedCorporaHashesUnchangedByDeclares(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "scenariobench", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		name := filepath.Base(f)
		golden, ok := committedCorpusGoldens[name]
		if !ok {
			t.Logf("%s: committed after the Declares goldens were recorded; not pinned here", name)
			continue
		}
		seen[name] = true
		c, err := LoadScenarioCorpus(f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sha, err := CanonicalCorpusSHA256(c)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if sha != golden.corpus {
			t.Errorf("%s: CanonicalCorpusSHA256 = %s, want %s", name, sha, golden.corpus)
		}
		if len(c.Arms) != len(golden.arms) {
			t.Errorf("%s: %d arms, goldens cover %d", name, len(c.Arms), len(golden.arms))
		}
		for armName, arm := range c.Arms {
			raw, err := json.Marshal(arm)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"declares"`) {
				t.Errorf("%s arm %s marshals a declares key: %s", name, armName, raw)
			}
			if got := armContentHash(arm); got != golden.arms[armName] {
				t.Errorf("%s arm %s: armContentHash = %s, want %s", name, armName, got, golden.arms[armName])
			}
		}
	}
	for name := range committedCorpusGoldens {
		if !seen[name] {
			t.Errorf("golden corpus %s is no longer committed under testdata/scenariobench", name)
		}
	}
}
