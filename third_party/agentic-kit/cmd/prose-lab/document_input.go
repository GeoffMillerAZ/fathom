package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

type documentFact struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	OccurredAt string `json:"occurred_at"`
	Kind       string `json:"kind"`
	Required   bool   `json:"required"`
}
type documentSection struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Instruction     string   `json:"instruction"`
	FactIDs         []string `json:"fact_ids"`
	RequiredFactIDs []string `json:"required_fact_ids"`
}
type documentGoal struct {
	ID         string `json:"id"`
	Statement  string `json:"statement"`
	Acceptance string `json:"acceptance"`
	Method     string `json:"method"`
}
type documentNegative struct {
	ID               string   `json:"id"`
	Fault            string   `json:"fault"`
	TargetFactID     string   `json:"target_fact_id"`
	MutatedAssertion string   `json:"mutated_assertion"`
	Expected         string   `json:"expected"`
	CriterionIDs     []string `json:"criterion_ids"`
}
type documentFixture struct {
	Version       string              `json:"version,omitempty"`
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	LengthBand    string              `json:"length_band"`
	MinWords      int                 `json:"min_words,omitempty"`
	MaxWords      int                 `json:"max_words,omitempty"`
	LengthGoal    *documentLengthGoal `json:"length_goal,omitempty"`
	Purpose       string              `json:"purpose"`
	Facts         []documentFact      `json:"facts"`
	Sections      []documentSection   `json:"sections"`
	RequiredGoals []documentGoal      `json:"required_goals"`
	NegativeCases []documentNegative  `json:"negative_cases"`
}
type documentCell struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	Audience   string `json:"audience"`
	Repeat     int    `json:"repeat"`
}
type documentManifest struct {
	Version   int            `json:"version"`
	Audiences []string       `json:"audiences"`
	Repeats   int            `json:"repeats"`
	Cells     []documentCell `json:"cells"`
}
type documentCorpus struct {
	Digest   string
	Manifest documentManifest
	Fixtures map[string]documentFixture
}

