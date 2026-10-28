// BX-60: arm parity. A scenario corpus compares arms, and a comparison is only
// about the treatment when the arms differ in the treatment and nothing else.
// CheckArmParity renders every arm of a corpus into its own copy of one tree
// (RenderArm, the one place arm data becomes harness input), compares each
// arm to a baseline field by field over a small fixed vocabulary
// (ParityFields), and reports a difference the arm did not declare
// (Arm.Declares) as a FAIL. It starts no daemon, runs no hook, executes no
// Setup line and touches no network.
package scenariobench

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ParityFields is the closed, sorted vocabulary of fields CheckArmParity
// compares and an Arm may declare (Arm.Declares). It returns a fresh copy.
//
// What each field covers:
//
//   - prompt: the task prompt (Task.Prompt, per selected task) and the
//     instruction files RenderArm leaves in the tree (CLAUDE.md,
//     .claude/CLAUDE.md, CLAUDE.local.md, AGENTS.md: presence and bytes,
//     which covers Arm.Instructions and Arm.InstructionsText), plus any
//     ArmFiles.SystemAppend difference that NotesFile does not explain
//     (SystemAppend differs while NotesFile is equal);
//   - tools: the rendered ArmFiles.AllowedTools and DisallowedTools;
//   - settings: the rendered .claude/settings.json bytes, the rendered MCP
//     config (.claude/scenariobench-mcp.json) bytes, and Arm.Env apart from
//     the model-override keys. No code applies Arm.Env to a session today;
//     parity compares it as data;
//   - notes_file: Arm.NotesFile, the SystemAppend text it renders and the
//     NOTES.md it seeds;
//   - turn_cap: Task.Sessions.MaxTurns, one value per task, so equal for
//     every arm of a task today. No Arm field exists for it and none is
//     added; it is computed per selected task all the same;
//   - model: the run's -model (one per run, equal for every arm) plus an
//     Arm.Env model override (parityModelEnvKeys);
//   - tree: the tree each arm's worktree starts from (one local tree copied
//     per arm, so equal by construction; its sha256 is reported in
//     ParityReport.TreeSHA256) plus Arm.Setup compared as DATA, as an
//     ordered list. Setup is NEVER executed: it is shell and may fetch (I56).
//
// Not compared: Surface and Prediction (labels), and Wired and CompileLayers
// (they act through `fathom init` and the scratch daemon, which parity does
// not start; a wired arm's rendered settings carry them, and offline that
// render reads INCOMPLETE, see CheckArmParity).
func ParityFields() []string {
	return []string{"model", "notes_file", "prompt", "settings", "tools", "tree", "turn_cap"}
}

// parityModelEnvKeys are the Arm.Env keys that select a model; a difference
// in them is a `model` difference, any other Env difference is `settings`.
var parityModelEnvKeys = map[string]bool{
	"ANTHROPIC_MODEL":                true,
	"ANTHROPIC_SMALL_FAST_MODEL":     true,
	"ANTHROPIC_DEFAULT_OPUS_MODEL":   true,
	"ANTHROPIC_DEFAULT_SONNET_MODEL": true,
	"ANTHROPIC_DEFAULT_HAIKU_MODEL":  true,
	"CLAUDE_CODE_SUBAGENT_MODEL":     true,
}

// renderedParityFields are the fields that need a successful RenderArm: when
// an arm (or the baseline) fails to render, these read Unrendered.
var renderedParityFields = []string{"notes_file", "prompt", "settings", "tools"}

// ParityVerdict is one arm's parity result against the baseline.
type ParityVerdict string

const (
	// ParityPass: no field differs except those the arm declares.
	ParityPass ParityVerdict = "PASS"
	// ParityFail: at least one differing field is not declared by the arm.
	ParityFail ParityVerdict = "FAIL"
	// ParityIncomplete: no undeclared difference, but at least one field could
	// not be rendered, so it was not compared.
	ParityIncomplete ParityVerdict = "INCOMPLETE"
)

