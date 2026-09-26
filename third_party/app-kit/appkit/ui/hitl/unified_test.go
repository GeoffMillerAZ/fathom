package hitl

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

const unifiedOld = `# retry policy
[retry]
attempts = 3
backoff = "1s"
jitter = false

[timeouts]
connect = "2s"
read = "5s"
write = "5s"
idle = "60s"
shutdown = "10s"

[limits]
queue = 64
workers = 4`

const unifiedNew = `# retry policy
[retry]
attempts = 5
backoff = "1s"
jitter = true

[timeouts]
connect = "2s"
read = "5s"
write = "5s"
idle = "60s"
shutdown = "10s"

[limits]
queue = 128
workers = 4
`

func mustUnified(t *testing.T, d UnifiedDiff) string {
	t.Helper()
	got, err := RenderUnifiedDiff(d)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func unifiedGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".html.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 go test ./appkit/ui/hitl): %v", err)
	}
	if string(want) != got {
		t.Errorf("output differs from %s; review and regenerate with UPDATE_GOLDEN=1\n got: %s", path, got)
	}
}

func TestRenderUnifiedDiffGolden(t *testing.T) {
	d := UnifiedDiff{Label: "service.toml", Old: unifiedOld, New: unifiedNew, Context: 1}
	got := mustUnified(t, d)
	unifiedGolden(t, "unified_config", got)
	if again := mustUnified(t, d); again != got {
		t.Fatal("RenderUnifiedDiff is not deterministic")
	}
}

func TestUnifiedDiffRendersMarkersNumbersAndLabels(t *testing.T) {
	got := mustUnified(t, UnifiedDiff{Label: "service.toml", Old: unifiedOld, New: unifiedNew, Context: 1})
	for _, want := range []string{
		`data-unified-diff`, `<figcaption`, `><bdi>service.toml</bdi></span>`, `current`, `proposed`,
		`4 lines added, 4 lines removed`,
		`tabindex="0" role="region" aria-label="service.toml line changes, current to proposed"`,
		`focus-visible:outline-focus`, `overflow-x-auto`, `min-w-max`, `whitespace-pre`,
		`<span aria-hidden="true">@@ -2,5 +2,5 @@</span><span class="sr-only">Changed region: current 2–6, proposed 2–6</span>`, `@@ -14,3 +14,3 @@`, `</span> 1 unchanged line (line 1)</summary>`,
		`data-kind="removed" data-old-line="3" data-new-line=""`,
		`<span class="sr-only select-none">Removed, current line 3: </span>`,
		`data-kind="added" data-old-line="" data-new-line="3"`,
		`<span class="sr-only select-none">Added, proposed line 3: </span>`,
		`<span class="w-5 text-center font-semibold">+</span>`, `<span class="w-5 text-center font-semibold">-</span>`,
		`<span class="sr-only select-none">Unchanged, line 2: </span>`,
		// Unchanged runs outside the context window collapse behind native details.
		`<details class="group border-y border-dashed border-line" data-diff-collapsed>`,
		`</span> 7 unchanged lines (lines 7–13)</summary>`, `<span class="group-open:hidden">Show</span><span class="hidden group-open:inline">Hide</span>`,
		// Old text had no final newline; the new one does.
		`data-no-newline`, `No newline at end of file`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("unified diff missing %q", want)
		}
	}
	if strings.Contains(got, "<table") {
		t.Error("unified diff must stay table-free")
	}
	if strings.Count(got, "<details") != 2 {
		t.Errorf("want two collapsed runs (leading line 1 and lines 7–13), got %d", strings.Count(got, "<details"))
	}
}

