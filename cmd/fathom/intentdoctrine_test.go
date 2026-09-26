package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDoctrineFixture adds a vision series (one accepted and planned, one
// naming nothing) and a scope with one bet and one refused goal to the
// shared intent-corpus fixture.
func writeDoctrineFixture(t *testing.T, root string) {
	t.Helper()
	writeCorpusFixture(t, root)
	files := map[string]string{
		// The router's docs map names the vision home, as this repo's does.
		"docs/intent/README.md": "# Project intent — router\n\n## Docs map\n\n| Question | Read |\n|---|---|\n" +
			"| Decisions | `../../ADR.md` |\n" +
			"| Design records and the vision series | `../vision/README.md` (status per doc) |\n",
		"docs/plans/alpha.md": "# alpha\n\n- [x] A-1 one {id: A-1, symbol: X, package: p}\n- [ ] A-2 two {id: A-2}\n",
		"docs/vision/README.md": "# Visions\n\n" +
			"1. **[01-planned.md](01-planned.md)** — ACCEPTED 2026-09-20 (ADR-001).\n" +
			"2. **[02-bare.md](02-bare.md)** — a promise with no status.\n",
		"docs/vision/01-planned.md": "# 01 — Planned\n\nStatus: accepted 2026-09-20. Plan: `docs/plans/alpha.md`.\n\n## 1\n",
		"docs/vision/02-bare.md":    "# 02 — Bare\n\nNothing stated.\n\n## 1\n",
		"docs/intent/scope.md": "# Scope\n\n## Current goals\n\n" +
			"**G1. The arm bench.** Done when: 20 units.\n" +
			"{bet: arm-bench, instrument: \"fathom flywheel\", threshold: \"≥20 units per arm\", review: 2099-01-01}\n\n" +
			"**G2. An adjective.** Done when: things are better.\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCmdIntentDoctrine_ListsVisionsAndBets (DP-3, DP-4): one verb shows
// where each vision stands in the promotion pipeline and every goal as a
// bet or a refusal — per vision, per goal, each finding with its remedy.
func TestCmdIntentDoctrine_ListsVisionsAndBets(t *testing.T) {
	root := t.TempDir()
	writeDoctrineFixture(t, root)
	beats := filepath.Join(root, "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beats)

	var out, errBuf bytes.Buffer
	if code := cmdIntent([]string{"doctrine", "--root", root}, &out, &errBuf); code != 0 {
		t.Fatalf("doctrine exit %d: %s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{
		"visions: 2",
		"01", "planned", "docs/plans/alpha.md", "1/2 claimed",
		"02", "unstated",
		"goals: 2 — 1 bet(s), 1 refused",
		"G1", "arm-bench", "fathom flywheel", "2099-01-01",
		"G2", "refused",
		"→", // every finding carries its remedy
	} {
		if !strings.Contains(got, want) {
			t.Errorf("doctrine output missing %q:\n%s", want, got)
		}
	}

	// Law 7: the verb's use is measurable without a daemon.
	b, err := os.ReadFile(beats)
	if err != nil || !strings.Contains(string(b), `"doctrine"`) {
		t.Errorf("no intent-cli/doctrine beat recorded (%v): %s", err, b)
	}

	// --json carries the same facts, per vision and per goal.
	out.Reset()
	if code := cmdIntent([]string{"doctrine", "--root", root, "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("doctrine --json exit %d: %s", code, errBuf.String())
	}
	var payload struct {
		VisionsHome string `json:"visions_home"`
		Visions     []struct {
			File  string `json:"file"`
			Stage string `json:"stage"`
		} `json:"visions"`
		Goals []struct {
			ID      string `json:"id"`
			Refused string `json:"refused"`
		} `json:"goals"`
		Findings []struct {
			Kind   string `json:"kind"`
			Remedy string `json:"remedy"`
		} `json:"findings"`
		Coverage struct {
			Complete bool `json:"complete"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out.String())
	}
	if payload.VisionsHome != "docs/vision" || len(payload.Visions) != 2 || len(payload.Goals) != 2 {
		t.Errorf("json = %+v", payload)
	}
	if !payload.Coverage.Complete {
		t.Error("a readable fixture must report complete coverage")
	}
	kinds := map[string]bool{}
	for _, f := range payload.Findings {
		kinds[f.Kind] = true
		if f.Remedy == "" {
			t.Errorf("finding without remedy: %+v", f)
		}
	}
	for _, k := range []string{"unstated", "refused"} {
		if !kinds[k] {
			t.Errorf("json findings lack kind %q: %+v", k, payload.Findings)
		}
	}
}

// TestCheckDoctrine_WarnsNeverFails (DP-3, DP-4): the doctor row reports the
// pipeline and the bets, warns on findings, and never FAILs — an unstated
// vision or a refused goal is honest debt, not a broken repository.
func TestCheckDoctrine_WarnsNeverFails(t *testing.T) {
	t.Run("findings warn", func(t *testing.T) {
		root := t.TempDir()
		writeDoctrineFixture(t, root)
		got := checkDoctrine(root)
		if got.Name != "doctrine" || got.Status != doctorWarn {
			t.Fatalf("want a doctrine WARN, got %+v", got)
		}
		for _, want := range []string{"2 vision(s)", "1 unstated", "1 bet(s)", "1 refused"} {
			if !strings.Contains(got.Reason, want) {
				t.Errorf("reason %q lacks %q", got.Reason, want)
			}
		}
		if !strings.Contains(got.Remedy, "fathom intent doctrine") {
			t.Errorf("remedy must point at the verb: %q", got.Remedy)
		}
	})
	t.Run("this repository", func(t *testing.T) {
		got := checkDoctrine(repoRootForDocs(t))
		if got.Status == doctorFail {
			t.Fatalf("the doctrine row must never fail: %+v", got)
		}
		for _, want := range []string{"vision(s)", "G2 arm-bench", "G3 thread-bet"} {
			if !strings.Contains(got.Reason, want) {
				t.Errorf("reason lacks %q: %s", want, got.Reason)
			}
		}
		t.Logf("%s %s — %s", got.Name, got.Status, got.Reason)
	})
	t.Run("nothing adopted passes with the absence named", func(t *testing.T) {
		got := checkDoctrine(t.TempDir())
		if got.Status != doctorPass || !strings.Contains(got.Reason, "no vision") {
			t.Errorf("an empty repo is a named absence, not a failure: %+v", got)
		}
	})
}
