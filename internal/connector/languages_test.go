package connector_test

// languages_test.go — TestLanguagesTrackerMatchesRegistry (SU-13,
// docs/plans/substrate.md ADR-041 decision 5): docs/languages.md is the
// tracker every wave updates; this test is what keeps it TRUE rather than
// aspirational. It asserts two things against the SAME registry
// connector.RegisteredExtensions() already exposes (ADR-021 §1 / LT-1, the
// one ext→language source of truth):
//
//  1. every language tag the registry actually classifies has a row in
//     docs/languages.md — a language nobody documents is a language nobody
//     can plan for;
//  2. that row's bolded typed-edge level word (native/scip/file/syntax)
//     is consistent with what the CODE provides: native is claimed only
//     for Go (always) and CUE (only once internal/fusion.EnrichCUEEdges
//     exists — until then the row must carry the "(SU-10)" pending
//     marker, which this test accepts as an honest "not yet"); file is
//     claimed only behind an "(SU-11)" pending marker for the same
//     reason (no per-file resolver ships this wave); scip requires only
//     that internal/scip's generic SCIP-import mechanism exist, since
//     ADR-041 decision 5's whole design is language-agnostic — the same
//     `fathom ingest --scip` mechanism serves every language a SCIP
//     producer exists for, whether or not this wave ran one for it; and
//     syntax is always a true baseline claim.
//
// The import below is itself part of assertion 3: if internal/scip (SU-9)
// ever regresses to not existing or not building, this test fails to
// compile — a stronger guarantee than a runtime check could give.

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	_ "github.com/geoffmilleraz/fathom/internal/scip"
)

// languagesDocPath is docs/languages.md relative to this package's test
// working directory.
const languagesDocPath = "../../docs/languages.md"

// langRowHint names, per registry Lang tag, a substring unique to its
// docs/languages.md row (or, for a tag folded into a combined row, unique
// to that row) — hand-maintained alongside the registry itself, the same
// way the table's own rows are hand-maintained.
var langRowHint = map[string]string{
	"go":               "| Go ",
	"cue":              "| CUE ",
	"ts":               "| TypeScript ",
	"js":               "| JavaScript ",
	"py":               "| Python ",
	"java":             "| Java ",
	"kotlin":           "| Kotlin ",
	"html":             "| HTML ",
	"css":              "| CSS ",
	"scss":             "| SCSS | ",
	"vue":              "| Vue single-file components | ",
	"svelte":           "| Svelte components | ",
	"astro":            "| Astro (",
	"scala":            "| Scala ",
	"rust":             "| Rust ",
	"c":                "| C ",
	"cpp":              "| C++ ",
	"ruby":             "| Ruby ",
	"cs":               "| C# ",
	"php":              "| PHP ",
	"sh":               "Bash",
	"powershell":       "PowerShell",
	"makefile":         "| Makefile ",
	"gradle":           "| Gradle builds ",
	"jenkinsfile":      "| Jenkins pipelines ",
	"lua":              "| Lua ",
	"luau":             "| Luau ",
	"dart":             "| Dart ",
	"emacs-lisp":       "| Emacs Lisp ",
	"commonlisp":       "| Common Lisp ",
	"ada":              "| Ada ",
	"vhdl":             "| VHDL ",
	"sql":              "| SQL ",
	"terraform":        "Terraform",
	"yaml":             "Kubernetes YAML",
	"md":               "Markdown",
	"dockerfile":       "Dockerfile",
	"cmake":            "CMake",
	"proto":            "proto",
	"graphql":          "GraphQL",
	"toml":             "TOML",
	"json":             "JSON",
	"package-json":     "package.json",
	"templ":            "templ",
	"python-packaging": "pyproject.toml",
	"maven":            "pom.xml",
	"msbuild":          ".csproj",
	"cargo":            "Cargo.toml",
	"vcpkg":            "vcpkg.json",
	"conan":            "conanfile.txt",
	"go-mod":           "go.mod, go.work,",
	"nginx":            "nginx.conf, haproxy.cfg, Caddyfile",
	"haproxy":          "nginx.conf, haproxy.cfg, Caddyfile",
	"caddyfile":        "nginx.conf, haproxy.cfg, Caddyfile",
	"rego":             "| Rego / OPA policy (.rego) | ",
	"terragrunt":       "Terragrunt (terragrunt.hcl",
	"hcl-tools":        "Terragrunt (terragrunt.hcl",
	"kustomize":        "Kustomize",
	"taskfile":         "Taskfile.yml",
	"jupyter":          "| Jupyter notebooks ",
	"git-hooks-config": "lefthook.yml, .pre-commit-config.yaml, CloudFormation",
	"meson":            "Meson",
	"compile-commands": "compile_commands.json",
	"bicep":            ".bicepparam",
	"serverless":       "serverless.yml, Pulumi.yaml, cdk.json",
	"pulumi-yaml":      "serverless.yml, Pulumi.yaml, cdk.json",
	"aws-cdk":          "serverless.yml, Pulumi.yaml, cdk.json",
}

