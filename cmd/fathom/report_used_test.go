package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/usedset"
)

// SV-7 (ADR-060 decision 6): `fathom report used <dir> [--json]` prints a
// dependency directory's used closure with each unit's reason (entry point,
// first importer) and the unreached remainder, as a sub-report of `report`.
func TestCmdReportUsedJSONReasonsAndUnreached(t *testing.T) {
	root := t.TempDir()
	w := func(rel, body string) { linesWrite(t, root, rel, body) }
	w("go.mod", "module example.com/app\n\ngo 1.22\n\nrequire example.com/kit v0.0.0\n\nreplace example.com/kit => ./third_party/kit\n")
	w("cmd/app/main.go", "package main\n\nimport _ \"example.com/kit/ui\"\n\nfunc main() {}\n")
	w("web/input.css", "@source \"../third_party/kit/ui/*.go\";\n")
	w("third_party/kit/go.mod", "module example.com/kit\n\ngo 1.22\n")
	w("third_party/kit/ui/ui.go", "package ui\n\nimport (\n\t\"embed\"\n\n\t_ \"example.com/kit/core\"\n)\n\n//go:embed assets/app.js\nvar A embed.FS\n")
	w("third_party/kit/ui/assets/app.js", "import './dep.js';\n")
	w("third_party/kit/ui/assets/dep.js", "export {};\n")
	w("third_party/kit/core/core.go", "package core\n")
	w("third_party/kit/extra/extra.go", "package extra\n")
	w("third_party/kit/docs/guide.md", "# guide\n")

	var out, errb bytes.Buffer
	if code := run([]string{"report", "used", "third_party/kit", "--root", root, "--json"}, &out, &errb); code != 0 {
		t.Fatalf("report used --json exit %d: %s", code, errb.String())
	}
	var res usedset.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	reason := map[string]usedset.Unit{}
	for _, u := range res.Used {
		reason[u.Kind+" "+u.ID] = u
	}
	if u := reason["go-package example.com/kit/core"]; u.Entry != "example.com/app/cmd/app" || u.FirstImporter != "example.com/kit/ui" {
		t.Errorf("core's reason = %+v", u)
	}
	if u := reason["embed third_party/kit/ui/assets/app.js"]; u.FirstImporter != "example.com/kit/ui" {
		t.Errorf("app.js = %+v", u)
	}
	if u := reason["js-module third_party/kit/ui/assets/dep.js"]; u.FirstImporter != "third_party/kit/ui/assets/app.js" {
		t.Errorf("dep.js = %+v", u)
	}
	if u := reason["css-source third_party/kit/ui/*.go"]; u.Entry != "web/input.css" {
		t.Errorf("css source = %+v (units %v)", u, res.Used)
	}
	var unreached []string
	for _, u := range res.Unreached {
		unreached = append(unreached, u.Kind+" "+u.ID)
	}
	if strings.Join(unreached, ",") != "go-package example.com/kit/extra" {
		t.Errorf("unreached = %v", unreached)
	}
	if res.Totals.GoPackages != 3 || res.Totals.UsedGoPackages != 2 {
		t.Errorf("totals = %+v", res.Totals)
	}
	for _, f := range res.UsedFiles {
		if strings.Contains(f, "extra/") || strings.Contains(f, "docs/") {
			t.Errorf("used files include %s", f)
		}
	}

	// The text report names the reasons and the remainder; an absolute
	// directory works too.
	var text bytes.Buffer
	if code := run([]string{"report", "used", "--root", root, filepath.Join(root, "third_party", "kit")}, &text, &errb); code != 0 {
		t.Fatalf("report used exit %d: %s", code, errb.String())
	}
	s := text.String()
	for _, want := range []string{"example.com/kit/core", "via example.com/kit/ui", "unreached", "example.com/kit/extra", "2 of 3 Go packages"} {
		if !strings.Contains(s, want) {
			t.Errorf("text lacks %q:\n%s", want, s)
		}
	}

	// A directory the consumer does not replace is refused with the reason.
	var bad bytes.Buffer
	if code := run([]string{"report", "used", "web", "--root", root}, &bad, &bad); code == 0 {
		t.Errorf("report used on a non-module directory succeeded:\n%s", bad.String())
	}
}
