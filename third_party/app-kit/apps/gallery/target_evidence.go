package gallery

import (
	"errors"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
)

type evidenceRequest struct {
	Query  designevidence.Query
	Values url.Values
}

type evidenceRow struct {
	Entry            designevidence.Entry
	Detail, Original string
}

type evidenceIndexView struct {
	Page                          designevidence.Page
	Query                         designevidence.Query
	Rows                          []evidenceRow
	Theme, Reset, Originals, Next string
	Conversions                   string
}

type evidenceGateView struct {
	Name   string
	Gate   designreceipts.Gate
	Result designreceipts.Result
}

type evidenceDetailView struct {
	Detail               designevidence.Detail
	Gates                []evidenceGateView
	Back, Original, JSON string
	Images               evidenceImagesView
}

// These routes share the licensed library's GET-only loopback/origin boundary.
// Metadata stays byte-free; ADR-0055 separately admits bounded retained PNG slots.
func registerTargetEvidence(root string, register func(string, bool, http.HandlerFunc)) {
	registerEvidenceImages(root, register)
	for _, api := range []bool{false, true} {
		base := "/target-evidence"
		if api {
			base = "/api/target-evidence"
		}
		register(base, api, func(w http.ResponseWriter, r *http.Request) {
			in, err := parseEvidenceRequest(r.URL.Query(), false)
			if err != nil || r.URL.EscapedPath() != base {
				evidenceProblem(w, r, api, 400, "Reset evidence filters", "Use exact archive, subject and profile filters, a limit from 1 to 50, and a current cursor.", "")
				return
			}
			c, ok := evidenceReady(w, r, root, api)
			if !ok {
				return
			}
			page, err := c.Search(in.Query)
			if err != nil {
				evidenceProblem(w, r, api, 400, "Reset evidence filters", "The filters or cursor are invalid. A changed index requires restarting the search.", "")
				return
			}
			if api {
				writeReferenceJSON(w, page, 200)
				return
			}
			v := evidenceIndexView{Page: page, Query: in.Query, Theme: in.Values.Get("theme"), Reset: referenceLink("/target-evidence", referenceThemeValues(in.Values)), Originals: referenceLink("/references", referenceThemeValues(in.Values)), Conversions: referenceLink("/conversions", referenceThemeValues(in.Values))}
			for _, entry := range page.Entries {
				v.Rows = append(v.Rows, evidenceRow{Entry: entry, Detail: referenceLink("/target-evidence/"+url.PathEscape(entry.ID), in.Values), Original: evidenceOriginalURL(entry, in.Values)})
			}
			if page.NextCursor != "" {
				next := maps.Clone(in.Values)
				next.Set("cursor", page.NextCursor)
				v.Next = referenceLink("/target-evidence", next)
			}
			body, renderErr := htmlf("target-evidence-index", evidenceIndexHTML, v)
			writePrivateReferenceStatus(w, r, "Target evidence", body, renderErr, 200, in.Values)
		})
		register(base+"/{id}", api, func(w http.ResponseWriter, r *http.Request) {
			in, err := parseEvidenceDetailRequest(r.URL.Query(), api)
			if err != nil || r.URL.EscapedPath() != base+"/"+r.PathValue("id") || strings.Contains(r.PathValue("id"), "/") {
				evidenceProblem(w, r, api, 400, "Invalid evidence request", "Use an exact evidence ID and valid parameters.", "")
				return
			}
			c, ok := evidenceReady(w, r, root, api)
			if !ok {
				return
			}
			if !api {
				if _, err := c.Search(in.Query); err != nil {
					evidenceProblem(w, r, api, 400, "Reset evidence filters", "This list context is stale or invalid. Restart the search.", "")
					return
				}
			}
			entry, found := c.Get(r.PathValue("id"))
			if !found {
				evidenceProblem(w, r, api, 404, "Evidence record not found", "Choose an exact ID from target evidence.", "")
				return
			}
			detail, err := designevidence.Read(root, entry.ID)
			if err != nil {
				evidenceProblem(w, r, api, 500, "Evidence record needs repair", "The recorded bundle or original failed its integrity checks. No files were changed and no target compatibility can be claimed.", evidenceOriginalURL(entry, in.Values))
				return
			}
			if api {
				writeReferenceJSON(w, detail, 200)
				return
			}
			v := evidenceDetailView{Detail: detail, Back: referenceLink("/target-evidence", in.Values), Original: evidenceOriginalURL(entry, in.Values), JSON: referenceLink("/api/target-evidence/"+url.PathEscape(entry.ID), referenceThemeValues(in.Values))}
			v.Images, err = buildEvidenceImages(root, detail, in.Values)
			if err != nil {
				evidenceProblem(w, r, false, 400, "Capture selection unavailable", "Choose a retained PNG image from this record. An unsupported or damaged capture cannot be displayed; original source remains available.", v.Original)
				return
			}
			v.Back = referenceLink("/target-evidence", v.Images.Context)
			for _, gate := range []string{"reproducibility", "structure", "visual", "interaction", "lifecycle", "security", "data", "performance", "integration"} {
				v.Gates = append(v.Gates, evidenceGateView{Name: gate, Gate: detail.Contract.Gates[gate], Result: detail.Receipt.Results[gate]})
			}
			body, renderErr := htmlf("target-evidence-detail", evidenceDetailHTML, v)
			writePrivateReferenceStatusWithCSP(w, r, "Recorded target evidence", body, renderErr, 200, in.Values, evidenceImagesCSP)
		})
	}
}