func exactObject(b []byte, keys ...string) error {
	m, e := jsonwire.Object(b)
	if e != nil || len(m) != len(keys) {
		return errors.New("invalid object shape")
	}
	for _, k := range keys {
		if v, ok := m[k]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return fmt.Errorf("missing %s", k)
		}
	}
	return nil
}
func exactArrayObjects(raw json.RawMessage, keys ...string) error {
	var a []json.RawMessage
	if e := json.Unmarshal(raw, &a); e != nil || a == nil {
		return errors.New("required array missing")
	}
	for _, x := range a {
		if e := exactObject(x, keys...); e != nil {
			return e
		}
	}
	return nil
}
func decodeDocumentFixture(b []byte) (documentFixture, error) {
	var d documentFixture
	m, e := jsonwire.Object(b)
	if e != nil {
		return d, e
	}
	keys := []string{"id", "type", "length_band", "min_words", "max_words", "purpose", "facts", "sections", "required_goals", "negative_cases"}
	if _, ok := m["version"]; ok {
		keys = []string{"version", "id", "type", "length_band", "length_goal", "purpose", "facts", "sections", "required_goals", "negative_cases"}
		if e := exactObject(m["length_goal"], "criterion_id", "class", "min_words", "max_words", "reason"); e != nil {
			return d, e
		}
	}
	if e := exactObject(b, keys...); e != nil {
		return d, e
	}
	for _, x := range []struct {
		key    string
		fields []string
	}{{"facts", []string{"id", "text", "occurred_at", "kind", "required"}}, {"sections", []string{"id", "title", "instruction", "fact_ids", "required_fact_ids"}}, {"required_goals", []string{"id", "statement", "acceptance", "method"}}, {"negative_cases", []string{"id", "fault", "target_fact_id", "mutated_assertion", "expected", "criterion_ids"}}} {
		if e := exactArrayObjects(m[x.key], x.fields...); e != nil {
			return d, e
		}
	}
	if e := decodeLabJSON(b, &d); e != nil {
		return d, e
	}
	return d, d.validate()
}
func parseDocumentOccurredAt(s string) (time.Time, error) {
	if !strings.HasSuffix(s, "Z") {
		return time.Time{}, errors.New("occurred_at requires UTC Z")
	}
	at, e := time.Parse(time.RFC3339Nano, s)
	if e != nil || at.IsZero() || at.Format(time.RFC3339Nano) != s {
		return time.Time{}, errors.New("invalid canonical UTC occurred_at")
	}
	return at, nil
}
func (d documentFixture) validate() error {
	minWords, maxWords := d.MinWords, d.MaxWords
	if d.Version == "prose-lab.fixture/2" && d.LengthGoal != nil {
		minWords, maxWords = d.LengthGoal.MinWords, d.LengthGoal.MaxWords
	} else if d.Version != "" || d.LengthGoal != nil {
		return errors.New("invalid fixture version")
	}
	if !validLabID(d.ID) || d.Type != d.ID || d.Purpose == "" || minWords < 100 || maxWords > 1600 || minWords > maxWords || len(d.Facts) < 1 || len(d.Facts) > 256 || len(d.Sections) < 2 || len(d.Sections) > 5 || len(d.RequiredGoals) < 3 || len(d.RequiredGoals) > 5 || len(d.NegativeCases) < 6 || len(d.NegativeCases) > 32 {
		return errors.New("invalid fixture bounds")
	}
	if (d.LengthBand == "short" && maxWords > 250) || (d.LengthBand == "medium" && (minWords < 400 || maxWords > 750)) || (d.LengthBand == "long" && (minWords < 1200 || maxWords > 1600)) || (d.LengthBand != "short" && d.LengthBand != "medium" && d.LengthBand != "long") {
		return errors.New("invalid length band")
	}
	facts := map[string]bool{}
	required := map[string]bool{}
	exposed := map[string]bool{}
	covered := map[string]bool{}
	for _, f := range d.Facts {
		if !validLabID(f.ID) || !strings.HasPrefix(f.ID, "f-") || f.Text == "" || facts[f.ID] {
			return errors.New("invalid fact")
		}
		at, err := parseDocumentOccurredAt(f.OccurredAt)
		if err != nil || at.IsZero() {
			return errors.New("invalid authored occurred_at")
		}
		facts[f.ID] = true
		required[f.ID] = f.Required
		if f.Kind != "observed" && f.Kind != "proposed" && f.Kind != "assumed" && f.Kind != "definition" {
			return errors.New("invalid fact kind")
		}
		if f.Kind != "observed" && !strings.HasPrefix(f.Text, strings.ToUpper(f.Kind[:1])+f.Kind[1:]+": ") {
			return errors.New("non-observed status not explicit")
		}
	}
	sections := map[string]bool{}
	for _, s := range d.Sections {
		if !validLabID(s.ID) || !strings.HasPrefix(s.ID, "s-") || sections[s.ID] || s.Title == "" || s.Instruction == "" || len(s.FactIDs) == 0 || len(s.RequiredFactIDs) == 0 {
			return errors.New("invalid section")
		}
		sections[s.ID] = true
		available := map[string]bool{}
		for _, id := range s.FactIDs {
			if !facts[id] || available[id] {
				return errors.New("invalid section fact reference")
			}
			available[id] = true
			exposed[id] = true
		}
		for _, id := range s.RequiredFactIDs {
			if !available[id] {
				return errors.New("invalid required fact reference")
			}
			covered[id] = true
		}
	}
	for id := range facts {
		if !exposed[id] || (required[id] && !covered[id]) {
			return errors.New("unexposed or uncovered fact")
		}
	}
	goals := map[string]bool{}
	for _, g := range d.RequiredGoals {
		if !validLabID(g.ID) || goals[g.ID] || g.Statement == "" || g.Acceptance == "" || g.Method == "" {
			return errors.New("invalid goal")
		}
		goals[g.ID] = true
	}
	if d.LengthGoal != nil {
		if !validLabID(d.LengthGoal.CriterionID) || goals[d.LengthGoal.CriterionID] || (d.LengthGoal.Class != "required" && d.LengthGoal.Class != "advisory") || (d.LengthGoal.Class == "required" && strings.TrimSpace(d.LengthGoal.Reason) == "") || len(d.LengthGoal.Reason) > 4096 {
			return errors.New("invalid length goal")
		}
	}
	negatives := map[string]bool{}
	for _, n := range d.NegativeCases {
		if !validLabID(n.ID) || negatives[n.ID] || !facts[n.TargetFactID] || n.Fault == "" || n.MutatedAssertion == "" || n.Expected != "hold" || len(n.CriterionIDs) == 0 {
			return errors.New("invalid negative")
		}
		negatives[n.ID] = true
		for _, id := range n.CriterionIDs {
			if !goals[id] {
				return errors.New("foreign negative criterion")
			}
		}
	}
	return nil
}
func loadDocumentCorpus(dir string) (documentCorpus, error) {
	var c documentCorpus
	c.Fixtures = map[string]documentFixture{}
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() {
		return c, errors.New("corpus directory unavailable")
	}
	mb, e := readLabBytes(filepath.Join(dir, "manifest.json"), 1<<20)
	if e != nil {
		return c, e
	}
	if e = exactObject(mb, "version", "audiences", "repeats", "cells"); e != nil {
		return c, e
	}
	mo, _ := jsonwire.Object(mb)
	if e = exactArrayObjects(mo["cells"], "id", "document_id", "audience", "repeat"); e != nil {
		return c, e
	}
	if e = decodeLabJSON(mb, &c.Manifest); e != nil {
		return c, e
	}
	m := c.Manifest
	if (m.Version != 1 && m.Version != 2) || len(m.Audiences) != 2 || m.Audiences[0] != "executive" || m.Audiences[1] != "technical" || m.Repeats != 3 || len(m.Cells) != 36 {
		return c, errors.New("manifest must fix 36 cells")
	}
	ids := []string{"architecture-decision", "draft-service-agreement", "freeform-brief", "incident-summary", "operational-runbook", "technical-design"}
	parts := [][]byte{mb}
	total := int64(len(mb))
	for _, id := range ids {
		b, x := readLabBytes(filepath.Join(dir, id+".json"), 8<<20)
		if x != nil {
			return c, x
		}
		total += int64(len(b))
		if total > 64<<20 {
			return c, errors.New("corpus byte bound")
		}
		d, x := decodeDocumentFixture(b)
		if x != nil {
			return c, fmt.Errorf("invalid fixture %s: %w", id, x)
		}
		if d.ID != id {
			return c, errors.New("fixture id mismatch " + id)
		}
		if (m.Version == 1 && d.Version != "") || (m.Version == 2 && d.Version != "prose-lab.fixture/2") {
			return c, errors.New("fixture/manifest version mismatch")
		}
		c.Fixtures[id] = d
		parts = append(parts, []byte(id+"\x00"), b)
	}
	seen := map[string]bool{}
	for _, cell := range m.Cells {
		if _, ok := c.Fixtures[cell.DocumentID]; !ok || cell.Repeat < 1 || cell.Repeat > 3 || (cell.Audience != "executive" && cell.Audience != "technical") || cell.ID != fmt.Sprintf("%s-%s-r%d", cell.DocumentID, cell.Audience, cell.Repeat) || seen[cell.ID] {
			return c, errors.New("invalid manifest cell")
		}
		seen[cell.ID] = true
	}
	for _, id := range ids {
		for _, aud := range m.Audiences {
			for r := 1; r <= 3; r++ {
				if !seen[fmt.Sprintf("%s-%s-r%d", id, aud, r)] {
					return c, errors.New("missing manifest cell")
				}
			}
		}
	}
	c.Digest = hashBytes([]byte(strings.Join(byteParts(parts), "")))
	return c, nil
}
func byteParts(parts [][]byte) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = string(p)
	}
	return out
}
func (c documentCorpus) cell(id string) (documentCell, bool) {
	for _, x := range c.Manifest.Cells {
		if x.ID == id {
			return x, true
		}
	}
	return documentCell{}, false
}
func (c documentCorpus) sortedIDs() []string {
	out := make([]string, 0, len(c.Manifest.Cells))
	for _, x := range c.Manifest.Cells {
		out = append(out, x.ID)
	}
	sort.Strings(out)
	return out
}