func TestUnifiedDiffCollapsedRangesNameBothSidesWhenTheyDiffer(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\nh\n"
	new := "new\na\nb\nc\nd\ne\nf\ng\nH\n"
	got := mustUnified(t, UnifiedDiff{Label: "shifted", Old: old, New: new, Context: 1})
	if !strings.Contains(got, `</span> 5 unchanged lines (<bdi>current</bdi> 2–6, <bdi>proposed</bdi> 3–7)`) {
		t.Errorf("shifted collapsed range not described per side: %s", got)
	}
	if !strings.Contains(got, `Unchanged, current line 1, proposed line 2: `) {
		t.Error("shifted context line lacks per-side numbers")
	}
}

func TestUnifiedDiffPrecomputedHunksShowOmittedGaps(t *testing.T) {
	d, err := DiffText(unifiedOld, unifiedNew)
	if err != nil {
		t.Fatal(err)
	}
	hunks := d.Hunks(NoContext)
	before := make([]Hunk, len(hunks))
	for i, h := range hunks {
		before[i] = h
		before[i].Lines = append([]DiffLine(nil), h.Lines...)
	}
	got := mustUnified(t, UnifiedDiff{Label: "service.toml", OldName: "main", NewName: "branch", Hunks: hunks})
	for _, want := range []string{
		`>Lines 1–2 not included</p>`, `>Line 4 not included</p>`, `>Lines 6–14 not included</p>`,
		`@@ -3 +3 @@`, `@@ -5 +5 @@`, `@@ -15,2 +15,2 @@`, `Added, branch line 16: `,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("precomputed render missing %q", want)
		}
	}
	if strings.Contains(got, "<details") {
		t.Error("precomputed hunks cannot expand text the renderer was never given")
	}
	if !reflect.DeepEqual(hunks, before) {
		t.Error("RenderUnifiedDiff modified the caller's hunks")
	}
	// Numbers may be omitted: they are derived from the hunk range.
	bare := []Hunk{{OldStart: 10, OldCount: 2, NewStart: 10, NewCount: 1, Lines: []DiffLine{
		{Kind: DiffContext, Text: "keep"}, {Kind: DiffRemoved, Text: "drop"},
	}}}
	got = mustUnified(t, UnifiedDiff{Label: "bare", Hunks: bare})
	if !strings.Contains(got, `data-kind="removed" data-old-line="11" data-new-line=""`) || bare[0].Lines[1].OldNumber != 0 {
		t.Errorf("derived numbering wrong or caller mutated: %s", got)
	}
}

// TestUnifiedDiffOmittedRangesNeverClaimUnchanged: the renderer never received
// the lines between precomputed hunks, so it cannot know they are unchanged.
// An app that passes a subset of hunks (only those touching one symbol) must
// not have a real change in the gap reported as unchanged.
func TestUnifiedDiffOmittedRangesNeverClaimUnchanged(t *testing.T) {
	d, err := DiffText(unifiedOld, unifiedNew)
	if err != nil {
		t.Fatal(err)
	}
	h := d.Hunks(NoContext)
	if len(h) != 3 || h[1].Header() != "@@ -5 +5 @@" {
		t.Fatalf("fixture hunks changed: %v", h)
	}
	got := mustUnified(t, UnifiedDiff{Label: "service.toml", Hunks: []Hunk{h[0], h[2]}})
	omitted := regexp.MustCompile(`data-diff-omitted>([^<]*)</p>`).FindAllStringSubmatch(got, -1)
	if len(omitted) != 2 || omitted[0][1] != "Lines 1–2 not included" || omitted[1][1] != "Lines 4–14 not included" {
		t.Errorf("omitted ranges around a subset of hunks: %q", omitted)
	}
	if strings.Contains(strings.ToLower(got), "unchanged") {
		t.Error("lines the renderer never received were called unchanged")
	}
	if strings.Contains(got, "jitter") {
		t.Error("a hunk that was not supplied was rendered")
	}
	// A skipped hunk can change the line count, so each side's skipped range
	// is stated on its own: current line 2 against proposed lines 2–9.
	got = mustUnified(t, UnifiedDiff{Label: "x", Hunks: []Hunk{
		{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1, Lines: []DiffLine{{Kind: DiffContext, Text: "x"}}},
		{OldStart: 3, OldCount: 1, NewStart: 10, NewCount: 1, Lines: []DiffLine{{Kind: DiffRemoved, Text: "b"}, {Kind: DiffAdded, Text: "B"}}},
	}})
	if !strings.Contains(got, `data-diff-omitted>Not included: <bdi>current</bdi> 2, <bdi>proposed</bdi> 2–9</p>`) {
		t.Errorf("per-side skipped ranges: %s", got)
	}
	got = mustUnified(t, UnifiedDiff{Label: "x", Hunks: []Hunk{{OldStart: 2, OldCount: 0, NewStart: 5, NewCount: 1, Lines: []DiffLine{{Kind: DiffAdded, Text: "n"}}}}})
	if !strings.Contains(got, `data-diff-omitted>Not included: <bdi>current</bdi> 1–2, <bdi>proposed</bdi> 1–4</p>`) {
		t.Errorf("leading per-side range: %s", got)
	}
	// Every subset of a real comparison's hunks is accepted.
	for mask := 1; mask < 1<<len(h); mask++ {
		var subset []Hunk
		for i := range h {
			if mask&(1<<i) != 0 {
				subset = append(subset, h[i])
			}
		}
		if _, err := RenderUnifiedDiff(UnifiedDiff{Label: "x", Hunks: subset}); err != nil {
			t.Errorf("subset %b refused: %v", mask, err)
		}
	}
}