func parseEvidenceRequest(v url.Values, exact bool) (evidenceRequest, error) {
	in := evidenceRequest{Values: url.Values{}, Query: designevidence.Query{Limit: 10}}
	allowed := map[string]bool{"theme": true}
	if !exact {
		for _, key := range []string{"archive_id", "subject_id", "profile", "cursor", "limit"} {
			allowed[key] = true
		}
	}
	for key, values := range v {
		if !allowed[key] || len(values) != 1 || !utf8.ValidString(values[0]) || strings.ContainsRune(values[0], 0) {
			return in, errors.New("invalid evidence query")
		}
		if values[0] != "" {
			in.Values.Set(key, values[0])
		}
	}
	if theme := v.Get("theme"); theme != "" && theme != "light" && theme != "dark" {
		return in, errors.New("invalid theme")
	}
	in.Query.ArchiveID, in.Query.SubjectID, in.Query.Profile, in.Query.Cursor = v.Get("archive_id"), v.Get("subject_id"), v.Get("profile"), v.Get("cursor")
	var err error
	in.Query.Limit, err = conversionNumber(v, "limit", 10, 50)
	return in, err
}

func evidenceReady(w http.ResponseWriter, r *http.Request, root string, api bool) (*designevidence.Catalog, bool) {
	if _, ok := referenceReady(w, r, root, api); !ok {
		return nil, false
	}
	c, err := designevidence.Load(root)
	if err != nil {
		evidenceProblem(w, r, api, 500, "Evidence index needs repair", "The metadata index could not be verified. Original references remain independently available; no files were changed.", "")
		return nil, false
	}
	return c, true
}

func evidenceOriginalURL(entry designevidence.Entry, values url.Values) string {
	q := referenceThemeValues(values)
	q.Set("file", entry.SourceMember)
	return referenceLink("/references/"+url.PathEscape(entry.Recorded.Subject.SourceArchive), q) + "#reference-source"
}

func evidenceProblem(w http.ResponseWriter, r *http.Request, api bool, status int, title, message, original string) {
	if api {
		writeReferenceJSON(w, struct {
			Error string `json:"error"`
		}{message}, status)
		return
	}
	q := referenceThemeValues(r.URL.Query())
	v := conversionProblemView{Title: title, Message: message, Back: referenceLink("/target-evidence", q), Originals: referenceLink("/references", q), Original: original}
	body, err := htmlf("target-evidence-problem", evidenceProblemHTML, v)
	recovery := r.Clone(r.Context())
	u := *r.URL
	u.Path, u.RawPath, u.RawQuery = "/target-evidence", "", q.Encode()
	recovery.URL = &u
	writePrivateReferenceStatus(w, recovery, title, body, err, status, q)
}
