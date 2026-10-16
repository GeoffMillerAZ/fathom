package designconvert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

// This diagnostic independently compiles/renders every accepted row from one
// immutable syntax report. Private temporary derivatives are never published or
// saved in the conversion catalog. CSS, browser and behavior remain unverified.
func TestHTMLInventoryCompileDOMLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_HTML_DOM_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_HTML_DOM_KIT_ROOT to the existing absolute private kit checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private kit root must be absolute")
	}
	const syntaxSHA = "35067bfc3e591bfbecb2dd6ea20038b28159a97a02cf57fdda934004eddba79c"
	filename := filepath.Join(root, "data/design-assets/coverage", syntaxSHA+".json")
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal("frozen private syntax report unavailable")
	}
	body, readErr := io.ReadAll(io.LimitReader(f, 512<<10))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil || digest(body) != syntaxSHA {
		t.Fatal("syntax report integrity failed; do not substitute an unreviewed report")
	}
	var syntax struct {
		Verification    string      `json:"verification"`
		InventorySHA256 string      `json:"inventory_sha256"`
		Transformer     Transformer `json:"transformer"`
		Rows            []struct {
			Source       Source `json:"source"`
			Status       string `json:"status"`
			SourceBytes  int    `json:"source_bytes"`
			OutputBytes  int    `json:"output_bytes"`
			OutputSHA256 string `json:"output_sha256"`
		} `json:"rows"`
	}
	if json.Unmarshal(body, &syntax) != nil || len(syntax.Rows) != 657 || syntax.Verification != "not-verified" {
		t.Fatal("frozen syntax report shape changed")
	}
	type resultRow struct {
		Source       Source `json:"source"`
		OutputSHA256 string `json:"output_sha256"`
		CompilerDOM  string `json:"compiler_dom"`
	}
	report := struct {
		Scope                 string      `json:"scope"`
		Verification          string      `json:"verification"`
		SyntaxReportSHA256    string      `json:"syntax_report_sha256"`
		InventorySHA256       string      `json:"inventory_sha256"`
		Transformer           Transformer `json:"transformer"`
		InventoryInputs       int         `json:"inventory_inputs"`
		SyntaxAccepted        int         `json:"syntax_accepted"`
		PassedBothPhases      int         `json:"passed_both_phases"`
		FailedOrUnproven      int         `json:"failed_or_unproven"`
		OtherInventoryInputs  int         `json:"other_inventory_inputs"`
		DeclarationRenameOnly bool        `json:"test_declaration_rename_only"`
		Rows                  []resultRow `json:"rows"`
	}{Scope: "private-compiler-dom-diagnostic-before-and-after-formatting", Verification: "not-verified",
		SyntaxReportSHA256: syntaxSHA, InventorySHA256: syntax.InventorySHA256, Transformer: syntax.Transformer,
		InventoryInputs: 657, OtherInventoryInputs: 218, DeclarationRenameOnly: true, Rows: []resultRow{}}
	batch := []coverageDOMCase{}
	batchRows := []resultRow{}
	batchIndex := 0
	scratch := t.TempDir() // Shared private build cache, removed with all temporary derivatives.
	flush := func() {
		if len(batch) == 0 {
			return
		}
		batchIndex++
		name := fmt.Sprintf("batch-%02d", batchIndex)
		passed := runCoverageDOMBatch(t, name, func() error {
			return checkCoverageDOMBatch(root, batch, scratch)
		})
		if !passed {
			t.Errorf("%s failed or did not execute; full diagnostic requires every batch", name)
		}
		for _, row := range batchRows {
			if passed {
				row.CompilerDOM = "passed-before-and-after-formatting"
				report.PassedBothPhases++
			} else {
				row.CompilerDOM = "batch-failed-or-unproven"
				report.FailedOrUnproven++
			}
			report.Rows = append(report.Rows, row)
		}
		t.Logf("compiler_dom_assessed=%d/439 passed_both_phases=%d failed_or_unproven=%d; no graphical/behavior proof", len(report.Rows), report.PassedBothPhases, report.FailedOrUnproven)
		batch, batchRows = nil, nil
	}
	for index, row := range syntax.Rows {
		if row.Status != "translated" {
			continue // These 218 stay explicitly in the full inventory denominator.
		}
		report.SyntaxAccepted++
		file, err := designassets.ReadFile(filepath.Join(root, "data/design-assets"), row.Source.ArchiveID, row.Source.Path)
		if err != nil || file.SHA256 != row.Source.SHA256 || len(file.Content) != row.SourceBytes {
			t.Fatalf("source %d integrity failed; details omitted", index)
		}
		source := row.Source
		source.Size = row.SourceBytes
		translation, err := Translate(source, []byte(file.Content))
		if err != nil || translation.Status != "translated" || translation.Artifact == nil {
			t.Fatalf("source %d no longer translates; review rather than bypassing the frozen report", index)
		}
		if translation.Transformer != syntax.Transformer || translation.Artifact.SHA256 != row.OutputSHA256 || len(translation.Artifact.Content) != row.OutputBytes || translation.Verification != "not-verified" {
			t.Fatalf("source %d rule/output/evidence identity changed", index)
		}
		batch = append(batch, coverageDOMCase{ID: fmt.Sprintf("case-%03d", index), Original: file.Content, Converted: translation.Artifact.Content})
		batchRows = append(batchRows, resultRow{Source: source, OutputSHA256: row.OutputSHA256})
		if len(batch) == 32 {
			flush()
		}
	}
	flush()
	if report.SyntaxAccepted != 439 || len(report.Rows) != 439 || report.PassedBothPhases+report.FailedOrUnproven != 439 {
		t.Fatal("compiler/DOM denominator incomplete")
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("APPKIT_PROBE_HTML_DOM_REPORT"); output != "" {
		if !filepath.IsAbs(output) {
			t.Fatal("optional metadata report path must be absolute")
		}
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("metadata report must be a new private file; no overwrite")
		}
		_, writeErr := f.Write(append(encoded, '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal("metadata report write failed")
		}
	}
	t.Logf("inventory=657 syntax_accepted=439 compiler_dom_passed=%d failed_or_unproven=%d other_inputs=218 report_sha256=%s verified_targets=0 catalog_derivatives_saved=0", report.PassedBothPhases, report.FailedOrUnproven, digest(append(encoded, '\n')))
}

