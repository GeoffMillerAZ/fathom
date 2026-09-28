package specdialect

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// ChangedSymbol is one symbol added or changed on a spec's branch range:
// its definition overlaps a changed hunk of a file in the range's
// changed-file set (design §5, section 3).
type ChangedSymbol struct {
	Name    string `json:"symbol"`
	Package string `json:"package,omitempty"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// ConvergeItem is one task in the converge report.
type ConvergeItem struct {
	TaskID string `json:"id"`
	Title  string `json:"title"`
	Line   int    `json:"line"`
	// Criterion is the first failing criterion, "name: detail"
	// (claimed-unproven only).
	Criterion string `json:"criterion,omitempty"`
	// Accept is the task's acceptance text, shown, never gating.
	Accept string `json:"accept,omitempty"`
	// Proposals are anchor proposals for an unanchored task (design §6),
	// always marked as proposals. Empty until the decision port's anchor
	// calibration (SI-M4) passes; nothing here is ever a contract.
	Proposals []string `json:"proposals,omitempty"`
}

// ConvergeReport is the converge report for one spec (design §5).
type ConvergeReport struct {
	Spec    string `json:"spec"`
	Dialect string `json:"dialect"`
	Base    string `json:"base,omitempty"`
	// ClaimedUnproven: tasks marked done that fail their contract.
	ClaimedUnproven []ConvergeItem `json:"claimed_unproven"`
	// Unanchored: open or claimed tasks with no contract.
	Unanchored []ConvergeItem `json:"unanchored"`
	// Unclaimed: symbols changed on the branch range no task anchors.
	Unclaimed []ChangedSymbol `json:"unclaimed"`
	// UnclaimedNote says why the unclaimed-code section was not evaluated
	// ("" when it was) — never an empty section standing in for "none".
	UnclaimedNote string `json:"unclaimed_note,omitempty"`
}

// Converge builds the three sections from a parsed spec, its verify
// report, and the symbols changed on its branch range. Pure and
// deterministic: tasks in spec order (by line, then id), unclaimed symbols
// by file, line, name.
func Converge(plan verify.Plan, rep verify.Report, changed []ChangedSymbol) ConvergeReport {
	verdicts := make(map[string]verify.TaskVerdict, len(rep.Tasks))
	for _, v := range rep.Tasks {
		verdicts[v.TaskID] = v
	}
	tasks := append([]verify.Task(nil), plan.Tasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Line != tasks[j].Line {
			return tasks[i].Line < tasks[j].Line
		}
		return tasks[i].ID < tasks[j].ID
	})

	out := ConvergeReport{ClaimedUnproven: []ConvergeItem{}, Unanchored: []ConvergeItem{}, Unclaimed: []ChangedSymbol{}}
	for _, t := range tasks {
		if t.Skipped {
			continue
		}
		item := ConvergeItem{TaskID: t.ID, Title: t.Title, Line: t.Line, Accept: t.Accept}
		if !hasContract(t) && len(t.SpecIDs) == 0 {
			out.Unanchored = append(out.Unanchored, item)
			continue
		}
		v, ok := verdicts[t.ID]
		if t.Claimed && ok && v.Status == verify.Fail {
			for _, c := range v.Criteria {
				if c.Status == verify.Fail {
					item.Criterion = c.Name + ": " + c.Detail
					break
				}
			}
			out.ClaimedUnproven = append(out.ClaimedUnproven, item)
		}
	}

	for _, s := range changed {
		if !anchored(tasks, s) {
			out.Unclaimed = append(out.Unclaimed, s)
		}
	}
	sort.Slice(out.Unclaimed, func(i, j int) bool {
		a, b := out.Unclaimed[i], out.Unclaimed[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	return out
}

// anchored reports whether any task anchors s: a symbol: naming it (a
// method's bare name counts; package:, when set, must contain its file),
// or a lexical path, gone: paths or ctx pointer naming its file.
func anchored(tasks []verify.Task, s ChangedSymbol) bool {
	for _, t := range tasks {
		if sym := t.Expect.Symbol; sym != "" && (sym == s.Name || strings.HasSuffix(s.Name, "."+sym)) {
			if pkg := strings.Trim(t.Expect.Package, "/"); pkg == "" || path.Dir(s.File) == pkg || strings.HasPrefix(s.File, pkg+"/") {
				return true
			}
		}
		if t.OraclePath != "" && t.OraclePath == s.File {
			return true
		}
		for _, p := range append(append([]string(nil), t.GonePaths...), t.Ctx...) {
			if p == s.File {
				return true
			}
		}
	}
	return false
}

// LineRange is an inclusive 1-based line range.
type LineRange struct{ Start, End int }

var (
	diffFileRe = regexp.MustCompile(`^\+\+\+ (?:b/)?(.+?)\s*$`)
	diffHunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
)

// ParseHunks reads a `git diff -U0` and returns each file's head-side
// changed line ranges. A pure deletion contributes the one-line point
// where the content was removed, so a function that lost lines still
// reads as changed. A deleted file (+++ /dev/null) contributes nothing.
func ParseHunks(diff string) map[string][]LineRange {
	out := map[string][]LineRange{}
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		if m := diffFileRe.FindStringSubmatch(line); m != nil {
			file = m[1]
			if file == "/dev/null" {
				file = ""
			}
			continue
		}
		m := diffHunkRe.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		end := start + count - 1
		if count == 0 {
			end = start
		}
		out[file] = append(out[file], LineRange{Start: start, End: end})
	}
	return out
}

// SymbolSpan is a symbol's definition range in a file (EndLine 0: unknown,
// the definition line alone).
type SymbolSpan struct {
	Name, Package, File string
	Line, EndLine       int
}

// ChangedSymbols returns the symbols whose definition overlaps a changed
// hunk of their own file, sorted by file, line, name, deduplicated.
func ChangedSymbols(hunks map[string][]LineRange, symbols []SymbolSpan) []ChangedSymbol {
	seen := map[string]bool{}
	var out []ChangedSymbol
	for _, s := range symbols {
		end := s.EndLine
		if end < s.Line {
			end = s.Line
		}
		for _, h := range hunks[s.File] {
			if h.Start <= end && h.End >= s.Line {
				k := fmt.Sprintf("%s\x00%d\x00%s", s.File, s.Line, s.Name)
				if !seen[k] {
					seen[k] = true
					out = append(out, ChangedSymbol{Name: s.Name, Package: s.Package, File: s.File, Line: s.Line})
				}
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// RenderConverge writes r as text: one header per section with its count,
// then one line per item located in the spec file.
func RenderConverge(w io.Writer, r ConvergeReport) {
	base := ""
	if r.Base != "" {
		base = " since " + r.Base
	}
	fmt.Fprintf(w, "\nconverge: %s (%s)%s\n", r.Spec, r.Dialect, base)
	fmt.Fprintf(w, "  claimed-unproven (%d)\n", len(r.ClaimedUnproven))
	for _, it := range r.ClaimedUnproven {
		fmt.Fprintf(w, "    %s:%d [%s] %s — %s\n", r.Spec, it.Line, it.TaskID, it.Title, it.Criterion)
	}
	fmt.Fprintf(w, "  unanchored (%d)\n", len(r.Unanchored))
	for _, it := range r.Unanchored {
		fmt.Fprintf(w, "    %s:%d [%s] %s\n", r.Spec, it.Line, it.TaskID, it.Title)
		for _, p := range it.Proposals {
			fmt.Fprintf(w, "      proposal (not proof): %s\n", p)
		}
	}
	if r.UnclaimedNote != "" {
		fmt.Fprintf(w, "  unclaimed code: not evaluated — %s\n", r.UnclaimedNote)
		return
	}
	fmt.Fprintf(w, "  unclaimed code (%d)\n", len(r.Unclaimed))
	for _, s := range r.Unclaimed {
		fmt.Fprintf(w, "    %s:%d %s\n", s.File, s.Line, s.Name)
	}
}
