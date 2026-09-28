package coverage

// cobertura_jacoco.go — ParseCoberturaJaCoCo (TQ-11): Cobertura's and
// JaCoCo's XML report formats, both keyed by <line> elements nested under
// a per-source-file element, differing only in element/attribute names
// and which value carries the hit fact. One exported function, not two,
// because both formats can only be told apart by their DOCUMENT (the root
// element name), never by the file extension or a caller's say-so the way
// --format already must for lcov/JSON — sniffXMLRoot resolves that from
// the bytes themselves before either format-specific unmarshal runs.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// coberturaDoc is the subset of the Cobertura coverage.xml schema
// (https://cobertura.sourceforge.net) ParseCoberturaJaCoCo reads:
// packages > classes > lines > line, each line an instrumented source
// line and its hit count.
type coberturaDoc struct {
	XMLName  xml.Name `xml:"coverage"`
	Packages []struct {
		Classes []struct {
			Filename string `xml:"filename,attr"`
			Lines    []struct {
				Number int `xml:"number,attr"`
				Hits   int `xml:"hits,attr"`
			} `xml:"lines>line"`
		} `xml:"classes>class"`
	} `xml:"packages>package"`
}

// jacocoDoc is the subset of the JaCoCo report.xml schema
// (https://www.jacoco.org/jacoco/trunk/coverage) ParseCoberturaJaCoCo
// reads: package > sourcefile > line, each line carrying missed/covered
// INSTRUCTION counts (mi/ci) rather than a hit count — a line counts as
// covered here when ci > 0 (at least one instruction on it ran), the same
// "did this line run at all" question every other T3 format answers at
// line granularity.
type jacocoDoc struct {
	XMLName  xml.Name `xml:"report"`
	Packages []struct {
		Sourcefiles []struct {
			Name  string `xml:"name,attr"`
			Lines []struct {
				Nr int `xml:"nr,attr"`
				Mi int `xml:"mi,attr"`
				Ci int `xml:"ci,attr"`
			} `xml:"line"`
		} `xml:"sourcefile"`
	} `xml:"package"`
}

// sniffXMLRoot returns data's outermost element's local name (namespace
// prefix, if any, stripped by encoding/xml itself), reading only as far
// as the first StartElement — never the whole document twice.
func sniffXMLRoot(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("cobertura/jacoco: reading XML root element: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

// ParseCoberturaJaCoCo parses a Cobertura or JaCoCo XML coverage report,
// detected from its root element (<coverage> for Cobertura, <report> for
// JaCoCo — any other root is a reportable error, never a silent guess),
// into ForeignBlock at line granularity: Statements is always 1 per
// <line>, Covered 1 when Cobertura's hits > 0 or JaCoCo's ci > 0. Output
// order mirrors the document's own <line> order, which — given identical
// input bytes — is already deterministic; nothing here reorders it.
func ParseCoberturaJaCoCo(r io.Reader) ([]ForeignBlock, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cobertura/jacoco: read: %w", err)
	}
	root, err := sniffXMLRoot(data)
	if err != nil {
		return nil, err
	}
	switch root {
	case "coverage":
		return parseCobertura(data)
	case "report":
		return parseJaCoCo(data)
	default:
		return nil, fmt.Errorf("cobertura/jacoco: unrecognized XML root element %q (want <coverage> for Cobertura or <report> for JaCoCo)", root)
	}
}

func parseCobertura(data []byte) ([]ForeignBlock, error) {
	var doc coberturaDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("cobertura: unmarshal: %w", err)
	}
	var out []ForeignBlock
	for _, pkg := range doc.Packages {
		for _, cls := range pkg.Classes {
			for _, ln := range cls.Lines {
				covered := 0
				if ln.Hits > 0 {
					covered = 1
				}
				out = append(out, ForeignBlock{
					File:       cls.Filename,
					StartLine:  ln.Number,
					EndLine:    ln.Number,
					Statements: 1,
					Covered:    covered,
				})
			}
		}
	}
	return out, nil
}

func parseJaCoCo(data []byte) ([]ForeignBlock, error) {
	var doc jacocoDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("jacoco: unmarshal: %w", err)
	}
	var out []ForeignBlock
	for _, pkg := range doc.Packages {
		for _, sf := range pkg.Sourcefiles {
			for _, ln := range sf.Lines {
				covered := 0
				if ln.Ci > 0 {
					covered = 1
				}
				out = append(out, ForeignBlock{
					File:       sf.Name,
					StartLine:  ln.Nr,
					EndLine:    ln.Nr,
					Statements: 1,
					Covered:    covered,
				})
			}
		}
	}
	return out, nil
}