// registryLangTags returns the sorted, deduplicated set of Lang tags
// connector.RegisteredExtensions() actually classifies — the registry
// docs/languages.md must stay true against.
func registryLangTags(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, ext := range connector.RegisteredExtensions() {
		lang := connector.LanguageForPath(ext)
		if lang == "" || lang == "unclassified" {
			t.Fatalf("registered extension %q classifies to no language", ext)
		}
		seen[lang] = true
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// tableRows returns every "| ... |" line of docs/languages.md's table body
// (the header and separator rows excluded).
func tableRows(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(languagesDocPath)
	if err != nil {
		t.Fatalf("read %s: %v", languagesDocPath, err)
	}
	var rows []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if strings.Contains(trimmed, "---") {
			continue // the header/body separator row
		}
		if strings.HasPrefix(trimmed, "| Language") {
			continue // the header row itself
		}
		rows = append(rows, trimmed)
	}
	return rows
}

var boldWordRE = regexp.MustCompile(`\*\*([a-z]+)\*\*`)

// typedEdgeLevel returns the first bolded word in row — docs/languages.md's
// own convention for the Typed edges column's level (native/scip/file/
// syntax, per the table's header note) — and "" when the row has none.
func typedEdgeLevel(row string) string {
	m := boldWordRE.FindStringSubmatch(row)
	if m == nil {
		return ""
	}
	return m[1]
}

func TestLanguagesTrackerMatchesRegistry(t *testing.T) {
	rows := tableRows(t)
	if len(rows) == 0 {
		t.Fatalf("%s: no table rows found — is the doc still shaped as a table?", languagesDocPath)
	}

	for _, tag := range registryLangTags(t) {
		hint, known := langRowHint[tag]
		if !known {
			t.Errorf("language tag %q (from the extractor registry) has no docs/languages.md row hint registered in this test — add one", tag)
			continue
		}
		var row string
		for _, r := range rows {
			if strings.Contains(r, hint) {
				row = r
				break
			}
		}
		if row == "" {
			t.Errorf("language tag %q: no docs/languages.md row contains %q — the tracker is missing this language", tag, hint)
			continue
		}

		docLevel := typedEdgeLevel(row)
		switch docLevel {
		case "native", "scip", "file", "syntax":
		default:
			t.Errorf("language tag %q: row's typed-edge level %q is none of native/scip/file/syntax: %s", tag, docLevel, row)
			continue
		}

		// Cross-check against connector.TypedEdgeProducer's own table
		// (SU-13's code-side fact, also what `fathom capabilities` reports
		// as typed_edge_producers) — the doc and the code must never
		// silently drift. They may legitimately DISAGREE only when the doc
		// names a not-yet-delivered TARGET behind a "(SU-10)"/"(SU-11)"
		// pending marker (CUE's native target ahead of its current
		// syntax-level code being the one case today) — an honest "not yet"
		// this test accepts, never a silent claim of code that isn't there.
		producer := connector.TypedEdgeProducer(tag)
		if producer == "" {
			t.Errorf("language tag %q: connector.TypedEdgeProducer has no entry — add one to internal/connector/typededges.go", tag)
			continue
		}
		producerLevel := strings.SplitN(producer, ":", 2)[0]
		if producerLevel != docLevel {
			pending := strings.Contains(row, "(SU-10)") || strings.Contains(row, "(SU-11)")
			if !pending {
				t.Errorf("language tag %q: doc claims typed-edge level %q but connector.TypedEdgeProducer says %q, with no \"(SU-10)\"/\"(SU-11)\" pending marker on the row to explain the gap: %s", tag, docLevel, producer, row)
			}
		}
	}
}
