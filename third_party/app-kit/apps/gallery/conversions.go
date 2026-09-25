package gallery

import (
	"errors"
	"fmt"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designattempts"
)

type conversionRequest struct {
	Query                           designattempts.Query
	Values                          url.Values
	SourceLine, ArtifactLine, Lines int
}

// Conversion routes inherit the original library's private, same-origin,
// GET-only boundary. They never publish attempts or execute vendor code.
func registerConversions(root string, register func(string, bool, http.HandlerFunc)) {
	for _, api := range []bool{false, true} {
		base := "/conversions"
		if api {
			base = "/api/conversions"
		}
		register(base, api, func(w http.ResponseWriter, r *http.Request) {
			in, ok := conversionParse(w, r, api, "list", base)
			if !ok {
				return
			}
			c, ok := conversionReady(w, r, root, api)
			if !ok {
				return
			}
			page, err := c.Search(in.Query)
			if err != nil {
				conversionBadQuery(w, r, api)
				return
			}
			if api {
				writeReferenceJSON(w, page, 200)
				return
			}
			v := conversionIndexModel(c, page, in)
			body, err := htmlf("conversion-index", conversionIndexHTML, v)
			writePrivateReferenceStatus(w, r, "Saved conversions", body, err, 200, in.Values)
		})
		register(base+"/{id}", api, func(w http.ResponseWriter, r *http.Request) {
			mode := "detail"
			if api {
				mode = "exact"
			}
			in, ok := conversionParse(w, r, api, mode, base+"/"+r.PathValue("id"))
			if !ok {
				return
			}
			c, ok := conversionReady(w, r, root, api)
			if !ok {
				return
			}
			if !api {
				if _, err := c.Search(in.Query); err != nil {
					conversionBadQuery(w, r, false)
					return
				}
			}
			detail, ok := conversionExact(w, r, root, c, api)
			if !ok {
				return
			}
			if api {
				writeReferenceJSON(w, detail, 200)
				return
			}
			original, err := designassets.ReadFile(root, detail.Entry.Source.ArchiveID, detail.Entry.Source.Path)
			if err != nil {
				conversionProblem(w, r, false, 500, "Original source could not be read", "The source changed or failed its integrity checks. Repair the private archive before comparing this attempt.", conversionOriginalURL(detail.Entry, in.Values))
				return
			}
			v, err := conversionDetailModel(detail, original.Content, in)
			if err != nil {
				conversionBadQuery(w, r, false)
				return
			}
			body, err := htmlf("conversion-detail", conversionDetailHTML, v)
			writePrivateReferenceStatus(w, r, "Original and converted source", body, err, 200, in.Values)
		})
	}
	register("/conversions/{id}/artifact", false, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := conversionParse(w, r, false, "exact", "/conversions/"+r.PathValue("id")+"/artifact"); !ok {
			return
		}
		c, ok := conversionReady(w, r, root, false)
		if !ok {
			return
		}
		detail, ok := conversionExact(w, r, root, c, false)
		if !ok {
			return
		}
		a := detail.Translation.Artifact
		if a == nil {
			conversionProblem(w, r, false, 409, "No converted artifact", "This attempt requires adaptation. Keep the original source or inspect its diagnostics before choosing another renderer.", conversionOriginalURL(detail.Entry, r.URL.Query()))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
		w.Header().Set("Content-Length", strconv.Itoa(len(a.Content)))
		w.Header().Set("X-Artifact-SHA256", a.SHA256)
		w.Header().Set("X-Conversion-Replay", detail.Replay)
		w.Header().Set("X-Conversion-Verification", detail.Entry.Verification)
		_, _ = w.Write([]byte(a.Content))
	})
}

func conversionReady(w http.ResponseWriter, r *http.Request, root string, api bool) (*designattempts.Catalog, bool) {
	if _, ok := referenceReady(w, r, root, api); !ok {
		return nil, false
	}
	c, err := designattempts.Load(root)
	if err != nil {
		conversionProblem(w, r, api, 500, "Saved conversions need repair", "The conversion index could not be verified. Original references remain available independently; no files were changed.", "")
		return nil, false
	}
	return c, true
}

func conversionExact(w http.ResponseWriter, r *http.Request, root string, c *designattempts.Catalog, api bool) (designattempts.Detail, bool) {
	entry, found := c.Get(r.PathValue("id"))
	if !found {
		conversionProblem(w, r, api, 404, "Saved conversion not found", "Choose an exact attempt ID from saved conversions.", "")
		return designattempts.Detail{}, false
	}
	detail, err := designattempts.Read(root, entry.ID)
	if err != nil {
		conversionProblem(w, r, api, 500, "Saved conversion failed its integrity checks", "This known attempt or its original source is missing, changed, or inconsistent. No compatibility claim can be made. The original library remains a separate recovery path.", conversionOriginalURL(entry, r.URL.Query()))
		return designattempts.Detail{}, false
	}
	return detail, true
}

