package certify

// riskinputs.go — TQ-3 (docs/plans/test-intelligence.md; design doc
// docs/design/test-intelligence.md §1 owner decision, §3 T1): the two
// per-file T1 risk factors fathom holds from governance rather than from
// the graph or git.
//
//   - severity: 1.0 when an enforced guard (enforce: true in
//     .fathom/guard.cue) scopes the file, 0.5 when any FRESH obligation
//     claim's scope matches it, 0.1 otherwise. An advisory guard or a
//     stale claim does not raise it.
//   - incidents: incident records (docs/intent/incidents.md "### INC-N"
//     sections) whose text names the file, plus served finding claims
//     scoped to it. A refuted or superseded finding never counts.
//
// Pure over already-loaded inputs; the loaders below read exactly two
// named files (the guard file and the incidents doc), never the tree.

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// Severity levels (design doc §1 owner decision).
const (
	severityEnforcedGuard = 1.0
	severityObligation    = 0.5
	severityNone          = 0.1
)

// guardFilePath is the repo-relative guard file severity cites.
const guardFilePath = ".fathom/guard.cue"

// fileRisk is one file's governance severity and incident count, each
// with the citations that justify it.
type fileRisk struct {
	Severity      float64
	SeverityCite  string
	Incidents     int
	IncidentCites []string
}

// incidentRecord is one "### INC-N" section of docs/intent/incidents.md.
type incidentRecord struct {
	ID   string
	Line int
	Body string
}

// riskInputs computes fileRisk for every file in files.
func riskInputs(files []string, guards []guardspec.Guard, claims []core.SemanticClaim, incidents []incidentRecord) map[string]fileRisk {
	type guardM struct {
		m    *filter.Matcher
		cite string
	}
	var enforced []guardM
	for _, g := range guards {
		if !g.Enforce || len(g.Scopes) == 0 {
			continue
		}
		enforced = append(enforced, guardM{filter.New(g.Scopes), fmt.Sprintf("%s: enforced guard %s", guardFilePath, strings.Join(g.Scopes, ","))})
	}
	type claimM struct {
		ms   []*filter.Matcher
		cite string
	}
	var obligations, findings []claimM
	for _, c := range claims {
		var dst *[]claimM
		switch {
		case c.Kind == "obligation" && c.Status == core.ClaimFresh:
			dst = &obligations
		case c.Kind == "finding" && c.Status.Served():
			dst = &findings
		default:
			continue
		}
		cm := claimM{cite: c.Subject}
		if cm.cite == "" {
			cm.cite = c.ClaimID
		}
		for _, g := range c.Scope {
			cm.ms = append(cm.ms, filter.New([]string{g}))
		}
		*dst = append(*dst, cm)
	}
	matches := func(ms []*filter.Matcher, f string) bool {
		for _, m := range ms {
			if m.Match(f) {
				return true
			}
		}
		return false
	}

	out := make(map[string]fileRisk, len(files))
	for _, f := range files {
		if _, done := out[f]; done {
			continue
		}
		r := fileRisk{Severity: severityNone, SeverityCite: "no enforced guard or fresh obligation governs this file"}
		for _, g := range enforced {
			if g.m.Match(f) {
				r.Severity, r.SeverityCite = severityEnforcedGuard, g.cite
				break
			}
		}
		if r.Severity < severityObligation {
			for _, c := range obligations {
				if matches(c.ms, f) {
					r.Severity, r.SeverityCite = severityObligation, "obligation "+c.cite
					break
				}
			}
		}
		for _, inc := range incidents {
			if namesFile(inc.Body, f) {
				r.IncidentCites = append(r.IncidentCites, incidentHeadingPath+"#"+inc.ID)
			}
		}
		var fc []string
		for _, c := range findings {
			if matches(c.ms, f) {
				fc = append(fc, "finding "+c.cite)
			}
		}
		sort.Strings(fc)
		r.IncidentCites = append(r.IncidentCites, fc...)
		r.Incidents = len(r.IncidentCites)
		out[f] = r
	}
	return out
}

// namesFile reports whether text names path as a whole path: the byte
// before a match must not continue a path or identifier, so
// "internal/store/filechurn.go" never names "churn.go".
func namesFile(text, path string) bool {
	if path == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(text[i:], path)
		if j < 0 {
			return false
		}
		at := i + j
		if at == 0 || !isPathByte(text[at-1]) {
			return true
		}
		i = at + 1
	}
}

func isPathByte(b byte) bool {
	return b == '/' || b == '.' || b == '_' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// parseIncidentRecords splits an incidents doc into its "### INC-N"
// sections, each running to the next heading of level three or higher.
func parseIncidentRecords(body []byte) []incidentRecord {
	locs := incidentHeadingRe.FindAllSubmatchIndex(body, -1)
	var out []incidentRecord
	for _, m := range locs {
		start := m[0]
		end := len(body)
		// the section ends at the next line starting with "#", "##" or "###"
		rest := body[m[1]:]
		for off := 0; off < len(rest); {
			nl := bytes.IndexByte(rest[off:], '\n')
			if nl < 0 {
				break
			}
			line := rest[off+nl+1:]
			if bytes.HasPrefix(line, []byte("#")) && !bytes.HasPrefix(line, []byte("####")) {
				end = m[1] + off + nl + 1
				break
			}
			off += nl + 1
		}
		out = append(out, incidentRecord{
			ID:   string(body[m[2]:m[3]]),
			Line: bytes.Count(body[:start], []byte("\n")) + 1,
			Body: string(body[start:end]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// loadIncidentRecords reads corpusRoot's docs/intent/incidents.md; a
// missing file yields none, an unreadable one is recorded in reads.
func loadIncidentRecords(corpusRoot string, reads *readmanifest.Set) []incidentRecord {
	if corpusRoot == "" {
		return nil
	}
	body, ok := reads.ReadFile(filepath.Join(corpusRoot, "docs", "intent", "incidents.md"))
	if !ok {
		return nil
	}
	return parseIncidentRecords(body)
}

// loadGuards reads corpusRoot's .fathom/guard.cue; a missing or invalid
// file yields no guards (severity then rests on claims alone).
func loadGuards(corpusRoot string) []guardspec.Guard {
	if corpusRoot == "" {
		return nil
	}
	gs, err := guardspec.LoadGuards(filepath.Join(corpusRoot, ".fathom", "guard.cue"))
	if err != nil {
		return nil
	}
	return gs.Guards
}
