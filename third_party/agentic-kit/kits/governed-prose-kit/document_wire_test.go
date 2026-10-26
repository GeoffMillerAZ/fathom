package governedprosekit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func wireJSON(segs ...string) string { return `{"segments":[` + strings.Join(segs, ",") + `]}` }

func seg(kind, text, facts, reason string) string {
	t, _ := json.Marshal(text)
	return `{"kind":"` + kind + `","text":` + string(t) + `,"fact_ids":[` + facts + `],"contradictions":[],"reason":"` + reason + `"}`
}

func TestDocumentWireDerivesExactGoOwnedRanges(t *testing.T) {
	a, b, c := "café é", "\U0001F600 日本", "ño"
	d, err := decodeDocumentResponse(wireJSON(seg("assertion", a, `"f1"`, ""), seg("assertion", b, `"f2","f3"`, ""), seg("assertion", c, `"f1"`, "")), DefaultProseLimits(), "node-9", KindParagraph)
	if err != nil {
		t.Fatal(err)
	}
	u := d.Units[0]
	if len(d.Units) != 1 || u.ID != "node-9" || u.Text != a+" "+b+" "+c || len(d.Claims) != 3 || len(u.Claims) != 3 {
		t.Fatalf("%+v", d)
	}
	for i, want := range []string{a, b, c} {
		r, cl := u.Claims[i], d.Claims[i]
		if cl.ID != "node-9#c"+string(rune('1'+i)) || r.ClaimID != cl.ID || u.Text[r.Start:r.End] != want || cl.Assertion != want {
			t.Fatalf("segment %d: %+v %+v", i, r, cl)
		}
	}
	if u.Claims[1].Start != len(a)+1 || len(d.Claims[1].FactIDs) != 2 {
		t.Fatalf("offsets must come from exact bytes: %+v", u.Claims)
	}
	again, _ := decodeDocumentResponse(wireJSON(seg("assertion", a, `"f1"`, ""), seg("assertion", b, `"f2","f3"`, ""), seg("assertion", c, `"f1"`, "")), DefaultProseLimits(), "node-9", KindParagraph)
	if again.Units[0].Text != u.Text || again.Claims[2].ID != d.Claims[2].ID {
		t.Fatal("derivation must be deterministic")
	}
	l, err := decodeDocumentResponse(wireJSON(seg("label", "Findings", "", "structural label")), DefaultProseLimits(), "h1", KindHeading)
	if err != nil || l.Units[0].NonFactualReason != "structural label" || len(l.Claims) != 0 || len(l.Units[0].Claims) != 0 {
		t.Fatalf("label path: %+v %v", l, err)
	}
}

func TestDocumentWireRejectsMalformedAndForgedShapes(t *testing.T) {
	ok := seg("assertion", "fact", `"f1"`, "")
	many := make([]string, 129)
	for i := range many {
		many[i] = ok
	}
	small := DefaultProseLimits()
	small.MaxBytes = 80
	cases := map[string]struct {
		raw  string
		kind string
		lim  *ProseLimits
	}{
		"model id":            {`{"segments":[{"id":"x","kind":"assertion","text":"a","fact_ids":["f"],"contradictions":[],"reason":""}]}`, KindParagraph, nil},
		"model offsets":       {`{"segments":[{"start":0,"end":1,"kind":"assertion","text":"a","fact_ids":["f"],"contradictions":[],"reason":""}]}`, KindParagraph, nil},
		"model status":        {`{"segments":[{"status":"supported","kind":"assertion","text":"a","fact_ids":["f"],"contradictions":[],"reason":""}]}`, KindParagraph, nil},
		"unknown top key":     {`{"segments":[` + ok + `],"claims":[]}`, KindParagraph, nil},
		"duplicate key":       {`{"segments":[` + ok + `],"segments":[` + ok + `]}`, KindParagraph, nil},
		"missing field":       {`{"segments":[{"kind":"assertion","text":"a","fact_ids":["f"],"contradictions":[]}]}`, KindParagraph, nil},
		"null facts":          {`{"segments":[{"kind":"assertion","text":"a","fact_ids":null,"contradictions":[],"reason":""}]}`, KindParagraph, nil},
		"null contradictions": {`{"segments":[{"kind":"assertion","text":"a","fact_ids":["f"],"contradictions":null,"reason":""}]}`, KindParagraph, nil},
		"null segments":       {`{"segments":null}`, KindParagraph, nil},
		"empty segments":      {`{"segments":[]}`, KindParagraph, nil},
		"too many segments":   {wireJSON(many...), KindParagraph, nil},
		"over byte limit":     {wireJSON(seg("assertion", strings.Repeat("x", 100), `"f"`, "")), KindParagraph, &small},
		"leading space":       {wireJSON(seg("assertion", " a", `"f"`, "")), KindParagraph, nil},
		"trailing newline":    {wireJSON(seg("assertion", "a\n", `"f"`, "")), KindParagraph, nil},
		"blank text":          {wireJSON(seg("assertion", "  ", `"f"`, "")), KindParagraph, nil},
		"no facts":            {wireJSON(seg("assertion", "a", "", "")), KindParagraph, nil},
		"blank fact id":       {wireJSON(seg("assertion", "a", `" "`, "")), KindParagraph, nil},
		"assertion reason":    {wireJSON(seg("assertion", "a", `"f"`, "because")), KindParagraph, nil},
		"unknown kind":        {wireJSON(seg("claim", "a", `"f"`, "")), KindParagraph, nil},
		"label paragraph":     {wireJSON(seg("label", "Findings", "", "label")), KindParagraph, nil},
		"label with facts":    {wireJSON(seg("label", "Revenue grew", `"f"`, "label")), KindHeading, nil},
		"label no reason":     {wireJSON(seg("label", "Findings", "", "")), KindHeading, nil},
		"mixed label":         {wireJSON(seg("label", "Findings", "", "label"), seg("assertion", "a", `"f"`, "")), KindHeading, nil},
		"invalid utf8":        {"{\"segments\":[{\"kind\":\"assertion\",\"text\":\"a\xff\",\"fact_ids\":[\"f\"],\"contradictions\":[],\"reason\":\"\"}]}", KindParagraph, nil},
		"lone surrogate":      {`{"segments":[{"kind":"assertion","text":"a\ud800","fact_ids":["f"],"contradictions":[],"reason":""}]}`, KindParagraph, nil},
		"trailing value":      {wireJSON(ok) + `{}`, KindParagraph, nil},
		"empty response":      {"", KindParagraph, nil},
		"legacy draft":        {`{"claims":[],"units":[]}`, KindParagraph, nil},
	}
	for name, c := range cases {
		lim := DefaultProseLimits()
		if c.lim != nil {
			lim = *c.lim
		}
		if _, err := decodeDocumentResponse(c.raw, lim, "n", c.kind); !errors.Is(err, ErrArtifact) {
			t.Errorf("%s: want ErrArtifact, got %v", name, err)
		}
	}
}

