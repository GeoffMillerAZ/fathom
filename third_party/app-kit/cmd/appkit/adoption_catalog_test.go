package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/adoption"
)

type adoptionCatalogCapabilityOutput struct {
	Revision           string                      `json:"revision"`
	Entry              catalog.Entry               `json:"entry"`
	CapabilityRevision adoption.CapabilityRevision `json:"capability_revision"`
}

func adoptionCatalogJSON[T any](t *testing.T, args []string) T {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := runAdoption(context.Background(), args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatalf("runAdoption(%q) failed: %v, stderr=%q", args, err, stderr.String())
	}
	var got T
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&got); err != nil || decoder.Decode(new(any)) != io.EOF {
		t.Fatalf("runAdoption(%q) did not emit exactly one JSON value: %v, stdout=%q", args, err, stdout.String())
	}
	return got
}

func TestAdoptionCatalogSearchMatchesSharedCatalog(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		args  []string
		query catalog.Query
	}{
		{"default limit", []string{"search"}, catalog.Query{}},
		{"explicit shared default", []string{"search", "--limit", "0"}, catalog.Query{Limit: 0}},
		{"all filters", []string{"search", "--query", "bar chart", "--layer", "L3a", "--profile", "portable", "--kind", "primitive", "--maturity", "available", "--limit", "1"}, catalog.Query{Text: "bar chart", Layer: "L3a", Profile: "portable", Kind: "primitive", Maturity: "available", Limit: 1}},
		{"text mismatch is empty", []string{"search", "--query", "Wails"}, catalog.Query{Text: "Wails"}},
		{"compatible enums with no shared entry are empty", []string{"search", "--layer", "L3a", "--profile", "cli"}, catalog.Query{Layer: "L3a", Profile: "cli"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := c.Search(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			got := adoptionCatalogJSON[catalog.Page](t, tc.args)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("CLI search differs from shared catalog\ngot:  %+v\nwant: %+v", got, want)
			}
			if strings.Contains(tc.name, "empty") && (got.Total != 0 || len(got.Items) != 0 || got.NextCursor != "") {
				t.Fatalf("empty search recommended a capability: %+v", got)
			}
		})
	}
}

func TestAdoptionCatalogSearchContinuationMatchesSharedCatalog(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	firstQuery := catalog.Query{Limit: 1}
	wantFirst, err := c.Search(firstQuery)
	if err != nil || wantFirst.NextCursor == "" {
		t.Fatalf("shared catalog did not produce a continuation: %+v, %v", wantFirst, err)
	}
	gotFirst := adoptionCatalogJSON[catalog.Page](t, []string{"search", "--limit", "1"})
	if !reflect.DeepEqual(gotFirst, wantFirst) {
		t.Fatalf("first page differs from shared catalog: got %+v want %+v", gotFirst, wantFirst)
	}
	wantNext, err := c.Search(catalog.Query{Limit: 1, Cursor: wantFirst.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	gotNext := adoptionCatalogJSON[catalog.Page](t, []string{"search", "--limit", "1", "--cursor", gotFirst.NextCursor})
	if !reflect.DeepEqual(gotNext, wantNext) || gotNext.Items[0].ID == gotFirst.Items[0].ID {
		t.Fatalf("continuation differs from shared catalog: first=%+v got=%+v want=%+v", gotFirst, gotNext, wantNext)
	}
}

func TestAdoptionCatalogSearchNonemptyQueryAndProfile(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	query := catalog.Query{Text: "comparison", Profile: "go-web"}
	want, err := c.Search(query)
	if err != nil || len(want.Items) == 0 {
		t.Fatalf("shared catalog has no comparison/go-web fixture: %+v, %v", want, err)
	}
	got := adoptionCatalogJSON[catalog.Page](t, []string{"search", "--query", "comparison", "--profile", "go-web"})
	if !reflect.DeepEqual(got, want) || len(got.Items) == 0 {
		t.Fatalf("CLI did not preserve nonempty query/profile search: got %+v want %+v", got, want)
	}
}

func TestAdoptionCatalogSearchRefusesChangedOrStaleCursor(t *testing.T) {
	first := adoptionCatalogJSON[catalog.Page](t, []string{"search", "--limit", "1"})
	if first.NextCursor == "" {
		t.Fatal("fixture search did not produce a cursor")
	}
	cases := []struct {
		name string
		args []string
	}{
		{"changed filter", []string{"search", "--limit", "1", "--profile", "go-web", "--cursor", first.NextCursor}},
		{"changed limit", []string{"search", "--limit", "2", "--cursor", first.NextCursor}},
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	var cursor struct {
		Version  int    `json:"v"`
		Revision string `json:"revision"`
		Query    string `json:"query"`
		Offset   int    `json:"offset"`
	}
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.Revision == "" || cursor.Query == "" || cursor.Offset == 0 {
		t.Fatalf("fixture cursor is not structurally complete: %+v, %v", cursor, err)
	}
	cursor.Revision = strings.Repeat("0", 64)
	encoded, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	stale := base64.RawURLEncoding.EncodeToString(encoded)
	cases = append(cases, struct {
		name string
		args []string
	}{"stale revision", []string{"search", "--limit", "1", "--cursor", stale}})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := runAdoption(context.Background(), tc.args, &stdout, io.Discard); command.Code(err) != 2 || stdout.Len() != 0 {
				t.Fatalf("changed/stale cursor exit=%d stdout=%q", command.Code(err), stdout.String())
			}
		})
	}
}

