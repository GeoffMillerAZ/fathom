package custodian

// propose.go — CC-9 (docs/plans/custodian.md, vision 22 §2 and §3b): the
// PROPOSE verb. Fathom is the instrument, never the hand: for each violated
// concern it offers a unified diff where the mend is mechanical, and names
// the remedy where it is not — and it writes nothing. The session, with a
// human present, reads the diff and applies it (`git apply`, or its own
// edit tool) or does not.
//
// What is mechanical, and only that: a dead pointer or dead script path
// whose basename names exactly ONE file in the repo — the file moved, and
// the move is unambiguous. No match (the target is gone, not moved) or two
// or more (ambiguous) is named, never guessed. Everything else — prose
// that says a forbidden thing, a budget overrun, a renamed task target,
// fathom's own stanza region — gets the remedy a person or `fathom init` /
// `fleet reconcile` must supply, because a diff there would be fathom
// writing someone's prose for them (law 15, unsoftened).
//
// A line inside a region fathom writes — the stanza between its begin and
// end markers, the INTENT.md playbook `fathom intent render` generates — is
// never offered a diff, however mechanical the mend: a hand-applied edit to
// a digest-bearing v3 stanza is exactly what `fleet reconcile` refuses as
// drift (I13), and an edit to generated text is overwritten by the next
// render. Those are listed unmended with the regenerating remedy.
//
// The relocation index is the WORKING TREE on git's authority: files git
// lists that are on disk. An unstaged `mv` is a move, not an ambiguity with
// the path it left.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Edit is one mechanical line edit a proposal offers.
type Edit struct {
	File string `json:"file"` // repo-relative
	Line int    `json:"line"` // 1-based
	Old  string `json:"old"`
	New  string `json:"new"`
	Why  string `json:"why"`
	// apply re-applies this edit's substitution to any text, so two edits
	// on one line compose in Patch (both pointers mended, not the last).
	apply func(string) string
}

// Proposal is config propose's answer for one violated concern: the edits
// it can offer, the evidence it cannot mend (each with why), and the remedy
// for the rest.
type Proposal struct {
	Concern  Concern  `json:"concern"`
	Detail   string   `json:"detail"`
	Edits    []Edit   `json:"edits,omitempty"`
	Unmended []string `json:"unmended,omitempty"`
	Remedy   string   `json:"remedy,omitempty"`
}

// Propose turns an audit's violated findings into proposals, in finding
// order. It reads the tree (to find where a dead path's file went) and
// never writes. Non-violated findings produce nothing: there is nothing to
// mend in a concern that holds, and an unable-to-check one has no evidence
// to mend from.
func Propose(root string, findings []Finding) ([]Proposal, error) {
	var idx *repoIndex
	index := func() (*repoIndex, error) {
		if idx != nil {
			return idx, nil
		}
		i, err := indexRepo(root)
		if err != nil {
			return nil, err
		}
		idx = i
		return idx, nil
	}

	regions := regionIndex{root: root, got: map[string][]markerRegion{}}
	var out []Proposal
	for _, f := range findings {
		if f.Status != StatusViolated {
			continue
		}
		p := Proposal{Concern: f.Concern, Detail: f.Detail}
		owned := 0 // unmended because the line is fathom's own
		switch f.Concern.Check.Kind {
		case KindPointersResolve:
			dead, _, err := scanPointers(root, f.Files)
			if err != nil {
				return nil, err
			}
			ix, err := index()
			if err != nil {
				return nil, err
			}
			for _, d := range dead {
				if why, err := regions.owner(d.File, d.Line); err != nil {
					return nil, err
				} else if why != "" {
					p.Unmended = append(p.Unmended, fmt.Sprintf("%s:%d → %s: %s", d.File, d.Line, d.Target, why))
					owned++
					continue
				}
				e, why := relocatePointer(root, ix, d)
				if why != "" {
					p.Unmended = append(p.Unmended, fmt.Sprintf("%s:%d → %s: %s", d.File, d.Line, d.Target, why))
					continue
				}
				p.Edits = append(p.Edits, e)
			}
			p.Remedy = remedyIfUnmended(p, owned, "fix each remaining pointer by hand — point it at what replaced the file, or delete the sentence")
		case KindCommandsResolve:
			dead, _, _, err := deadCommands(root, f.Concern.Check.Where, f.Files)
			if err != nil {
				return nil, err
			}
			ix, err := index()
			if err != nil {
				return nil, err
			}
			for _, d := range dead {
				if why, err := regions.owner(d.File, d.Line); err != nil {
					return nil, err
				} else if why != "" {
					p.Unmended = append(p.Unmended, d.evidence()+": "+why)
					owned++
					continue
				}
				if !d.Script {
					p.Unmended = append(p.Unmended, d.evidence()+": a task target's rename is not mechanically inferable — `task --list` shows what exists")
					continue
				}
				e, why := relocateScript(root, ix, d)
				if why != "" {
					p.Unmended = append(p.Unmended, d.evidence()+": "+why)
					continue
				}
				p.Edits = append(p.Edits, e)
			}
			p.Remedy = remedyIfUnmended(p, owned, "restore each remaining target or script, or edit the line to what replaced it")
		default:
			p.Remedy = remedyFor(root, f)
		}
		out = append(out, p)
	}
	return out, nil
}

