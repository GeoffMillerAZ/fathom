package designevidence

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

func TestCatalogPreflightAllowsEntriesPastNestedBoundAndRejectsOverMax(t *testing.T) {
	entries := catalogEntries(1025)
	body, err := encodeCatalog(entries)
	if err != nil {
		t.Fatalf("1025 entries rejected below MaxEntries: %v", err)
	}
	if _, err := decodeCatalog(body); err != nil {
		t.Fatalf("1025 entries rejected by preflight: %v", err)
	}

	over := catalogEntries(MaxEntries + 1)
	if _, err := encodeCatalog(over); err == nil {
		t.Fatal("MaxEntries+1 encoded")
	}
	// Exercise the untrusted-read preflight too, before revision/schema work.
	raw, err := json.Marshal(struct {
		SchemaVersion int     `json:"schema_version"`
		Revision      string  `json:"revision"`
		Entries       []Entry `json:"entries"`
	}{1, strings.Repeat("0", 64), over})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCatalog(raw); err == nil {
		t.Fatal("MaxEntries+1 raw catalog accepted")
	}
}

func TestSearchMultiPageCursorBindsLimitQueryAndRevision(t *testing.T) {
	entries := catalogEntries(5)
	entries[3].Recorded.Subject.ID, entries[4].Recorded.Subject.ID = "other", "other"
	body, err := encodeCatalog(entries)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := decodeCatalog(body)
	if err != nil {
		t.Fatal(err)
	}
	first, err := catalog.Search(Query{SubjectID: "subject", Limit: 2})
	if err != nil || first.Total != 3 || len(first.Entries) != 2 || first.NextCursor == "" {
		t.Fatalf("first page: %#v %v", first, err)
	}
	second, err := catalog.Search(Query{SubjectID: "subject", Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Entries) != 1 || second.NextCursor != "" || second.Entries[0].ID <= first.Entries[1].ID {
		t.Fatalf("second page: %#v %v", second, err)
	}
	if _, err := catalog.Search(Query{SubjectID: "subject", Limit: 1, Cursor: first.NextCursor}); err == nil {
		t.Fatal("cursor accepted with changed limit")
	}
	if _, err := catalog.Search(Query{SubjectID: "other", Limit: 2, Cursor: first.NextCursor}); err == nil {
		t.Fatal("cursor accepted with changed query")
	}

	newerBody, err := encodeCatalog(append(entries, catalogEntry(9)))
	if err != nil {
		t.Fatal(err)
	}
	newer, err := decodeCatalog(newerBody)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newer.Search(Query{SubjectID: "subject", Limit: 2, Cursor: first.NextCursor}); err == nil {
		t.Fatal("cursor accepted across catalog revision")
	}
}

func catalogEntries(n int) []Entry {
	out := make([]Entry, n)
	for i := range out {
		out[i] = catalogEntry(i)
	}
	return out
}
func catalogEntry(i int) Entry {
	hash := fmt.Sprintf("%064x", i+1)
	claimed := map[string]string{}
	for _, name := range evidenceGateNames {
		claimed[name] = "not-applicable"
	}
	return Entry{ID: "evidence@" + hash, SourceMember: fmt.Sprintf("member-%d.html", i), Recorded: designreceipts.Report{Integrity: "matched", Verification: "not-verified", ContractSHA256: hash, ReceiptSHA256: hash, Subject: designreceipts.Subject{ID: "subject", Profile: "static-web", Variant: "variant", SourceArchive: "syntax@" + strings.Repeat("a", 64), SourceRevision: strings.Repeat("b", 64), ArtifactSHA256: strings.Repeat("c", 64)}, Mode: "retrospective", ClaimedStatuses: claimed, IncompleteGates: []string{}, MatchedArtifacts: 0}}
}
