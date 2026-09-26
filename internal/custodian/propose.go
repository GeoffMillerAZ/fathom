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

import (
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

	var out []Proposal
	for _, f := range findings {
		if f.Status != StatusViolated {
			continue
		}
		p := Proposal{Concern: f.Concern, Detail: f.Detail}
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
				e, why := relocatePointer(root, ix, d)
				if why != "" {
					p.Unmended = append(p.Unmended, fmt.Sprintf("%s:%d → %s: %s", d.File, d.Line, d.Target, why))
					continue
				}
				p.Edits = append(p.Edits, e)
			}
			p.Remedy = remedyIfUnmended(p, "fix each remaining pointer by hand — point it at what replaced the file, or delete the sentence")
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
			p.Remedy = remedyIfUnmended(p, "restore each remaining target or script, or edit the line to what replaced it")
		default:
			p.Remedy = remedyFor(root, f)
		}
		out = append(out, p)
	}
	return out, nil
}

func remedyIfUnmended(p Proposal, remedy string) string {
	if len(p.Unmended) == 0 {
		return ""
	}
	return remedy
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
// audit's skip list; vendored trees are never relocation targets.
func indexRepo(root string) (*repoIndex, error) {
	var files []string
	if fi, err := os.Stat(filepath.Join(root, ".git")); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
		out, gerr := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard").Output()
		if gerr == nil {
			for _, ln := range strings.Split(string(out), "\n") {
				if ln = strings.TrimSpace(ln); ln != "" {
					files = append(files, ln)
				}
			}
		}
	}
	if files == nil {
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
		if vendoredPath(f) {
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
	newPath := cand
	if strings.HasPrefix(d.Name, "./") {
		newPath = "./" + cand
	}
	re := regexp.MustCompile(`(^|[\s"'=])` + regexp.QuoteMeta(d.Name) + `($|[\s"';&|)])`)
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
