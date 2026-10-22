package semantic

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRawRuleText_ImperativeExtraction(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"heading only", "## Build discipline\n", ""},
		{"modal sentences", "## Rules\nThe caller must check input. The result may be cached. It must never skip validation! Always record failures?", "The caller must check input.\nIt must never skip validation!"},
		{"whole words", "## Rules\nThe mustard alwaysness is requiredly true. This must pass.", "This must pass."},
		{"only forms", "## Rules\nOnly read valid files. Files only differ by name. You may only write local files. You only may write local files.", "Only read valid files.\nYou may only write local files."},
		{"only verbs beyond initial allowlist", "## Rules\nOnly serve current rules. Only the first rule is current. Only compile changed sections.", "Only serve current rules.\nOnly compile changed sections."},
		{"base verbs", "## Rules\nRead the file. Create a file. Wrap the result. Run the gate.", "Read the file.\nWrap the result."},
		{"list continuation", "## Rules\n- The caller must\n  preserve the order of values.\n- Return the result.", "The caller must preserve the order of values.\nReturn the result."},
		{"noise skipped", "## Rules\n```go\n// must bypass the check\n```\n<!-- always ignore this -->\n| Rule | must bypass |\n|---|---|\n### must ignore heading\nThe process must remain stable.", "The process must remain stable."},
		{"inline syntax", "## Rules\nKeep `CGO_ENABLED=0` and `_test.go` intact. See [the guide](docs/guide.md); it must remain linked. **Always preserve emphasis.**", "Keep `CGO_ENABLED=0` and `_test.go` intact.\nSee the guide; it must remain linked."},
		{"comment marker in code span", "## Rules\nKeep `<!-- marker -->` intact.", "Keep `<!-- marker -->` intact."},
		{"code span in link text", "## Rules\nRead [`CGO_ENABLED`](guide.md) before changing it.", "Read `CGO_ENABLED` before changing it."},
		{"abbreviations and numbers", "## Rules\nFor e.g. 1.2 inputs, do not split here. It must handle i.e. examples and vs. labels.", "For e.g. 1.2 inputs, do not split here.\nIt must handle i.e. examples and vs. labels."},
		{"apostrophes", "## Rules\nYou don't drop errors. You don’t erase state. The key is required to exist.", "You don't drop errors.\nYou don’t erase state."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rawRuleText(Section{Text: tc.body}); got != tc.want {
				t.Errorf("rawRuleText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRawRuleText_ADRDecisionBlock(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"numbered", "## ADR-080: Advice\n**Status.** Accepted.\n**Decision.**\n1. *First.* Measure output before serving it. More explanation follows.\n2. *Second.* Keep the cap fixed. Additional detail.\n3. *Third.* Never expand it.\n**Consequences.** Ignore this.", "First.\nSecond."},
		{"prose", "## ADR-081: Evidence\n**Decision.** The receipt records the tree. Its digest is checked. **Why.** Never trust the context.", "The receipt records the tree.\nIts digest is checked."},
		{"not ADR", "## General rules\n**Decision.** The receipt records the tree.\nAlways check it.", "Always check it."},
		{"decision boundary", "## ADR-082: Evidence\n**Decision.**\n1. Read the input.\n**Alternatives rejected.** Always do the other thing.", "Read the input."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rawRuleText(Section{Text: tc.body}); got != tc.want {
				t.Errorf("rawRuleText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRawRuleText_CapsEachSentenceAtWordBoundary(t *testing.T) {
	long := "Always " + strings.Repeat("preserve evidence ", 20) + "without loss."
	got := rawRuleText(Section{Text: "## Rules\n" + long + "\nNever drop failures."})
	parts := strings.Split(got, "\n")
	if len(parts) != 2 || parts[1] != "Never drop failures." {
		t.Fatalf("sentences = %q", got)
	}
	if utf8.RuneCountInString(parts[0]) > 240 || !strings.HasSuffix(parts[0], "…") || strings.HasSuffix(parts[0], "eviden…") {
		t.Errorf("cap broke word/rune limit: %q", parts[0])
	}
}

func TestRawRuleText_InitialVerbAllowlist(t *testing.T) {
	for _, verb := range strings.Fields("Use Put Keep Run Add Write Read Prefer Avoid Check Extend Sort Inject Pass Return Record Treat Make Call Wrap Start Stop Ask Name Mark Route") {
		t.Run(verb, func(t *testing.T) {
			want := verb + " the value."
			if got := rawRuleText(Section{Text: "## Rules\n" + want}); got != want {
				t.Errorf("%q: got %q", want, got)
			}
		})
	}
}

func TestRawRuleText_MarkdownCleanup(t *testing.T) {
	got := rawRuleText(Section{Text: "## Rules\n**Always** keep `CGO_ENABLED=0` and `_test.go` untouched. Read [the guide](docs/guide.md) before changing them. [Do not](https://example.invalid) follow that link."})
	want := "Always keep `CGO_ENABLED=0` and `_test.go` untouched.\nRead the guide before changing them."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRawRuleText_RepositoryADRs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "ADR.md"))
	if err != nil {
		t.Fatal(err)
	}
	sections := SplitMarkdownSections("ADR.md", data)
	layer := rawFallbackTestLayer(t)
	corpus := []core.SourceFile{{Path: "ADR.md", Content: data}}
	wants := map[string]string{
		"adr-004": "",
		"adr-005": "Pipeline runs: Pass 1 Filter (sequential, ignore vendor/testdata/generated), Pass 2 Fetch (N parallel workers into pre-allocated slots, first error via atomic CAS, no mid-flight cancel), Pass 3 post-fetch (sequential, in input order:…\nThen bucket pending by language and extract.",
		"adr-046": "Every judgment is a typed decision-port question.\nThe only egress is the decision port (`[decide] provider`, the new `assess` site).",
		"adr-047": "A declared budget, measured on the OS quantity.\nA ladder the daemon enforces on itself, sampled every 2 s from `phys_footprint`, `runtime/metrics`, host memory pressure, swap and volume free space: shed at 0.75 × budget or host pressure warn (drop caches, pause background work, serve…",
		"adr-048": "Four support levels with thresholds and an oracle rule.\nA level is a reading, not a registration.",
		"adr-049": "The kitchen-sink gate.\nPillars and a lane.",
		"adr-056": "The decision-model bet (principle P24).\nProgram rules R1 to R8.",
		"adr-058": "The kind.\nThe core facet is the suspect list, computed and never stored.",
		"adr-061": "Four zones.\nOne schema entry point, extended by key.",
		"adr-077": "", // Proposed at this source revision, despite its later accepted index entry.
	}
	for id, prefix := range wants {
		var found bool
		for _, sec := range sections {
			if !strings.HasPrefix(sec.Subject, "ADR.md#"+id+"-") {
				continue
			}
			found = true
			rows, err := RawFallbackRules(nil, layer, corpus, "pkg/a.go", 1, "coverage gap", nil, map[string]bool{sec.Subject: true})
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if len(rows) > 0 {
				got = rows[0].Rule
			}
			if got != prefix {
				t.Errorf("%s = %q, want %q", id, got, prefix)
			}
			break
		}
		if !found {
			t.Errorf("missing %s", id)
		}
	}
}

func TestRawRuleText_RootNonNegotiablesTableIsSilent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range SplitMarkdownSections("AGENTS.md", data) {
		if strings.HasSuffix(sec.Subject, "#non-negotiables-one-line-each-the-pointer-carries-the-full-rules") {
			if got := rawRuleText(sec); got != "" {
				t.Errorf("table served as raw instructions: %q", got)
			}
			return
		}
	}
	t.Fatal("missing Non-negotiables section")
}

func TestRawRuleText_RealInternalAgentInstructions(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range SplitMarkdownSections("internal/AGENTS.md", data) {
		if !strings.HasSuffix(sec.Subject, "#determinism-is-a-contract") {
			continue
		}
		want := "Sort every output slice; map iteration order must never reach a result.\nInject clocks (`now func() time.Time`) — never call `time.Now()` in query/analysis logic."
		if got := rawRuleText(sec); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		return
	}
	t.Fatal("missing determinism section")
}

func TestRawRuleText_InlineCodeAndComments(t *testing.T) {
	text := "## Rules\n<!-- Never serve this.\nAlways serve this too. -->\nKeep `pkg/**` and `foo. bar` verbatim. Do not split at 2. items or at 1.5 numbers."
	want := "Keep `pkg/**` and `foo. bar` verbatim.\nDo not split at 2. items or at 1.5 numbers."
	if got := rawRuleText(Section{Text: text}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRawRuleText_PairedEmphasisDoesNotEatGlobsOrIdentifiers(t *testing.T) {
	text := "## Rules\nKeep pkg/** and **bold** words with __strong__ emphasis. Read _word_ before punctuation, and keep CGO_ENABLED and _test.go intact."
	want := "Keep pkg/** and bold words with strong emphasis.\nRead word before punctuation, and keep CGO_ENABLED and _test.go intact."
	if got := rawRuleText(Section{Text: text}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRawRuleText_CodeSpanSpacingIsVerbatim(t *testing.T) {
	text := "## Rules\nKeep ``a  b *c*`` and `x  y` verbatim."
	want := "Keep ``a  b *c*`` and `x  y` verbatim."
	if got := rawRuleText(Section{Text: text}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRawRuleText_MixedPlanChecks(t *testing.T) {
	text := "## Tasks\n- [x] Always delete the cache.\n\n  Never retain a copy.\n- [ ] Keep the result.\n- [X] Run old gate.\n- [ ] Check the new gate."
	want := "Keep the result.\nCheck the new gate."
	if got := rawRuleText(Section{Path: "docs/plans/example.md", Text: text}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRawRuleText_ADRColonAndFreeformBoldBoundary(t *testing.T) {
	text := "## ADR-080: Rules\n**Decision**: Keep this.\n**Alternatives / rejected.** Never serve this."
	if got := rawRuleText(Section{Text: text}); got != "Keep this." {
		t.Errorf("got %q", got)
	}
}

func TestRawRuleText_ADRChildHeadingIsSilent(t *testing.T) {
	sec := Section{Path: "ADR.md", Text: "### Consequences\nNever serve this outside its parent decision."}
	if got := rawRuleText(sec); got != "" {
		t.Errorf("child heading served %q", got)
	}
}

func TestRawRuleText_ImperativeWords(t *testing.T) {
	for _, tt := range []struct {
		sentence string
		served   bool
	}{
		{"The only exception is described below.", false},
		{"This is not only fast but predictable.", false},
		{"No migration is required.", false},
		{"Don’t discard evidence.", true},
		{"The caller shall preserve ordering.", true},
		{"The caller is required to check errors.", true},
		{"The mustard is ready.", false},
		{"Nevermore is a name.", false},
		{"The caller MUST preserve ordering.", true},
		{"Only serve current rules.", true},
		{"The caller may only read local files.", true},
	} {
		t.Run(tt.sentence, func(t *testing.T) {
			want := ""
			if tt.served {
				want = tt.sentence
			}
			if got := rawRuleText(Section{Text: "# Rules\n" + tt.sentence}); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
	want := "Use Put Keep Run Add Write Read Prefer Avoid Check Extend Sort Inject Pass Return Record Treat Make Call Wrap Start Stop Ask Name Mark Route"
	if rawInitialVerbs != want {
		t.Errorf("base-form verb constant = %q, want %q", rawInitialVerbs, want)
	}
}

func TestRawRuleText_MarkdownCleanupTable(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{"Keep CGO_ENABLED intact.", "Keep CGO_ENABLED intact."},
		{"Read _test.go first.", "Read _test.go first."},
		{"Keep pkg/** intact.", "Keep pkg/** intact."},
		{"Keep **bold** text.", "Keep bold text."},
		{"Keep pre**bold**suffix intact.", "Keep preboldsuffix intact."},
		{"Use `a | b` intact.", "Use `a | b` intact."},
		{"Rule|Description\n---|---\nAlways bypass validation.|Never serve this.", ""},
		{"Keep *emphasis* text.", "Keep emphasis text."},
		{"Keep __bold__ text.", "Keep bold text."},
		{"Keep _emphasis_ text.", "Keep emphasis text."},
		{"| Always skip this | Never serve tables |", ""},
		{"Keep `**literal** _test.go` intact.", "Keep `**literal** _test.go` intact."},
	} {
		t.Run(tt.body, func(t *testing.T) {
			if got := rawRuleText(Section{Text: "# Rules\n" + tt.body}); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}
