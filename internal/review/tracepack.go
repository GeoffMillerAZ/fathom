package review

// tracepack.go — the standard mapping (CX-15, schema/trace/schema.cue) and
// the evidence pack's deterministic renderings (CX-16/CX-17): the matrix as
// JSON and Markdown, and the sha256 manifest. Pure — the caller reads and
// writes files; `fathom trace verify` re-renders and compares bytes.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// StandardMapping is a parsed `.fathom/trace/<standard>.cue` file. Fathom
// ships none (ADR-011): every clause is repository configuration.
type StandardMapping struct {
	Standard string          `json:"standard"`
	Title    string          `json:"title,omitempty"`
	Clauses  []MappingClause `json:"clauses"`
}

// MappingClause is one clause and the requirements that satisfy it.
type MappingClause struct {
	ID           string   `json:"id"`
	Title        string   `json:"title,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	Patterns     []string `json:"patterns,omitempty"`
}

// ParseStandardMapping decodes one mapping file's CUE source.
func ParseStandardMapping(filename string, data []byte) (StandardMapping, error) {
	v := cuecontext.New().CompileBytes(data, cue.Filename(filename))
	if v.Err() != nil {
		return StandardMapping{}, fmt.Errorf("trace mapping %s: %w", filename, v.Err())
	}
	var m StandardMapping
	if err := v.Decode(&m); err != nil {
		return StandardMapping{}, fmt.Errorf("trace mapping %s: %w", filename, err)
	}
	if m.Standard == "" {
		return StandardMapping{}, fmt.Errorf("trace mapping %s: missing standard", filename)
	}
	if len(m.Clauses) == 0 {
		return StandardMapping{}, fmt.Errorf("trace mapping %s: no clauses", filename)
	}
	for _, c := range m.Clauses {
		if c.ID == "" {
			return StandardMapping{}, fmt.Errorf("trace mapping %s: a clause has no id", filename)
		}
		for _, p := range c.Patterns {
			if _, err := regexp.Compile(p); err != nil {
				return StandardMapping{}, fmt.Errorf("trace mapping %s: clause %s pattern %q: %w", filename, c.ID, p, err)
			}
		}
	}
	return m, nil
}

// ClauseCoverage is one clause's resolved requirements.
type ClauseCoverage struct {
	ID           string   `json:"id"`
	Title        string   `json:"title,omitempty"`
	Requirements []string `json:"requirements"`
	// Missing names exact requirement ids the mapping cites that the matrix
	// does not carry — a stale mapping is named, never silently dropped.
	Missing []string `json:"missing,omitempty"`
}

// ApplyMapping resolves each clause against the matrix's requirement ids.
func ApplyMapping(m StandardMapping, mx TraceMatrix) []ClauseCoverage {
	ids := make([]string, 0, len(mx.Rows))
	have := map[string]bool{}
	for _, r := range mx.Rows {
		if !have[r.Requirement.ID] {
			have[r.Requirement.ID] = true
			ids = append(ids, r.Requirement.ID)
		}
	}
	sort.Strings(ids)
	out := make([]ClauseCoverage, 0, len(m.Clauses))
	for _, c := range m.Clauses {
		cc := ClauseCoverage{ID: c.ID, Title: c.Title, Requirements: []string{}}
		got := map[string]bool{}
		for _, r := range c.Requirements {
			if have[r] {
				got[r] = true
			} else {
				cc.Missing = append(cc.Missing, r)
			}
		}
		for _, p := range c.Patterns {
			re := regexp.MustCompile(p) // validated by ParseStandardMapping
			for _, id := range ids {
				if re.MatchString(id) {
					got[id] = true
				}
			}
		}
		for id := range got {
			cc.Requirements = append(cc.Requirements, id)
		}
		sort.Strings(cc.Requirements)
		sort.Strings(cc.Missing)
		out = append(out, cc)
	}
	return out
}

// TracePack is the evidence pack's matrix document (matrix.json).
type TracePack struct {
	Standard string           `json:"standard"`
	Commit   string           `json:"commit"`
	Service  string           `json:"service"`
	Clauses  []ClauseCoverage `json:"clauses"`
	Matrix   TraceMatrix      `json:"matrix"`
}

// RenderTraceJSON renders the pack's matrix document deterministically.
func RenderTraceJSON(p TracePack) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(p) // plain structs of strings/bools: cannot fail
	return buf.Bytes()
}

func mdCell(c TraceCell) string {
	r := strings.ReplaceAll(c.Reading, "|", `\|`)
	r = strings.ReplaceAll(r, "\n", " ")
	return c.State + ": " + r
}

// RenderTraceMarkdown renders the matrix for a human reader.
func RenderTraceMarkdown(p TracePack) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Traceability matrix: %s\n\n", p.Standard)
	fmt.Fprintf(&b, "Service `%s` at commit `%s`. Every cell is `state: reading`.\n\n", p.Service, p.Commit)
	if len(p.Clauses) > 0 {
		b.WriteString("## Clauses\n\n| Clause | Requirements | Missing |\n|---|---|---|\n")
		for _, c := range p.Clauses {
			reqs := strings.Join(c.Requirements, ", ")
			if reqs == "" {
				reqs = "(none)"
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", c.ID, reqs, strings.Join(c.Missing, ", "))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Matrix\n\n| Requirement | Kind | Status | Code | Test | Certificate | Attested run | Decision |\n|---|---|---|---|---|---|---|---|\n")
	for _, r := range p.Matrix.Rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.Requirement.ID, r.Requirement.Kind, mdCell(r.Status), mdCell(r.Code), mdCell(r.Test),
			mdCell(r.Certificate), mdCell(r.AttestedRun), mdCell(r.Decision))
	}
	return []byte(b.String())
}

// ManifestName is the pack's manifest file.
const ManifestName = "MANIFEST.sha256"

// RenderManifest renders "sha256  path" lines, sorted by path, over every
// file of the pack except the manifest itself.
func RenderManifest(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for n := range files {
		if n != ManifestName {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		sum := sha256.Sum256(files[n])
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), n)
	}
	return []byte(b.String())
}

// PackDifference names one file (and, for the matrix, the cells) that
// differ between a pack and its recomputation.
type PackDifference struct {
	File  string
	Cells []string
}

// DiffTracePacks compares a pack on disk with its recomputation, file by
// file; for matrix.json it also names every differing cell as
// "<requirement>.<column>".
func DiffTracePacks(onDisk, recomputed map[string][]byte) []PackDifference {
	names := map[string]bool{}
	for n := range onDisk {
		names[n] = true
	}
	for n := range recomputed {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	var out []PackDifference
	for _, n := range sorted {
		a, aok := onDisk[n]
		b, bok := recomputed[n]
		if aok && bok && bytes.Equal(a, b) {
			continue
		}
		d := PackDifference{File: n}
		switch {
		case !aok:
			d.Cells = []string{"(missing from pack)"}
		case !bok:
			d.Cells = []string{"(not produced by recomputation)"}
		case n == "matrix.json":
			d.Cells = diffMatrixCells(a, b)
		}
		out = append(out, d)
	}
	return out
}

func diffMatrixCells(a, b []byte) []string {
	var pa, pb TracePack
	if json.Unmarshal(a, &pa) != nil || json.Unmarshal(b, &pb) != nil {
		return []string{"(matrix.json does not parse)"}
	}
	var out []string
	if pa.Commit != pb.Commit {
		out = append(out, "commit")
	}
	if pa.Standard != pb.Standard {
		out = append(out, "standard")
	}
	rowsA := map[string]TraceRow{}
	for _, r := range pa.Matrix.Rows {
		rowsA[r.Requirement.Source+"#"+r.Requirement.ID] = r
	}
	rowsB := map[string]TraceRow{}
	for _, r := range pb.Matrix.Rows {
		rowsB[r.Requirement.Source+"#"+r.Requirement.ID] = r
	}
	keys := map[string]bool{}
	for k := range rowsA {
		keys[k] = true
	}
	for k := range rowsB {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		ra, aok := rowsA[k]
		rb, bok := rowsB[k]
		if !aok || !bok {
			out = append(out, k+" (row added or removed)")
			continue
		}
		cols := []struct {
			name string
			a, b TraceCell
		}{
			{"status", ra.Status, rb.Status}, {"code", ra.Code, rb.Code}, {"test", ra.Test, rb.Test},
			{"certificate", ra.Certificate, rb.Certificate}, {"attested_run", ra.AttestedRun, rb.AttestedRun},
			{"decision", ra.Decision, rb.Decision},
		}
		for _, c := range cols {
			if c.a != c.b {
				out = append(out, k+"."+c.name)
			}
		}
		if fmt.Sprint(ra.Requirement) != fmt.Sprint(rb.Requirement) {
			out = append(out, k+".requirement")
		}
	}
	if len(out) == 0 {
		out = append(out, "(bytes differ; no cell differs)")
	}
	return out
}
