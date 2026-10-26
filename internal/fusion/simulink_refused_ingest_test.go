package fusion

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_RefusedSimulinkModelKeepsService is S5-R1 at ingest: a model the reader refuses
// (8,300 blocks named A, one line to 1,000 of two letters: 8.3M call entries of 4 bytes each,
// under the 32 MiB byte cap and past SimulinkMaxCalls)
// yields nothing, and the Go file beside it is still stored.
func TestIngest_RefusedSimulinkModelKeepsService(t *testing.T) {
	var part strings.Builder
	part.WriteString(`<System>` + strings.Repeat("<Block Name=\"A\"/>\n", 8300))
	const set = "abcdefghijklmnopqrstuvwxyz0123456789"
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&part, "<Block Name=\"%c%c\"/>\n", set[i/36], set[i%36])
	}
	part.WriteString(`<Line><P Name="SrcBlock">A</P>`)
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&part, "<P Name=\"DstBlock\">%c%c</P>\n", set[i/36], set[i%36])
	}
	part.WriteString(`</Line></System>`)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Stored, not deflated, with a line per element: ingest's minified gate (an average line
	// over 500 bytes) would otherwise skip so small a package before the reader sees it.
	f, _ := w.CreateHeader(&zip.FileHeader{Name: "simulink/systems/system_root.xml", Method: zip.Store})
	f.Write([]byte(part.String()))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"go.mod":       "module example.com/m\n\ngo 1.22\n",
		"main.go":      "package main\n\nfunc main() { helper() }\n\nfunc helper() {}\n",
		"models/m.slx": buf.String(),
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "slxflood", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ModelsRefused != 1 {
		t.Errorf("ModelsRefused = %d; want 1 (a refused model is counted, never stored as an empty file)", rep.ModelsRefused)
	}
	if rep.SkippedGenerated != 0 {
		t.Fatalf("%d files skipped as generated; the model must reach the reader", rep.SkippedGenerated)
	}
	obs, err := st.QueryObservations(ctx, "slxflood", "")
	if err != nil {
		t.Fatal(err)
	}
	goSyms, slx := map[string]bool{}, 0
	for _, o := range obs {
		switch strings.SplitN(o.SourceRef, ":", 2)[0] {
		case "main.go":
			goSyms[o.Symbol] = true
		case "models/m.slx":
			slx++
		}
	}
	if !goSyms["main"] || !goSyms["helper"] {
		t.Errorf("main.go symbols %v; want main and helper stored", goSyms)
	}
	if slx != 0 {
		t.Errorf("the refused model stored %d observations; want none", slx)
	}
}
