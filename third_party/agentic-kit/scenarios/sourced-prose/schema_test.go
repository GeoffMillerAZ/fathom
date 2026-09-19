package sourcedprose

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestReportMarshalsToCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE tool unavailable; task cue-vet has the same optional prerequisite")
	}
	report, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, cue, "vet", "schema.cue", path, "-d", "#Report").CombinedOutput(); err != nil {
		t.Fatalf("actual scenario report violates CUE: %v\n%s", err, out)
	}
}
