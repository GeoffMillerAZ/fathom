package code

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

// fixture is a synthetic review excerpt: every state, a gap, a revealed
// direction override and a long line.
func fixture() Excerpt {
	lines := SplitLines(118, "func Conserve(plan Plan) error {\n\tif plan.Empty() {\n\t\treturn nil\n\t}\n")
	lines[1].State = LineChanged
	lines[2].State = LineNotExecuted
	lines = append(lines,
		Line{Number: 130, Text: "\treturn apply(plan) // " + strings.Repeat("long-identifier-", 12), State: LineHighlighted},
		Line{Number: 131, Text: "\taccess := \"user\u202e\" // admin", State: LineChanged},
		Line{Number: 132, Text: "}"},
	)
	return Excerpt{
		Label:        "conserve.go, lines 118 to 132",
		Caption:      "internal/conserve/conserve.go",
		SourceHref:   "/files/internal/conserve/conserve.go?rev=abc123",
		AnchorPrefix: "conserve-",
		Lines:        lines,
	}
}

func mustRender(t *testing.T, e Excerpt) string {
	t.Helper()
	got, err := RenderExcerpt(e)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func assertGolden(t *testing.T, name, got string) {
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
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 go test ./appkit/ui/code): %v", err)
	}
	if string(want) != got {
		t.Errorf("output differs from %s; review and regenerate with UPDATE_GOLDEN=1\n got: %s", path, got)
	}
}

func TestRenderExcerptGolden(t *testing.T) {
	got := mustRender(t, fixture())
	assertGolden(t, "excerpt_review", got)
	if again := mustRender(t, fixture()); again != got {
		t.Fatal("RenderExcerpt is not deterministic")
	}
}

