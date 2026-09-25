package designconvert

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type coverageDOMCase struct {
	ID, Original, Converted string
}

// This is a private compiler/DOM diagnostic, not publication or browser proof.
// Only the generated declaration is renamed; originals and literal bodies stay
// unchanged. Errors disclose stages, never source bodies or compiler output.
func checkCoverageDOMBatch(root string, cases []coverageDOMCase, scratch string) (err error) {
	stageError := func(stage string) error { return fmt.Errorf("coverage DOM batch: %s", stage) }
	if !filepath.IsAbs(root) || len(cases) == 0 || len(cases) > 32 {
		return stageError("input-bounds")
	}
	const header = "package converted\n\ntempl Specimen() {\n"
	for _, c := range cases {
		if len(c.ID) > 1024 || len(c.Original) > MaxSourceBytes || len(c.Converted) > maxOutputBytes {
			return stageError("input-bounds")
		}
		if !strings.HasPrefix(c.Converted, header) {
			return stageError("input-header")
		}
		if validateTempl(c.Converted) != nil {
			return stageError("input-literal-grammar")
		}
	}
	env, err := coverageDOMEnv(scratch)
	if err != nil {
		return stageError("input-scratch")
	}
	dir, err := os.MkdirTemp(scratch, "batch-")
	if err != nil {
		return stageError("temp-directory")
	}
	defer func() {
		if cleanupErr := os.RemoveAll(dir); cleanupErr != nil && err == nil {
			err = stageError("cleanup")
		}
	}()
	var entries strings.Builder
	goFiles := make([]string, 0, len(cases)+1)
	for i, c := range cases {
		name := fmt.Sprintf("Specimen%03d", i)
		stem := fmt.Sprintf("case%03d", i)
		converted := "package converted\n\ntempl " + name + "() {\n" + c.Converted[len(header):]
		if os.WriteFile(filepath.Join(dir, stem+".templ"), []byte(converted), 0600) != nil {
			return stageError("write-converted")
		}
		fmt.Fprintf(&entries, "{%s, %s, %s},\n", strconv.Quote(c.ID), strconv.Quote(c.Original), name)
		goFiles = append(goFiles, filepath.Join(dir, stem+"_templ.go"))
	}
	fixture := strings.Replace(coverageDOMFixture, "CASE_ENTRIES", entries.String(), 1)
	testFile := filepath.Join(dir, "coverage_test.go")
	if os.WriteFile(testFile, []byte(fixture), 0600) != nil {
		return stageError("write-originals")
	}
	goFiles = append(goFiles, testFile)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(stage string, args ...string) error {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir, cmd.Env = root, env
		// Unset stdout/stderr are discarded, including diagnostics quoting source.
		if cmd.Run() != nil {
			return stageError(stage)
		}
		return nil
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		if phase == "formatted" {
			if err = run("format", "tool", "templ", "fmt", "-w", "2", dir); err != nil {
				return err
			}
		}
		if err = run("generate-"+phase, "tool", "templ", "generate", "-path", dir, "-w", "2", "-include-version=false"); err != nil {
			return err
		}
		for _, check := range []struct{ stage, pattern string }{{"compile", "^$"}, {"render", "^TestCoverageRenderedDOM$"}} {
			args := append([]string{"test", "-count=1", "-run", check.pattern}, goFiles...)
			if err = run(check.stage+"-"+phase, args...); err != nil {
				return err
			}
		}
	}
	return nil
}