// ArmParity is one arm's row of a ParityReport. Undeclared lists the
// differing fields the arm does not declare, Declared the differing fields it
// does, Unrendered the fields that could not be rendered (so were not
// compared). All three are sorted. RenderError is the RenderArm error that
// caused Unrendered: the arm's own, else the baseline's prefixed with
// "baseline <name>: ".
type ArmParity struct {
	Arm         string        `json:"arm"`
	Verdict     ParityVerdict `json:"verdict"`
	Undeclared  []string      `json:"undeclared,omitempty"`
	Declared    []string      `json:"declared,omitempty"`
	Unrendered  []string      `json:"unrendered,omitempty"`
	RenderError string        `json:"render_error,omitempty"`
}

// ParityReport is CheckArmParity's result: the baseline compared against, the
// sorted ids of the tasks it covered, the sha256 of the source tree every arm
// was copied from, the source entries that were not copied (by relative slash
// path, sorted: `.git`, symlinks and non-regular files), and one row per arm
// of the corpus, sorted by arm name.
type ParityReport struct {
	Baseline   string      `json:"baseline"`
	Tasks      []string    `json:"tasks"`
	TreeSHA256 string      `json:"tree_sha256"`
	Skipped    []string    `json:"skipped,omitempty"`
	Arms       []ArmParity `json:"arms"`
}

// ParityOptions selects what CheckArmParity compares. Baseline "" means the
// corpus criterion's control arm. TaskID "" means every task whose
// Disposition is not "dropped"; a TaskID names its task even when dropped.
// Tree "" means an empty scratch directory. ScratchDir "" means a fresh
// os.MkdirTemp("", "scenariobench-parity-") that is removed before
// CheckArmParity returns; a caller-supplied ScratchDir is created when absent
// and kept, holding one copy per arm named by its index in the sorted arm
// list ("0", "1", ...), so it must not already hold such a directory.
type ParityOptions struct {
	Baseline   string
	TaskID     string
	Tree       string
	ScratchDir string
}

