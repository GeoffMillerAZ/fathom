package main

// verify_wired_test.go — "done means proven AND wired" end to end (ADR-018
// amendment 2026-09-28): a fixture service with one binary is ingested for
// real and a fixture plan verified with `--strict --store`. The row whose
// symbol only a test reaches fails the gate as library-only; the row the
// binary calls and the row that declares `library: true` with a reason pass.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func wiredFixtureStore(t *testing.T) (storePath, service string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		// A go.mod, so the typed pass runs: wired passes on typed edges
		// only (RF-5, I57).
		"go.mod":          "module wiredfix\n\ngo 1.22\n",
		"cmd/app/main.go": "package main\n\nimport \"wiredfix/lib\"\n\nfunc main() {\n\t_ = lib.Wired()\n}\n",
		"lib/lib.go": "package lib\n\n// Wired is what the binary calls.\nfunc Wired() int { return 1 }\n\n" +
			"// TestOnly is reached by a test and by nothing the binary runs.\nfunc TestOnly() int { return 2 }\n\n" +
			"// API is reached by a test only, and its row says the API is the deliverable.\nfunc API() int { return 3 }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestLib(t *testing.T) {\n\t_ = Wired()\n\t_ = TestOnly()\n\t_ = API()\n}\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	storePath = filepath.Join(t.TempDir(), "wired.sqlite")
	service = "wiredfix"
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return storePath, service
}

const (
	planWiredRow    = "- [x] the binary calls it {id: W-1, symbol: Wired, package: lib}\n"
	planTestOnlyRow = "- [x] only a test calls it {id: W-2, symbol: TestOnly, package: lib}\n"
	planLibraryRow  = "- [x] the API is the deliverable {id: W-3, symbol: API, package: lib, library: true, reason: \"a fixture library: the API is the deliverable\"}\n"
)

func TestCmdVerify_Strict_TestOnlyRowIsLibraryOnly(t *testing.T) {
	storePath, service := wiredFixtureStore(t)
	pf := writeTempPlan(t, planWiredRow+planTestOnlyRow+planLibraryRow)

	var out, errBuf bytes.Buffer
	code := run([]string{"verify", "--strict", "--service", service, "--plan", pf, "--store", storePath, "--format", "json"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("--strict with a test-only row: want exit 1, got %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
	var doc struct {
		Gate struct {
			Failures []string `json:"failures"`
		} `json:"gate"`
		Reports []struct {
			Tasks []struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Label    string `json:"label"`
				Criteria []struct {
					Name, Status, Detail string
				} `json:"criteria"`
			} `json:"tasks"`
		} `json:"reports"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if len(doc.Gate.Failures) != 1 || !strings.HasPrefix(doc.Gate.Failures[0], "W-2 ") {
		t.Fatalf("gate failures = %q, want exactly W-2", doc.Gate.Failures)
	}
	want := map[string][2]string{
		"W-1": {"pass", "wired"},
		"W-2": {"unknown", "wired"},
		"W-3": {"pass", "library"},
	}
	for _, tv := range doc.Reports[0].Tasks {
		w, ok := want[tv.ID]
		if !ok {
			continue
		}
		found := false
		for _, c := range tv.Criteria {
			if c.Name == w[1] {
				found = c.Status == "pass" || tv.ID == "W-2" && c.Status == "unknown"
			}
		}
		if tv.Status != w[0] || !found {
			t.Errorf("%s: status %s label %s criteria %+v, want %s with a %s criterion", tv.ID, tv.Status, tv.Label, tv.Criteria, w[0], w[1])
		}
		if tv.ID == "W-2" && tv.Label != "library-only" {
			t.Errorf("W-2 label = %q, want library-only", tv.Label)
		}
	}

	// The default gate reports it without refusing it: library-only is
	// Unknown, and only --strict (the pre-push gate) blocks on Unknown.
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"verify", "--service", service, "--plan", pf, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Errorf("default gate: want exit 0, got %d\n%s%s", code, out.String(), errBuf.String())
	}
}

func TestCmdVerify_Strict_WiredAndLibraryRowsPass(t *testing.T) {
	storePath, service := wiredFixtureStore(t)
	pf := writeTempPlan(t, planWiredRow+planLibraryRow)
	var out, errBuf bytes.Buffer
	if code := run([]string{"verify", "--strict", "--service", service, "--plan", pf, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("--strict with a wired row and a library row: want exit 0, got %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
}
