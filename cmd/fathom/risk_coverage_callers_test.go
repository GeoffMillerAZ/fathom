package main

// risk_coverage_callers_test.go — TQ-26 (docs/plans/test-intelligence.md
// wave 5, ADR-067): every risk-rank caller reads coverage from the source
// the repository names and says which it read.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// lcovSubject is a small JavaScript repository whose `[tests] results_dir`
// holds an lcov report: an lcov subject.
func lcovSubject(t *testing.T, withReport bool) (root, storePath, service string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"src/math.js":         "export function add(a, b) {\n  return a + b;\n}\n\nexport function mul(a, b) {\n  let r = 0;\n  for (let i = 0; i < b; i++) {\n    r = add(r, a);\n  }\n  return r;\n}\n",
		"src/app.js":          "import { add, mul } from './math.js';\n\nexport function main() {\n  return mul(add(1, 2), 3);\n}\n",
		".fathom/config.toml": "[tests]\nresults_dir = \"coverage\"\n",
	}
	if withReport {
		files["coverage/lcov.info"] = "TN:\nSF:src/math.js\nDA:2,4\nDA:6,1\nDA:7,1\nDA:8,0\nDA:10,1\nend_of_record\nSF:src/app.js\nDA:4,0\nend_of_record\n"
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
	storePath = filepath.Join(t.TempDir(), "lcov.sqlite")
	service = "lcov-subject"
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return root, storePath, service
}

// TestRiskRankReadsNamedCoverageSourceInEveryCaller proves TQ-26 on an
// lcov subject: test_impact rank:"risk" (the MCP tool, input schema
// unchanged), the console's test portfolio read (the risk rank and the
// portfolio behind /tests) and the console's hotspots all read the lcov
// report the repository names under `[tests] results_dir`, answer
// coverage_supplied true and name the source; with no report named, each
// says so, as before. Coverage is read from the file, never a store.
func TestRiskRankReadsNamedCoverageSourceInEveryCaller(t *testing.T) {
	const wantSource = "lcov=coverage/lcov.info"
	for _, withReport := range []bool{true, false} {
		root, storePath, service := lcovSubject(t, withReport)
		c, err := client.NewConstantClient(client.ModeLocal, storePath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		ctx := context.Background()

		// 1. test_impact rank:"risk" (internal/mcp).
		srv := mcp.NewServer(c)
		resp, _ := srv.HandleRequest(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_impact","arguments":{"service":"`+service+`","rank":"risk"}}}`))
		var rpc struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resp, &rpc); err != nil || len(rpc.Result.Content) == 0 {
			t.Fatalf("test_impact: %s (%v)", resp, err)
		}
		var answer struct {
			Summary string `json:"summary"`
			Risk    struct {
				CoverageSupplied bool   `json:"coverage_supplied"`
				CoverageSource   string `json:"coverage_source"`
				Rows             []struct {
					Symbol  string `json:"symbol"`
					Factors []struct {
						Name, Cite string
						Value      float64
					} `json:"factors"`
				} `json:"rows"`
			} `json:"risk"`
		}
		if err := json.Unmarshal([]byte(rpc.Result.Content[0].Text), &answer); err != nil {
			t.Fatalf("test_impact answer: %v\n%s", err, rpc.Result.Content[0].Text)
		}
		if len(answer.Risk.Rows) == 0 {
			t.Fatalf("test_impact rank:risk ranked nothing: %s", rpc.Result.Content[0].Text)
		}

		// 2. The console's portfolio read: the risk rank and the portfolio.
		deps := newConsoleDeps(c, "http://127.0.0.1:1", time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
		sr := serviceRoot{Service: service, Root: root}
		r := readTestPortfolio(ctx, plansFor(deps), sr)
		if r.err != "" {
			t.Fatalf("readTestPortfolio: %s", r.err)
		}
		// 3. The console's hotspots.
		hs := workHotspots(ctx, deps, sr, r.risk, deps.now())

		if withReport {
			if !answer.Risk.CoverageSupplied || answer.Risk.CoverageSource != wantSource || !strings.Contains(answer.Summary, wantSource) {
				t.Errorf("test_impact: supplied %v source %q summary %q, want %s", answer.Risk.CoverageSupplied, answer.Risk.CoverageSource, answer.Summary, wantSource)
			}
			cited := false
			for _, row := range answer.Risk.Rows {
				if f := row.Factors[0]; f.Name == "uncovered_share" && strings.Contains(f.Cite, wantSource) && f.Value < 1 {
					cited = true
				}
			}
			if !cited {
				t.Errorf("test_impact: no row's uncovered_share reads the lcov report: %+v", answer.Risk.Rows)
			}
			if !r.risk.CoverageSupplied || r.risk.CoverageSource != wantSource {
				t.Errorf("console risk: supplied %v source %q", r.risk.CoverageSupplied, r.risk.CoverageSource)
			}
			if !r.portfolio.CoverageSupplied || r.portfolio.CoverageSource != wantSource {
				t.Errorf("portfolio: supplied %v source %q", r.portfolio.CoverageSupplied, r.portfolio.CoverageSource)
			}
			if !hs.CoverageSupplied || hs.CoverageSource != wantSource || !strings.Contains(hotspotsCoverageSource(hs), wantSource) {
				t.Errorf("hotspots: supplied %v source %q", hs.CoverageSupplied, hs.CoverageSource)
			}
			if page := renderTestPortfolio(testPortfolioModel{Service: service, Risk: &r.risk, Portfolio: &r.portfolio}); !strings.Contains(page, "Coverage read from "+wantSource) {
				t.Errorf("/tests does not name the coverage source")
			}
			continue
		}
		if answer.Risk.CoverageSupplied || r.risk.CoverageSupplied || r.portfolio.CoverageSupplied || hs.CoverageSupplied ||
			!strings.Contains(answer.Summary, "no coverage source named") || !strings.Contains(hotspotsCoverageSource(hs), "none named") {
			t.Errorf("no report named: test_impact %v %q, risk %v, portfolio %v, hotspots %v — each must say none was read",
				answer.Risk.CoverageSupplied, answer.Summary, r.risk.CoverageSupplied, r.portfolio.CoverageSupplied, hs.CoverageSupplied)
		}
		for _, row := range answer.Risk.Rows {
			if f := row.Factors[0]; f.Value != 1 || !strings.Contains(f.Cite, "no coverage supplied") {
				t.Errorf("no report named: %s uncovered_share %+v, want 1, said so", row.Symbol, f)
			}
		}
	}
}