// CheckArmParity renders every arm of corpus (the baseline included) with
// RenderArm into its own fresh copy of the source tree and compares each arm
// to the baseline over ParityFields, field by field.
//
// The source tree (a symlinked root is resolved first) is hashed (sorted
// walk; per directory and regular file the relative slash path, the
// permission bits and the sha256 of its bytes). `.git`, symlinks and
// non-regular files are not copied and are listed in ParityReport.Skipped,
// never silently dropped. RenderArm writes only into
// the copies; the source tree is never modified. Nothing is written into a
// copy before RenderArm (no stand-in hook stack to get past
// assertWiredHooksPresent): a wired arm in a tree `fathom init` never wired
// fails to render, and that reads INCOMPLETE, not PASS. A RenderArm error
// leaves the arm's prompt, settings, tools and notes_file Unrendered (a
// failed render returns no ArmFiles) with the error text in RenderError
// (the copy's path reads "<arm copy>"). A rendered part that is unrendered
// on either side is not compared, but the arm and task data still are: an
// undeclared Env or NotesFile difference fails even when a render failed.
//
// Verdict: any Undeclared field is FAIL; else any Unrendered field is
// INCOMPLETE; else PASS. The baseline's own row compares it to itself (PASS,
// or INCOMPLETE when its render failed). turn_cap and the task-prompt part of
// prompt compare Task values per selected task: equal for every arm by
// construction, still computed.
//
// It returns an error, and no report, when the baseline is empty and the
// criterion names no control, the baseline is not an arm of the corpus (the
// message names the arms), TaskID is unknown, no task is selected, ANY arm of
// the corpus (not only the compared ones) declares a field outside
// ParityFields, or the tree cannot be read (I38: an unreadable tree is never
// treated as empty).
func CheckArmParity(corpus Corpus, opts ParityOptions) (ParityReport, error) {
	armNames := make([]string, 0, len(corpus.Arms))
	for name := range corpus.Arms {
		armNames = append(armNames, name)
	}
	sort.Strings(armNames)

	baseline := opts.Baseline
	if baseline == "" {
		baseline = corpus.Criterion.Control
	}
	if baseline == "" {
		return ParityReport{}, fmt.Errorf("parity: no baseline: none was given and the corpus criterion names no control arm")
	}
	if _, ok := corpus.Arms[baseline]; !ok {
		return ParityReport{}, fmt.Errorf("parity: unknown baseline %q (corpus arms: %s)", baseline, strings.Join(armNames, ", "))
	}

	vocabulary := ParityFields()
	for _, name := range armNames {
		for _, d := range corpus.Arms[name].Declares {
			if !slices.Contains(vocabulary, d) {
				return ParityReport{}, fmt.Errorf("parity: arm %q declares unknown parity field %q (want one of %s)", name, d, strings.Join(vocabulary, ", "))
			}
		}
	}

	var tasks []Task
	for _, task := range corpus.Tasks {
		switch {
		case opts.TaskID != "":
			if task.ID == opts.TaskID {
				tasks = append(tasks, task)
			}
		case task.Disposition != "dropped":
			tasks = append(tasks, task)
		}
	}
	if opts.TaskID != "" && len(tasks) == 0 {
		return ParityReport{}, fmt.Errorf("parity: unknown task %q", opts.TaskID)
	}
	if len(tasks) == 0 {
		return ParityReport{}, fmt.Errorf("parity: no tasks selected (every task is dropped)")
	}
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	taskIDs := make([]string, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	scratch := opts.ScratchDir
	if scratch == "" {
		dir, err := os.MkdirTemp("", "scenariobench-parity-")
		if err != nil {
			return ParityReport{}, fmt.Errorf("parity: creating scratch dir: %w", err)
		}
		scratch = dir
		defer os.RemoveAll(dir)
	} else if err := os.MkdirAll(scratch, 0o755); err != nil {
		return ParityReport{}, fmt.Errorf("parity: creating scratch dir: %w", err)
	}

	tree := opts.Tree
	if tree == "" {
		tree = filepath.Join(scratch, "empty-tree")
		if err := os.Mkdir(tree, 0o755); err != nil {
			return ParityReport{}, fmt.Errorf("parity: creating empty tree: %w", err)
		}
	}
	// A symlinked root is resolved once and walked; WalkDir alone would
	// Lstat it, see a non-directory and read the tree as empty (I38).
	resolved, err := filepath.EvalSymlinks(tree)
	if err != nil {
		return ParityReport{}, fmt.Errorf("parity: reading tree: %w", err)
	}
	tree = resolved
	files, skipped, treeSHA, err := hashTree(tree)
	if err != nil {
		return ParityReport{}, err
	}

	views := make(map[string]armView, len(armNames))
	for i, name := range armNames {
		copyDir := filepath.Join(scratch, strconv.Itoa(i))
		if err := os.Mkdir(copyDir, 0o755); err != nil {
			return ParityReport{}, fmt.Errorf("parity: creating the copy for arm %q: %w", name, err)
		}
		if err := copyTreeFiles(tree, copyDir, files); err != nil {
			return ParityReport{}, err
		}
		views[name] = renderView(corpus.Arms[name], copyDir, tasks)
	}

	rep := ParityReport{Baseline: baseline, Tasks: taskIDs, TreeSHA256: treeSHA, Skipped: skipped}
	base := views[baseline]
	for _, name := range armNames {
		view := views[name]
		row := ArmParity{Arm: name}
		var differing []string
		if name != baseline {
			differing = diffViews(view, base, view.renderErr == "" && base.renderErr == "")
		}
		switch {
		case view.renderErr != "":
			row.Unrendered = slices.Clone(renderedParityFields)
			row.RenderError = view.renderErr
		case base.renderErr != "" && name != baseline:
			row.Unrendered = slices.Clone(renderedParityFields)
			row.RenderError = "baseline " + baseline + ": " + base.renderErr
		}
		declares := corpus.Arms[name].Declares
		for _, field := range differing {
			if slices.Contains(declares, field) {
				row.Declared = append(row.Declared, field)
			} else {
				row.Undeclared = append(row.Undeclared, field)
			}
		}
		switch {
		case len(row.Undeclared) > 0:
			row.Verdict = ParityFail
		case len(row.Unrendered) > 0:
			row.Verdict = ParityIncomplete
		default:
			row.Verdict = ParityPass
		}
		rep.Arms = append(rep.Arms, row)
	}
	return rep, nil
}

// armView is everything CheckArmParity compares about one arm: the rendered
// harness inputs read back from its copy, plus the arm and task data that
// need no render. renderErr is non-empty when RenderArm (or reading its
// output back) failed; the rendered members are then zero and ignored, and
// the arm and task data are still compared. renderErr never carries the
// copy's own path (it reads "<arm copy>"), so a report does not depend on
// the scratch directory.
type armView struct {
	renderErr string

	// Rendered.
	systemAppend string
	allowed      []string
	disallowed   []string
	settings     string
	mcp          string
	instr        map[string]string // instruction file rel path -> bytes (absent when the file is)
	notesMD      string
	notesMDSet   bool

	// Arm and task data.
	notes    bool
	env      map[string]string // Arm.Env without the model-override keys
	modelEnv map[string]string // Arm.Env model-override keys only
	setup    []string
	prompts  []string // Task.Prompt per selected task
	turnCaps []int    // Task.Sessions.MaxTurns per selected task
}

// renderView renders arm into dir, which is already a fresh copy of the
// source tree, and reads the result back.
func renderView(arm Arm, dir string, tasks []Task) armView {
	v := armView{notes: arm.NotesFile, setup: arm.Setup, env: map[string]string{}, modelEnv: map[string]string{}}
	for k, val := range arm.Env {
		if parityModelEnvKeys[k] {
			v.modelEnv[k] = val
		} else {
			v.env[k] = val
		}
	}
	for _, task := range tasks {
		v.prompts = append(v.prompts, task.Prompt)
		v.turnCaps = append(v.turnCaps, task.Sessions.MaxTurns)
	}

	scrub := func(msg string) string { return strings.ReplaceAll(msg, dir, "<arm copy>") }
	files, err := RenderArm(arm, dir)
	if err != nil {
		v.renderErr = scrub(err.Error())
		return v
	}
	v.systemAppend = files.SystemAppend
	v.allowed = files.AllowedTools
	v.disallowed = files.DisallowedTools

	read := func(path string) (string, bool, error) {
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return string(raw), err == nil, err
	}
	var rerr error
	if v.settings, _, rerr = read(files.SettingsPath); rerr != nil {
		v.renderErr = scrub("reading back the rendered settings: " + rerr.Error())
		return v
	}
	if v.mcp, _, rerr = read(files.MCPConfigPath); rerr != nil {
		v.renderErr = scrub("reading back the rendered MCP config: " + rerr.Error())
		return v
	}
	v.instr = map[string]string{}
	for _, rel := range armInstructionFiles {
		text, ok, rerr := read(filepath.Join(dir, filepath.FromSlash(rel)))
		if rerr != nil {
			v.renderErr = scrub("reading back " + rel + ": " + rerr.Error())
			return v
		}
		if ok {
			v.instr[rel] = text
		}
	}
	if v.notesMD, v.notesMDSet, rerr = read(filepath.Join(dir, "NOTES.md")); rerr != nil {
		v.renderErr = scrub("reading back NOTES.md: " + rerr.Error())
		return v
	}
	return v
}

// diffViews returns the sorted parity fields on which a and b differ. The
// arm and task data (task prompts, Env, NotesFile, turn caps, Setup) are
// always compared; the rendered members only when rendered is true (both
// renders succeeded). A failed render therefore leaves its fields
// Unrendered without hiding a difference in data no render produces.
func diffViews(a, b armView, rendered bool) []string {
	var out []string
	if !slices.Equal(a.prompts, b.prompts) ||
		rendered && (!maps.Equal(a.instr, b.instr) || (a.notes == b.notes && a.systemAppend != b.systemAppend)) {
		out = append(out, "prompt")
	}
	if rendered && (!slices.Equal(a.allowed, b.allowed) || !slices.Equal(a.disallowed, b.disallowed)) {
		out = append(out, "tools")
	}
	if !maps.Equal(a.env, b.env) || rendered && (a.settings != b.settings || a.mcp != b.mcp) {
		out = append(out, "settings")
	}
	// A SystemAppend difference with NotesFile differing is explained by it
	// (notes_file); with NotesFile equal it is a prompt difference (above).
	if a.notes != b.notes || rendered && (a.notesMDSet != b.notesMDSet || a.notesMD != b.notesMD) {
		out = append(out, "notes_file")
	}
	if !slices.Equal(a.turnCaps, b.turnCaps) {
		out = append(out, "turn_cap")
	}
	if !maps.Equal(a.modelEnv, b.modelEnv) {
		out = append(out, "model")
	}
	if !slices.Equal(a.setup, b.setup) {
		out = append(out, "tree")
	}
	sort.Strings(out)
	return out
}

// treeFile is one directory or regular file of the source tree.
type treeFile struct {
	rel  string // slash path relative to the tree root
	dir  bool
	perm os.FileMode
	sum  string // hex sha256 of the bytes; "dir" for a directory
}

// hashTree walks root in sorted order and returns its directories and
// regular files, the entries it will not copy (`.git` at any depth, symlinks
// and non-regular files, by relative slash path, sorted) and a sha256 over
// each entry's relative path, permission bits and content hash ("dir" for a
// directory, so an empty directory counts). Any read error refuses: an
// unreadable tree is never treated as empty (I38). root must not itself be
// a symlink (CheckArmParity resolves it first): WalkDir would not descend.
func hashTree(root string) (files []treeFile, skipped []string, sum string, err error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, nil, "", fmt.Errorf("parity: reading tree: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, nil, "", fmt.Errorf("parity: tree %s is a symlink; resolve it first", root)
	}
	if !info.IsDir() {
		return nil, nil, "", fmt.Errorf("parity: tree %s is not a directory", root)
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.Name() == ".git" {
			skipped = append(skipped, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			fi, ierr := d.Info()
			if ierr != nil {
				return ierr
			}
			files = append(files, treeFile{rel: rel, dir: true, perm: fi.Mode().Perm(), sum: "dir"})
			return nil
		}
		if !d.Type().IsRegular() { // symlinks, sockets, devices, pipes
			skipped = append(skipped, rel)
			return nil
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		h, herr := hashFile(path)
		if herr != nil {
			return herr
		}
		files = append(files, treeFile{rel: rel, perm: fi.Mode().Perm(), sum: h})
		return nil
	})
	if err != nil {
		return nil, nil, "", fmt.Errorf("parity: reading tree %s: %w", root, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	sort.Strings(skipped)
	outer := sha256.New()
	for _, f := range files {
		fmt.Fprintf(outer, "%s\x00%04o\x00%s\n", f.rel, f.perm, f.sum)
	}
	return files, skipped, hex.EncodeToString(outer.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyTreeFiles copies exactly the directories and regular files hashTree
// listed from src into dst, restoring each file's permission bits.
// Directories are created writable (0o755) so RenderArm can write into them.
func copyTreeFiles(src, dst string, files []treeFile) error {
	for _, f := range files {
		from := filepath.Join(src, filepath.FromSlash(f.rel))
		to := filepath.Join(dst, filepath.FromSlash(f.rel))
		if f.dir {
			if err := os.MkdirAll(to, 0o755); err != nil {
				return fmt.Errorf("parity: copying %s: %w", f.rel, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return fmt.Errorf("parity: copying %s: %w", f.rel, err)
		}
		raw, err := os.ReadFile(from)
		if err != nil {
			return fmt.Errorf("parity: copying %s: %w", f.rel, err)
		}
		if err := os.WriteFile(to, raw, f.perm); err != nil {
			return fmt.Errorf("parity: copying %s: %w", f.rel, err)
		}
		if err := os.Chmod(to, f.perm); err != nil {
			return fmt.Errorf("parity: copying %s: %w", f.rel, err)
		}
	}
	return nil
}
