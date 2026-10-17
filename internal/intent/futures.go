package intent

// futures.go — the futures parser (IV-5, ADR-074 decision 5). A future is
// authored in Markdown ("## F<n>. <title>" and its bullets); ParseFutures
// reads its parsed form, schema/intent #Future declares that form, and
// VetFuture checks one against the embedded copy of the schema, so the
// gate's test and `fathom intent audit` (U16) vet one shape.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
)

// intentSchema is byte-identical to schema/intent/schema.cue, held to it
// by TestIntentSchemaMatchesSource.
//
//go:embed intent_schema.cue
var intentSchema []byte

// Future is one future direction's parsed form. The JSON field names are
// schema/intent #Future's; File and Line are its Markdown position, never
// part of the contract.
type Future struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Confidence string            `json:"confidence,omitempty"`
	Horizon    string            `json:"horizon,omitempty"`
	Value      *FutureValue      `json:"value,omitempty"`
	Basis      *FutureBasis      `json:"basis,omitempty"`
	Conditions []FutureCondition `json:"conditions,omitempty"`
	File       string            `json:"-"`
	Line       int               `json:"-"`
}

// FutureValue is "Value: <level>, <why>".
type FutureValue struct {
	Level string `json:"level"`
	Why   string `json:"why"`
}

// FutureBasis is "Basis: <kind>[, <cite>]".
type FutureBasis struct {
	Kind string `json:"kind"`
	Cite string `json:"cite,omitempty"`
}

// FutureCondition is one labelled condition, its annotation removed.
type FutureCondition struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// futureFieldRe finds a rating label where a field opens: at the start of a
// bullet or a sentence (after "." or ";"), optionally bold. Evidence is
// matched only so it ends the field before it; a label mid-sentence is
// prose, never a field.
var futureFieldRe = regexp.MustCompile(`(?:^|[.;]\s+)\**(Confidence|Horizon|Value|Basis|Evidence)\**:\s*`)

// ParseFutures reads every "## F<n>. <title>" record in body (rel is its
// repo-relative path). Each field is the first of its label in the record;
// its value is the token after the label, kept whole up to whitespace or
// punctuation (so "later/someday" stays off-list and fails the vet), and a
// Value's why or a Basis's cite is the rest of the field. A record missing
// a field leaves it empty, which the vet names. Conditions come from the
// corpus's own condition parser, so the two never disagree.
func ParseFutures(rel string, body []byte) []Future {
	lines := splitLines(string(body))
	var scratch Corpus
	scratch.parseConditionsWith(rel, string(body), newCondState("futures"))

	var out []Future
	var blocks []string
	var cur strings.Builder
	open := false // inside a future's record
	flushBlock := func() {
		if cur.Len() > 0 {
			blocks = append(blocks, cur.String())
			cur.Reset()
		}
	}
	finish := func() {
		flushBlock()
		if open {
			readFutureFields(&out[len(out)-1], blocks)
		}
		blocks, open = nil, false
	}
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if m := futureRe.FindStringSubmatch(t); m != nil {
			finish()
			title := strings.TrimSpace(t[len(m[0]):])
			out = append(out, Future{ID: m[1], Title: title, File: rel, Line: i + 1})
			open = true
			continue
		}
		if strings.HasPrefix(t, "## ") || t == "##" || strings.HasPrefix(t, "# ") {
			finish() // any other heading at or above a record's closes it
			continue
		}
		if !open {
			continue
		}
		switch {
		case t == "":
			flushBlock()
		case strings.HasPrefix(t, "- "):
			flushBlock()
			cur.WriteString(strings.TrimSpace(t[2:]))
		default:
			if cur.Len() > 0 {
				cur.WriteByte(' ')
			}
			cur.WriteString(t)
		}
	}
	finish()
	for i := range out {
		for _, c := range scratch.Conditions {
			if c.Record == out[i].ID {
				out[i].Conditions = append(out[i].Conditions, FutureCondition{Label: c.Label, Text: c.Text})
			}
		}
	}
	return out
}

// readFutureFields fills f's rating fields from its record's blocks.
func readFutureFields(f *Future, blocks []string) {
	seen := map[string]bool{}
	for _, b := range blocks {
		locs := futureFieldRe.FindAllStringSubmatchIndex(b, -1)
		for i, loc := range locs {
			label := b[loc[2]:loc[3]]
			end := len(b)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			if seen[label] || label == "Evidence" {
				continue
			}
			seen[label] = true
			tok, rest := fieldToken(b[loc[1]:end])
			switch label {
			case "Confidence":
				f.Confidence = tok
			case "Horizon":
				f.Horizon = tok
			case "Value":
				f.Value = &FutureValue{Level: tok, Why: rest}
			case "Basis":
				f.Basis = &FutureBasis{Kind: tok, Cite: rest}
			}
		}
	}
}

