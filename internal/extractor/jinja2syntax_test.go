package extractor

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// These authored controls are independently graded by Jinja's own parser.
// Regenerate/check them with calibration/oracles/jinja2-meta/syntax_controls.py.
func TestJinja2SyntaxControls(t *testing.T) {
	raw, err := os.ReadFile("dialects/jinja2/testdata/syntax-controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controls struct {
		Oracle string `json:"oracle"`
		Cases  []struct {
			Name, Source    string
			JinjaValid      bool   `json:"jinja_valid"`
			GrammarValid    *bool  `json:"grammar_valid"`
			KnownDivergence string `json:"known_divergence"`
			LoopFilter      string `json:"loop_filter"`
			LoopIterable    string `json:"loop_iterable"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &controls); err != nil {
		t.Fatal(err)
	}
	if controls.Oracle != "Jinja2 3.1.6" || len(controls.Cases) < 22 {
		t.Fatal("missing independent Jinja syntax controls")
	}
	p, err := tsengine.NewParser(context.Background(), "jinja2", "tree_sitter_jinja")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q, err := p.CompileQuery("[(ERROR) (MISSING)] @error")
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	loop, err := p.CompileQuery("(for_statement iterable: (_) @iterable)\n(for_condition) @filter")
	if err != nil {
		t.Fatal(err)
	}
	defer loop.Close()
	for _, c := range controls.Cases {
		t.Run(c.Name, func(t *testing.T) {
			want := c.JinjaValid
			if c.GrammarValid != nil {
				if c.KnownDivergence == "" || *c.GrammarValid == c.JinjaValid {
					t.Fatal("grammar override requires an explicit oracle divergence")
				}
				want = *c.GrammarValid
				t.Log(c.KnownDivergence)
			} else if c.KnownDivergence != "" {
				t.Fatal("known divergence requires a grammar verdict")
			}
			tree, err := p.Parse([]byte(c.Source))
			if err != nil {
				t.Fatal(err)
			}
			defer tree.Close()
			matches, err := q.Run(tree)
			if err != nil {
				t.Fatal(err)
			}
			if valid := len(matches) == 0; valid != want {
				t.Errorf("grammar_valid=%v, want %v (Jinja=%v); review any known divergence", valid, want, c.JinjaValid)
			}
			if c.LoopIterable != "" {
				matches, err := loop.Run(tree)
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]string{}
				for _, match := range matches {
					for _, capture := range match.Captures {
						got[capture.Name] = capture.Node.Text([]byte(c.Source))
					}
				}
				if got["iterable"] != c.LoopIterable || got["filter"] != c.LoopFilter {
					t.Errorf("loop AST=%v, want iterable=%q filter=%q", got, c.LoopIterable, c.LoopFilter)
				}
			}
		})
	}
}
