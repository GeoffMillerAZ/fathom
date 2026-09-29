package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

const linguistCmdRev = "5fbdfcb8133be2bed88bf3ce62b2335f50474525"

// linguistCmdRoot lays out a throwaway repository root (a queue and a
// mapping-only data file) and a linguist checkout built from langbench's
// fixture yml.
func linguistCmdRoot(t *testing.T) (root, checkout string) {
	t.Helper()
	root, checkout = t.TempDir(), t.TempDir()
	entry := func(id, name, kind string) string {
		return `{"id":"` + id + `","name":"` + name + `","kind":"` + kind + `","target_level":"L1","engine":"tags","grammar_source":"x/y","licence":"MIT","plan_row":"LB-Q-` + id + `"}`
	}
	queue := "[" + strings.Join([]string{
		entry("objective-c", "Objective-C (.m .h by sniff)", "language"),
		entry("matlab", "MATLAB (.m by sniff)", "language"),
		entry("perl", "Perl (.pl .pm .t by sniff)", "language"),
		entry("prolog", "Prolog (.pl .pro)", "language"),
		entry("bazel", "Bazel / Starlark", "tooling"),
		entry("tiltfile", "Tiltfile (Starlark)", "tooling"),
	}, ",") + "]"
	plans := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"items":{
	  "objective-c":{"linguist":["Objective-C"]},
	  "matlab":{"linguist":["MATLAB"]},
	  "perl":{"linguist":["Perl"]},
	  "prolog":{"linguist":["Prolog"]},
	  "bazel":{"linguist":["Starlark"],"allow_filenames":["BUILD","BUILD.bazel"]},
	  "tiltfile":{"linguist":["Starlark"],"allow_filenames":["Tiltfile"]}}}`
	for name, body := range map[string]string{"language-breadth.queue.json": queue, "language-breadth.linguist.json": seed} {
		if err := os.WriteFile(filepath.Join(plans, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib := filepath.Join(checkout, "lib", "linguist")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"languages.yml", "heuristics.yml"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "internal", "langbench", "testdata", "linguist", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lib, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, checkout
}

// TestCmdLangLinguist (LB-F48, LB-F49): `indexbench lang-linguist` imports
// the data from a checkout at a pinned revision, re-reading the hand-kept
// mapping from its own output so a regeneration is byte-stable, and `-match
// <id>` prints the draft match block with its warnings.
func TestCmdLangLinguist(t *testing.T) {
	root, checkout := linguistCmdRoot(t)
	dataPath := langbench.LinguistDataPath(root)

	var out, errb bytes.Buffer
	if code := mainRun([]string{"lang-linguist", "-root", root, "-linguist", checkout, "-rev", linguistCmdRev}, &out, &errb); code != 0 {
		t.Fatalf("import: exit %d: %s", code, errb.String())
	}
	first, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatal(err)
	}
	d, err := langbench.ParseLinguist(first)
	if err != nil {
		t.Fatal(err)
	}
	if d.Revision != linguistCmdRev || d.Source != "github-linguist/linguist" || d.Licence != "MIT" || len(d.Items) != 6 {
		t.Errorf("data = %s %s %s with %d items", d.Source, d.Revision, d.Licence, len(d.Items))
	}
	if got := d.Items["perl"].Extensions; strings.Join(got, " ") != ".pl .pm .t" {
		t.Errorf("perl extensions = %v", got)
	}
	if got := d.Items["bazel"].Filenames; strings.Join(got, " ") != "BUILD BUILD.bazel" {
		t.Errorf("bazel filenames = %v: the seed's allowlist was not re-read", got)
	}
	if !strings.Contains(out.String(), "6 items") {
		t.Errorf("summary = %q", out.String())
	}

	// Regenerating from the generated file changes nothing.
	out.Reset()
	if code := mainRun([]string{"lang-linguist", "-root", root, "-linguist", checkout, "-rev", linguistCmdRev}, &out, &errb); code != 0 {
		t.Fatalf("regenerate: exit %d: %s", code, errb.String())
	}
	if second, _ := os.ReadFile(dataPath); !bytes.Equal(first, second) {
		t.Error("regenerating changed the file")
	}

	// -match prints the block; the claimed extension is a warning on stderr.
	out.Reset()
	errb.Reset()
	if code := mainRun([]string{"lang-linguist", "-root", root, "-match", "perl"}, &out, &errb); code != 0 {
		t.Fatalf("match: exit %d: %s", code, errb.String())
	}
	var m struct {
		Extensions, Basenames, Globs, Shebangs []string
		Sniff                                  *struct {
			Extensions []string
			Pattern    string
			Priority   int
		}
	}
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("match output is not JSON: %v\n%s", err, out.String())
	}
	if strings.Join(m.Extensions, " ") != ".pm .t" || strings.Join(m.Basenames, " ") != "Makefile.PL cpanfile" ||
		strings.Join(m.Shebangs, " ") != "cperl perl" || m.Sniff == nil || strings.Join(m.Sniff.Extensions, " ") != ".pl" || m.Sniff.Priority != 10 {
		t.Errorf("perl draft = %s", out.String())
	}
	if !strings.Contains(errb.String(), "warning: extension .pl is also claimed by prolog") {
		t.Errorf("stderr lacks the .pl claim:\n%s", errb.String())
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("{\n  \"extensions\": [")) {
		t.Errorf("draft is not 2-space indented:\n%s", out.String())
	}

	// Refusals: an id with no data, a missing checkout file, no mode at all.
	for name, args := range map[string][]string{
		"unknown id":          {"lang-linguist", "-root", root, "-match", "nowhere"},
		"not a linguist dir":  {"lang-linguist", "-root", root, "-linguist", root, "-rev", linguistCmdRev},
		"no mode":             {"lang-linguist", "-root", root},
		"short revision":      {"lang-linguist", "-root", root, "-linguist", checkout, "-rev", "5fbdfcb"},
		"missing revision":    {"lang-linguist", "-root", root, "-linguist", checkout},
		"mapping not present": {"lang-linguist", "-root", root, "-linguist", checkout, "-rev", linguistCmdRev, "-out", filepath.Join(root, "nowhere.json")},
	} {
		out.Reset()
		errb.Reset()
		if code := mainRun(args, &out, &errb); code == 0 {
			t.Errorf("%s: exit 0, want a refusal (%s)", name, out.String())
		}
	}
}

