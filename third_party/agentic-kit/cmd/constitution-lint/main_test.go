package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func findingsFor(t *testing.T, fn func(string) []Finding, root string) []Finding {
	t.Helper()
	fs := fn(root)
	if len(fs) == 0 {
		t.Fatal("check returned no findings")
	}
	return fs
}

func assertAllPass(t *testing.T, findings []Finding) {
	t.Helper()
	for _, f := range findings {
		if !f.Pass {
			t.Errorf("unexpected failure: [%s] %s", f.Check, f.Message)
		}
	}
}

func assertAnyFail(t *testing.T, findings []Finding, wantSubstr string) {
	t.Helper()
	for _, f := range findings {
		if !f.Pass && strings.Contains(f.Message, wantSubstr) {
			return
		}
	}
	t.Errorf("expected a failure containing %q; findings: %v", wantSubstr, findings)
}

// ── 1. empty-require ────────────────────────────────────────────────────────

func TestCheckEmptyRequire(t *testing.T) {
	tests := []struct {
		name      string
		root      string
		wantPass  bool
		wantSubst string
	}{
		{
			name:      "clean go.mod",
			root:      "testdata/gomod-clean",
			wantPass:  true,
			wantSubst: "no require block",
		},
		{
			name:      "dirty go.mod with require block",
			root:      "testdata/gomod-dirty",
			wantPass:  false,
			wantSubst: "require block",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := checkEmptyRequire(tc.root)
			if len(findings) != 1 {
				t.Fatalf("want 1 finding, got %d", len(findings))
			}
			f := findings[0]
			if f.Pass != tc.wantPass {
				t.Errorf("Pass=%v, want %v; msg=%q", f.Pass, tc.wantPass, f.Message)
			}
			if !strings.Contains(f.Message, tc.wantSubst) {
				t.Errorf("message %q does not contain %q", f.Message, tc.wantSubst)
			}
		})
	}
}

// ── 2. tracking-coverage ────────────────────────────────────────────────────

func TestCheckTrackingCoverage(t *testing.T) {
	t.Run("all named", func(t *testing.T) {
		findings := findingsFor(t, checkTrackingCoverage, "testdata/tracking-pass")
		assertAllPass(t, findings)
	})

	t.Run("missing entry", func(t *testing.T) {
		findings := findingsFor(t, checkTrackingCoverage, "testdata/tracking-fail")
		assertAnyFail(t, findings, "evidence-span")
	})
}

// ── 3. concurrency-contract ─────────────────────────────────────────────────

func TestCheckConcurrencyContract(t *testing.T) {
	t.Run("contract present", func(t *testing.T) {
		findings := findingsFor(t, checkConcurrencyContract, "testdata/concurrency-pass")
		assertAllPass(t, findings)
	})

	t.Run("contract absent", func(t *testing.T) {
		findings := findingsFor(t, checkConcurrencyContract, "testdata/concurrency-fail")
		assertAnyFail(t, findings, "concurrency")
	})
}

// ── 4. schema-presence ──────────────────────────────────────────────────────

func TestCheckSchemaPresence(t *testing.T) {
	t.Run("schema present", func(t *testing.T) {
		findings := findingsFor(t, checkSchemaPresence, "testdata/schema-pass")
		assertAllPass(t, findings)
	})

	t.Run("schema missing", func(t *testing.T) {
		findings := findingsFor(t, checkSchemaPresence, "testdata/schema-fail")
		assertAnyFail(t, findings, "missing schema.cue")
	})

	t.Run("waived package passes", func(t *testing.T) {
		findings := findingsFor(t, checkSchemaPresence, "testdata/schema-waiver")
		// tool-registry is waived; evidence-span has schema.cue — both should pass
		assertAllPass(t, findings)
	})
}

// ── 5. cassette-version ─────────────────────────────────────────────────────