// t.Run returns true for filtered subtests too. A successful return is evidence
// only when the full check actually completed; filtered work stays unproven.
func runCoverageDOMBatch(t *testing.T, name string, check func() error) bool {
	t.Helper()
	completed := false
	passed := t.Run(name, func(t *testing.T) {
		if err := check(); err != nil {
			t.Fatal(err) // Controlled stage-only errors, never paid source.
		}
		completed = true
	})
	return passed && completed
}

func TestCoverageDOMRunAccounting(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCoverageDOMRunAccountingProbe$/^executed$", "-test.v")
	cmd.Env = append(os.Environ(), "APPKIT_PROBE_DOM_ACCOUNTING_CANARY=1")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "filtered batch remains unproven") {
		t.Fatalf("filtered-subtest accounting regression: %v\n%s", err, out)
	}
}

func TestCoverageDOMRunAccountingProbe(t *testing.T) {
	if os.Getenv("APPKIT_PROBE_DOM_ACCOUNTING_CANARY") != "1" {
		t.Skip("subprocess-only synthetic accounting canary")
	}
	calls := 0
	check := func() error { calls++; return nil }
	if !runCoverageDOMBatch(t, "executed", check) {
		t.Fatal("executed passing check was not counted")
	}
	if runCoverageDOMBatch(t, "filtered", check) || calls != 1 {
		t.Fatal("filtered batch incorrectly counted as executed/passed")
	}
	t.Log("filtered batch remains unproven")
}
