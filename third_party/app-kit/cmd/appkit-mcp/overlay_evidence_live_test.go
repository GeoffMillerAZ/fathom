package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/gallery"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
)

func TestRetainedOverlayEvidenceMCPAndGalleryParity(t *testing.T) {
	root, ok := os.LookupEnv("APPKIT_OVERLAY_EVIDENCE_ROOT")
	if !ok {
		t.Skip("set APPKIT_OVERLAY_EVIDENCE_ROOT to the reviewed private evidence root")
	}
	catalog, err := designevidence.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, session := designEvidenceSession(t, root)
	handler := gallery.NewWithOptions(gallery.Options{DesignAssetsRoot: root})
	wants := []struct {
		subject string
		id      string
		images  int
	}{
		{"reference.menu-adapter", "evidence@66aff1381147060cae5488e8c26a3c4491fb37109ce33a78391fba1a5d5c926f", 52},
		{"reference.drawer-adapter", "evidence@1ce15a0ce6e58db27a03c87654f3c8a97580c10111483c8cc5489159364a53dc", 44},
	}
	bounded := func(label string, body []byte) {
		t.Helper()
		if len(body) > 64<<10 {
			t.Fatalf("%s metadata response exceeds 64 KiB: %d bytes", label, len(body))
		}
		if bytes.Contains(body, []byte(`"content"`)) {
			t.Fatalf("%s metadata response exposed retained content", label)
		}
	}
	for _, want := range wants {
		t.Run(want.subject, func(t *testing.T) {
			query := designevidence.Query{SubjectID: want.subject}
			page, err := catalog.Search(query)
			if err != nil || page.Total != 1 || len(page.Entries) != 1 || page.Entries[0].ID != want.id {
				t.Fatalf("exact retained search: %v %#v", err, page)
			}
			pageJSON, err := json.Marshal(page)
			if err != nil {
				t.Fatal(err)
			}
			mcpSearch := catalogCall(t, ctx, session, "search_design_evidence", query)
			bounded("MCP search", mcpSearch)
			catalogJSONEqual(t, mcpSearch, pageJSON)
			var galleryPage designevidence.Page
			gallerySearch := galleryEvidenceJSON(t, handler, "/api/target-evidence?subject_id="+url.QueryEscape(want.subject), &galleryPage)
			bounded("Gallery search", gallerySearch)
			catalogJSONEqual(t, gallerySearch, pageJSON)

			detail, err := designevidence.Read(root, want.id)
			if err != nil {
				t.Fatal(err)
			}
			images := 0
			for _, result := range detail.Receipt.Results {
				for _, evidence := range result.Evidence {
					if evidence.Role == "image" {
						images++
					}
				}
			}
			if images != want.images || detail.Entry.Recorded.Verification != "not-verified" || detail.Rechecked.Verification != "not-verified" || len(detail.Entry.Recorded.IncompleteGates) != 9 || len(detail.Rechecked.IncompleteGates) != 9 || detail.Entry.Recorded.ClaimedStatuses["interaction"] != "failed" || detail.Rechecked.ClaimedStatuses["interaction"] != "failed" || detail.OriginalMembership != "matched" {
				t.Fatalf("retained boundary changed: images=%d entry=%#v rechecked=%#v membership=%q", images, detail.Entry.Recorded, detail.Rechecked, detail.OriginalMembership)
			}
			detailJSON, err := json.Marshal(detail)
			if err != nil {
				t.Fatal(err)
			}
			mcpDetail := catalogCall(t, ctx, session, "get_design_evidence", designEvidenceGetInput{ID: want.id})
			bounded("MCP exact get", mcpDetail)
			catalogJSONEqual(t, mcpDetail, detailJSON)
			var galleryDetail designevidence.Detail
			galleryExact := galleryEvidenceJSON(t, handler, "/api/target-evidence/"+url.PathEscape(want.id), &galleryDetail)
			bounded("Gallery exact get", galleryExact)
			catalogJSONEqual(t, galleryExact, detailJSON)
			t.Logf("bounded metadata bytes: MCP search=%d Gallery search=%d MCP exact=%d Gallery exact=%d", len(mcpSearch), len(gallerySearch), len(mcpDetail), len(galleryExact))
		})
	}
}
