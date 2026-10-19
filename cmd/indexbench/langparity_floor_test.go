package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func TestWriteParityShowsFloorsOnlyOnRefusal(t *testing.T) {
	inc := langbench.Reading{Dialect: "example", Kind: "tooling", LevelMet: "L1",
		Tenancy:  langbench.Tenancy{ThroughputMBs: 2.5, ThroughputBasis: "cpu", MaxFileMS: 12},
		Subjects: []langbench.Subject{{Files: 1, Bytes: 1024}}}
	dial := inc
	dial.LevelMet = langbench.LevelNone
	dial.Tenancy.ThroughputMBs = 0.5
	var out bytes.Buffer
	writeParity(&out, langbench.ComparePortParity(inc, dial, "old", "L3", nil, nil, nil))
	for _, want := range []string{"incumbent floor: ", "dialect floor: ", `"throughput_mb_s":0.5`, `"throughput_basis":"cpu"`, `"files":1`, "verdict: REFUSED"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing original floor %q in refusal:\n%s", want, out.String())
		}
	}
	out.Reset()
	writeParity(&out, langbench.ComparePortParity(inc, inc, "old", "L3", nil, nil, nil))
	if strings.Contains(out.String(), " floor: ") || !strings.Contains(out.String(), "verdict: MERGE") {
		t.Fatalf("passing output or decision changed:\n%s", out.String())
	}
}

func TestCmdLangParityRefusalShowsMeasuredFloors(t *testing.T) {
	// Only this negative fixture injects the existing clock seam. The original
	// TestCmdLangParity and production measurement clocks stay unchanged.
	stubFileClocks(t, time.Second, time.Second, true)
	subject := t.TempDir()
	if err := os.WriteFile(filepath.Join(subject, "pyproject.toml"), []byte("[project]\nname = \"tiny\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	if err := os.WriteFile(oracle, []byte(`{"tool":"tomllib","version":"3","tier":"C","definitions":[{"path":"pyproject.toml","name":"tiny","line":1,"kind":"package"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "refusal.json")
	args := []string{"-dialect", "python-packaging", "-manifest", "../../internal/extractor/dialects/python-packaging/dialect.json",
		"-incumbent", "tomlextract", "-subject", "fixture=" + subject, "-tool-oracle", "fixture=" + oracle,
		"-target", "L3", "-kind-explained", "TOMLTable=the dialect names the package", "-out", report}
	var stdout, stderr bytes.Buffer
	if code := cmdLangParity(args, &stdout, &stderr); code != 1 {
		t.Fatalf("the genuine measured floor must refuse, exit %d:\n%s\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "dialect floor: ") || !strings.Contains(stdout.String(), `"throughput_basis":"cpu"`) {
		t.Fatalf("CLI refusal lost the actual floor inputs:\n%s", stdout.String())
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Dialect *struct {
			Tenancy struct {
				Throughput float64 `json:"throughput_mb_s"`
				Basis      string  `json:"throughput_basis"`
				Maximum    int64   `json:"max_file_ms"`
			} `json:"tenancy"`
			Files int     `json:"files"`
			CPU   float64 `json:"extract_cpu_ms"`
		} `json:"dialect_floor"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Dialect == nil || got.Dialect.Files != 1 || got.Dialect.CPU != 1000 ||
		got.Dialect.Tenancy.Maximum != 1000 || got.Dialect.Tenancy.Basis != "cpu" || got.Dialect.Tenancy.Throughput >= 1 {
		t.Fatalf("JSON did not preserve the measured refusal: %s", data)
	}
}