// The caller owns cleanup of scratch, including the cache shared across batches.
func coverageDOMEnv(scratch string) ([]string, error) {
	privateDir := func(path string) bool {
		info, err := os.Lstat(path)
		return err == nil && info.IsDir() && info.Mode().Perm()&0077 == 0
	}
	info, err := os.Lstat(scratch)
	if !filepath.IsAbs(scratch) || err != nil || !info.IsDir() {
		return nil, fmt.Errorf("coverage DOM batch: input-scratch")
	}
	// testing.T.TempDir may create a 0755 child beneath its private 0700 root.
	protected := false
	for dir := scratch; ; dir = filepath.Dir(dir) {
		if privateDir(dir) {
			protected = true
			break
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	if !protected {
		return nil, fmt.Errorf("coverage DOM batch: input-scratch")
	}
	overrides := map[string]string{"GOENV": "off", "GOCACHEPROG": ""}
	for _, key := range []string{"GOCACHE", "GOTMPDIR", "TMPDIR"} {
		dir := filepath.Join(scratch, strings.ToLower(key))
		if os.MkdirAll(dir, 0700) != nil || !privateDir(dir) {
			return nil, fmt.Errorf("coverage DOM batch: input-scratch")
		}
		overrides[key] = dir
	}
	env := []string{}
	for _, item := range offlineGoEnv() {
		key, _, _ := strings.Cut(item, "=")
		if _, replaced := overrides[key]; !replaced {
			env = append(env, item)
		}
	}
	for _, key := range []string{"GOCACHE", "GOTMPDIR", "TMPDIR", "GOCACHEPROG", "GOENV"} {
		env = append(env, key+"="+overrides[key])
	}
	return env, nil
}

func TestCoverageDOMBatchSynthetic(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	const original = `<p class="text-slate-900 dark:text-white">original</p>`
	translate := func(source string) string {
		t.Helper()
		r, err := HTMLToTempl([]byte(source))
		if err != nil || r.Templ == "" {
			t.Fatalf("synthetic translation failed: %v", err)
		}
		return r.Templ
	}
	const second = `<svg viewBox="0 0 10 10"><path d="M0 0h10v10z"/></svg><!-- keep -->`
	cases := []coverageDOMCase{
		{ID: "text-and-dark-class", Original: original, Converted: translate(original)},
		{ID: "svg-and-comment", Original: second, Converted: translate(second)},
	}
	scratch := t.TempDir()
	if err := checkCoverageDOMBatch(root, cases, scratch); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ name, old, replacement string }{
		{"text", "original", "changed"},
		{"dark-class", "dark:text-white", "dark:text-black"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			planted := append([]coverageDOMCase{}, cases...)
			planted[0].Converted = translate(strings.Replace(original, mutation.old, mutation.replacement, 1))
			if err := checkCoverageDOMBatch(root, planted, scratch); err == nil || err.Error() != "coverage DOM batch: render-unformatted" {
				t.Fatalf("planted DOM mismatch was not detected at render: %v", err)
			}
		})
	}
}

func TestCoverageDOMBatchRejectsInvalidInputs(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, cases := range [][]coverageDOMCase{
		nil,
		make([]coverageDOMCase, 33),
		{{ID: "wrong-header", Original: "<p>x</p>", Converted: "package converted\n\ntempl Other() {\n<p>x</p>\n}"}},
		{{ID: "executable", Original: "<p>x</p>", Converted: "package converted\n\ntempl Specimen() {\n<p>{ dangerous() }</p>\n}"}},
	} {
		if err := checkCoverageDOMBatch(root, cases, t.TempDir()); err == nil || !strings.HasPrefix(err.Error(), "coverage DOM batch: input-") {
			t.Fatalf("invalid batch was not refused before compilation: %v", err)
		}
	}
}

func TestCoverageDOMEnvIsolation(t *testing.T) {
	scratch := t.TempDir()
	want := map[string]string{"GOCACHEPROG": "", "GOENV": "off"}
	for _, key := range []string{"GOCACHE", "GOTMPDIR", "TMPDIR"} {
		want[key] = filepath.Join(scratch, strings.ToLower(key))
	}
	for key := range want {
		t.Setenv(key, "ambient-must-not-be-used")
	}
	env, err := coverageDOMEnv(scratch)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range want {
		count := 0
		for _, item := range env {
			if k, v, _ := strings.Cut(item, "="); k == key {
				count++
				if v != value {
					t.Fatalf("%s did not use the isolated setting", key)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s has %d entries, want one", key, count)
		}
	}
	if _, err := coverageDOMEnv("relative"); err == nil {
		t.Fatal("relative scratch accepted")
	}
}

const coverageDOMFixture = `package converted
import (
 "bytes"
 "context"
 "reflect"
 "strings"
 "testing"
 "github.com/a-h/templ"
 "golang.org/x/net/html"
 "golang.org/x/net/html/atom"
)
type nodeShape struct { Type html.NodeType; Data, Namespace string; Attr []html.Attribute; Children []nodeShape }
func shape(n *html.Node) nodeShape {
 s := nodeShape{Type:n.Type, Data:n.Data, Namespace:n.Namespace, Attr:n.Attr}
 for c := n.FirstChild; c != nil; c = c.NextSibling { s.Children = append(s.Children, shape(c)) }
 return s
}
func fragment(t *testing.T, s string) []nodeShape {
 t.Helper()
 nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type:html.ElementNode, Data:"div", DataAtom:atom.Div})
 if err != nil { t.Fatal("parse failed") }
 result := []nodeShape{}
 for _, n := range nodes { result = append(result, shape(n)) }
 return result
}
func TestCoverageRenderedDOM(t *testing.T) {
 cases := []struct { id, original string; component func() templ.Component }{
CASE_ENTRIES
 }
 for _, c := range cases {
  t.Run(c.id, func(t *testing.T) {
   var out bytes.Buffer
   if err := c.component().Render(context.Background(), &out); err != nil { t.Fatal("render failed") }
   if !reflect.DeepEqual(fragment(t, c.original), fragment(t, out.String())) { t.Fatal("namespace-aware DOM/text/attribute/comment mismatch") }
  })
 }
}
`
