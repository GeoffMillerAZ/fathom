package designevidence

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
)

func TestPublishReadSearchAndDetachedCatalog(t *testing.T) {
	root, member, contract, receipt, expected, content := evidenceFixture(t)
	c, err := Load(root)
	if err != nil || len(c.List()) != 0 {
		t.Fatalf("optional evidence store: %#v %v", c, err)
	}
	e, err := Publish(root, member, contract, receipt, expected, content)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if e.Recorded.Verification != "not-verified" {
		t.Fatal("historical metadata must never claim verification")
	}
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	p.Entries[0].SourceMember = "changed"
	if got, _ := p.Get(e.ID); got.SourceMember != member {
		t.Fatal("catalog exposed mutable internal entry")
	}
	page, err := p.Search(Query{ArchiveID: e.Recorded.Subject.SourceArchive, Limit: 1})
	if err != nil || page.Total != 1 || len(page.Entries) != 1 {
		t.Fatalf("search: %#v %v", page, err)
	}
	detail, err := Read(root, e.ID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if detail.OriginalMembership != "matched" || detail.Rechecked.Verification != "not-verified" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if _, err := p.Search(Query{Cursor: page.NextCursor, Limit: 1}); page.NextCursor != "" || err != nil {
		t.Fatalf("terminal page cursor: %v", err)
	}
}

func TestPublishBundleRejectsStalePinsAndCanonicalTamper(t *testing.T) {
	root, member, contract, receipt, expected, content := evidenceFixture(t)
	body, err := EncodeBundle(member, contract, receipt, content)
	if err != nil {
		t.Fatal(err)
	}
	stale := expected
	stale.SourceRevision = strings.Repeat("0", 64)
	if _, err := PublishBundle(root, body, stale); err == nil {
		t.Fatal("stale independent source pin accepted")
	}
	if got, err := Load(root); err != nil || len(got.List()) != 0 {
		t.Fatalf("failed publication wrote metadata: %v %#v", err, got)
	}
	if _, err := PublishBundle(root, append(body, ' '), expected); err == nil {
		t.Fatal("noncanonical bundle accepted")
	}
	if _, err := PublishBundle(root, body, expected); err != nil {
		t.Fatal(err)
	}
}

func TestReadRejectsTamperAndRefusesAbandonedLock(t *testing.T) {
	root, member, contract, receipt, expected, content := evidenceFixture(t)
	e, err := Publish(root, member, contract, receipt, expected, content)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "evidence", "entries", strings.TrimPrefix(e.ID, "evidence@")+".json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, e.ID); err == nil {
		t.Fatal("tampered bundle accepted")
	}

	root2, member2, contract2, receipt2, expected2, content2 := evidenceFixture(t)
	if err := os.Mkdir(filepath.Join(root2, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root2, "evidence", ".publish.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(root2, member2, contract2, receipt2, expected2, content2); err == nil {
		t.Fatal("abandoned lock was silently removed")
	}
}

func TestReadRequiresCurrentOriginalMembershipAndRejectsEvidenceSymlink(t *testing.T) {
	root, member, contract, receipt, expected, content := evidenceFixture(t)
	e, err := Publish(root, member, contract, receipt, expected, content)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := designassets.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "archives", assets.List()[0].SHA256+".zip")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, e.ID); err == nil {
		t.Fatal("missing original archive accepted")
	}

	root2 := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root2, "evidence")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := Load(root2); err == nil {
		t.Fatal("symlink evidence directory accepted")
	}
}

func evidenceFixture(t *testing.T) (string, string, []byte, []byte, designreceipts.Expected, map[string][]byte) {
	t.Helper()
	root, src := t.TempDir(), t.TempDir()
	archivePath := filepath.Join(src, "syntax.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("index.html")
	if err != nil {
		t.Fatal(err)
	}
	member := "<main>private evidence</main>\n"
	if _, err := w.Write([]byte(member)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := designassets.Import(context.Background(), src, root); err != nil {
		t.Fatalf("import: %v", err)
	}
	assets, err := designassets.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	a := assets.List()[0]
	original, err := designassets.ReadFile(root, a.ID, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	content := map[string][]byte{}
	asset := func(id, body string) designreceipts.Asset {
		b := []byte(body)
		content[id] = b
		return designreceipts.Asset{ID: id, SHA256: designreceipts.Digest(b), Bytes: len(b)}
	}
	source := designreceipts.Asset{ID: "source", SHA256: original.SHA256, Bytes: len(original.Content)}
	content[source.ID] = []byte(original.Content)
	transformer := asset("transformer", "tool")
	output := asset("output", "rendered")
	fixture := asset("fixture", "fixture")
	toolchain := asset("toolchain", "go")
	baseline := asset("baseline", "baseline")
	gates, results := map[string]designreceipts.Gate{}, map[string]designreceipts.Result{}
	for _, name := range []string{"reproducibility", "structure", "visual", "interaction", "lifecycle", "security", "data", "performance", "integration"} {
		gates[name] = designreceipts.Gate{Applicable: false, Scope: "out of test scope", Reason: "not run", ReviewAssertion: "no assertion", Slots: []designreceipts.Slot{}}
		results[name] = designreceipts.Result{Status: "not-applicable", Reason: "not run", Evidence: []designreceipts.Evidence{}}
	}
	contractValue := designreceipts.Contract{SchemaVersion: 1, Subject: designreceipts.Subject{ID: "specimen", Profile: "static-web", Variant: "test", SourceArchive: a.ID, SourceRevision: original.SHA256, ArtifactSHA256: output.SHA256}, Mode: "retrospective", Inputs: designreceipts.Inputs{Source: []designreceipts.Asset{source}, Transformer: []designreceipts.Asset{transformer}, Config: []designreceipts.Asset{}, Patch: []designreceipts.Asset{}, Runtime: []designreceipts.Asset{}, Output: []designreceipts.Asset{output}, Fixture: []designreceipts.Asset{fixture}, Toolchain: []designreceipts.Asset{toolchain}, Baseline: []designreceipts.Asset{baseline}}, Gates: gates}
	contract, err := designreceipts.EncodeContract(contractValue)
	if err != nil {
		t.Fatal(err)
	}
	receiptValue := designreceipts.Receipt{SchemaVersion: 1, ContractSHA256: designreceipts.Digest(contract), SourceRevision: original.SHA256, ArtifactSHA256: output.SHA256, Results: results}
	receipt, err := designreceipts.EncodeReceipt(receiptValue)
	if err != nil {
		t.Fatal(err)
	}
	return root, "index.html", contract, receipt, designreceipts.Expected{ContractSHA256: designreceipts.Digest(contract), SourceRevision: original.SHA256, ArtifactSHA256: output.SHA256}, content
}