// remedyIfUnmended names what the session must do for the evidence no
// diff mends: the remedy for the ones it may edit, and — when some sit
// inside fathom's markers — that those are regenerated, never hand-edited.
func remedyIfUnmended(p Proposal, owned int, remedy string) string {
	const regen = "the ones inside fathom's markers: regenerate as each says, never hand-edit (law 15)"
	switch {
	case len(p.Unmended) == 0:
		return ""
	case owned == len(p.Unmended):
		return regen
	case owned > 0:
		return remedy + "; " + regen
	}
	return remedy
}

// markerRegion is a span of lines (1-based, inclusive) fathom writes.
type markerRegion struct {
	begin, end int
	family     string // "" for the stanza; "intent-playbook"; any other fathom:<family>
}

var (
	markerBeginRe = regexp.MustCompile(`<!--\s*fathom:(?:([a-z][a-z0-9-]*)\s+)?begin\b`)
	markerEndRe   = regexp.MustCompile(`<!--\s*fathom:(?:([a-z][a-z0-9-]*)\s+)?end\b`)
)

// markerRegions finds fathom's marker regions in a file's lines, markers
// included. An unclosed begin runs to the end of the file: a corrupted
// pair is still fathom's to repair, never a session's to hand-edit.
func markerRegions(lines []string) []markerRegion {
	var out []markerRegion
	open := -1
	family := ""
	for i, ln := range lines {
		if open < 0 {
			if m := markerBeginRe.FindStringSubmatch(ln); m != nil {
				open, family = i+1, m[1]
			}
			continue
		}
		if m := markerEndRe.FindStringSubmatch(ln); m != nil && m[1] == family {
			out = append(out, markerRegion{begin: open, end: i + 1, family: family})
			open = -1
		}
	}
	if open >= 0 {
		out = append(out, markerRegion{begin: open, end: len(lines), family: family})
	}
	return out
}

// regionIndex reads each file's marker regions once.
type regionIndex struct {
	root string
	got  map[string][]markerRegion
}

// owner says why line n of rel is fathom's to write, with the remedy
// that regenerates it — "" when the line is the session's. An unreadable
// file is an error (I38): its lines cannot be vouched for either way.
func (ri *regionIndex) owner(rel string, n int) (string, error) {
	regs, ok := ri.got[rel]
	if !ok {
		lines, err := readFileLines(ri.root, rel)
		if err != nil {
			return "", fmt.Errorf("%s unreadable: %w", rel, err)
		}
		regs = markerRegions(lines)
		ri.got[rel] = regs
	}
	for _, r := range regs {
		if n < r.begin || n > r.end {
			continue
		}
		at := fmt.Sprintf("%s:%d", rel, r.begin)
		switch r.family {
		case "":
			return fmt.Sprintf("inside fathom's stanza region (%s) — never hand-edit it (law 15; a hand-edited v3 region reads as drift and fleet reconcile refuses it, I13): `fathom fleet reconcile --repo %s --apply` rewrites it from the current template", at, ri.root), nil
		case "intent-playbook":
			return fmt.Sprintf("inside the playbook `fathom intent render --playbook` generates (%s) — fix the pointer in its source under docs/intent/, then re-render; an edit here is overwritten", at), nil
		default:
			return fmt.Sprintf("inside a region fathom generates (%s, fathom:%s) — regenerate it rather than editing it by hand", at, r.family), nil
		}
	}
	return "", nil
}

