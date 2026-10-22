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
		rows = append(rows, semantic.Obligation{Subject: sec.Subject, Src: "AGENTS.md#L1-L1", Tier: "raw", CitationStatus: &semantic.CitationStatus{State: "stale"}})
	}
	got := (&Client{}).refindRawRows(ctx, root, "pkg/a.go", rows, nil, nil)
	reads := stop()
	if len(got) != 2 || len(reads) != 1 {
		t.Fatalf("rows=%+v reads=%v; want two rows and one file read", got, reads)
	}
}

func TestAD5RawRefindOnlyStaleCitations(t *testing.T) {
	for _, state := range []string{"", "fresh", "unknown", "stale", "refreshing"} {
		t.Run("state="+state, func(t *testing.T) {
			root := t.TempDir()
			body := []byte("# Background\nContext.\n\n# Rules\nAlways sort outputs.\n")
			if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), body, 0600); err != nil {
				t.Fatal(err)
			}
			var status *semantic.CitationStatus
			if state != "" {
				status = &semantic.CitationStatus{State: state}
			}
			row := semantic.Obligation{
				Subject: "AGENTS.md#rules", Rule: "Always sort outputs.",
				Src: "AGENTS.md#L1-L2", Tier: "raw", CitationStatus: status,
			}
			ctx := withRawRefind(context.Background())
			stop := ObserveReadsUnder(root)
			defer stop()
			got := (&Client{}).refindRawRows(ctx, root, "pkg/a.go", []semantic.Obligation{row, row}, nil, nil)
			reads := stop()
			wantSrc, wantReads := row.Src, 0
			if state == "stale" || state == "refreshing" {
				wantSrc, wantReads = "AGENTS.md#L4-L6", 1
			}
			if len(got) != 2 || len(reads) != wantReads {
				t.Fatalf("rows=%+v reads=%v; want two rows and %d reads", got, reads, wantReads)
			}
			for _, r := range got {
				if r.Src != wantSrc || r.Rule != row.Rule || r.CitationStatus != status {
					t.Errorf("row=%+v; want src=%s, unchanged rule and freshness", r, wantSrc)
				}
			}
		})
	}
}