func TestAdoptionCatalogCapabilityMatchesExactEntryAndCanonicalPin(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.Get("foundation.tokens")
	if !ok {
		t.Fatal("fixture capability is absent")
	}
	pin, err := adoption.PinCapability(c.SchemaVersion, entry)
	if err != nil {
		t.Fatal(err)
	}
	want := adoptionCatalogCapabilityOutput{Revision: c.Revision, Entry: entry, CapabilityRevision: pin}
	for _, args := range [][]string{
		{"capability", "--id", entry.ID},
		{"capability", "--id", entry.ID, "--expected-catalog-revision", c.Revision},
	} {
		got := adoptionCatalogJSON[adoptionCatalogCapabilityOutput](t, args)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("CLI capability differs from exact shared record and pin\ngot:  %+v\nwant: %+v", got, want)
		}
	}
}

func TestAdoptionCatalogEveryExactRecordFitsOutputBudget(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) == 0 {
		t.Fatal("embedded catalog is empty")
	}
	for _, entry := range c.Entries {
		t.Run(entry.ID, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runAdoption(context.Background(), []string{"capability", "--id", entry.ID, "--expected-catalog-revision", c.Revision}, &stdout, &stderr)
			if err != nil || stderr.Len() != 0 {
				t.Fatalf("exact record failed: %v, stderr=%q", err, stderr.String())
			}
			if stdout.Len() > 128*1024 {
				t.Fatalf("exact record exceeds 128KiB output cap including newline: %d bytes", stdout.Len())
			}
			var got adoptionCatalogCapabilityOutput
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got.Revision != c.Revision || got.Entry.ID != entry.ID || got.CapabilityRevision.ID != entry.ID {
				t.Fatalf("invalid exact record output: %+v, %v", got, err)
			}
		})
	}
}