func conversionParse(w http.ResponseWriter, r *http.Request, api bool, mode, route string) (conversionRequest, bool) {
	in, err := parseConversionRequest(r.URL.Query(), mode)
	// Canonical route segments prevent encoded aliases for exact private IDs.
	if err != nil || r.URL.EscapedPath() != route || strings.Contains(r.PathValue("id"), "/") {
		conversionBadQuery(w, r, api)
		return conversionRequest{}, false
	}
	return in, true
}

func parseConversionRequest(v url.Values, mode string) (conversionRequest, error) {
	in := conversionRequest{Values: url.Values{}, SourceLine: 1, ArtifactLine: 1, Lines: 100}
	allowed := map[string]bool{"theme": true}
	if mode != "exact" {
		for _, key := range []string{"archive_id", "path", "status", "limit", "cursor"} {
			allowed[key] = true
		}
	}
	if mode == "detail" {
		for _, key := range []string{"source_line", "artifact_line", "lines"} {
			allowed[key] = true
		}
	}
	for key, values := range v {
		if !allowed[key] || len(values) != 1 || !utf8.ValidString(values[0]) || strings.ContainsRune(values[0], 0) {
			return in, errors.New("invalid query field")
		}
		if values[0] != "" {
			in.Values.Set(key, values[0])
		}
	}
	if theme := v.Get("theme"); theme != "" && theme != "light" && theme != "dark" {
		return in, errors.New("invalid theme")
	}
	var err error
	in.Query = designattempts.Query{ArchiveID: v.Get("archive_id"), Path: v.Get("path"), Status: v.Get("status"), Cursor: v.Get("cursor")}
	if in.Query.Limit, err = conversionNumber(v, "limit", 10, 50); err != nil {
		return in, err
	}
	if in.SourceLine, err = conversionNumber(v, "source_line", 1, 4<<20); err != nil {
		return in, err
	}
	if in.ArtifactLine, err = conversionNumber(v, "artifact_line", 1, 4<<20); err != nil {
		return in, err
	}
	if in.Lines, err = conversionNumber(v, "lines", 100, 400); err != nil {
		return in, err
	}
	return in, nil
}

func conversionNumber(v url.Values, key string, fallback, maximum int) (int, error) {
	if !v.Has(key) {
		return fallback, nil
	}
	n, err := strconv.Atoi(v.Get(key))
	if err != nil || n < 1 || n > maximum || strconv.Itoa(n) != v.Get(key) {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}

func conversionListValues(v url.Values) url.Values {
	out := referenceThemeValues(v)
	for _, key := range []string{"archive_id", "path", "status", "limit", "cursor"} {
		if value := v.Get(key); value != "" {
			out.Set(key, value)
		}
	}
	return out
}

func conversionOriginalURL(entry designattempts.Entry, values url.Values) string {
	q := referenceThemeValues(values)
	q.Set("file", entry.Source.Path)
	return referenceLink("/references/"+url.PathEscape(entry.Source.ArchiveID), q) + "#reference-source"
}

func conversionBadQuery(w http.ResponseWriter, r *http.Request, api bool) {
	conversionProblem(w, r, api, 400, "Reset conversion filters or source lines", "Use exact archive and member IDs, a valid status, limit 1..50, source lines within this file, and lines 1..400. An updated index can invalidate a cursor. Unknown, repeated or malformed parameters are refused.", "")
}

func conversionProblem(w http.ResponseWriter, r *http.Request, api bool, status int, title, message, original string) {
	if api {
		writeReferenceJSON(w, struct {
			Error string `json:"error"`
		}{message}, status)
		return
	}
	q := referenceThemeValues(r.URL.Query())
	v := conversionProblemView{Title: title, Message: message, Back: referenceLink("/conversions", q), Originals: referenceLink("/references", q), Original: original}
	body, err := htmlf("conversion-problem", conversionProblemHTML, v)
	recovery := r.Clone(r.Context())
	u := *r.URL
	u.Path, u.RawPath, u.RawQuery = "/conversions", "", q.Encode()
	recovery.URL = &u
	writePrivateReferenceStatus(w, recovery, title, body, err, status, maps.Clone(q))
}
