package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/gallery"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var evidenceGateNames = []string{
	"reproducibility", "structure", "visual", "interaction", "lifecycle",
	"security", "data", "performance", "integration",
}

func designEvidenceFixture(t *testing.T) (string, []designevidence.Entry) {
	t.Helper()
	sourceBody := []byte("SYNTHETIC_EVIDENCE_ORIGINAL_ONLY")
	var zipped bytes.Buffer
	z := zip.NewWriter(&zipped)
	w, err := z.Create("html/source.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(sourceBody); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	sourceDir, root := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "marketing-v4.zip"), zipped.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	assets, err := designassets.Import(context.Background(), sourceDir, root)
	if err != nil {
		t.Fatal(err)
	}
	archiveID := assets.List()[0].ID
	sourceSHA := designreceipts.Digest(sourceBody)

	publish := func(subject, profile, variant string) designevidence.Entry {
		content := map[string][]byte{}
		asset := func(id, text string) designreceipts.Asset {
			body := []byte(text)
			content[id] = body
			return designreceipts.Asset{ID: id, SHA256: designreceipts.Digest(body), Bytes: len(body)}
		}
		content["source"] = append([]byte(nil), sourceBody...)
		source := designreceipts.Asset{ID: "source", SHA256: sourceSHA, Bytes: len(sourceBody)}
		output := asset("output", "output "+subject)
		contract := designreceipts.Contract{SchemaVersion: 1, Subject: designreceipts.Subject{
			ID: subject, Profile: profile, Variant: variant, SourceArchive: archiveID,
			SourceRevision: sourceSHA, ArtifactSHA256: output.SHA256,
		}, Mode: "prospective", Inputs: designreceipts.Inputs{
			Source: []designreceipts.Asset{source}, Transformer: []designreceipts.Asset{asset("transformer", "transformer")},
			Config: []designreceipts.Asset{}, Patch: []designreceipts.Asset{}, Runtime: []designreceipts.Asset{},
			Output: []designreceipts.Asset{output}, Fixture: []designreceipts.Asset{asset("fixture", "fixture")},
			Toolchain: []designreceipts.Asset{asset("toolchain", "toolchain")}, Baseline: []designreceipts.Asset{asset("baseline", "baseline")},
		}, Gates: map[string]designreceipts.Gate{}}
		receipt := designreceipts.Receipt{SchemaVersion: 1, Results: map[string]designreceipts.Result{}}
		for _, name := range evidenceGateNames {
			e := asset("evidence-"+name, subject+" "+name)
			contract.Gates[name] = designreceipts.Gate{Applicable: true, Scope: "synthetic " + name, Slots: []designreceipts.Slot{{ID: e.ID, Role: "report"}}}
			receipt.Results[name] = designreceipts.Result{Status: "passed", Evidence: []designreceipts.Evidence{{Asset: e, Role: "report"}}}
		}
		contractBody, err := designreceipts.EncodeContract(contract)
		if err != nil {
			t.Fatal(err)
		}
		receipt.ContractSHA256 = designreceipts.Digest(contractBody)
		receipt.SourceRevision = sourceSHA
		receipt.ArtifactSHA256 = output.SHA256
		receiptBody, err := designreceipts.EncodeReceipt(receipt)
		if err != nil {
			t.Fatal(err)
		}
		entry, err := designevidence.Publish(root, "html/source.html", contractBody, receiptBody, designreceipts.Expected{
			ContractSHA256: receipt.ContractSHA256, SourceRevision: sourceSHA, ArtifactSHA256: output.SHA256,
		}, content)
		if err != nil {
			t.Fatal(err)
		}
		return entry
	}
	return root, []designevidence.Entry{
		publish("alpha", "static-web", "candidate-a"),
		publish("beta", "go-web", "candidate-b"),
	}
}

func designEvidenceSession(t *testing.T, root string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	srv := mcp.NewServer(&mcp.Implementation{Name: "design-evidence-server-test", Version: "test"}, nil)
	registerDesignEvidenceTools(srv, root)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "design-evidence-client-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func galleryEvidenceJSON(t *testing.T, h http.Handler, target string, out any) []byte {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.Host, r.RemoteAddr = "127.0.0.1:8765", "127.0.0.1:12345"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("Gallery %s: status=%d body=%s", target, w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
	return w.Body.Bytes()
}

func TestDesignEvidenceMCPMetadataParityAndBounds(t *testing.T) {
	root, _ := designEvidenceFixture(t)
	catalog, err := designevidence.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, session := designEvidenceSession(t, root)
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		if tool.Name != "search_design_evidence" && tool.Name != "get_design_evidence" {
			continue
		}
		found[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint || tool.InputSchema == nil || tool.OutputSchema == nil || !strings.Contains(tool.Description, "not-verified") {
			t.Fatalf("%s omitted its typed local read-only evidence boundary", tool.Name)
		}
	}
	if len(found) != 2 {
		t.Fatal("both evidence tools were not registered")
	}

	handler := gallery.NewWithOptions(gallery.Options{DesignAssetsRoot: root})
	queries := []designevidence.Query{{Limit: 1}, {ArchiveID: catalog.List()[0].Recorded.Subject.SourceArchive}, {SubjectID: "alpha"}, {Profile: "go-web"}}
	for _, query := range queries {
		mcpBody := catalogCall(t, ctx, session, "search_design_evidence", query)
		want, err := catalog.Search(query)
		if err != nil {
			t.Fatal(err)
		}
		wantBody, _ := json.Marshal(want)
		catalogJSONEqual(t, mcpBody, wantBody)
		values := url.Values{}
		if query.ArchiveID != "" {
			values.Set("archive_id", query.ArchiveID)
		}
		if query.SubjectID != "" {
			values.Set("subject_id", query.SubjectID)
		}
		if query.Profile != "" {
			values.Set("profile", query.Profile)
		}
		if query.Limit != 0 {
			values.Set("limit", "1")
		}
		var galleryPage designevidence.Page
		galleryBody := galleryEvidenceJSON(t, handler, "/api/target-evidence?"+values.Encode(), &galleryPage)
		catalogJSONEqual(t, galleryBody, wantBody)
		if len(mcpBody) > 64<<10 || bytes.Contains(mcpBody, sourceBodyMarker) || !reflect.DeepEqual(galleryPage, want) {
			t.Fatal("metadata response leaked bytes, exceeded bounds or diverged from Gallery")
		}
	}

	for _, entry := range catalog.List() {
		mcpBody := catalogCall(t, ctx, session, "get_design_evidence", designEvidenceGetInput{ID: entry.ID})
		want, err := designevidence.Read(root, entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		wantBody, _ := json.Marshal(want)
		catalogJSONEqual(t, mcpBody, wantBody)
		var galleryDetail designevidence.Detail
		galleryBody := galleryEvidenceJSON(t, handler, "/api/target-evidence/"+entry.ID, &galleryDetail)
		catalogJSONEqual(t, galleryBody, wantBody)
		if len(mcpBody) > 64<<10 || bytes.Contains(mcpBody, sourceBodyMarker) || want.Entry.Recorded.Verification != "not-verified" || want.Rechecked.Verification != "not-verified" || want.OriginalMembership != "matched" || !reflect.DeepEqual(galleryDetail, want) {
			t.Fatal("exact response leaked bytes, exceeded bounds, overstated verification or diverged from Gallery")
		}
	}

	for _, query := range []designevidence.Query{{Limit: 51}, {Profile: "desktop"}, {Cursor: "bad"}} {
		conversionToolError(t, ctx, session, "search_design_evidence", query, "")
	}
	conversionToolError(t, ctx, session, "get_design_evidence", designEvidenceGetInput{ID: "bad"}, "")
}

var sourceBodyMarker = []byte("SYNTHETIC_EVIDENCE_ORIGINAL_ONLY")

func TestDesignEvidenceMCPMissingOptionalStoreIsEmpty(t *testing.T) {
	root := t.TempDir()
	ctx, session := designEvidenceSession(t, root)
	body := catalogCall(t, ctx, session, "search_design_evidence", map[string]any{})
	var page designevidence.Page
	if json.Unmarshal(body, &page) != nil || page.Total != 0 || page.Entries == nil {
		t.Fatalf("optional missing evidence store was not an empty page: %s", body)
	}
	if _, err := os.Stat(filepath.Join(root, "evidence")); !os.IsNotExist(err) {
		t.Fatal("read-only MCP search created an evidence store")
	}
}
