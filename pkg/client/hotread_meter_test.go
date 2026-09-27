package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestHotPathReadsChargeTheAdmissionMeter (review finding, RT-66, I48):
// the request path's file reads in this package (the obligations corpus,
// the ancestor-guide probe, the edited-file freshness and token reads) go
// through internal/corpus, so every byte they read is charged to the
// call's admission ticket, and a file that sniffs binary is skipped by name
// after at most the sniff.
func TestHotPathReadsChargeTheAdmissionMeter(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "CLAUDE.md")
	body := []byte("# rules\n\nnever do the thing\n")
	if err := os.WriteFile(txt, body, 0o644); err != nil {
		t.Fatal(err)
	}
	g := tenancy.NewGate(tenancy.GateConfig{Mode: tenancy.ModeOff})
	tk, ref := g.Admit(context.Background(), tenancy.Request{Tool: "obligations", Class: tenancy.ClassPoint})
	if ref != nil {
		t.Fatalf("refused: %+v", ref)
	}
	defer tk.Release(tenancy.OutcomeOK)
	data, info, skipped, err := readFileCapped(tk.Context(), txt, HotPathReadCap)
	if err != nil || skipped != "" || string(data) != string(body) || info == nil {
		t.Fatalf("read: %q %v %q %v", data, info != nil, skipped, err)
	}
	if got := tk.BytesRead(); got != int64(len(body)) {
		t.Fatalf("meter charged %d bytes, want %d", got, len(body))
	}
	bin := filepath.Join(dir, "blob")
	if err := os.WriteFile(bin, append([]byte("x\x00y"), make([]byte, 20000)...), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _, skipped, err = readFileCapped(tk.Context(), bin, HotPathReadCap)
	if err != nil || data != nil || skipped == "" {
		t.Fatalf("binary: data %d, skipped %q, err %v; want skipped by name", len(data), skipped, err)
	}
}