// TestCmdLangLinguistPinsTheCheckout: a checkout that is a git repository
// must be at the revision the data says it is.
func TestCmdLangLinguistPinsTheCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root, checkout := linguistCmdRoot(t)
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", checkout, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		var env []string
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "GIT_") {
				env = append(env, e)
			}
		}
		cmd.Env = env
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "linguist")
	head := git("rev-parse", "HEAD")

	var out, errb bytes.Buffer
	if code := mainRun([]string{"lang-linguist", "-root", root, "-linguist", checkout, "-rev", linguistCmdRev}, &out, &errb); code == 0 {
		t.Errorf("a checkout at %s was accepted as %s", head, linguistCmdRev)
	} else if !strings.Contains(errb.String(), head) {
		t.Errorf("the refusal does not name the checkout's HEAD:\n%s", errb.String())
	}
	errb.Reset()
	if code := mainRun([]string{"lang-linguist", "-root", root, "-linguist", checkout, "-rev", head}, &out, &errb); code != 0 {
		t.Errorf("a checkout at its pinned revision was refused: %s", errb.String())
	}
}

// TestCmdLangLinguistCommittedData: the committed data drafts a match block
// for the items the recipe will start from, each valid JSON with the
// manifest keys, from a command that needs no checkout.
func TestCmdLangLinguistCommittedData(t *testing.T) {
	for _, id := range []string{"perl", "objective-c", "bazel", "gemfile-podfile", "erlang"} {
		var out, errb bytes.Buffer
		if code := mainRun([]string{"lang-linguist", "-root", "../..", "-match", id}, &out, &errb); code != 0 {
			t.Errorf("-match %s: exit %d: %s", id, code, errb.String())
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Errorf("-match %s: not JSON: %v", id, err)
			continue
		}
		for _, k := range []string{"extensions", "basenames", "globs", "shebangs", "sniff"} {
			if _, ok := m[k]; !ok {
				t.Errorf("-match %s: no %q key in %s", id, k, out.String())
			}
		}
	}
}