// TestUnifiedDiffPrecomputedNoNewlineAndBudget covers the precomputed forms of
// two states the texts path already has: a final line without a newline, and
// a comparison beyond the edit budget.
func TestUnifiedDiffPrecomputedNoNewlineAndBudget(t *testing.T) {
	for _, pair := range [][2]string{{"a\nb", "a\nb\n"}, {"a", "b"}, {"a\nb\n", "a\nc"}, {"x\ny", "x\ny\nz"}, {"k\nx", "k\nx"}} {
		d, err := DiffText(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Count(mustUnified(t, UnifiedDiff{Label: "eol", Old: pair[0], New: pair[1]}), "data-no-newline")
		for _, context := range []int{NoContext, 0} {
			got := mustUnified(t, UnifiedDiff{Label: "eol", Hunks: d.Hunks(context)})
			if n := strings.Count(got, "data-no-newline"); n != want {
				t.Errorf("%q -> %q context %d: %d no-newline rows, texts path has %d", pair[0], pair[1], context, n, want)
			}
		}
	}

	var old, new strings.Builder
	for i := range 600 {
		fmt.Fprintf(&old, "old%d\n", i)
		fmt.Fprintf(&new, "new%d\n", i)
	}
	old.WriteString("shared middle line\n")
	new.WriteString("shared middle line\n")
	for i := range 600 {
		fmt.Fprintf(&old, "old2-%d\n", i)
		fmt.Fprintf(&new, "new2-%d\n", i)
	}
	big, err := DiffText(old.String(), new.String())
	if err != nil {
		t.Fatal(err)
	}
	if !big.Replaced {
		t.Fatal("fixture no longer exceeds the edit budget")
	}
	got := mustUnified(t, UnifiedDiff{Label: "rewrite", Hunks: big.Hunks(0), Replaced: big.Replaced})
	if !strings.Contains(got, "exceeded the diff budget") {
		t.Error("precomputed hunks from a replaced comparison lost the budget disclosure")
	}
	if strings.Contains(mustUnified(t, UnifiedDiff{Label: "rewrite", Hunks: big.Hunks(0)}), "exceeded the diff budget") {
		t.Error("budget note shown without Replaced")
	}
}

func TestUnifiedDiffHonestEmptyAndBudgetStates(t *testing.T) {
	same := mustUnified(t, UnifiedDiff{Label: "same", Old: "x\n", New: "x\n"})
	if !strings.Contains(same, "No changes: the texts are identical.") || strings.Contains(same, `role="region"`) || strings.Contains(same, "added,") {
		t.Errorf("identical texts: %s", same)
	}
	none := mustUnified(t, UnifiedDiff{Label: "none", Hunks: []Hunk{}})
	if !strings.Contains(none, "No changed lines were supplied.") {
		t.Errorf("empty precomputed hunks: %s", none)
	}
	var a, b strings.Builder
	for i := 0; i < editBudget+10; i++ {
		a.WriteString("a" + string(rune('A'+i%26)) + "\n")
		b.WriteString("b" + string(rune('A'+i%26)) + "\n")
	}
	budget := mustUnified(t, UnifiedDiff{Label: "rewrite", Old: a.String(), New: b.String()})
	if !strings.Contains(budget, "exceeded the diff budget") {
		t.Error("budget fallback was not disclosed")
	}
}

func TestUnifiedDiffEscapesAndRevealsHiddenCharacters(t *testing.T) {
	got := mustUnified(t, UnifiedDiff{
		Label: `<i>label</i>`, OldName: `<old>`, NewName: `"new"`,
		Old: "<script>alert(1)</script>\n", New: "ok\u202e\r\n",
	})
	for _, bad := range []string{"<script>", "<i>", "<old>", "\u202e", "\r"} {
		if strings.Contains(got, bad) {
			t.Errorf("raw %q reached markup", bad)
		}
	}
	for _, want := range []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "&lt;i&gt;label&lt;/i&gt;", ">U+202E</span>", ">U+000D</span>", "data-revealed"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// rowOf returns the first <li> of the given kind.
func rowOf(t *testing.T, html, kind string) string {
	t.Helper()
	i := strings.Index(html, `data-kind="`+kind+`"`)
	if i < 0 {
		t.Fatalf("no %s row in %s", kind, html)
	}
	row := html[i:]
	return row[:strings.Index(row, "</li>")]
}

// TestUnifiedDiffRevealsLineTerminatorsAndTagCharacters covers two ways an
// added line can hide what it does: U+2028 draws as a space but ends a
// JavaScript line, so the call after "//" runs; tag characters have no width,
// so the added line would look identical to the removed one.
func TestUnifiedDiffRevealsLineTerminatorsAndTagCharacters(t *testing.T) {
	head := "let ran=false; const sendTokens=()=>{ran=true};\n"
	tail := "console.log(\"sendTokens ran:\", ran);\n"
	got := mustUnified(t, UnifiedDiff{
		Label: "ls.js",
		Old:   head + "const debug = false;\n" + tail,
		New:   head + "const debug = false; // disabled:\u2028sendTokens()\n" + tail,
	})
	if added := rowOf(t, got, "added"); !strings.Contains(added, `// disabled:<span class="mx-px rounded-sm border border-danger px-0.5 text-ink" data-revealed>U+2028</span>sendTokens()`) {
		t.Errorf("U+2028 not revealed in the added line: %s", added)
	}
	got = mustUnified(t, UnifiedDiff{Label: "policy.toml", Old: "role = \"user\"\n", New: "role = \"user\U000E0061\U000E0064\"\n"})
	added, removed := rowOf(t, got, "added"), rowOf(t, got, "removed")
	if !strings.Contains(added, `>U+E0061</span><span class="mx-px rounded-sm border border-danger px-0.5 text-ink" data-revealed>U+E0064</span>`) {
		t.Errorf("tag characters not revealed in the added line: %s", added)
	}
	if strings.Contains(removed, "data-revealed") {
		t.Errorf("removed line gained a notation: %s", removed)
	}
	for _, raw := range []string{"\u2028", "\U000E0061", "\U000E0064"} {
		if strings.Contains(got, raw) {
			t.Errorf("raw %q reached markup", raw)
		}
	}
}

// TestUnifiedDiffRevealsNamesAndLabel keeps the file and side names honest: a
// right-to-left override in a path or branch name must not spoof what is being
// reviewed, in the caption, the collapsed and omitted ranges or the spoken
// labels.
func TestUnifiedDiffRevealsNamesAndLabel(t *testing.T) {
	d := UnifiedDiff{
		Label: "deploy/prod\u202elmot.sh", OldName: "main", NewName: "fix\u202eniam",
		Old: "a\nb\nc\nd\ne\n", New: "new\na\nb\nc\nd\nE\n", Context: 1,
	}
	got := mustUnified(t, d)
	if strings.Contains(got, "\u202e") {
		t.Fatalf("raw U+202E reached markup: %s", got)
	}
	reveal := `<span class="mx-px rounded-sm border border-danger px-0.5 text-ink" data-revealed>U+202E</span>`
	for _, want := range []string{
		`<bdi>deploy/prod` + reveal + `lmot.sh</bdi></span>`,
		`<bdi>main</bdi> <span aria-hidden="true">→ + </span><span class="sr-only">compared with </span><bdi>fix` + reveal + `niam</bdi>`,
		`aria-label="deploy/prodU+202Elmot.sh line changes, main to fixU+202Eniam"`,
		`2 unchanged lines (<bdi>main</bdi> 2–3, <bdi>fix` + reveal + `niam</bdi> 3–4)</summary>`,
		// Spoken labels carry the notation as plain text.
		`Added, fixU+202Eniam line 1: `,
		`Changed region: main 4–5, fixU+202Eniam 5–6`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	hunks, err := DiffText(d.Old, d.New)
	if err != nil {
		t.Fatal(err)
	}
	got = mustUnified(t, UnifiedDiff{Label: d.Label, OldName: d.OldName, NewName: d.NewName, Hunks: hunks.Hunks(NoContext)})
	if strings.Contains(got, "\u202e") || !strings.Contains(got, `Not included: <bdi>main</bdi> 1–4, <bdi>fix`+reveal+`niam</bdi> 2–5`) {
		t.Errorf("precomputed omitted range does not reveal the side name: %s", got)
	}
}

func TestUnifiedDiffRefusesInvalidInputAtomically(t *testing.T) {
	good := []DiffLine{{Kind: DiffRemoved, Text: "a"}, {Kind: DiffAdded, Text: "b"}}
	cases := map[string]UnifiedDiff{
		"blank label":        {Label: " ", Old: "a", New: "b"},
		"texts and hunks":    {Label: "x", Old: "a", Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1, Lines: good}}},
		"count mismatch":     {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 2, NewStart: 1, NewCount: 1, Lines: good}}},
		"unknown kind":       {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 0, Lines: []DiffLine{{Kind: "moved", Text: "a"}}}}},
		"number disagrees":   {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1, Lines: []DiffLine{{Kind: DiffRemoved, OldNumber: 7, Text: "a"}, {Kind: DiffAdded, Text: "b"}}}}},
		"empty hunk":         {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 0, NewStart: 1, NewCount: 0}}},
		"newline in line":    {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 0, NewCount: 0, Lines: []DiffLine{{Kind: DiffRemoved, Text: "a\nb"}}}}},
		"nonempty from zero": {Label: "x", Hunks: []Hunk{{OldStart: 0, OldCount: 1, NewStart: 1, NewCount: 1, Lines: good}}},
		"overlapping hunks": {Label: "x", Hunks: []Hunk{
			{OldStart: 3, OldCount: 1, NewStart: 3, NewCount: 1, Lines: good},
			{OldStart: 3, OldCount: 1, NewStart: 4, NewCount: 1, Lines: good},
		}},
		"oversized text": {Label: "x", Old: strings.Repeat("x\n", MaxDiffLines+1)},
		"new side overlaps": {Label: "x", Hunks: []Hunk{
			{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1, Lines: good},
			{OldStart: 5, OldCount: 1, NewStart: 1, NewCount: 1, Lines: good},
		}},
		// "No newline at end of file" belongs to the last line of a side only.
		"no newline mid hunk": {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 2, NewStart: 1, NewCount: 2, Lines: []DiffLine{
			{Kind: DiffContext, Text: "a", NoNewline: true}, {Kind: DiffRemoved, Text: "b"}, {Kind: DiffAdded, Text: "B"},
		}}}},
		"no newline before a later removal": {Label: "x", Hunks: []Hunk{{OldStart: 1, OldCount: 2, NewStart: 1, NewCount: 0, Lines: []DiffLine{
			{Kind: DiffRemoved, Text: "a", NoNewline: true}, {Kind: DiffRemoved, Text: "b"},
		}}}},
		"no newline before a later hunk": {Label: "x", Hunks: []Hunk{
			{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1, Lines: []DiffLine{{Kind: DiffRemoved, Text: "a"}, {Kind: DiffAdded, Text: "A", NoNewline: true}}},
			{OldStart: 5, OldCount: 1, NewStart: 5, NewCount: 1, Lines: good},
		}},
		"budget flag with texts": {Label: "x", Old: "a", New: "b", Replaced: true},
		"oversized hunks": {Label: "x", Hunks: func() []Hunk {
			lines := make([]DiffLine, 2*MaxDiffLines+1)
			for i := range lines {
				lines[i] = DiffLine{Kind: DiffAdded, Text: "x"}
			}
			return []Hunk{{OldStart: 0, OldCount: 0, NewStart: 1, NewCount: len(lines), Lines: lines}}
		}()},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := RenderUnifiedDiff(d)
			if err == nil || got != "" {
				t.Fatalf("accepted invalid diff: err=%v output=%d bytes", err, len(got))
			}
		})
	}
}

