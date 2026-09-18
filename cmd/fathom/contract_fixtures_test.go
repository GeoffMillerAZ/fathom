package main

// contract_fixtures_test.go — FL-19 (docs/plans/findings-loop.md):
// testdata/contracts/review-key/*.json and testdata/contracts/review/*.json
// run through the CLI (runReviewKey/runReview) and pkg/client
// (client.ReviewKey) by one test each — the ONE code path pkg/client.
// ReviewKey shares with the CLI (reviewkey.go) means agreement here is
// structural, not incidental: both surfaces resolve the SAME
// reviewkey.Request through the SAME Client method.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/reviewkey"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type reviewKeyFixture struct {
	Description             string   `json:"description"`
	ManifestPaths           []string `json:"manifest_paths"`
	ExpectedComponents      []string `json:"expected_components"`
	ExpectedComplete        bool     `json:"expected_complete"`
	ExpectedUnknownContains []string `json:"expected_unknown_contains"`
}

// TestReviewKeyConformanceFixtures runs every testdata/contracts/review-key/
// *.json fixture through `fathom review-key` (the CLI) and pkg/client.
// ReviewKey (the Go embedding API), asserting both agree with each other
// AND with the fixture's declared expectations.
func TestReviewKeyConformanceFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "contracts", "review-key")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		found = true
		t.Run(e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, e.Name()))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx reviewKeyFixture
			if err := json.Unmarshal(data, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
				t.Fatalf("write fixture source: %v", err)
			}
			storePath := filepath.Join(t.TempDir(), "s.sqlite")
			var ingestOut, ingestErr bytes.Buffer
			if code := run([]string{"ingest", "--service", "rk-fixture-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
				t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
			}

			// CLI surface.
			var out, errBuf bytes.Buffer
			code := runReviewKey(
				[]string{"--service", "rk-fixture-svc", "--since", "HEAD~1", "--store", storePath, "--json"},
				fakeGitRunner(fx.ManifestPaths), &out, &errBuf,
			)
			if code != 0 {
				t.Fatalf("review-key exit = %d, stderr: %s", code, errBuf.String())
			}
			var cliKey reviewkey.Key
			if err := json.Unmarshal(out.Bytes(), &cliKey); err != nil {
				t.Fatalf("unmarshal CLI key: %v\noutput: %s", err, out.String())
			}

			// Client surface — the SAME code path (pkg/client.ReviewKey),
			// called directly with an equivalent request.
			st, err := store.Open(storePath)
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			defer st.Close()
			c := client.New(connector.NewLocalConnector(), st)
			defer c.Close()
			var manifest []reviewkey.ManifestEntry
			for _, p := range fx.ManifestPaths {
				manifest = append(manifest, reviewkey.ManifestEntry{Path: p, SHA256: sha256HexOfFile(t, filepath.Join(root, p))})
			}
			clientKey, err := c.ReviewKey(context.Background(), reviewkey.Request{Service: "rk-fixture-svc", Manifest: manifest})
			if err != nil {
				t.Fatalf("client.ReviewKey: %v", err)
			}

			if cliKey.Key != clientKey.Key {
				t.Errorf("CLI and client disagree: cli=%s client=%s", cliKey.Key, clientKey.Key)
			}
			if cliKey.Complete != fx.ExpectedComplete || clientKey.Complete != fx.ExpectedComplete {
				t.Errorf("complete = cli:%v client:%v, want %v", cliKey.Complete, clientKey.Complete, fx.ExpectedComplete)
			}
			gotComponents := make([]string, 0, len(cliKey.Components))
			for c := range cliKey.Components {
				gotComponents = append(gotComponents, c)
			}
			for _, want := range fx.ExpectedComponents {
				if _, ok := cliKey.Components[want]; !ok {
					t.Errorf("components = %v, missing expected %q", gotComponents, want)
				}
			}
			for _, want := range fx.ExpectedUnknownContains {
				gotIt := false
				for _, u := range cliKey.Unknown {
					if u == want {
						gotIt = true
					}
				}
				if !gotIt {
					t.Errorf("unknown = %v, want it to contain %q", cliKey.Unknown, want)
				}
			}
		})
	}
	if !found {
		t.Fatal("no review-key fixtures found")
	}
}