// remedyFor names what the session (or fathom's own writer, inside its
// own markers) must do for a violation no diff can mend.
func remedyFor(root string, f Finding) string {
	c := f.Concern.Check
	switch c.Kind {
	case KindForbiddenPattern:
		return "reword or remove the matching line(s) — fathom will not write the replacement prose"
	case KindRequiredPattern:
		return fmt.Sprintf("add a line matching `%s` to the %s config — the want: %s", c.Pattern, c.Where, f.Concern.Want)
	case KindCommandDocumentedAndExists:
		return fmt.Sprintf("the detail names which half failed: document `%s` where the %s corpus will find it, or restore its target", c.Command, c.Where)
	case KindMaxLines, KindMaxBytes:
		return fmt.Sprintf("trim %s: move sections into path-scoped files and leave a pointer row (the thin-router shape, vision 22 §2)", c.File)
	case KindStanzaPresent:
		return "fathom init --path " + root + " — the stanza region is fathom's own, written only through its markers (law 15)"
	case KindAgentsMDReachesClaude:
		if c.ImportOnly {
			return "make each named CLAUDE.md exactly `@AGENTS.md` (move any other line into the AGENTS.md beside it), and give each named nested AGENTS.md a CLAUDE.md beside it holding only `@AGENTS.md` — fathom writes no CLAUDE file for you (ADR-059)"
		}
		if c.AllowImport != nil && !*c.AllowImport {
			return "move each CLAUDE file's content into the AGENTS.md beside it (git mv a nested one), then delete it — fathom never deletes an adopter's CLAUDE file (ADR-059)"
		}
		return "fathom init --path " + root + " — adds a marked @AGENTS.md import region to the root CLAUDE file (ADR-059), or delete that file if AGENTS.md already says everything"
	case KindStanzaToolsServed:
		return "fathom fleet reconcile --repo " + root + " --apply — the stanza region is fathom's own (law 15); deploy the daemon if it is the one behind"
	case KindWiringCurrent:
		return "read each drifted file; if the change is yours: fathom fleet reconcile --repo " + root + " --apply --rebaseline"
	case KindADRReferencesResolve:
		return "write the missing decision record, or correct the citation to the record that exists"
	case KindHandoffCitesInstruments:
		return "fathom thread close <id> --why <sha|row> for each uncited note"
	}
	return "no mechanical proposal for this kind — the evidence says what is untrue"
}

// repoIndex maps a basename to the repo files carrying it.
type repoIndex struct {
	byBase map[string][]string
}

// indexRepo lists the repo's files on git's authority (tracked plus
// untracked-not-ignored) inside a work tree, else by a walk with the
// audit's skip list; vendored trees are never relocation targets. What
// git lists is kept only when it is on disk: `--cached` still names a
// path moved or deleted in the working tree and not yet staged, and that
// path is where a file WAS. `-z` keeps a non-ASCII name unquoted.
func indexRepo(root string) (*repoIndex, error) {
	var files []string
	listed := false
	if fi, err := os.Stat(filepath.Join(root, ".git")); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
		//fathom:tenancy-allow exec-noctx custodian propose CLI path; its API takes no context
		out, gerr := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
		if gerr == nil {
			listed = true
			seen := map[string]bool{}
			for _, f := range strings.Split(string(out), "\x00") {
				if f == "" || seen[f] {
					continue // unmerged paths list once per stage
				}
				seen[f] = true
				fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(f)))
				switch {
				case errors.Is(err, fs.ErrNotExist):
					continue // moved or deleted in the working tree
				case err == nil && fi.IsDir():
					continue // a submodule, not a file
				}
				// A path that exists and cannot be stat'd stays a candidate:
				// dropping it would call a file gone that is there (I38).
				files = append(files, f)
			}
		}
	}
	if !listed {
		err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if _, skip := walkSkipDirs[d.Name()]; skip && p != "." {
					return fs.SkipDir
				}
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("indexing the repo for relocations: %w", err)
		}
	}
	ix := &repoIndex{byBase: map[string][]string{}}
	for _, f := range files {
		if VendoredPath(f) {
			continue
		}
		ix.byBase[path.Base(f)] = append(ix.byBase[path.Base(f)], f)
	}
	for _, v := range ix.byBase {
		sort.Strings(v)
	}
	return ix, nil
}