func TestAdoptionCatalogRejectsInvalidRequestsWithoutStdout(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Repeat("0", 64)
	if stale == c.Revision {
		stale = strings.Repeat("1", 64)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"search positional", []string{"search", "extra"}},
		{"search json flag", []string{"search", "--json"}},
		{"search root flag", []string{"search", "--root", "."}},
		{"search missing query value", []string{"search", "--query"}},
		{"unknown layer", []string{"search", "--layer", "L9"}},
		{"unknown profile", []string{"search", "--profile", "desktop"}},
		{"unknown kind", []string{"search", "--kind", "framework"}},
		{"unknown maturity", []string{"search", "--maturity", "complete"}},
		{"negative limit", []string{"search", "--limit", "-1"}},
		{"oversize limit", []string{"search", "--limit", "51"}},
		{"invalid cursor", []string{"search", "--cursor", "bad"}},
		{"capability missing id", []string{"capability"}},
		{"capability empty id", []string{"capability", "--id", ""}},
		{"capability unknown id", []string{"capability", "--id", "missing"}},
		{"capability positional", []string{"capability", "--id", "foundation.tokens", "extra"}},
		{"capability json flag", []string{"capability", "--id", "foundation.tokens", "--json"}},
		{"capability root flag", []string{"capability", "--id", "foundation.tokens", "--root", "."}},
		{"malformed revision", []string{"capability", "--id", "foundation.tokens", "--expected-catalog-revision", "bad"}},
		{"uppercase revision", []string{"capability", "--id", "foundation.tokens", "--expected-catalog-revision", strings.Repeat("A", 64)}},
		{"stale revision", []string{"capability", "--id", "foundation.tokens", "--expected-catalog-revision", stale}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := runAdoption(context.Background(), tc.args, &stdout, io.Discard); command.Code(err) != 2 || stdout.Len() != 0 {
				t.Fatalf("invalid request exit=%d stdout=%q", command.Code(err), stdout.String())
			}
		})
	}
}

func TestAdoptionCatalogHelpCancellationAndWriterFailures(t *testing.T) {
	for _, mode := range []string{"search", "capability"} {
		t.Run(mode+" help", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := runAdoption(context.Background(), []string{mode, "--help"}, &stdout, &stderr); command.Code(err) != 0 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("help exit=%d stdout=%q stderr=%q", command.Code(err), stdout.String(), stderr.String())
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, args := range [][]string{{"search"}, {"capability", "--id", "foundation.tokens"}} {
		var stdout bytes.Buffer
		if err := runAdoption(ctx, args, &stdout, io.Discard); command.Code(err) != 2 || stdout.Len() != 0 {
			t.Fatalf("canceled %q exit=%d stdout=%q", args, command.Code(err), stdout.String())
		}
		for _, writer := range []io.Writer{adoptionBrokenWriter{}, adoptionDraftShortWriter{}} {
			if err := runAdoption(context.Background(), args, writer, io.Discard); command.Code(err) != 2 {
				t.Fatalf("writer failure for %q exit=%d", args, command.Code(err))
			}
		}
	}
}

func TestAdoptionCatalogSyntheticSearchGetDraftCheckWalkthrough(t *testing.T) {
	root, _ := draftCLIRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "app-kit.md"), []byte("# Synthetic app-kit contract\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	page := adoptionCatalogJSON[catalog.Page](t, []string{"search", "--query", "tokens", "--layer", "L0", "--profile", "portable"})
	if len(page.Items) == 0 {
		t.Fatal("search did not find a candidate; no adoption is inferred from discovery")
	}
	selected := page.Items[0].ID
	detail := adoptionCatalogJSON[adoptionCatalogCapabilityOutput](t, []string{"capability", "--id", selected, "--expected-catalog-revision", page.Revision})
	if detail.Entry.ID != selected || detail.CapabilityRevision.ID != selected {
		t.Fatalf("exact review changed selected capability: %+v", detail)
	}
	var draft bytes.Buffer
	draftArgs := []string{"draft", "--root", root, "--consumer", "catalog-walkthrough", "--profile", "portable", "--contract", "docs/app-kit.md", "--expected-catalog-revision", page.Revision, "--layer", "L0", "--capability", selected, "--artifact", "web/a.css"}
	if err := runAdoption(context.Background(), draftArgs, &draft, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, adoption.DefaultManifest), draft.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var checked bytes.Buffer
	if err := runAdoption(context.Background(), []string{"check", "--root", root, "--json"}, &checked, io.Discard); err != nil {
		t.Fatalf("saved synthetic selection did not pass its static check: %v, report=%s", err, checked.Bytes())
	}
	var report adoption.Report
	if err := json.Unmarshal(checked.Bytes(), &report); err != nil || report.Outcome != "passed" {
		t.Fatalf("invalid static-check report: %+v, %v", report, err)
	}
	// Search, exact review, drafting and a static check remain distinct from runtime adoption.
}
