package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests drive the public entrypoint with the independently reviewed source
// files. Passing them proves binding and refusal, not the reviewers' semantics.
func ownerFloorFiles(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"scope.json", "floor-review.json", "fixtures/review-floor-v1.json", "reviewers/reader-a.json", "reviewers/reader-b.json"} {
		b, err := os.ReadFile(filepath.Join("testdata/oracle-floor-v2", name))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	args := append(ownerOracleArgs("review-floor-report"),
		"--packet-digest", "f5fc178e7fea3042e3e5ff8f70aa8b00046891b8f13f9961703a03908d198e65",
		"--assessments", filepath.Join(dir, "reviewers/reader-a.json"),
		"--floor-review", filepath.Join(dir, "floor-review.json"),
		"--future-scope", filepath.Join(dir, "scope.json"),
		"--future-scope-digest", ownerOracleHash(b),
		"--fixtures", filepath.Join(dir, "fixtures"),
		"--review-dir", filepath.Join(dir, "reviewers"))
	return dir, args
}

func ownerFloorMutate(t *testing.T, path string, change func(map[string]any)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := ownerOracleJSON(t, b)
	change(m)
	b, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func ownerFloorRebindFixture(t *testing.T, dir string, args []string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "fixtures/review-floor-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	ownerFloorMutate(t, filepath.Join(dir, "scope.json"), func(m map[string]any) {
		m["fixtures"].([]any)[0].(map[string]any)["sha256"] = ownerOracleHash(b)
	})
	b, err = os.ReadFile(filepath.Join(dir, "scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	pin := ownerOracleHash(b)
	ownerFloorMutate(t, filepath.Join(dir, "floor-review.json"), func(m map[string]any) { m["future_scope_digest"] = pin })
	for i := range args {
		if args[i] == "--future-scope-digest" {
			args[i+1] = pin
		}
	}
}

func TestOwnerFloorCLIReviewedDataAndPrivacy(t *testing.T) {
	_, args := ownerFloorFiles(t)
	code, b, errOut := ownerOracleRun(context.Background(), args)
	if code != 0 || len(errOut) != 0 {
		t.Fatalf("ready=%d %s", code, errOut)
	}
	m := ownerOracleJSON(t, b)
	if m["declared_floor_ready"] != true {
		t.Fatal("reviewed receipt not structurally ready")
	}
	counts := m["counts"].(map[string]any)
	if counts["old_cases"] != float64(70) || counts["prospective_cases"] != float64(85) || counts["contrasts"] != float64(7) || counts["reviewers"] != float64(2) {
		t.Fatalf("coverage %v", counts)
	}
	for _, key := range []string{"provider_qualified", "identity_authenticated", "publication_authority", "new_study_admission"} {
		if v, ok := m[key].(bool); !ok || v {
			t.Fatalf("authority %s=%v", key, m[key])
		}
	}
	for _, forbidden := range []string{"assertion", "facts", "basis", "exposure", "content", "reader-a", "operational-runbook", "/Volumes/", "/Users/"} {
		if bytes.Contains(b, []byte(forbidden)) {
			t.Fatalf("default output exposed %q", forbidden)
		}
	}
	code2, again, _ := ownerOracleRun(context.Background(), args)
	if code2 != 0 || !bytes.Equal(b, again) {
		t.Fatal("nondeterministic report")
	}
	code, full, errOut := ownerOracleRun(context.Background(), append(args, "--include-content"))
	if code != 0 || !bytes.Contains(full, []byte(`"content":`)) || len(errOut) != 0 {
		t.Fatalf("explicit content=%d %s", code, errOut)
	}
	if ownerOracleJSON(t, full)["future_scope_digest"] != m["future_scope_digest"] {
		t.Fatal("presentation changed scope binding")
	}
}

func TestOwnerFloorCLIHeldAndRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*testing.T, string, []string)
		want int
	}{
		{"held", func(t *testing.T, d string, _ []string) {
			ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) { m["verdict"] = "held" })
		}, 1},
		{"missing-old-negative", func(t *testing.T, d string, _ []string) {
			ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) { m["old_case_dispositions"] = m["old_case_dispositions"].([]any)[1:] })
		}, 2},
		{"duplicate-old-case", func(t *testing.T, d string, _ []string) {
			ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) { a := m["old_case_dispositions"].([]any); a[0] = a[1] })
		}, 2},
		{"wrong-report-pin", func(t *testing.T, d string, _ []string) {
			ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) { m["assessment_report_digest"] = strings.Repeat("0", 64) })
		}, 2},
		{"missing-contrast-side", func(t *testing.T, d string, _ []string) {
			ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) {
				a := m["contrasts"].([]any)[0].(map[string]any)
				a["cases"] = a["cases"].([]any)[:1]
			})
		}, 2},
		{"forged-reviewer", func(t *testing.T, d string, _ []string) {
			ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) { m["reviewers"].([]any)[0].(map[string]any)["ref"] = "unregistered-reader" })
		}, 2},
		{"changed-source-stale-case-review", func(t *testing.T, d string, a []string) {
			ownerFloorMutate(t, filepath.Join(d, "fixtures/review-floor-v1.json"), func(m map[string]any) {
				m["cases"].([]any)[0].(map[string]any)["facts"].([]any)[0].(map[string]any)["text"] = "A changed synthetic source requires re-review."
			})
			ownerFloorRebindFixture(t, d, a)
		}, 2},
		{"missing-fact-required-literal", func(t *testing.T, d string, a []string) {
			ownerFloorMutate(t, filepath.Join(d, "fixtures/review-floor-v1.json"), func(m map[string]any) {
				delete(m["cases"].([]any)[0].(map[string]any)["facts"].([]any)[0].(map[string]any), "required")
			})
			ownerFloorRebindFixture(t, d, a)
		}, 2},
		{"null-fact-required-literal", func(t *testing.T, d string, a []string) {
			ownerFloorMutate(t, filepath.Join(d, "fixtures/review-floor-v1.json"), func(m map[string]any) {
				m["cases"].([]any)[0].(map[string]any)["facts"].([]any)[0].(map[string]any)["required"] = nil
			})
			ownerFloorRebindFixture(t, d, a)
		}, 2},
		{"symlink-fixture", func(t *testing.T, d string, _ []string) {
			p := filepath.Join(d, "fixtures/review-floor-v1.json")
			if err := os.Rename(p, p+".original"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(p+".original", p); err != nil {
				t.Fatal(err)
			}
		}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, args := ownerFloorFiles(t)
			tc.edit(t, d, args)
			code, b, errOut := ownerOracleRun(context.Background(), args)
			if code != tc.want {
				t.Fatalf("code=%d want=%d %s", code, tc.want, errOut)
			}
			if tc.want == 2 && (len(b) != 0 || bytes.Contains(errOut, []byte(d))) {
				t.Fatalf("invalid output leaked content or input path: %s %s", b, errOut)
			}
			if tc.want == 1 && ownerOracleJSON(t, b)["declared_floor_ready"] != false {
				t.Fatal("held claimed ready")
			}
		})
	}
	for _, key := range []string{"provider_qualified", "identity_authenticated", "publication_authority", "new_study_admission"} {
		for _, mode := range []string{"omit", "null", "true"} {
			t.Run(key+"/"+mode, func(t *testing.T) {
				d, args := ownerFloorFiles(t)
				ownerFloorMutate(t, filepath.Join(d, "floor-review.json"), func(m map[string]any) {
					switch mode {
					case "omit":
						delete(m, key)
					case "null":
						m[key] = nil
					case "true":
						m[key] = true
					}
				})
				code, b, _ := ownerOracleRun(context.Background(), args)
				if code != 2 || len(b) != 0 {
					t.Fatalf("authority literal admitted: code=%d", code)
				}
			})
		}
	}
}

func TestOwnerFloorCLIHelpCancellationAndWriterFailure(t *testing.T) {
	code, b, errOut := ownerOracleRun(context.Background(), []string{"review-floor-report", "--help"})
	if code != 0 || len(errOut) != 0 || !bytes.Contains(b, []byte("--future-scope-digest")) {
		t.Fatalf("help=%d %s", code, errOut)
	}
	_, args := ownerFloorFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, b, _ = ownerOracleRun(ctx, args)
	if code != 2 || len(b) != 0 {
		t.Fatal("canceled command delivered result")
	}
	var errors bytes.Buffer
	if code = runLab(context.Background(), args, ownerOracleBadWriter{}, &errors); code != 2 {
		t.Fatalf("broken writer=%d", code)
	}
	for _, flag := range []string{"--live", "--api-key", "--journal", "--variant"} {
		code, b, _ = ownerOracleRun(context.Background(), append(args, flag, "unused"))
		if code != 2 || len(b) != 0 {
			t.Fatalf("effect flag admitted: %s", flag)
		}
	}
}