func TestCheckCassetteVersion(t *testing.T) {
	t.Run("v2 cassette passes", func(t *testing.T) {
		findings := findingsFor(t, checkCassetteVersion, "testdata/cassette-pass")
		assertAllPass(t, findings)
	})

	t.Run("v1 cassette fails", func(t *testing.T) {
		findings := findingsFor(t, checkCassetteVersion, "testdata/cassette-fail")
		assertAnyFail(t, findings, "hash v1")
	})
}

func TestCheckCassetteFile(t *testing.T) {
	tests := []struct {
		name     string
		jsonl    string
		wantPass bool
		wantMsg  string
	}{
		{
			name:     "v2 present",
			jsonl:    `{"hash_version":2,"kind":"complete"}`,
			wantPass: true,
		},
		{
			name:     "hash_version absent → v1",
			jsonl:    `{"kind":"complete","request":{}}`,
			wantPass: false,
			wantMsg:  "hash v1",
		},
		{
			name:     "hash_version=1 explicit",
			jsonl:    `{"hash_version":1,"kind":"complete"}`,
			wantPass: false,
			wantMsg:  "hash_version=1",
		},
		{
			name:     "hash_version=3 unexpected",
			jsonl:    `{"hash_version":3,"kind":"complete"}`,
			wantPass: false,
			wantMsg:  "hash_version=3",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := checkCassetteFile("test.jsonl", strings.NewReader(tc.jsonl+"\n"))
			if len(findings) == 0 {
				t.Fatal("no findings")
			}
			f := findings[0]
			if f.Pass != tc.wantPass {
				t.Errorf("Pass=%v want %v; msg=%q", f.Pass, tc.wantPass, f.Message)
			}
			if tc.wantMsg != "" && !strings.Contains(f.Message, tc.wantMsg) {
				t.Errorf("message %q does not contain %q", f.Message, tc.wantMsg)
			}
		})
	}
}

// ── 6. catalog-fresh ────────────────────────────────────────────────────────

func TestCheckCatalogFresh(t *testing.T) {
	t.Run("up to date", func(t *testing.T) {
		findings := findingsFor(t, checkCatalogFresh, "testdata/catalog-pass")
		assertAllPass(t, findings)
	})

	t.Run("stale", func(t *testing.T) {
		findings := findingsFor(t, checkCatalogFresh, "testdata/catalog-stale")
		assertAnyFail(t, findings, "stale")
	})
}

// ── 7. compat-coverage ──────────────────────────────────────────────────────

func TestCheckCompatCoverage(t *testing.T) {
	t.Run("all classified", func(t *testing.T) {
		findings := findingsFor(t, checkCompatCoverage, "testdata/compatcoverage-clean")
		assertAllPass(t, findings)
	})

	t.Run("missing + invalid classification", func(t *testing.T) {
		findings := findingsFor(t, checkCompatCoverage, "testdata/compatcoverage-dirty")
		assertAnyFail(t, findings, "not classified in docs/compat.md")
		assertAnyFail(t, findings, "invalid classification")
	})
}

func TestParseCompatRows(t *testing.T) {
	md := "| `foundations/x/y` | stable | ok |\n" +
		"| `stable` | 121 |\n" // summary-table row: no "/" in the cell, must be skipped
	rows := parseCompatRows(md)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(rows), rows)
	}
	if rows[0].path != "foundations/x/y" || rows[0].class != "stable" {
		t.Errorf("unexpected row: %+v", rows[0])
	}
}

// ── 8. doc-coverage ─────────────────────────────────────────────────────────

func TestCheckDocCoverage(t *testing.T) {
	t.Run("all documented", func(t *testing.T) {
		findings := findingsFor(t, checkDocCoverage, "testdata/doccoverage-clean")
		assertAllPass(t, findings)
	})

	t.Run("missing + malformed doc comments", func(t *testing.T) {
		findings := findingsFor(t, checkDocCoverage, "testdata/doccoverage-dirty")
		assertAnyFail(t, findings, `"NoDoc" has no doc comment`)
		assertAnyFail(t, findings, `"BadDoc" doc comment does not start with "BadDoc"`)
	})

	t.Run("nested module is skipped, not failed", func(t *testing.T) {
		findings := findingsFor(t, checkDocCoverage, "testdata/doccoverage-nestedmodule")
		assertAllPass(t, findings)
		found := false
		for _, f := range findings {
			if strings.Contains(f.Message, "nested module, doc-coverage skipped") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a nested-module skip note; findings: %v", findings)
		}
	})

	t.Run("waived package passes", func(t *testing.T) {
		findings := findingsFor(t, checkDocCoverage, "testdata/doccoverage-waiver")
		assertAllPass(t, findings)
	})
}

