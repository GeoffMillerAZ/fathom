package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/findings/external"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// A caller's explicit subject/producer flags must not bypass a configured
// producer boundary. Exercise the actual CLI and store, not a fake runner.
func TestFindingsImportConfiguredProducers(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(findingsImportFixtureDir(t), "criteria-import.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture findingsImportFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, declaration, producer string
		delivered, rejected         int
		invalidPolicy               bool
	}{
		{"omitted preserves legacy", "", "", 3, 0, false},
		{"empty denies every producer", "producers = []", "", 0, 3, false},
		{"one allowed producer", `producers = ["reviewkit.criteria-import.model"]`, "", 1, 2, false},
		{"mixed allowed producers", `producers = ["reviewkit.criteria-import.model", "reviewkit.criteria-import.human", "reviewkit.criteria-import.deterministic"]`, "", 3, 0, false},
		{"explicit flag cannot widen config", `producers = ["reviewkit.criteria-import.model"]`, "reviewkit.criteria-import.human", 0, 3, false},
		{"malformed policy refuses despite explicit subject", `producers = [`, "", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FATHOM_HOME", t.TempDir())
			root := writeConformanceCorpus(t)
			if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
				t.Fatal(err)
			}
			config := "[findings]\nsubject = \"acme/widget/api\"\n" + tc.declaration + "\n"
			if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			storePath := filepath.Join(t.TempDir(), "findings.sqlite")
			seedConformanceStore(t, storePath, fixture.Service, root)
			outbox := filepath.Join(t.TempDir(), "projection.json")
			if err := os.WriteFile(outbox, fixture.Projection, 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--store", storePath, "--service", fixture.Service, "--subject", fixture.Subject, "--file", outbox, "--json"}
			if tc.producer != "" {
				args = append(args, "--producer", tc.producer)
			}
			var stdout, stderr bytes.Buffer
			code := cmdFindingsImport(args, &stdout, &stderr)
			wantCode := 0
			if tc.rejected != 0 {
				wantCode = 1
			}
			if !tc.invalidPolicy && code != wantCode {
				t.Fatalf("exit=%d, want %d; stdout=%s stderr=%s", code, wantCode, &stdout, &stderr)
			}
			if tc.invalidPolicy {
				if code != 2 || !strings.Contains(stderr.String(), "load findings configuration") {
					t.Fatalf("invalid policy exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
				}
				if stdout.Len() != 0 {
					t.Fatalf("invalid policy returned a success receipt: %s", &stdout)
				}
				return
			}
			var receipt external.Receipt
			if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
				t.Fatalf("receipt: %v; %s", err, &stdout)
			}
			if receipt.Delivered != tc.delivered || len(receipt.Rejected) != tc.rejected {
				t.Fatalf("receipt=%+v; want delivered=%d rejected=%d", receipt, tc.delivered, tc.rejected)
			}
			if tc.producer == "" {
				for _, rejection := range receipt.Rejected {
					if rejection.Reason != "producer-not-allowed" {
						t.Errorf("rejection=%+v, want named producer-not-allowed", rejection)
					}
				}
			}
			st, err := store.Open(storePath)
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			claims, err := st.ClaimsForService(context.Background(), fixture.Service, "findings")
			if err != nil || len(claims) != tc.delivered {
				t.Fatalf("claims=%+v, err=%v; disallowed producer must write nothing", claims, err)
			}
			if tc.delivered == 1 && !strings.Contains(claims[0].Subject, "reviewkit.criteria-import.model/") {
				t.Fatalf("disallowed claim written: %+v", claims[0])
			}
		})
	}
}
