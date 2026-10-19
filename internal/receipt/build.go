package receipt

// build.go — EVX-7 (docs/plans/portable-receipts.md; vision 43 D4, D11,
// section 6.2): BuildReceipt, the one allowlist constructor for an exported
// receipt, after internal/research/bundle's Build. It writes the existing
// core.Receipt JSON, only the fields the receipt's kind carries, with
// anchored facts added beside them, so `fathom receipt verify` reads an
// exported receipt as it reads a saved one.

import (
	"bytes"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
	evidencesignature "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-signature"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// MaxExcerptLines bounds an anchor's excerpt (D11). The excerpt digest
// still covers the whole span.
const MaxExcerptLines = 40

// FactKind is the closed set of facts an exported receipt carries. A
// judgment is never one (I62).
type FactKind string

const (
	// FactDefinition: the subject is defined at the anchored span.
	FactDefinition FactKind = "definition"
	// FactCaller: the subject (a caller) is defined at the anchored span
	// and calls the receipt's symbol.
	FactCaller FactKind = "caller"
	// FactAbsence: the reads found none; the fact carries its read set.
	FactAbsence FactKind = "absence"
)

// ExportInput is the only channel by which data reaches BuildReceipt.
type ExportInput struct {
	// Receipt is the envelope; BuildReceipt copies only the fields its
	// Kind carries.
	Receipt core.Receipt
	Facts   []FactInput
	// NoExcerpts keeps digests only (--no-excerpts).
	NoExcerpts bool
}

// FactInput is one fact before the allowlist.
type FactInput struct {
	Kind    FactKind
	Subject string
	Anchors []CitedAnchor
	// Reads, TreeHash, Tool and Noun are an absence fact's: what the
	// producer read (I38's read manifest), the tree it read, the engine,
	// and what the count counts ("caller(s)").
	Reads    *readmanifest.Manifest
	TreeHash string
	Tool     string
	Noun     string
}

// CitedAnchor is an anchor and the bytes its excerpt digest covers
// (AnchorExcerpt).
type CitedAnchor struct {
	Anchor  Anchor
	Excerpt []byte
}

// ExportedReceipt is the exported receipt: core.Receipt's JSON with the
// anchored facts added.
type ExportedReceipt struct {
	core.Receipt
	Facts []Fact `json:"facts,omitempty"`
}

// Fact is one anchored fact.
type Fact struct {
	Kind    FactKind          `json:"kind"`
	Subject string            `json:"subject,omitempty"`
	Anchors []ExportAnchor    `json:"anchors,omitempty"`
	ReadSet *CoverageManifest `json:"read_set,omitempty"`
	// MissingReads names every read the count could not count on.
	MissingReads []MissingRead `json:"missing_reads,omitempty"`
	// Count is an absence fact's count, a named lower bound when a read
	// is missing (I38).
	Count      string `json:"count,omitempty"`
	LowerBound bool   `json:"lower_bound,omitempty"`
}

// CoverageManifest is agentic-kit's evidence-signature shape for a read
// set.
type CoverageManifest = evidencesignature.CoverageManifest

// MissingRead is a path the producer could not read, and why.
type MissingRead struct {
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// ExportAnchor is an anchor with its bounded excerpt.
type ExportAnchor struct {
	Anchor
	Excerpt string `json:"excerpt,omitempty"`
	// ExcerptEndLine is the excerpt's last line when MaxExcerptLines cut
	// it short of the span.
	ExcerptEndLine int `json:"excerpt_end_line,omitempty"`
}

// withheldExportReason is the one reason a withheld anchor carries: a
// suppression's own text never leaves.
const withheldExportReason = "withheld: no excerpt and no digest leave"

// BuildReceipt assembles the exported receipt from in. Every field written
// is copied from a named field of in or computed here, per kind, so a
// field the allowlist does not name cannot leak through. A receipt or fact
// kind outside its closed set, and an absence fact with no read set, are
// errors.
func BuildReceipt(in ExportInput) (ExportedReceipt, error) {
	src := in.Receipt
	r := core.Receipt{Kind: src.Kind, Tool: src.Tool, Service: src.Service, Symbol: src.Symbol, IndexCommit: src.IndexCommit}
	switch src.Kind {
	case core.ReceiptKindCallers:
		r.Callers = sortedCopy(src.Callers)
	case core.ReceiptKindReferences:
		r.Files = sortedCopy(src.Files)
	case core.ReceiptKindSymbolLocation:
		r.File, r.Line = src.File, src.Line
	case core.ReceiptKindCertification:
		r.Test, r.Target, r.Strength, r.RunDigest = src.Test, src.Target, src.Strength, src.RunDigest
		r.Reached = sortedCopy(src.Reached)
	default:
		return ExportedReceipt{}, fmt.Errorf("receipt: kind %q is not one an exported receipt carries", src.Kind)
	}
	out := ExportedReceipt{Receipt: r}
	for _, fi := range in.Facts {
		f, err := buildFact(fi, in.NoExcerpts)
		if err != nil {
			return ExportedReceipt{}, err
		}
		out.Facts = append(out.Facts, f)
	}
	sort.SliceStable(out.Facts, func(i, j int) bool {
		if out.Facts[i].Kind != out.Facts[j].Kind {
			return out.Facts[i].Kind < out.Facts[j].Kind
		}
		return out.Facts[i].Subject < out.Facts[j].Subject
	})
	return out, nil
}

func buildFact(fi FactInput, noExcerpts bool) (Fact, error) {
	switch fi.Kind {
	case FactDefinition, FactCaller, FactAbsence:
	default:
		return Fact{}, fmt.Errorf("receipt: fact kind %q is not one an exported receipt carries", fi.Kind)
	}
	f := Fact{Kind: fi.Kind, Subject: fi.Subject}
	for _, a := range fi.Anchors {
		f.Anchors = append(f.Anchors, exportAnchor(a, noExcerpts))
	}
	sort.SliceStable(f.Anchors, func(i, j int) bool {
		a, b := f.Anchors[i], f.Anchors[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.EndLine < b.EndLine
	})
	if fi.Kind != FactAbsence {
		return f, nil
	}
	if fi.Reads == nil {
		return Fact{}, fmt.Errorf("receipt: the absence of %q carries no read set", fi.Subject)
	}
	cov, blind := coverageOf(*fi.Reads, fi.TreeHash, fi.Tool)
	f.ReadSet = &cov
	for _, e := range blind.Entries {
		f.MissingReads = append(f.MissingReads, MissingRead{Path: e.Path, Detail: e.Outcome.Detail})
	}
	f.LowerBound = len(f.MissingReads) > 0
	f.Count = readcoverage.RenderCount(blind, 0, fi.Noun)
	return f, nil
}

// exportAnchor copies a's allowlisted fields and its excerpt, bounded. A
// withheld anchor keeps no blob id, digest, fingerprint or excerpt, and
// none of its reason's text; an anchor with no excerpt digest carries no
// excerpt.
func exportAnchor(c CitedAnchor, noExcerpts bool) ExportAnchor {
	a := c.Anchor
	e := ExportAnchor{Anchor: Anchor{
		Commit: a.Commit, ObjectFormat: a.ObjectFormat, Path: a.Path, StartLine: a.StartLine, EndLine: a.EndLine,
		BlobID: a.BlobID, ExcerptSHA256: a.ExcerptSHA256, FathomFingerprint: a.FathomFingerprint,
		State: a.State, Clean: a.Clean, Reason: a.Reason,
	}}
	if a.State == StateWithheld {
		e.BlobID, e.ExcerptSHA256, e.FathomFingerprint, e.Reason = "", "", "", withheldExportReason
		return e
	}
	if noExcerpts || a.ExcerptSHA256 == "" || len(c.Excerpt) == 0 || !utf8.Valid(c.Excerpt) {
		return e
	}
	text, cut := boundLines(c.Excerpt, MaxExcerptLines)
	e.Excerpt = string(text)
	if cut {
		first := a.StartLine
		if first == 0 {
			first = 1
		}
		e.ExcerptEndLine = first + MaxExcerptLines - 1
	}
	return e
}

// boundLines is b's first n lines, and whether that cut b short.
func boundLines(b []byte, n int) ([]byte, bool) {
	at := 0
	for i := 0; i < n; i++ {
		j := bytes.IndexByte(b[at:], '\n')
		if j < 0 {
			return b, false
		}
		at += j + 1
	}
	return b[:at], at < len(b)
}

// coverageOf renders m in the kit's CoverageManifest shape: read files,
// skips by the kit's categories. blind is what the count cannot count on,
// as unreadable entries: m's unreadable reads, skips the kit calls
// unexplained (they could hold a match), and skips in a category the kit
// does not name.
func coverageOf(m readmanifest.Manifest, tree, tool string) (cov CoverageManifest, blind readmanifest.Manifest) {
	cov = CoverageManifest{Files: []string{}, TreeHash: tree, Tool: tool}
	missing := func(path, detail string) {
		blind.Entries = append(blind.Entries, readmanifest.Entry{Path: path, Outcome: readmanifest.Outcome{Kind: readmanifest.KindUnreadable, Detail: detail}})
	}
	for _, e := range m.Entries {
		switch e.Outcome.Kind {
		case readmanifest.KindRead:
			cov.Files = append(cov.Files, e.Path)
		case readmanifest.KindUnreadable:
			missing(e.Path, e.Outcome.Detail)
		case readmanifest.KindSkipped:
			sk := &cov.Skipped
			switch e.Outcome.Detail {
			case "ignored":
				sk.Ignored = append(sk.Ignored, e.Path)
			case "symlink":
				sk.Symlink = append(sk.Symlink, e.Path)
			case "binary":
				sk.Binary = append(sk.Binary, e.Path)
				missing(e.Path, "skipped (binary)")
			case "oversize":
				sk.Oversize = append(sk.Oversize, e.Path)
				missing(e.Path, "skipped (oversize)")
			case "lfs_stub":
				sk.LFSStub = append(sk.LFSStub, e.Path)
				missing(e.Path, "skipped (lfs_stub)")
			case "non_utf8":
				sk.NonUTF8 = append(sk.NonUTF8, e.Path)
				missing(e.Path, "skipped (non_utf8)")
			default:
				missing(e.Path, "skipped ("+e.Outcome.Detail+")")
			}
		}
		// An absent path is not there to read: neither read nor missing.
	}
	sort.Strings(cov.Files)
	for _, s := range []*[]string{&cov.Skipped.Ignored, &cov.Skipped.Symlink, &cov.Skipped.Binary, &cov.Skipped.Oversize, &cov.Skipped.LFSStub, &cov.Skipped.NonUTF8} {
		sort.Strings(*s)
	}
	sort.Slice(blind.Entries, func(i, j int) bool { return blind.Entries[i].Path < blind.Entries[j].Path })
	return cov, blind
}

func sortedCopy(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}