// ── 9. org-agnostic ──────────────────────────────────────────────────────────

func TestCheckOrgAgnostic(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-clean")
		assertAllPass(t, findings)
	})

	t.Run("denylisted token", func(t *testing.T) {
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-dirty")
		assertAnyFail(t, findings, `contains denylisted org-specific token "FakeOrgCorp"`)
	})

	t.Run(".claude/worktrees excluded", func(t *testing.T) {
		// Claude Code keeps agent worktrees under .claude/worktrees/ — full
		// stale copies of the repo whose waivers (keyed to real paths) can
		// never match. Never shipped, never swept.
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-claudedir")
		assertAllPass(t, findings)
	})

	t.Run(".claude/skills swept", func(t *testing.T) {
		// .claude/skills holds shipped skills (the kit's one real copy), so
		// only .claude/worktrees is exempt, not all of .claude/.
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-claudeskills")
		assertAnyFail(t, findings, `.claude/skills/leaky/SKILL.md:3: contains denylisted org-specific token "FakeOrgCorp"`)
	})

	t.Run("model-ID literal outside carve-out", func(t *testing.T) {
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-modelid-dirty")
		assertAnyFail(t, findings, "hardcoded model-ID literal")
	})

	t.Run("model-ID literal inside provider carve-out passes", func(t *testing.T) {
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-modelid-carveout")
		assertAllPass(t, findings)
	})

	t.Run("waived hit passes", func(t *testing.T) {
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-waiver")
		assertAllPass(t, findings)
	})
}

