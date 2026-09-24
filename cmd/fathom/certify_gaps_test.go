package main

// certify_gaps_test.go — TC-11's own test: TestCmdCertifyGaps_RemedyLinesAndJSON.
// Fixture: one live certificate (TestAdd certifies invariant I1, an
// attested passing run), one invariant row with nothing declared for it
// (I2 — a pure corpus gap) and one incident record with nothing declared
// (INC-1) — exercising both "declared but not live" and "never declared
// at all" gap sources, plus the always-live case that must carry NO
// remedy.
import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const certifyGapsFixtureManifest = `
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`

const certifyGapsFixtureSrc = `package cart

// certifies: invariant I1 (asserts)
func TestAdd(t *T) {}

type T struct{}
`

const certifyGapsFixtureRouter = "# Router\n\ndocs/intent/invariants.md and docs/intent/incidents.md are the living docs.\n"

const certifyGapsFixtureInvariants = "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n| I1 | Widgets never negative | test (`TestAdd`) |\n| I2 | Quota never exceeded | test (`TestQuota`) |\n"

const certifyGapsFixtureIncidents = "# Incidents\n\n## Open\n\n### INC-1\n\nSomething broke.\n"

func certifyGapsFixtureEnvelope() string {
	return `{
  "suite": {"component": "cart", "name": "unit", "class": "unit"},
  "key": "k1",
  "verdict": "pass",
  "cases": [
    {"id": "1", "name": "TestAdd", "status": "pass", "duration": "1ms", "attempts": 1}
  ],
  "totals": {"run": 1, "passed": 1, "failed": 0, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "2026-09-23T00:00:00Z", "wallClock": "1ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "amd64", "os": "darwin"},
  "artifacts": []
}`
}

// setUpCertifyGapsFixture builds a repo with the manifest+marker+intent
// corpus above, ingests it, then ingests one attested passing run so
// TestAdd's certificate compiles to state "live". Returns (root,
// storePath, service).
func setUpCertifyGapsFixture(t *testing.T) (root, storePath, service string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "intent"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tests/cart.cue", certifyGapsFixtureManifest)
	write("cart_test.go", certifyGapsFixtureSrc)
	write("docs/intent/README.md", certifyGapsFixtureRouter)
	write("docs/intent/invariants.md", certifyGapsFixtureInvariants)
	write("docs/intent/incidents.md", certifyGapsFixtureIncidents)

	service = "cart-svc"
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, errBuf.String())
	}

	resultsPath := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(resultsPath, []byte(certifyGapsFixtureEnvelope()), 0o644); err != nil {
		t.Fatal(err)
	}
	var tiOut, tiErr bytes.Buffer
	if code := cmdTestsIngest([]string{"--service", service, "--store", storePath, "--manifest", root, "--results", resultsPath}, &tiOut, &tiErr); code != 0 {
		t.Fatalf("tests ingest exit = %d, stderr: %s", code, tiErr.String())
	}
	return root, storePath, service
}

// certifyGapsGolden is the certifications kind/id/gap/remedy/certificate
// shape TestCmdCertifyGaps_RemedyLinesAndJSON compares against, with the
// two genuinely volatile fields (generated_at, index_commit) and every
// certificate's run_digest (a content hash, not worth pinning here)
// zeroed before comparison — the schema string, field names, sort order
// and gap/remedy discipline are the actual contract under test.
const certifyGapsGolden = `{
  "schema": "fathom.certify.gaps/v1",
  "service": "cart-svc",
  "index_commit": "",
  "generated_at": "",
  "kinds": [
    "incident",
    "invariant"
  ],
  "targets": [
    {
      "kind": "incident",
      "id": "INC-1",
      "severity": "",
      "gap": true,
      "remedy": "add \"// certifies: incident INC-1 (asserts)\" above a Test* function, or a #Certifies entry naming incident INC-1 in the suite's tests/*.cue manifest",
      "certificates": []
    },
    {
      "kind": "invariant",
      "id": "I1",
      "severity": "",
      "gap": false,
      "remedy": "",
      "certificates": [
        {
          "test": "TestAdd",
          "suite": "unit",
          "class": "",
          "strength": "asserts",
          "state": "live",
          "tier": "verified",
          "run_digest": ""
        }
      ]
    },
    {
      "kind": "invariant",
      "id": "I2",
      "severity": "",
      "gap": true,
      "remedy": "add \"// certifies: invariant I2 (asserts)\" above a Test* function, or a #Certifies entry naming invariant I2 in the suite's tests/*.cue manifest",
      "certificates": []
    }
  ],
  "summary": {
    "declared": 3,
    "live": 1,
    "gaps": 2,
    "joined_cases": 1,
    "cases": 1
  }
}
`

func redactCertifyGapsPayload(t *testing.T, raw []byte) string {
	t.Helper()
	var p certifyGapsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal certify gaps JSON: %v\n%s", err, raw)
	}
	p.GeneratedAt = ""
	p.IndexCommit = ""
	for i := range p.Targets {
		for j := range p.Targets[i].Certificates {
			p.Targets[i].Certificates[j].RunDigest = ""
		}
	}
	out, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return string(out) + "\n"
}

func TestCmdCertifyGaps_RemedyLinesAndJSON(t *testing.T) {
	_, storePath, service := setUpCertifyGapsFixture(t)

	t.Run("table: remedy line on every gap, none on a live target", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := cmdCertifyGaps([]string{"--service", service, "--store", storePath}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("certify gaps exit = %d, stderr: %s", code, errBuf.String())
		}
		got := out.String()
		for _, want := range []string{"invariant  I1", "invariant  I2", "incident   INC-1"} {
			if !strings.Contains(got, want) {
				t.Errorf("table missing %q: %q", want, got)
			}
		}
		if !strings.Contains(got, "add \"// certifies: invariant I2 (asserts)\"") {
			t.Errorf("table missing I2's remedy line: %q", got)
		}
		if !strings.Contains(got, "add \"// certifies: incident INC-1 (asserts)\"") {
			t.Errorf("table missing INC-1's remedy line: %q", got)
		}
		// I1 is live: its row must carry no remedy text after "no".
		for _, ln := range strings.Split(got, "\n") {
			if strings.HasPrefix(ln, "invariant") && strings.Contains(ln, "I1") {
				if strings.Contains(ln, "certifies:") {
					t.Errorf("I1 is live and must carry no remedy: %q", ln)
				}
			}
		}
	})

	t.Run("--json emits the exact fathom.certify.gaps/v1 shape", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := cmdCertifyGaps([]string{"--service", service, "--store", storePath, "--json"}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("certify gaps --json exit = %d, stderr: %s", code, errBuf.String())
		}
		got := redactCertifyGapsPayload(t, out.Bytes())
		if got != certifyGapsGolden {
			t.Fatalf("certify gaps --json mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, certifyGapsGolden)
		}
	})

	t.Run("--kind filters the target universe", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := cmdCertifyGaps([]string{"--service", service, "--store", storePath, "--kind", "incident", "--json"}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("certify gaps --kind incident exit = %d, stderr: %s", code, errBuf.String())
		}
		var p certifyGapsPayload
		if err := json.Unmarshal(out.Bytes(), &p); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(p.Targets) != 1 || p.Targets[0].Kind != "incident" || p.Targets[0].ID != "INC-1" {
			t.Fatalf("expected only INC-1, got %+v", p.Targets)
		}
		if len(p.Kinds) != 1 || p.Kinds[0] != "incident" {
			t.Errorf("kinds = %v, want [incident]", p.Kinds)
		}
	})
}
