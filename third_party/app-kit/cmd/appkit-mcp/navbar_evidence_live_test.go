package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/gallery"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
)

// This opt-in protocol test checks an independently selected retained packet.
// It never derives a trusted bundle identity from a receipt or certifies claims.
func TestRetainedNavbarEvidenceMCPAndGalleryParity(t *testing.T) {
	root, requested := os.LookupEnv("APPKIT_NAVBAR_EVIDENCE_ROOT")
	if !requested {
		t.Skip("set the reviewed private root and APPKIT_NAVBAR_EVIDENCE_SHA")
	}
	pin := os.Getenv("APPKIT_NAVBAR_EVIDENCE_SHA")
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(pin) {
		t.Fatal("independently selected exact bundle hash required")
	}
	id := "evidence@" + pin
	catalog, err := designevidence.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, session := designEvidenceSession(t, root)
	handler := gallery.NewWithOptions(gallery.Options{DesignAssetsRoot: root})
	query := designevidence.Query{SubjectID: "reference.navbar-adapter"}
	page, err := catalog.Search(query)
	if err != nil || page.Total != 1 || len(page.Entries) != 1 || page.Entries[0].ID != id {
		t.Fatalf("exact subject search: %v %#v", err, page)
	}
	compare := func(label string, want any, body []byte) {
		t.Helper()
		if len(body) > 64<<10 || bytes.Contains(body, []byte(`"content"`)) {
			t.Fatalf("%s exceeded metadata boundary: %d bytes", label, len(body))
		}
		expected, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		catalogJSONEqual(t, body, expected)
		t.Logf("%s: %d bytes", label, len(body))
	}
	compare("MCP search", page, catalogCall(t, ctx, session, "search_design_evidence", query))
	var galleryPage designevidence.Page
	compare("Gallery search", page, galleryEvidenceJSON(t, handler, "/api/target-evidence?subject_id="+url.QueryEscape(query.SubjectID), &galleryPage))
	detail, err := designevidence.Read(root, id)
	if err != nil {
		t.Fatal(err)
	}
	images := 0
	for _, result := range detail.Receipt.Results {
		if result.Status != "failed" && result.Status != "not-run" {
			t.Fatal("historical result was promoted")
		}
		for _, evidence := range result.Evidence {
			if evidence.Role == "image" {
				images++
			}
		}
	}
	if images != 114 || detail.OriginalMembership != "matched" || detail.Entry.Recorded.Verification != "not-verified" || detail.Rechecked.Verification != "not-verified" || len(detail.Rechecked.IncompleteGates) != 9 || len(detail.Entry.Recorded.IncompleteGates) != 9 || detail.Rechecked.ClaimedStatuses["interaction"] != "failed" || detail.Entry.Recorded.ClaimedStatuses["interaction"] != "failed" {
		t.Fatalf("retained boundary changed: images=%d rechecked=%#v", images, detail.Rechecked)
	}
	if detail.Rechecked.Subject.ID != query.SubjectID || detail.Rechecked.Subject.SourceRevision != "79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7" || detail.Rechecked.Subject.ArtifactSHA256 != "67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137" {
		t.Fatal("independent source/output selection changed")
	}
	compare("MCP exact get", detail, catalogCall(t, ctx, session, "get_design_evidence", designEvidenceGetInput{ID: id}))
	var galleryDetail designevidence.Detail
	compare("Gallery exact get", detail, galleryEvidenceJSON(t, handler, "/api/target-evidence/"+url.PathEscape(id), &galleryDetail))
}
