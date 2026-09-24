package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdTestsStatus_JoinRateAndNoManifests(t *testing.T) {
	t.Run("no manifests found is a named skip, never a clean table", func(t *testing.T) {
		storePath := filepath.Join(t.TempDir(), "s.sqlite")
		var ingestOut, ingestErr bytes.Buffer
		if code := run([]string{"ingest", "--service", "empty-svc", "--path", t.TempDir(), "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
			t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
		}

		var out, errBuf bytes.Buffer
		code := cmdTestsStatus([]string{"--service", "empty-svc", "--store", storePath}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("tests status exit = %d, stderr: %s", code, errBuf.String())
		}
		if !strings.Contains(out.String(), "no manifests found") {
			t.Fatalf("want a named skip for a service with no declared suites, got %q", out.String())
		}
	})

	t.Run("prints class, tier, last-run verdict and the join rate", func(t *testing.T) {
		root, storePath, service := setUpTestsIngestFixture(t)

		resultsPath := filepath.Join(t.TempDir(), "run.json")
		if err := os.WriteFile(resultsPath, []byte(testsIngestFixtureEnvelope("2026-09-23T00:00:00Z")), 0o644); err != nil {
			t.Fatal(err)
		}
		var ingestOut, ingestErr bytes.Buffer
		if code := cmdTestsIngest([]string{"--service", service, "--store", storePath, "--manifest", root, "--results", resultsPath}, &ingestOut, &ingestErr); code != 0 {
			t.Fatalf("tests ingest exit = %d, stderr: %s", code, ingestErr.String())
		}

		var out, errBuf bytes.Buffer
		code := cmdTestsStatus([]string{"--service", service, "--store", storePath}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("tests status exit = %d, stderr: %s", code, errBuf.String())
		}
		got := out.String()
		if !strings.Contains(got, "cart/unit") || !strings.Contains(got, "unit") || !strings.Contains(got, "blocking") {
			t.Fatalf("table missing class/tier: %q", got)
		}
		if !strings.Contains(got, "pass") {
			t.Fatalf("table missing last-run verdict: %q", got)
		}
		if !strings.Contains(got, "joined 1 of 2 cases") {
			t.Fatalf("table missing the join rate: %q", got)
		}

		var jsonOut, jsonErr bytes.Buffer
		jcode := cmdTestsStatus([]string{"--service", service, "--store", storePath, "--json"}, &jsonOut, &jsonErr)
		if jcode != 0 {
			t.Fatalf("tests status --json exit = %d, stderr: %s", jcode, jsonErr.String())
		}
		var payload struct {
			Service string `json:"service"`
			Suites  []struct {
				Component string `json:"component"`
				Suite     string `json:"suite"`
				Joined    int    `json:"joined"`
				Total     int    `json:"total"`
				Verdict   string `json:"verdict"`
			} `json:"suites"`
		}
		if err := json.Unmarshal(jsonOut.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal --json output: %v (%s)", err, jsonOut.String())
		}
		if len(payload.Suites) != 1 || payload.Suites[0].Joined != 1 || payload.Suites[0].Total != 2 || payload.Suites[0].Verdict != "pass" {
			t.Fatalf("--json payload = %+v", payload)
		}
	})
}
