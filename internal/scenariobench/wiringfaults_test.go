package scenariobench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWiringFaultStreamsInstrumentTruth(t *testing.T) {
	root := filepath.Join("..", "..", "cmd", "fathom", "testdata", "wiring-faults")
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases []struct {
			Directory string `json:"directory"`
			Fault     string `json:"fault"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, c := range manifest.Cases {
		t.Run(c.Directory, func(t *testing.T) {
			res, err := ParseSessionStream(filepath.Join(root, c.Directory, "stream.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			hooksError := hooksInstrumentError(true, res)
			wantHooksError := ""
			if c.Fault == "hook-unregistered" {
				wantHooksError = "instrument: no SessionStart hook fired"
			}
			if hooksError != wantHooksError {
				t.Errorf("hooksInstrumentError = %q, want %q", hooksError, wantHooksError)
			}
			mcpError := instrumentTruthError([]string{"fathom"}, res)
			wantMCPError := ""
			if c.Fault == "mcp-not-connected" {
				wantMCPError = "instrument: mcp server fathom absent"
			}
			if mcpError != wantMCPError {
				t.Errorf("instrumentTruthError = %q, want %q", mcpError, wantMCPError)
			}
		})
	}
}