func TestExcerptLinesCarryAnchorsPermalinksAndTextLabels(t *testing.T) {
	got := mustRender(t, fixture())
	for _, want := range []string{
		`<li id="conserve-L118" `, `href="#conserve-L118"`, `aria-label="Line 118"`,
		`data-line="119" data-state="changed"`, `<span class="sr-only select-none">Changed: </span>`,
		`data-line="120" data-state="not-executed"`, `<span class="sr-only select-none">Not executed: </span>`,
		`data-line="130" data-state="highlighted"`, `<span class="sr-only select-none">Highlighted: </span>`,
		`data-line="132" data-state="normal"`,
		`Lines 122–129 not shown`,
		`aria-label="Line states"`, `>Changed</span> · 2 lines`, `>Not executed</span> · 1 line`, `>Highlighted</span> · 1 line`,
		`tabindex="0" role="region" aria-label="conserve.go, lines 118 to 132"`,
		`focus-visible:outline-focus`, `whitespace-pre`, `min-w-max`, `overflow-x-auto`,
		`target:after:border-focus`, `<figcaption`, `internal/conserve/conserve.go`,
		`href="/files/internal/conserve/conserve.go?rev=abc123"`, `>View source</bdi></a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("excerpt missing %q", want)
		}
	}
	// Normal lines carry no state label: a screen reader hears only the text.
	normal := got[strings.Index(got, `data-line="132"`):]
	normal = normal[:strings.Index(normal, "</li>")]
	if strings.Contains(normal, "sr-only") {
		t.Errorf("normal line announced a state: %s", normal)
	}
	// Without a prefix, anchors are GitHub-style #L<n>.
	plain := mustRender(t, Excerpt{Label: "x", Lines: []Line{{Number: 7, Text: "x"}}})
	if !strings.Contains(plain, `id="L7"`) || !strings.Contains(plain, `href="#L7"`) || strings.Contains(plain, "<figcaption") || strings.Contains(plain, "Line states") {
		t.Errorf("default anchor/caption/legend wrong: %s", plain)
	}
}

func TestExcerptStateLabelsAreOverridable(t *testing.T) {
	e := fixture()
	e.Labels = StateLabels{NotExecuted: "Uncovered", Changed: "  "}
	got := mustRender(t, e)
	if !strings.Contains(got, `<span class="sr-only select-none">Uncovered: </span>`) || !strings.Contains(got, `>Uncovered</span> · 1 line`) {
		t.Error("custom label not used for line and legend")
	}
	if !strings.Contains(got, `<span class="sr-only select-none">Changed: </span>`) {
		t.Error("blank override did not keep the default label")
	}
}

func TestExcerptEscapesAndRevealsHiddenCharacters(t *testing.T) {
	e := Excerpt{Label: `<b>label</b>`, Caption: `<script>alert(1)</script>`, SourceLabel: `"x"`, SourceHref: "?a=1&b=2", Lines: []Line{
		{Number: 1, Text: `<img src=x onerror=alert(1)>`},
		{Number: 2, Text: "ok\u202e\u2066\r" + string([]byte{0xfe})},
	}}
	got := mustRender(t, e)
	for _, bad := range []string{"<script>", "<img", "<b>", "\u202e", "\u2066", "\r"} {
		if strings.Contains(got, bad) {
			t.Errorf("raw %q reached markup", bad)
		}
	}
	for _, want := range []string{"&lt;img src=x onerror=alert(1)&gt;", "&lt;script&gt;", `href="?a=1&amp;b=2"`, ">U+202E</span>", ">U+2066</span>", ">U+000D</span>", `>\xfe</span>`, "data-revealed"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

const revealed = `<span class="mx-px rounded-sm border border-danger px-0.5 text-ink" data-revealed>`

// TestExcerptRevealsLineTerminatorsAndTagCharacters: U+2028 draws as a space
// but ends a JavaScript line; tag characters have no width at all.
func TestExcerptRevealsLineTerminatorsAndTagCharacters(t *testing.T) {
	got := mustRender(t, Excerpt{Label: "x", Lines: []Line{
		{Number: 1, Text: "const debug = false; // disabled:\u2028sendTokens()"},
		{Number: 2, Text: "role = \"user\U000E0061\U000E0064\""},
	}})
	for _, want := range []string{
		`// disabled:` + revealed + `U+2028</span>sendTokens()`,
		`user` + revealed + `U+E0061</span>` + revealed + `U+E0064</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, raw := range []string{"\u2028", "\U000E0061"} {
		if strings.Contains(got, raw) {
			t.Errorf("raw %q reached markup", raw)
		}
	}
}

// TestExcerptRevealsCaptionLabelAndLinkText keeps what is under review honest:
// a right-to-left override in a path ("admin\u202eog.txt" displays as
// "admintxt.go") is shown as a code point, in an isolated run.
func TestExcerptRevealsCaptionLabelAndLinkText(t *testing.T) {
	got := mustRender(t, Excerpt{
		Label: "admin\u202eog.txt, line 1", Caption: "internal/auth/admin\u202eog.txt",
		SourceHref: "/files/x", SourceLabel: "Open \u202eecruos",
		Lines: []Line{{Number: 1, Text: "x"}},
	})
	if strings.Contains(got, "\u202e") {
		t.Fatalf("raw U+202E reached markup: %s", got)
	}
	for _, want := range []string{
		`<bdi>internal/auth/admin` + revealed + `U+202E</span>og.txt</bdi>`,
		`<bdi>Open ` + revealed + `U+202E</span>ecruos</bdi></a>`,
		`aria-label="adminU+202Eog.txt, line 1"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// TestExcerptLineRowsMeetMinimumTargetSize: every line number is a permalink
// stretched over its row, so the row pitch is the target height. leading-6
// gives 24px rows (WCAG 2.2 2.5.8); 20px rows stacked 20px apart fail it.
func TestExcerptLineRowsMeetMinimumTargetSize(t *testing.T) {
	got := mustRender(t, fixture())
	if !strings.Contains(got, `<ol class="min-w-max py-2 font-mono text-xs leading-6" role="list">`) || strings.Contains(got, "leading-5") {
		t.Errorf("excerpt rows are not 24px: %s", got[:min(len(got), 800)])
	}
	// The link must stretch with the row: no own height, line-height or
	// cross-axis alignment that would shrink it back to the text box.
	link := got[strings.Index(got, `<a href="#conserve-L118"`):]
	link = link[:strings.Index(link, ">")]
	for _, shrink := range []string{"leading-", " h-", "items-", "self-", "inline"} {
		if strings.Contains(link, shrink) {
			t.Errorf("permalink carries %q: %s", shrink, link)
		}
	}
}

func TestExcerptRefusesInvalidInputAtomically(t *testing.T) {
	good := fixture()
	cases := map[string]func(*Excerpt){
		"blank label":        func(e *Excerpt) { e.Label = " " },
		"zero line number":   func(e *Excerpt) { e.Lines[0].Number = 0 },
		"descending numbers": func(e *Excerpt) { e.Lines[1].Number = 118 },
		"newline in text":    func(e *Excerpt) { e.Lines[0].Text = "a\nb" },
		"unknown state":      func(e *Excerpt) { e.Lines[0].State = "covered" },
		"unsafe source link": func(e *Excerpt) { e.SourceHref = "javascript:alert(1)" },
		"protocol-relative":  func(e *Excerpt) { e.SourceHref = "//evil.example/x" },
		"bad anchor prefix":  func(e *Excerpt) { e.AnchorPrefix = "1 bad" },
		"too many lines":     func(e *Excerpt) { e.Lines = SplitLines(1, strings.Repeat("x\n", MaxLines+1)) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := good
			e.Lines = append([]Line(nil), good.Lines...)
			mutate(&e)
			got, err := RenderExcerpt(e)
			if err == nil || got != "" {
				t.Fatalf("accepted invalid excerpt: err=%v output=%d bytes", err, len(got))
			}
		})
	}
	if _, err := RenderExcerpt(Excerpt{Label: "x", Lines: SplitLines(1, strings.Repeat("x\n", MaxLines))}); err != nil {
		t.Fatalf("MaxLines excerpt refused: %v", err)
	}
}

func TestExcerptEmptyStateIsExplicit(t *testing.T) {
	got := mustRender(t, Excerpt{Label: "empty", Caption: "a.go"})
	if !strings.Contains(got, "No lines to show in this excerpt.") || strings.Contains(got, `role="region"`) {
		t.Fatalf("empty excerpt: %s", got)
	}
}

func TestSplitLines(t *testing.T) {
	lines := SplitLines(40, "a\n\nb\n")
	if len(lines) != 3 || lines[0] != (Line{Number: 40, Text: "a"}) || lines[1] != (Line{Number: 41}) || lines[2] != (Line{Number: 42, Text: "b"}) {
		t.Fatalf("SplitLines = %#v", lines)
	}
	if got := SplitLines(1, ""); len(got) != 0 {
		t.Fatalf("empty text produced lines: %#v", got)
	}
}

var literalColor = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|rgba?\(|oklch\(|hsla?\(|\bdark:`)

func TestExcerptUsesRoleUtilitiesOnly(t *testing.T) {
	got := mustRender(t, fixture())
	if literalColor.MatchString(strings.ReplaceAll(got, `href="#conserve-`, "")) {
		t.Errorf("literal color or dark: variant in excerpt markup")
	}
	if strings.Contains(got, "style=") {
		t.Error("inline style emitted; a strict style-src CSP would drop it")
	}
	if strings.Contains(got, "ink-faint") {
		t.Error("meaningful text uses ink-faint (hazard H-004)")
	}
}

// TestExcerptRoleContrast measures the role pairs the excerpt draws text on,
// in both schemes of the default palette: code text and line numbers on every
// row background, and the source link on the caption surface.
func TestExcerptRoleContrast(t *testing.T) {
	source, err := os.ReadFile("../../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	rows := []string{"surface-sunken", "info-soft", "danger-soft", "accent-soft"}
	for _, scheme := range []string{"light", "dark"} {
		for _, bg := range rows {
			for _, fg := range []string{"ink", "ink-muted"} {
				ratio, err := tokens.ContrastRoles(tok, fg, bg, scheme)
				if err != nil || ratio < tokens.TextFloor {
					t.Errorf("%s on %s (%s) = %.2f:1, need %.1f:1 (%v)", fg, bg, scheme, ratio, tokens.TextFloor, err)
				}
			}
			// The target/focus outline is drawn inside every row treatment.
			if ratio, err := tokens.ContrastRoles(tok, "focus", bg, scheme); err != nil || ratio < tokens.MeaningFloor {
				t.Errorf("focus outline on %s (%s) = %.2f:1 (%v)", bg, scheme, ratio, err)
			}
		}
		for _, pair := range [][2]string{{"accent", "surface-raised"}, {"ink-muted", "surface-raised"}, {"ink", "surface-raised"}} {
			ratio, err := tokens.ContrastRoles(tok, pair[0], pair[1], scheme)
			if err != nil || ratio < tokens.TextFloor {
				t.Errorf("%s on %s (%s) = %.2f:1 (%v)", pair[0], pair[1], scheme, ratio, err)
			}
		}
		// State rules and the revealed-character border.
		for _, mark := range []string{"info", "danger", "accent"} {
			for _, bg := range []string{"surface-sunken", "surface-raised", mark + "-soft"} {
				ratio, err := tokens.ContrastRoles(tok, mark, bg, scheme)
				if err != nil || ratio < tokens.MeaningFloor {
					t.Errorf("rule %s on %s (%s) = %.2f:1 (%v)", mark, bg, scheme, ratio, err)
				}
			}
		}
	}
}