// fieldToken splits a field's text into its value token and the rest: the
// token runs to the first whitespace or punctuation, bold and code marks
// around it removed; the rest drops the separator after the token and a
// closing period.
func fieldToken(v string) (tok, rest string) {
	v = strings.TrimLeft(strings.TrimSpace(v), "*`")
	i := strings.IndexAny(v, " \t.,;:()*`")
	if i < 0 {
		return v, ""
	}
	tok = v[:i]
	rest = strings.TrimLeft(strings.TrimSpace(strings.TrimLeft(v[i:], "*")), ",;:")
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ".")
	if strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")") {
		rest = rest[1 : len(rest)-1]
	}
	return tok, strings.TrimSpace(rest)
}

// VetFuture vets one future against the embedded schema/intent #Future,
// concretely: an off-list value, a missing Confidence or Horizon, a value
// with no why or a measured basis that cites nothing all fail. The error
// names the id first.
func VetFuture(f Future) error {
	errs := VetFutures([]Future{f})
	return errs[0]
}

// VetFutures vets each future (one compile of the schema for all of them);
// the result is parallel to fs, nil where a future is sound.
func VetFutures(fs []Future) []error {
	out := make([]error, len(fs))
	cctx := cuecontext.New()
	schema := cctx.CompileBytes(intentSchema, cue.Filename("schema/intent/schema.cue"))
	def := schema.LookupPath(cue.ParsePath("#Future"))
	if err := schema.Err(); err != nil || !def.Exists() {
		for i, f := range fs {
			out[i] = fmt.Errorf("%s: the embedded schema/intent #Future does not compile: %v", f.ID, err)
		}
		return out
	}
	for i, f := range fs {
		data, err := json.Marshal(f)
		if err != nil {
			out[i] = fmt.Errorf("%s: %w", f.ID, err)
			continue
		}
		v := cctx.CompileBytes(data)
		if err := def.Unify(v).Validate(cue.Concrete(true)); err != nil {
			out[i] = fmt.Errorf("%s: %s", f.ID, cueErrorLine(err, v))
		}
	}
	return out
}

// cueErrorLine renders a CUE validation error as one line, one clause per
// failing field in the parsed form's own words: "horizon "soon" is off the
// list", "confidence missing", "basis.cite "we looked" does not match the
// declared form". data is the vetted value, read for the offending text.
func cueErrorLine(err error, data cue.Value) string {
	var clauses []string
	seen := map[string]bool{}
	for _, e := range cueerrors.Errors(err) {
		path := strings.TrimPrefix(strings.Join(e.Path(), "."), "#Future.")
		if seen[path] {
			continue
		}
		seen[path] = true
		format, args := e.Msg()
		msg := fmt.Sprintf(format, args...)
		got := ""
		if fv := data.LookupPath(cue.ParsePath(path)); fv.Exists() {
			got = fmt.Sprint(fv)
		}
		switch {
		case strings.Contains(msg, "incomplete value"):
			clauses = append(clauses, path+" missing")
		case got != "" && (strings.Contains(msg, "empty disjunction") || strings.Contains(msg, "conflicting values")):
			clauses = append(clauses, path+" "+got+" is off the list")
		case got != "" && strings.Contains(msg, "out of bound"):
			clauses = append(clauses, path+" "+got+" does not match the declared form")
		default:
			clauses = append(clauses, path+": "+msg)
		}
	}
	if len(clauses) == 0 {
		return err.Error()
	}
	return strings.Join(clauses, "; ")
}

// auditFutures adds U16 (IV-5): a future whose fields fail schema/intent
// #Future, named by id with its file and line.
func (a *Audit) auditFutures(in AuditInput) {
	for i, err := range VetFutures(in.Corpus.Futures) {
		if err == nil {
			continue
		}
		f := in.Corpus.Futures[i]
		a.Unproven = append(a.Unproven, Claim{
			Kind: "U16", Ref: f.ID,
			Detail: "future's fields fail schema/intent #Future: " + strings.TrimPrefix(err.Error(), f.ID+": "),
			Remedy: "set Confidence (likely|plausible|speculative) and Horizon (next|later|someday), the key in " +
				"docs/intent/futures.md; a Value is low|medium|high|very-high with a why, a Basis measured|argued|guess, " +
				"and a measured basis cites a path, an instrument or a commit",
			Evidence: f.File + ":" + itoa(f.Line),
		})
	}
}