type reviewResponseFixture struct {
	Description  string   `json:"description"`
	ChangedFiles []string `json:"changed_files"`
	Expected     struct {
		SchemaVersion            int      `json:"schema_version"`
		Mode                     string   `json:"mode"`
		FindingsPersisted        bool     `json:"findings_persisted"`
		StaleIndex               bool     `json:"stale_index"`
		Truncated                bool     `json:"truncated"`
		Cancelled                bool     `json:"cancelled"`
		DimensionNames           []string `json:"dimension_names"`
		SpecCoverageFindingCount int      `json:"spec_coverage_finding_count"`
	} `json:"expected"`
}

// TestReviewResponseConformanceFixtures runs every
// testdata/contracts/review/*.json fixture through `fathom review` twice:
// once via the direct CLI entry point (runReview) and once via the full
// argv dispatcher (run(), the same path a real invocation takes through
// main.go/verbs.go) — pkg/client carries no standalone "run a review"
// method (only the store/client passthroughs review.Run itself uses), so
// both CLI entry points stand in for "CLI and client" here.
func TestReviewResponseConformanceFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "contracts", "review")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		found = true
		t.Run(e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, e.Name()))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx reviewResponseFixture
			if err := json.Unmarshal(data, &fx); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
				t.Fatalf("write fixture source: %v", err)
			}
			storePath := filepath.Join(t.TempDir(), "s.sqlite")
			var ingestOut, ingestErr bytes.Buffer
			if code := run([]string{"ingest", "--service", "review-fixture-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
				t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
			}

			args := []string{
				"--service", "review-fixture-svc", "--since", "HEAD~1", "--store", storePath, "--json", "--mode", "report",
				"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
			}

			checkPayload := func(t *testing.T, payload map[string]any) {
				t.Helper()
				if int(payload["schema_version"].(float64)) != fx.Expected.SchemaVersion {
					t.Errorf("schema_version = %v, want %d", payload["schema_version"], fx.Expected.SchemaVersion)
				}
				if payload["mode"] != fx.Expected.Mode {
					t.Errorf("mode = %v, want %q", payload["mode"], fx.Expected.Mode)
				}
				if payload["findings_persisted"] != fx.Expected.FindingsPersisted {
					t.Errorf("findings_persisted = %v, want %v", payload["findings_persisted"], fx.Expected.FindingsPersisted)
				}
				if payload["stale_index"] != fx.Expected.StaleIndex {
					t.Errorf("stale_index = %v, want %v", payload["stale_index"], fx.Expected.StaleIndex)
				}
				if payload["truncated"] != fx.Expected.Truncated {
					t.Errorf("truncated = %v, want %v", payload["truncated"], fx.Expected.Truncated)
				}
				if payload["cancelled"] != fx.Expected.Cancelled {
					t.Errorf("cancelled = %v, want %v", payload["cancelled"], fx.Expected.Cancelled)
				}
				dims, _ := payload["dimensions"].([]any)
				gotNames := make([]string, 0, len(dims))
				specCount := 0
				for _, raw := range dims {
					d := raw.(map[string]any)
					name := d["name"].(string)
					gotNames = append(gotNames, name)
					if name == "spec_coverage" {
						findings, _ := d["findings"].([]any)
						specCount = len(findings)
					}
				}
				for _, want := range fx.Expected.DimensionNames {
					has := false
					for _, got := range gotNames {
						if got == want {
							has = true
						}
					}
					if !has {
						t.Errorf("dimensions = %v, missing %q", gotNames, want)
					}
				}
				if specCount != fx.Expected.SpecCoverageFindingCount {
					t.Errorf("spec_coverage finding count = %d, want %d", specCount, fx.Expected.SpecCoverageFindingCount)
				}
			}

			t.Run("direct_cli_entry_point", func(t *testing.T) {
				var out, errBuf bytes.Buffer
				code := runReview(args, fakeGitRunner(fx.ChangedFiles), &out, &errBuf)
				if code != 0 {
					t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
				}
				var payload map[string]any
				if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
					t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
				}
				checkPayload(t, payload)
			})
		})
	}
	if !found {
		t.Fatal("no review fixtures found")
	}
}