// candidate returns the one repo file named like target, or why not.
func (ix *repoIndex) candidate(target string) (string, string) {
	base := path.Base(strings.TrimSuffix(target, "/"))
	if strings.HasSuffix(target, "/") {
		return "", "a directory pointer — no mechanical relocation"
	}
	hits := ix.byBase[base]
	switch len(hits) {
	case 0:
		return "", fmt.Sprintf("no file named %s anywhere in the repo — the target is gone, not moved", base)
	case 1:
		return hits[0], ""
	}
	shown := hits
	if len(shown) > 4 {
		shown = shown[:4]
	}
	return "", fmt.Sprintf("%d files named %s (%s) — ambiguous, needs a human", len(hits), base, strings.Join(shown, ", "))
}

func relocatePointer(root string, ix *repoIndex, d pointerRef) (Edit, string) {
	cand, why := ix.candidate(d.Target)
	if why != "" {
		return Edit{}, why
	}
	line, err := lineAt(root, d.File, d.Line)
	if err != nil {
		return Edit{}, err.Error()
	}
	var sub func(string) string
	if d.Link {
		// Markdown links resolve file-relative: keep that contract.
		rel, err := filepath.Rel(filepath.Dir(filepath.FromSlash(d.File)), filepath.FromSlash(cand))
		if err != nil {
			return Edit{}, err.Error()
		}
		re := regexp.MustCompile(`\]\(` + regexp.QuoteMeta(d.Target) + `([#)])`)
		repl := "](" + strings.ReplaceAll(filepath.ToSlash(rel), "$", "$$") + "$1"
		sub = func(s string) string { return re.ReplaceAllString(s, repl) }
	} else {
		// Backtick pointers cite from the repo root in this style.
		from, to := "`"+d.Target+"`", "`"+cand+"`"
		sub = func(s string) string { return strings.ReplaceAll(s, from, to) }
	}
	updated := sub(line)
	if updated == line {
		return Edit{}, "could not locate the pointer's text on its line"
	}
	return Edit{File: d.File, Line: d.Line, Old: line, New: updated, Why: fmt.Sprintf("%s moved to %s (the only file named %s)", d.Target, cand, path.Base(cand)), apply: sub}, ""
}

func relocateScript(root string, ix *repoIndex, d deadCommand) (Edit, string) {
	cand, why := ix.candidate(d.Name)
	if why != "" {
		return Edit{}, why
	}
	line, err := lineAt(root, d.File, d.Line)
	if err != nil {
		return Edit{}, err.Error()
	}
	// The new path is written from where the command runs: the root for a
	// doc or a root-run hook, the task's / hook's own directory otherwise.
	newPath := cand
	if len(d.Dirs) == 1 && d.Dirs[0] != "." {
		r, err := filepath.Rel(filepath.FromSlash(d.Dirs[0]), filepath.FromSlash(cand))
		if err != nil {
			return Edit{}, err.Error()
		}
		newPath = filepath.ToSlash(r)
	}
	if strings.HasPrefix(d.Name, "./") && !strings.HasPrefix(newPath, "../") {
		newPath = "./" + newPath
	}
	re := scriptWordRe(d.Name)
	repl := "${1}" + strings.ReplaceAll(newPath, "$", "$$") + "${2}"
	sub := func(s string) string { return re.ReplaceAllString(s, repl) }
	updated := sub(line)
	if updated == line {
		return Edit{}, "could not locate the script on its cited line"
	}
	return Edit{File: d.File, Line: d.Line, Old: line, New: updated, Why: fmt.Sprintf("%s moved to %s (the only file named %s)", d.Name, cand, path.Base(cand)), apply: sub}, ""
}