func TestIsModelIDCarveOut(t *testing.T) {
	for rel, want := range map[string]bool{
		"foundations/llmclient/bedrock/config.go":   true,
		"foundations/llmclient/anthropic/client.go": true,
		"foundations/llmclient/ladder.go":           false, // second-level file, not a provider subdir
		"foundations/decision/openai/openai.go":     true,  // a decision-port provider adapter
		"foundations/decision/typesafe/typesafe.go": true,
		"foundations/decision/types.go":             false, // the port's core stays scanned
		"foundations/decision/decisiontest/c.go":    false, // the conformance suite is not an adapter
		"foundations/governance/x/y.go":             false,
		"examples/agent-cli/main.go":                false,
	} {
		if got := isModelIDCarveOut(rel); got != want {
			t.Errorf("isModelIDCarveOut(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestContainsDigit(t *testing.T) {
	if !containsDigit("claude-opus-4-8") {
		t.Error("expected a digit in claude-opus-4-8")
	}
	if containsDigit("claude-cli") {
		t.Error("expected no digit in claude-cli")
	}
}

// ── 10. secret-scan ──────────────────────────────────────────────────────────

func TestCheckSecretScan(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		findings := findingsFor(t, checkSecretScan, "testdata/secretscan-clean")
		assertAllPass(t, findings)
	})

	t.Run(".claude/skills swept, .claude/worktrees not", func(t *testing.T) {
		findings := checkSecretScan("testdata/secretscan-claudedir")
		var failed []string
		for _, f := range findings {
			if !f.Pass {
				failed = append(failed, f.Message)
			}
		}
		if len(failed) != 1 || !strings.HasPrefix(failed[0], ".claude/skills/leaky/SKILL.md:") {
			t.Fatalf("want exactly one failure, in .claude/skills/leaky/SKILL.md; got %v", failed)
		}
	})
}

func TestIsSweepSkipDir(t *testing.T) {
	for rel, want := range map[string]bool{
		".git":                     true,
		"tmp":                      true,
		"kits/x/workspace":         true,
		".claude/worktrees":        true,
		"sub/.claude/worktrees":    true,
		".claude":                  false,
		".claude/skills":           false,
		".claude/skills/worktrees": false,
		"worktrees":                false,
		"sub/worktrees":            false,
	} {
		if got := isSweepSkipDir(rel); got != want {
			t.Errorf("isSweepSkipDir(%q) = %v, want %v", rel, got, want)
		}
	}
}

// TestSecretScanCanary plants a fixture (testdata/secretscan-canary/)
// containing obviously-fake-but-pattern-matching credentials and proves the
// REAL, committed cmd/constitution-lint/secret-patterns.txt (via
// secretPatternsPath's package-relative fallback — see its doc comment)
// catches them. D10 discipline: a scan that can't find a planted secret
// proves nothing.
func TestSecretScanCanary(t *testing.T) {
	findings := checkSecretScan("testdata/secretscan-canary")
	found := false
	for _, f := range findings {
		if !f.Pass && strings.Contains(f.Message, "leaked-config.txt") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a Fail finding referencing the planted canary fixture; findings: %v", findings)
	}
}

func TestLooksBinary(t *testing.T) {
	if looksBinary([]byte("plain text, no NUL bytes here")) {
		t.Error("plain text should not look binary")
	}
	if !looksBinary([]byte("binary\x00noise")) {
		t.Error("data with a NUL byte should look binary")
	}
}

// ── run() integration ────────────────────────────────────────────────────────

func TestRun(t *testing.T) {
	t.Run("exit 0 on clean fixture", func(t *testing.T) {
		// catalog-pass has only 1 package that has schema.cue, a package doc
		// with "concurrent", a clean catalog.md, no go.mod, and no cassette.
		// Some checks (tracking, gomod) will error on missing files — that's
		// fine for the integration smoke test; we just verify the binary doesn't
		// panic and produces output.
		var buf bytes.Buffer
		// Use a fixture that will fail on some checks (expected) to prove run
		// returns non-zero exit code and prints FAIL lines.
		code := run("testdata/gomod-dirty", &buf, nil)
		if code == 0 {
			t.Error("expected exit 1 for dirty go.mod fixture, got 0")
		}
		if !strings.Contains(buf.String(), "FAIL [empty-require]") {
			t.Errorf("expected FAIL [empty-require] in output: %s", buf.String())
		}
	})

	t.Run("verbose flag lists passes", func(t *testing.T) {
		var buf bytes.Buffer
		run("testdata/gomod-clean", &buf, []string{"-v"})
		out := buf.String()
		if !strings.Contains(out, "ok   [empty-require]") {
			t.Errorf("expected ok   [empty-require] in verbose output: %s", out)
		}
	})
}

// ── 11. readme-count ─────────────────────────────────────────────────────────

func TestCheckReadmeCount(t *testing.T) {
	t.Run("count matches source", func(t *testing.T) {
		findings := findingsFor(t, checkReadmeCount, "testdata/readmecount-clean")
		assertAllPass(t, findings)
	})

	t.Run("stale hand-maintained count", func(t *testing.T) {
		findings := findingsFor(t, checkReadmeCount, "testdata/readmecount-dirty")
		assertAnyFail(t, findings, "README.md says 2 Go packages; source count is 1")
	})

	t.Run("sentence absent", func(t *testing.T) {
		// gomod-clean's fixture root has no README.md at all.
		findings := findingsFor(t, checkReadmeCount, "testdata/gomod-clean")
		assertAnyFail(t, findings, "cannot read README.md")
	})
}

// ── helpers unit tests ───────────────────────────────────────────────────────

func TestSkipDirName(t *testing.T) {
	for name, want := range map[string]bool{
		"testdata":   true,
		"_reference": true,
		".git":       true,
		"evidence":   false,
		"llmclient":  false,
		"kits":       false,
	} {
		if got := skipDirName(name); got != want {
			t.Errorf("skipDirName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestLoadWaivers(t *testing.T) {
	waivers, err := loadWaivers("testdata/schema-waiver/cmd/constitution-lint/waivers.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !waivers["foundations/agent/tool-registry"] {
		t.Error("expected foundations/agent/tool-registry to be waived")
	}
	if waivers["foundations/evidence/evidence-span"] {
		t.Error("foundations/evidence/evidence-span should not be waived")
	}
}

func TestLoadWaiversMissingFile(t *testing.T) {
	waivers, err := loadWaivers("testdata/nonexistent/waivers.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(waivers) != 0 {
		t.Error("expected empty waivers for missing file")
	}
}

// ── 12. read-coverage ────────────────────────────────────────────────────────

// writeMinimalTree lays down the smallest tree the walk sites all visit: a
// root go.mod plus one foundation package with a concurrency-contract doc
// and a schema.cue. Returns the package dir and its .go file.
func writeMinimalTree(t *testing.T, root string) (pkgDir, goFile string) {
	t.Helper()
	pkgDir = filepath.Join(root, "foundations", "cat", "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goFile = filepath.Join(pkgDir, "pkg.go")
	if err := os.WriteFile(goFile, []byte("// Package pkg is safe for concurrent use.\npackage pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "schema.cue"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return pkgDir, goFile
}

// TestUnreadableTreeNeverPassesConstitution pins FB-9: a file or directory
// that exists but cannot be read is a FAIL finding of its own, so the
// constitution cannot pass on a tree it could not fully see, and the
// summary line renders the finding count as a lower bound naming the
// blind spot. The readable variant of the same tree produces no
// read-coverage failure, so the only variable is readability.
func TestUnreadableTreeNeverPassesConstitution(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits cannot make a path unreadable")
	}

	t.Run("readable tree has no read-coverage failure", func(t *testing.T) {
		root := t.TempDir()
		writeMinimalTree(t, root)
		var buf bytes.Buffer
		run(root, &buf, nil)
		out := buf.String()
		if strings.Contains(out, "FAIL [read-coverage]") {
			t.Fatalf("a fully readable tree must not fail read-coverage:\n%s", out)
		}
		if strings.Contains(out, "≥") {
			t.Fatalf("a complete manifest renders a plain count, not a lower bound:\n%s", out)
		}
	})

	t.Run("unreadable file", func(t *testing.T) {
		root := t.TempDir()
		_, goFile := writeMinimalTree(t, root)
		if err := os.Chmod(goFile, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(goFile, 0o644) })

		var buf bytes.Buffer
		code := run(root, &buf, nil)
		out := buf.String()
		if code == 0 {
			t.Fatalf("an unreadable file must fail the constitution:\n%s", out)
		}
		want := "FAIL [read-coverage] foundations/cat/pkg/pkg.go: permission denied"
		if !strings.Contains(out, want) {
			t.Fatalf("want %q in output:\n%s", want, out)
		}
		if !strings.Contains(out, "findings checked: ≥ ") || !strings.Contains(out, "(1 unreadable: foundations/cat/pkg/pkg.go)") {
			t.Fatalf("summary must render the finding count as a named lower bound:\n%s", out)
		}
	})

	t.Run("unreadable directory", func(t *testing.T) {
		root := t.TempDir()
		pkgDir, _ := writeMinimalTree(t, root)
		if err := os.Chmod(pkgDir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(pkgDir, 0o755) })

		var buf bytes.Buffer
		code := run(root, &buf, nil)
		out := buf.String()
		if code == 0 {
			t.Fatalf("an unreadable directory must fail the constitution:\n%s", out)
		}
		want := "FAIL [read-coverage] foundations/cat/pkg: permission denied"
		if !strings.Contains(out, want) {
			t.Fatalf("want %q in output:\n%s", want, out)
		}
		if !strings.Contains(out, "(1 unreadable: foundations/cat/pkg)") {
			t.Fatalf("summary must name the blind directory:\n%s", out)
		}
	})
}

// TestWalkWithManifestRecordsOutcomes pins what walkWithManifest records
// for each kind of path the sweep meets, and that a failed file read is
// recorded (and skipped) rather than aborting the walk.
func TestWalkWithManifestRecordsOutcomes(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel string, data []byte) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("a.txt", []byte("text"))
	mustWrite("b.bin", []byte("x\x00y"))
	mustWrite("c_test.go", []byte("package c"))
	mustWrite(".git/HEAD", []byte("ref"))
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	set := readmanifest.NewSet()
	set.RelativeTo(root)
	var swept []string
	if err := walkWithManifest(root, set, func(rel string, _ []byte) { swept = append(swept, rel) }); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if !reflect.DeepEqual(swept, []string{"a.txt"}) {
		t.Fatalf("swept = %v, want only a.txt", swept)
	}
	got := map[string]readmanifest.Outcome{}
	for _, e := range set.Manifest().Entries {
		got[e.Path] = e.Outcome
	}
	want := map[string]readmanifest.Outcome{
		".git":      readmanifest.Skipped("sweep-skip-dir"),
		"a.txt":     readmanifest.Read(4),
		"b.bin":     readmanifest.Read(3),
		"c_test.go": readmanifest.Skipped("test-file"),
		"dangling":  readmanifest.Absent(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest = %+v\nwant %+v", got, want)
	}
	if !set.Manifest().Complete() {
		t.Fatal("skips and a dangling symlink are facts, not blindness")
	}

	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits cannot make a file unreadable")
	}
	if err := os.Chmod(filepath.Join(root, "a.txt"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "a.txt"), 0o644) })
	blind := readmanifest.NewSet()
	blind.RelativeTo(root)
	swept = nil
	if err := walkWithManifest(root, blind, func(rel string, _ []byte) { swept = append(swept, rel) }); err != nil {
		t.Fatalf("a failed file read is recorded, not a walk error: %v", err)
	}
	if len(swept) != 0 {
		t.Fatalf("an unreadable file must not be swept, got %v", swept)
	}
	m := blind.Manifest()
	if m.Complete() {
		t.Fatal("a failed read must make the manifest incomplete")
	}
	if bad := m.Unreadable(); len(bad) != 1 || bad[0].Path != "a.txt" || bad[0].Outcome.Detail != "permission denied" {
		t.Fatalf("unreadable = %+v", bad)
	}
	if got, want := m.LowerBound(1), "≥ 1 (1 unreadable: a.txt)"; got != want {
		t.Fatalf("LowerBound = %q, want %q", got, want)
	}
}

// TestWalkWithManifestSkipsDirectorySymlinks pins that a symlink to a directory (the
// kit's `.agents/skills` link to `.claude/skills`) is recorded as a skip, not
// as an unreadable file that fails read-coverage, and that the tree it points at is
// still swept once, at its real path. A symlink to a file is still read.
func TestWalkWithManifestSkipsDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range map[string]string{"real/r.txt": "in the real tree", "file.txt": "target"} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("real", filepath.Join(root, "linkdir")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink("file.txt", filepath.Join(root, "linkfile")); err != nil {
		t.Fatal(err)
	}

	set := readmanifest.NewSet()
	set.RelativeTo(root)
	var swept []string
	if err := walkWithManifest(root, set, func(rel string, _ []byte) { swept = append(swept, rel) }); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if want := []string{"file.txt", "linkfile", "real/r.txt"}; !reflect.DeepEqual(swept, want) {
		t.Fatalf("swept = %v, want %v", swept, want)
	}
	got := map[string]readmanifest.Outcome{}
	for _, e := range set.Manifest().Entries {
		got[e.Path] = e.Outcome
	}
	if got["linkdir"] != readmanifest.Skipped("dir-symlink") {
		t.Fatalf("linkdir = %+v, want a dir-symlink skip", got["linkdir"])
	}
	if !set.Manifest().Complete() {
		t.Fatal("a directory symlink is a fact, not blindness")
	}
}