var unifiedLiteralColor = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|rgba?\(|oklch\(|hsla?\(|\bdark:`)

func TestUnifiedDiffUsesRoleUtilitiesOnly(t *testing.T) {
	got := mustUnified(t, UnifiedDiff{Label: "service.toml", Old: unifiedOld, New: unifiedNew})
	if unifiedLiteralColor.MatchString(got) || strings.Contains(got, "style=") || strings.Contains(got, "ink-faint") {
		t.Error("unified diff must use role utilities only, without inline style or ink-faint text")
	}
}

// TestUnifiedDiffRoleContrast measures the text, rule and focus pairs the diff
// draws, in both schemes of the default palette.
func TestUnifiedDiffRoleContrast(t *testing.T) {
	source, err := os.ReadFile("../../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"light", "dark"} {
		for _, bg := range []string{"surface-sunken", "surface-raised", "positive-soft", "danger-soft", "info-soft", "caution-soft"} {
			for _, fg := range []string{"ink", "ink-muted"} {
				if bg == "caution-soft" && fg == "ink-muted" {
					continue // the budget note uses ink only
				}
				if ratio, err := tokens.ContrastRoles(tok, fg, bg, scheme); err != nil || ratio < tokens.TextFloor {
					t.Errorf("%s on %s (%s) = %.2f:1 (%v)", fg, bg, scheme, ratio, err)
				}
			}
			if ratio, err := tokens.ContrastRoles(tok, "focus", bg, scheme); err != nil || ratio < tokens.MeaningFloor {
				t.Errorf("focus on %s (%s) = %.2f:1 (%v)", bg, scheme, ratio, err)
			}
		}
		for _, mark := range []string{"positive", "danger"} {
			for _, bg := range []string{"surface-sunken", mark + "-soft"} {
				if ratio, err := tokens.ContrastRoles(tok, mark, bg, scheme); err != nil || ratio < tokens.MeaningFloor {
					t.Errorf("rule %s on %s (%s) = %.2f:1 (%v)", mark, bg, scheme, ratio, err)
				}
			}
		}
	}
}
