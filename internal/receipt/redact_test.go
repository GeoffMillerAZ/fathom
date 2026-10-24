package receipt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestRedactReceipt(t *testing.T) {
	key := func(service, name string) string { return core.SymbolKey(service, "pkg", name) }
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "keep-service"}[keep], func(t *testing.T) {
			r := ExportedReceipt{Receipt: core.Receipt{
				Service: "fixture-service", Symbol: key("fixture-service", "Greet"),
				Callers: []string{key("fixture-service", "Hello"), key("second-service", "Remote")},
				Test:    key("fixture-service", "TestGreet"), Target: key("fixture-service", "Greet"),
				Reached: []string{key("second-service", "Remote"), "Bare"},
				File:    "fixture-service/a.go", IndexCommit: strings.Repeat("a", 40),
			}, Facts: []Fact{{Kind: FactDefinition, Subject: key("fixture-service", "Greet"),
				Anchors: []ExportAnchor{exportAnchor(committedAnchor("a.go", 1, 1, []byte("package pkg\n")), false)},
			}}}
			before := string(marshalExported(t, r))
			RedactReceipt(&r, keep)
			if keep {
				if got := string(marshalExported(t, r)); got != before {
					t.Errorf("--keep-service changed receipt: %s", got)
				}
				return
			}
			if r.Service != "@" || r.Symbol != "@::pkg.Greet" || r.Test != "@::pkg.TestGreet" || r.Target != "@::pkg.Greet" ||
				!reflect.DeepEqual(r.Callers, []string{"@::pkg.Hello", "@::pkg.Remote"}) ||
				!reflect.DeepEqual(r.Reached, []string{"@::pkg.Remote", "Bare"}) || r.Facts[0].Subject != "@::pkg.Greet" {
				t.Fatalf("service identity remains in parameters or symbol keys: %+v", r)
			}
			if r.File != "fixture-service/a.go" || r.IndexCommit != strings.Repeat("a", 40) || r.Facts[0].Anchors[0].Excerpt != "package pkg\n" {
				t.Fatal("redaction changed source evidence")
			}
			first := string(marshalExported(t, r))
			RedactReceipt(&r, false)
			if got := string(marshalExported(t, r)); got != first {
				t.Errorf("redaction is not idempotent: %s", got)
			}
		})
	}
	bare := ExportedReceipt{Receipt: core.Receipt{Symbol: "Bare", Test: "pkg.TestBare", Target: "invariant:I1"}}
	redactReceipt(&bare)
	if bare.Service != "" || bare.Symbol != "Bare" || bare.Test != "pkg.TestBare" || bare.Target != "invariant:I1" {
		t.Errorf("redaction invented a service or changed bare parameters: %+v", bare)
	}
}

// privacyViolations examines decoded JSON strings, including keys. Only
// facts[*].anchors[*].excerpt values are source evidence: scan them for
// planted identity canaries, and apply path/address shapes everywhere else.
func privacyViolations(data []byte) ([]string, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	shape := regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://|[a-z0-9_.+-]+@[a-z0-9.-]+|(?-i:(?:[a-z0-9-]+\.)+(?:invalid|test|local|com|org|net|io|dev|ai|internal)\b)|\blocalhost\b|\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b|(?:^|[\s"':=(])(?:/|[a-z]:[\\/]|\\\\|~[\\/]))`)
	var leaks []string
	var walk func(any, string)
	walk = func(v any, path string) {
		switch x := v.(type) {
		case string:
			if (path != `["facts"][]["anchors"][]["excerpt"]` && (shape.MatchString(x) || strings.Contains(x, "fixture-user"))) ||
				strings.Contains(x, "fixture-service") || strings.Contains(x, "second-service") {
				leaks = append(leaks, x)
			}
		case []any:
			for _, e := range x {
				walk(e, path+"[]")
			}
		case map[string]any:
			for k, e := range x {
				walk(k, "")
				walk(e, path+"["+strconv.Quote(k)+"]")
			}
		}
	}
	walk(v, "")
	sort.Strings(leaks)
	return leaks, nil
}

