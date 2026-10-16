package gallery

import (
	"errors"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designattempts"
)

type conversionRow struct {
	Entry                            designattempts.Entry
	Title, Status, Compare, Original string
}
type conversionArchive struct{ ID, Label string }
type conversionIndexView struct {
	Page                          designattempts.Page
	Query                         designattempts.Query
	Rows                          []conversionRow
	Archives                      []conversionArchive
	Theme, Originals, Reset, Next string
}
type conversionPane struct {
	Content, Previous, Next  string
	Start, End, Total, Bytes int
	TooLarge                 bool
}
type conversionDetailView struct {
	Detail                                            designattempts.Detail
	Source, Artifact                                  conversionPane
	Status, Replay, ReplayNotice                      string
	Back, Original, SourceJSON, ReceiptJSON, Download string
	Route                                             string
	PreparedComparison                                string
	Lines                                             int
	Context                                           []referenceInput
}
type conversionProblemView struct{ Title, Message, Back, Originals, Original string }

func conversionStatus(status string) string {
	if status == "translated" {
		return "Translated source"
	}
	return "Requires adaptation"
}

func conversionIndexModel(c *designattempts.Catalog, page designattempts.Page, in conversionRequest) conversionIndexView {
	v := conversionIndexView{Page: page, Query: in.Query, Theme: in.Values.Get("theme"), Originals: referenceLink("/references", referenceThemeValues(in.Values)), Reset: referenceLink("/conversions", referenceThemeValues(in.Values))}
	seen := map[string]bool{}
	for _, entry := range c.List() {
		seen[entry.Source.ArchiveID] = true
	}
	// Retain an exact filter even if this archive has no saved attempts yet.
	if in.Query.ArchiveID != "" {
		seen[in.Query.ArchiveID] = true
	}
	for id := range seen {
		family, hash, _ := strings.Cut(id, "@")
		v.Archives = append(v.Archives, conversionArchive{ID: id, Label: referenceTitle(family) + " · " + hash[:min(12, len(hash))]})
	}
	slices.SortFunc(v.Archives, func(a, b conversionArchive) int { return strings.Compare(a.ID, b.ID) })
	for _, entry := range page.Entries {
		family, _, _ := strings.Cut(entry.Source.ArchiveID, "@")
		v.Rows = append(v.Rows, conversionRow{Entry: entry, Title: referenceTitle(family), Status: conversionStatus(entry.Status), Compare: referenceLink("/conversions/"+url.PathEscape(entry.ID), in.Values), Original: conversionOriginalURL(entry, in.Values)})
	}
	if page.NextCursor != "" {
		next := maps.Clone(in.Values)
		next.Set("cursor", page.NextCursor)
		v.Next = referenceLink("/conversions", next)
	}
	return v
}

func conversionDetailModel(detail designattempts.Detail, original string, in conversionRequest) (conversionDetailView, error) {
	v := conversionDetailView{Detail: detail, Status: conversionStatus(detail.Entry.Status), Back: referenceLink("/conversions", conversionListValues(in.Values)), Original: conversionOriginalURL(detail.Entry, in.Values)}
	base := "/conversions/" + url.PathEscape(detail.Entry.ID)
	v.Route, v.Lines = base, in.Lines
	for _, key := range []string{"archive_id", "path", "status", "limit", "cursor", "theme"} {
		if value := in.Values.Get(key); value != "" {
			v.Context = append(v.Context, referenceInput{Name: key, Value: value})
		}
	}
	theme := referenceThemeValues(in.Values)
	if d, err := pricingReferenceDescriptor(); err == nil && detail.Entry.ID == d.Receipt.AttemptID {
		v.PreparedComparison = referenceLink(pricingReferenceRoute, theme)
	}
	v.ReceiptJSON = referenceLink("/api/conversions/"+url.PathEscape(detail.Entry.ID), theme)
	v.Download = referenceLink(base+"/artifact", theme)
	sourceQuery := maps.Clone(theme)
	sourceQuery.Set("path", detail.Entry.Source.Path)
	v.SourceJSON = referenceLink("/api/references/"+url.PathEscape(detail.Entry.Source.ArchiveID)+"/source", sourceQuery)
	v.Replay = "Matches current transformer"
	v.ReplayNotice = "The stored result replayed exactly against the verified original for this request. This checks deterministic translation, not visual or behavioral equivalence."
	if detail.Replay == "stale-transformer" {
		v.Replay = "Stale transformer"
		v.ReplayNotice = "This historical attempt passed stored integrity checks, but its transformer differs from the current one. Current replay has not passed. Inspect or download the saved bytes; regenerate explicitly before relying on current rules."
	}
	var err error
	if v.Source, err = conversionSourcePane(original, in.SourceLine, in.Lines, "source_line", "conversion-original", base, in.Values); err != nil {
		return v, err
	}
	if artifact := detail.Translation.Artifact; artifact != nil {
		if v.Artifact, err = conversionSourcePane(artifact.Content, in.ArtifactLine, in.Lines, "artifact_line", "conversion-artifact", base, in.Values); err != nil {
			return v, err
		}
	} else if in.ArtifactLine != 1 {
		return v, errors.New("unsupported attempt has no artifact lines")
	}
	return v, nil
}

// A line page preserves every byte, including CRLF and the final newline. It
// scans offsets without allocating a slice per line. Oversized windows expose
// no truncated code; the complete bounded JSON/download remains available.
func conversionSourcePane(content string, start, limit int, parameter, anchor, route string, values url.Values) (conversionPane, error) {
	total := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") || content == "" {
		total++
	}
	if start < 1 || start > total || limit < 1 || limit > 400 {
		return conversionPane{}, errors.New("line window out of bounds")
	}
	end := min(start+limit-1, total)
	from, to, line := 0, len(content), 1
	for offset := 0; offset < len(content); offset++ {
		if content[offset] != '\n' {
			continue
		}
		if line < start {
			from = offset + 1
		}
		if line == end {
			to = offset + 1
			break
		}
		line++
	}
	p := conversionPane{Start: start, End: end, Total: total, Bytes: to - from, TooLarge: to-from > 32<<10}
	if !p.TooLarge {
		p.Content = content[from:to]
	}
	link := func(n int) string {
		q := maps.Clone(values)
		q.Set(parameter, strconv.Itoa(n))
		return referenceLink(route, q) + "#" + anchor
	}
	if start > 1 {
		p.Previous = link(max(1, start-limit))
	}
	if end < total {
		p.Next = link(end + 1)
	}
	return p, nil
}
