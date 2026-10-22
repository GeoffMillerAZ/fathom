package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestAD5RawRefindReadsFileOncePerCall(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	body := []byte("# First\nAlways sort outputs.\n\n# Second\nNever skip validation.\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := withRawRefind(context.Background())
	stop := ObserveReadsUnder(root)
	defer stop()
	if _, _, _, err := readFileCapped(ctx, path, HotPathReadCap); err != nil {
		t.Fatal(err)
	}
	var rows []semantic.Obligation
	for _, sec := range semantic.SplitMarkdownSections("AGENTS.md", body) {
		rows = append(rows, semantic.Obligation{Subject: sec.Subject, Src: "AGENTS.md#L1-L1", Tier: "raw"})
	}
	got := (&Client{}).refindRawRows(ctx, root, "pkg/a.go", rows, nil)
	reads := stop()
	if len(got) != 2 || len(reads) != 1 {
		t.Fatalf("rows=%+v reads=%v; want two rows and one file read", got, reads)
	}
}