func TestRedactReceiptPrivacyScan(t *testing.T) {
	files, err := filepath.Glob("testdata/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden receipts: %v", err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			golden, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			leaks, err := privacyViolations(golden)
			if err != nil || len(leaks) > 0 {
				t.Fatalf("golden privacy scan: %v, %v", leaks, err)
			}
			var r ExportedReceipt
			if err := json.Unmarshal(golden, &r); err != nil {
				t.Fatal(err)
			}
			// Check the no-op redactor control before applying redaction to the
			// generated export. The planted identity must make the scan fail.
			input := strings.ReplaceAll(string(golden), "@", "fixture-service")
			if err := json.Unmarshal([]byte(input), &r); err != nil {
				t.Fatal(err)
			}
			leaks, err = privacyViolations(marshalExported(t, r))
			if err != nil || len(leaks) == 0 {
				t.Fatalf("no-op redactor passed the privacy scan: %v, %v", leaks, err)
			}
			RedactReceipt(&r, false)
			got := marshalExported(t, r)
			leaks, err = privacyViolations(got)
			if err != nil || len(leaks) > 0 {
				t.Fatalf("export privacy scan: %v, %v", leaks, err)
			}
			var want any
			var actual any
			if err := json.Unmarshal(golden, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(got, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Errorf("redacted export differs from golden:\n%s", got)
			}
		})
	}
}

func TestRedactReceiptPrivacyScanDetectsLeaks(t *testing.T) {
	for _, leak := range []string{"/opt/private/a.go", `C:\Users\fixture-user\a.go`, `\\build.invalid\repo`, "https://example.invalid/repo", "build.example.invalid", "127.0.0.1", "fixture-user", "fixture-user@example.invalid", "fixture-service::pkg.Greet", "second-service"} {
		data, err := json.Marshal(map[string]string{"value": leak})
		if err != nil {
			t.Fatal(err)
		}
		leaks, err := privacyViolations(data)
		if err != nil || len(leaks) != 1 {
			t.Errorf("scan missed %q: %v, %v", leak, leaks, err)
		}
	}
}

// Source excerpts are evidence: comments and imports stay verbatim, while
// metadata and JSON keys still receive the full privacy scan.
func TestRedactReceiptPrivacyScanExcerpts(t *testing.T) {
	excerpt := "// See https://example.invalid/guide\nimport _ \"example.invalid/library\"\n// /opt/source and fixture-user@example.invalid\n"
	data, err := json.Marshal(map[string]any{"facts": []any{map[string]any{"anchors": []any{map[string]string{"excerpt": excerpt}}}}})
	if err != nil {
		t.Fatal(err)
	}
	leaks, err := privacyViolations(data)
	if err != nil || len(leaks) != 0 {
		t.Errorf("verbatim excerpt flagged: %v, %v", leaks, err)
	}
	for _, canary := range []string{"fixture-service", "second-service"} {
		data := []byte(strings.ReplaceAll(string(data), "example.invalid/library", canary))
		leaks, err := privacyViolations(data)
		if err != nil || len(leaks) != 1 {
			t.Errorf("excerpt identity %q missed: %v, %v", canary, leaks, err)
		}
	}
	for _, field := range []string{"path", "subject", "excerpt", "facts.*.anchors.*.excerpt"} {
		data, err := json.Marshal(map[string]string{field: "https://example.invalid/guide"})
		if err != nil {
			t.Fatal(err)
		}
		leaks, err := privacyViolations(data)
		if err != nil || len(leaks) != 1 {
			t.Errorf("metadata %q missed: %v, %v", field, leaks, err)
		}
	}
	data = []byte(`{"facts":[{"anchors":[{"https://example.invalid/key":"safe"}]}]}`)
	leaks, err = privacyViolations(data)
	if err != nil || len(leaks) != 1 {
		t.Errorf("JSON key missed: %v, %v", leaks, err)
	}
}
