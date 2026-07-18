package main

import (
	"bytes"
	"strings"
	"testing"
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

	t.Run(".claude session state excluded", func(t *testing.T) {
		// Claude Code keeps agent worktrees and other session state under
		// .claude/ — full stale copies of the repo whose waivers (keyed to
		// real paths) can never match. Never shipped, never swept.
		findings := findingsFor(t, checkOrgAgnostic, "testdata/orgagnostic-claudedir")
		assertAllPass(t, findings)
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