func lineAt(root, rel string, n int) (string, error) {
	lines, err := readFileLines(root, rel)
	if err != nil {
		return "", fmt.Errorf("%s unreadable: %v", rel, err)
	}
	if n < 1 || n > len(lines) {
		return "", fmt.Errorf("%s has no line %d", rel, n)
	}
	return lines[n-1], nil
}

// Patch renders edits as one unified diff with a/ b/ paths — `git apply`
// accepts it from the repo root. Two edits on one line compose in order.
// It reads the files it diffs and writes nothing.
func Patch(root string, edits []Edit) (string, error) {
	byFile := map[string][]Edit{}
	var files []string
	for _, e := range edits {
		if _, ok := byFile[e.File]; !ok {
			files = append(files, e.File)
		}
		byFile[e.File] = append(byFile[e.File], e)
	}
	sort.Strings(files)

	var b strings.Builder
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return "", fmt.Errorf("%s unreadable: %w", file, err)
		}
		text := string(data)
		noEOL := text != "" && !strings.HasSuffix(text, "\n")
		old := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		if text == "" {
			old = nil
		}
		cur := append([]string(nil), old...)
		changed := map[int]bool{}
		for _, e := range byFile[file] {
			i := e.Line - 1
			if i < 0 || i >= len(cur) {
				return "", fmt.Errorf("%s: edit at line %d is outside the file", file, e.Line)
			}
			if old[i] != e.Old {
				return "", fmt.Errorf("%s:%d changed since it was read", file, e.Line)
			}
			// Compose: a second edit on a line re-applies its own
			// substitution to the line as it stands, so both land.
			switch {
			case !changed[i]:
				cur[i] = e.New
			case e.apply != nil:
				cur[i] = e.apply(cur[i])
			default:
				return "", fmt.Errorf("%s:%d: two edits on one line and the second cannot compose", file, e.Line)
			}
			changed[i] = true
		}
		writeFileDiff(&b, file, old, cur, changed, noEOL)
	}
	return b.String(), nil
}

const diffContext = 3

func writeFileDiff(b *strings.Builder, file string, old, cur []string, changed map[int]bool, noEOL bool) {
	var idxs []int
	for i := range changed {
		if old[i] != cur[i] {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) == 0 {
		return
	}
	sort.Ints(idxs)
	fmt.Fprintf(b, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n", file, file, file, file)

	// Group changed lines into hunks whose context windows touch.
	type hunk struct{ lo, hi int } // inclusive line range, 0-based
	var hunks []hunk
	for _, i := range idxs {
		lo, hi := max(0, i-diffContext), min(len(old)-1, i+diffContext)
		if n := len(hunks); n > 0 && lo <= hunks[n-1].hi+1 {
			hunks[n-1].hi = hi
			continue
		}
		hunks = append(hunks, hunk{lo, hi})
	}
	last := len(old) - 1
	eolMark := func(i int) {
		if noEOL && i == last {
			b.WriteString("\\ No newline at end of file\n")
		}
	}
	for _, h := range hunks {
		n := h.hi - h.lo + 1
		fmt.Fprintf(b, "@@ -%d,%d +%d,%d @@\n", h.lo+1, n, h.lo+1, n)
		for i := h.lo; i <= h.hi; i++ {
			if old[i] == cur[i] {
				b.WriteString(" " + old[i] + "\n")
				eolMark(i)
				continue
			}
			b.WriteString("-" + old[i] + "\n")
			eolMark(i)
			b.WriteString("+" + cur[i] + "\n")
			eolMark(i)
		}
	}
}