// Text alone fitting the limit cannot bypass the complete JSON artifact bound.
func TestDocumentWireByteLimitIncludesMetadata(t *testing.T) {
	text := "Supported fact."
	raw := wireJSON(seg("assertion", text, `"f1"`, ""))
	limits := DefaultProseLimits()
	limits.MaxBytes = len(text) + 1
	if len(raw) <= limits.MaxBytes {
		t.Fatal("fixture must exceed the limit through metadata")
	}
	if _, err := decodeDocumentResponse(raw, limits, "node", KindParagraph); !errors.Is(err, ErrArtifact) {
		t.Fatalf("whole wire must fit, even when text fits: %v", err)
	}
	limits.MaxBytes = len(raw)
	if _, err := decodeDocumentResponse(raw, limits, "node", KindParagraph); err != nil {
		t.Fatalf("exact complete wire bound: %v", err)
	}
}

// TestDocumentWireEndToEnd drives the real engine with two assertions in one node:
// the model supplies no IDs or offsets and every assertion still passes the full
// independent support review; a wrong citation in the second segment refuses.
func TestDocumentWireEndToEnd(t *testing.T) {
	e := newDocEnv(t)
	for i := range e.target.Spec.Nodes {
		if e.target.Spec.Nodes[i].ID == "p1" {
			e.target.Spec.Nodes[i].FactIDs = []string{"fact-a", "fact-b"}
		}
	}
	e.client.raw = func(n string) (string, bool) {
		return wireJSON(seg("assertion", "12 incidents", `"fact-a"`, ""), seg("assertion", "30 minutes", `"fact-b"`, "")), n == "p1"
	}
	v := e.generate()
	p1 := nodeByID(v, "p1")
	if p1.Text != "12 incidents 30 minutes" || p1.Ranges[0].End != 12 || p1.Ranges[1].Start != 13 || p1.Claims[1].ID != "p1#c2" || len(p1.Support) != 2 {
		t.Fatalf("%+v", p1)
	}
	if !strings.Contains(e.client.prompts["p1"], `"node":{"id":"p1"`) {
		t.Fatal("prompt must carry the host node ID")
	}
	for name, raw := range map[string]string{
		"wrong but valid citation": wireJSON(seg("assertion", "12 incidents", `"fact-a"`, ""), seg("assertion", "30 minutes", `"fact-a"`, "")),
		"unexposed fact":           wireJSON(seg("assertion", "12 incidents", `"fact-a"`, ""), seg("assertion", "Resolved", `"fact-c"`, "")),
	} {
		e2 := newDocEnv(t)
		e2.target.Spec = e.target.Spec
		e2.client.raw = func(n string) (string, bool) { return raw, n == "p1" }
		if _, err := e2.engine.Generate(context.Background(), e2.target); err == nil {
			t.Errorf("%s must refuse", name)
		}
	}
}

func TestDocumentResponseMatchesCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE tool unavailable; task cue-vet has the same optional prerequisite")
	}
	vet := func(raw string) error {
		path := filepath.Join(t.TempDir(), "r.json")
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		return exec.Command(cue, "vet", "schema.cue", path, "-d", "#DocumentResponse").Run()
	}
	if err := vet(wireJSON(seg("assertion", "12 incidents", `"fact-a"`, ""), seg("label", "x", "", "r"))); err != nil {
		t.Fatalf("positive response rejected: %v", err)
	}
	for name, raw := range map[string]string{
		"null array":    `{"segments":[{"kind":"assertion","text":"a","fact_ids":null,"contradictions":[],"reason":""}]}`,
		"extra field":   `{"segments":[{"id":"x","kind":"assertion","text":"a","fact_ids":["f"],"contradictions":[],"reason":""}]}`,
		"missing field": `{"segments":[{"kind":"assertion","text":"a","fact_ids":["f"],"contradictions":[]}]}`,
		"bad kind":      wireJSON(seg("claim", "a", `"f"`, "")),
		"empty":         `{"segments":[]}`,
	} {
		if vet(raw) == nil {
			t.Errorf("%s: malformed shape accepted by CUE", name)
		}
	}
}
