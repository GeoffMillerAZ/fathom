package readout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rule(id, body string) string {
	return "schema = 1\n[[rule]]\nid = \"" + id + "\"\nversion = 1\nabout = \"a test rule\"\n" + body
}

const goodTail = `match.argv0 = ["make"]
recovery = "repeat"
[[rule.stage]]
kind = "drop"
pattern = '^cc '
[[rule.test]]
input = "cc a\nok\n"
expect = "ok\n[readout %s@1 observe: 2→1 lines; dropped 1 (plain 1); raw: repeat the command]\n"
[[rule.test]]
input = "cc error\n"
trip = true
`

func good(id string) string { return rule(id, strings.ReplaceAll(goodTail, "%s", id)) }

// TestLoadRulesRefusesARuleWhoseFixtureFails is RO-7's contract: the user
// files load after the built-ins, each validated by the embedded CUE schema
// and held to its fixtures and the contract; a rule that fails is not served
// and is named, and the rest are served.
func TestLoadRulesRefusesARuleWhoseFixtureFails(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		UserFile: good("make-quiet"),
		"readouts.d/a-bad-fixture.toml": rule("bad-fixture", strings.Replace(strings.ReplaceAll(goodTail, "%s", "bad-fixture"),
			`expect = "ok\n`, `expect = "OK\n`, 1)),
		"readouts.d/b-no-trip.toml": rule("no-trip", `match.argv0 = ["make"]
recovery = "repeat"
[[rule.stage]]
kind = "drop"
pattern = '^cc '
[[rule.test]]
input = "cc a\nok\n"
expect = "ok\n[readout no-trip@1 observe: 2→1 lines; dropped 1 (plain 1); raw: repeat the command]\n"
`),
		"readouts.d/c-unknown-key.toml": strings.Replace(good("unknown-key"), "recovery =", "command = \"make all\"\nrecovery =", 1),
		"readouts.d/d-join.toml": rule("bad-join", `match.argv0 = ["make"]
[[rule.stage]]
kind = "join"
join = "curl"
[[rule.test]]
input = "a\n"
expect = "a\n"
`),
		"readouts.d/e-argv.toml":       strings.Replace(good("argv"), `["make"]`, `["make && rm -rf /"]`, 1),
		"readouts.d/f-parse.toml":      "schema = 1\n[[rule]\n",
		"readouts.d/g-url.toml":        strings.Replace(good("url"), `about = "a test rule"`, `about = "see https://example.invalid"`, 1),
		"readouts.d/h-stream.toml":     strings.Replace(good("stream"), "trip = true\n", "trip = true\nstream = \"content\"\n", 1),
		"readouts.d/i-override.toml":   strings.ReplaceAll(strings.Replace(good("make-quiet"), "version = 1", "version = 4", 1), "@1", "@4"),
		"readouts.d/j-file.toml":       strings.Replace(good("from-file"), `input = "cc a\nok\n"`, `input_file = "j-input.txt"`, 1),
		"readouts.d/j-input.txt":       "cc a\nok\n",
		"readouts.d/k-no-trip.toml":    strings.Replace(good("trip-none"), "input = \"cc error\\n\"", "input = \"plain\\n\"", 1),
		"readouts.d/l-dup.toml":        good("dup") + strings.TrimPrefix(good("dup"), "schema = 1\n"),
		"readouts.d/notes.txt":         "not a rule",
		"readouts.d/z-schema-two.toml": strings.Replace(good("schema-two"), "schema = 1", "schema = 2", 1),
	})
	rs, err := LoadRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	var served []string
	for _, r := range rs.Rules {
		if !strings.HasPrefix(r.Source, "builtin:") {
			served = append(served, r.Ref()+" "+filepath.Base(r.Source))
		}
	}
	if want := "dup@1 l-dup.toml|from-file@1 j-file.toml|make-quiet@4 i-override.toml"; strings.Join(sortedCopy(served), "|") != want {
		t.Fatalf("served = %q, want %q", served, want)
	}
	want := map[string]string{
		"a-bad-fixture.toml": "bad-fixture: fixture 1: line 1: got \"ok\", want \"OK\"",
		"b-no-trip.toml":     "no-trip: lossy rule needs a protected-line fixture for its diagnostic stream",
		"c-unknown-key.toml": ": schema: rule.0.command: field not allowed",
		"d-join.toml":        ": schema: rule.0.stage.0.join",
		"e-argv.toml":        ": schema: rule.0.match.argv0.0",
		"f-parse.toml":       ": parse TOML",
		"g-url.toml":         ": schema: rule.0.about",
		"h-stream.toml":      "stream: fixture 2: stream content is not one this rule's match reaches",
		"k-no-trip.toml":     "trip-none: fixture 2: expected a protected-line trip, none tripped",
		"l-dup.toml":         "dup: duplicate id in the file",
		"z-schema-two.toml":  ": schema: schema",
	}
	got := map[string]string{}
	for _, r := range rs.Refused {
		got[filepath.Base(r.Source)] = r.ID + ": " + r.Reason
	}
	for file, w := range want {
		if !strings.HasPrefix(got[file], w) {
			t.Errorf("%s: refusal %q, want it to start %q", file, got[file], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("refused %d files, want %d: %+v", len(got), len(want), rs.Refused)
	}
}

func sortedCopy(xs []string) []string {
	out := append([]string(nil), xs...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func TestLoadRulesEmbedsTheBuiltinsAndLoadsUserFilesAfterThem(t *testing.T) {
	rs, err := LoadRules("")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Refused) != 0 {
		t.Fatalf("embedded built-ins refused: %+v", rs.Refused)
	}
	names, err := os.ReadDir("rules")
	if err != nil {
		t.Fatal(err)
	}
	toml := 0
	for _, n := range names {
		if strings.HasSuffix(n.Name(), ".toml") {
			b, err := os.ReadFile(filepath.Join("rules", n.Name()))
			if err != nil {
				t.Fatal(err)
			}
			toml += strings.Count(string(b), "\n[[rule]]\n")
		}
	}
	if len(rs.Rules) != toml {
		t.Fatalf("loaded %d embedded rules, rules/*.toml hold %d", len(rs.Rules), toml)
	}

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"readouts.d/override.toml": strings.Replace(
		strings.Replace(good("make-chatter"), "version = 1", "version = 2", 1), "@1", "@2", -1)})
	rs, err = loadRules(os.DirFS("testdata/builtins"), dir)
	if err != nil || len(rs.Refused) != 0 {
		t.Fatalf("load: %v, refused %+v", err, rs.Refused)
	}
	if len(rs.Rules) != 1 || rs.Rules[0].Ref() != "make-chatter@2" || !strings.HasSuffix(rs.Rules[0].Source, "override.toml") {
		t.Fatalf("rules = %+v, want the user's make-chatter@2 replacing the built-in", rs.Rules)
	}
	rs, _ = loadRules(os.DirFS("testdata/builtins"), "")
	if len(rs.Rules) != 1 || rs.Rules[0].Source != "builtin:make-chatter.toml" || Lossiness(rs.Rules[0].Stages) != LossLines {
		t.Fatalf("built-in rules = %+v", rs.Rules)
	}
}
