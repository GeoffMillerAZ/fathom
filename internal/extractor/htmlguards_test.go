package extractor

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// htmlGuardSources are pages that exercise every guarded pattern of the HTML
// dialect: a custom element (open and self-closing), a form with an action, a
// field of each tag (open and self-closing) with and without a name, and the
// near misses that satisfy one guard but not the pattern.
var htmlGuardSources = []string{
	`<my-widget id="w1"></my-widget><x-y/><a href="u">a-b</a>`,
	`<form action="/go" method="post"><input name="q"><select name="s"></select><textarea name="t"></textarea><button name="b"></button></form>`,
	`<form method="get"></form><p>the form of an action</p>`,
	`<input type="text"/><input name="n"/><br/>`,
	`<p>a-b <b>c</b> name</p>`,
	`<custom-element><input name=bare action=x></custom-element><FORM action=y></FORM>`,
	``,
	`plain text, no markup`,
}

// TestHTMLGuardsAreNecessary (LB-F180): the HTML dialect's #requires-text!
// guards drop no match. The tags and imports query runs over the testdata,
// every HTML file in the repository and the synthetic pages above with its
// guards and without them, and the matches are equal.
func TestHTMLGuardsAreNecessary(t *testing.T) {
	reg, err := tsRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec := reg["html"]
	if spec == nil {
		t.Fatal("no html dialect")
	}
	v := spec.defaultVariant
	if !strings.Contains(v.rules.query, "#requires-text!") {
		t.Fatal("the html query carries no guard")
	}
	p, err := tsengine.NewParser(context.Background(), v.blob, v.entryPoint)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q, err := p.CompileQuery(v.rules.query)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	var srcs [][]byte
	for _, s := range htmlGuardSources {
		srcs = append(srcs, []byte(s))
	}
	root := filepath.Join("..", "..")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "workspace" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			if b, err := os.ReadFile(path); err == nil && len(b) < 1<<20 {
				srcs = append(srcs, b)
			}
		}
		return nil
	})
	pruned := 0
	for i, src := range srcs {
		tree, err := p.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := q.Run(tree)
		if err != nil {
			t.Fatal(err)
		}
		if q.LastRunPatterns() < q.PatternCount() {
			pruned++
		}
		want, err := q.RunUnguarded(tree)
		if err != nil {
			t.Fatal(err)
		}
		tree.Close()
		if len(got) != len(want) || (len(got) > 0 && !reflect.DeepEqual(got, want)) {
			for k := range got {
				if k < len(want) && !reflect.DeepEqual(got[k], want[k]) {
					t.Errorf("source %d match %d: guarded %+v\n unguarded %+v", i, k, got[k], want[k])
					break
				}
			}
			t.Errorf("source %d (%.60q): guarded matches differ from unguarded\n guarded   %d matches\n unguarded %d matches", i, src, len(got), len(want))
		}
	}
	if pruned == 0 {
		t.Error("no source pruned a pattern: the guards would prove nothing")
	}
}
