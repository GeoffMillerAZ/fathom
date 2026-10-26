package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A private diagnostic receipt must name actual tenant files, including files
// with no definitions or calls, so equal-size but different populations cannot
// masquerade as a paired oracle comparison.
func TestCmdLangOracleUnmatchedTenancyFiles(t *testing.T) {
	subject := tsFixtureSubject(t)
	if err := os.WriteFile(filepath.Join(subject, "aa-empty.ts"), []byte("// no calls or definitions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "ignored.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	oracle := filepath.Join(dir, "oracle.json")
	if err := os.WriteFile(oracle, []byte(`{"tool":"neutral","tier":"B","definitions":[],"scope":["main.ts"],"calls":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(dir, "unmatched.json")
	args := []string{"-dialect", "typescript", "-subject", subject,
		"-tool-oracle", oracle, "-ctags", filepath.Join(dir, "absent"),
		"-unmatched", receipt, "-out", filepath.Join(dir, "reading.json"),
		"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	data, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Files []string `json:"tenancy_files"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"aa-empty.ts", "greeter.ts", "main.ts", "math.ts"}
	if !reflect.DeepEqual(got.Files, want) {
		t.Fatalf("tenancy_files = %v, want exact sorted measured paths %v", got.Files, want)
	}
}
