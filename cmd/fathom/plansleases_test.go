package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestLeasesReportCountsOverlaps pins WF-7: `fathom plans leases` lists
// active/expired leases for the repo and counts OVERLAPS — rows where ≥2
// holders are simultaneously fresh — the collision instrument this
// feature exists to surface.
func TestLeasesReportCountsOverlaps(t *testing.T) {
	root := t.TempDir()

	// Two DIFFERENT holders both lease A-1 without seeing each other's
	// claim: an overlap. B-1 has a single fresh holder: no overlap. C-1's
	// only lease has already expired: neither fresh nor an overlap.
	for _, args := range [][]string{
		{"plans", "lease", "A-1", "--plan", "p.md", "--holder", "sess-1", "--ttl", "2h", "--root", root},
		{"plans", "lease", "A-1", "--plan", "p.md", "--holder", "sess-2", "--ttl", "2h", "--root", root},
		{"plans", "lease", "B-1", "--plan", "p.md", "--holder", "sess-3", "--ttl", "2h", "--root", root},
		{"plans", "lease", "C-1", "--plan", "p.md", "--holder", "sess-4", "--ttl", "-1h", "--root", root},
	} {
		var out, errBuf bytes.Buffer
		if code := run(args, &out, &errBuf); code != 0 {
			t.Fatalf("plans lease %v exit = %d, stderr: %s", args, code, errBuf.String())
		}
	}

	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "leases", "--root", root, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans leases exit = %d, stderr: %s", code, errBuf.String())
	}

	var payload struct {
		Leases []struct {
			Plan   string `json:"plan"`
			RowID  string `json:"row_id"`
			Holder string `json:"holder"`
			Fresh  bool   `json:"fresh"`
		} `json:"leases"`
		Overlaps int `json:"overlaps"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("plans leases --json not valid JSON: %v\n%s", err, out.String())
	}
	if payload.Overlaps != 1 {
		t.Errorf("overlaps = %d, want 1 (A-1 has two fresh holders)", payload.Overlaps)
	}
	if len(payload.Leases) != 4 {
		t.Fatalf("leases = %d, want 4 (one row per (plan,row_id,holder) triple), got %+v", len(payload.Leases), payload.Leases)
	}
	freshCount := 0
	for _, l := range payload.Leases {
		if l.Fresh {
			freshCount++
		}
	}
	if freshCount != 3 {
		t.Errorf("fresh leases = %d, want 3 (A-1 x2, B-1 x1; C-1 expired)", freshCount)
	}

	// Human table also reports the overlap count.
	var human, humanErr bytes.Buffer
	if code := run([]string{"plans", "leases", "--root", root}, &human, &humanErr); code != 0 {
		t.Fatalf("plans leases (human) exit = %d, stderr: %s", code, humanErr.String())
	}
	if !bytes.Contains(human.Bytes(), []byte("overlaps: 1")) {
		t.Errorf("human output missing overlap count:\n%s", human.String())
	}
}
