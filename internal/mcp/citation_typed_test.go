package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestCitationIdenticalFileDuringRefreshIsClean(t *testing.T) {
	c, _ := citationFixture(t)
	p, _ := citationContext(t, &citationProbe{Client: c, refresh: true})
	if p["def"].(map[string]any)["citation_status"] != nil {
		t.Fatalf("identical file marked: %v", p)
	}
	if p["snippet"] == nil {
		t.Fatalf("identical snippet withheld: %v", p)
	}
}

func TestCitationSummaryFirstAndQualified(t *testing.T) {
	c, root := citationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0600); err != nil {
		t.Fatal(err)
	}
	p, raw := citationCall(t, &citationProbe{Client: c}, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	if !strings.HasPrefix(raw, `{"summary":`) {
		t.Errorf("summary not first: %s", raw)
	}
	if !strings.Contains(p["summary"].(string), "[citation stale]") {
		t.Errorf("unqualified summary: %v", p["summary"])
	}
}

func TestCitationFingerprintChangesDuringRead(t *testing.T) {
	c, _ := citationFixture(t)
	b := &citationProbe{Client: c}
	b.afterFingerprint = func() {
		if err := c.Store().SetFingerprint(context.Background(), "sample", "sample.go", fusion.FingerprintHash([]byte("replacement"))); err != nil {
			t.Fatal(err)
		}
	}
	p, _ := citationContext(t, b)
	requireCitationState(t, p["def"].(map[string]any), "refreshing")
	if p["snippet"] != nil {
		t.Fatalf("snippet served across index change: %v", p)
	}
}

func TestCitationRefreshStateReadTwice(t *testing.T) {
	c, _ := citationFixture(t)
	b := &countCitationIngest{citationProbe: citationProbe{Client: c}}
	citationContext(t, b)
	if b.reads != 2 {
		t.Fatalf("ingest reads=%d, want call start/end only", b.reads)
	}
}

type countCitationIngest struct {
	citationProbe
	reads int
}

func (b *countCitationIngest) IngestState(ctx context.Context, service string) (store.IngestState, bool, error) {
	b.reads++
	return b.citationProbe.IngestState(ctx, service)
}

// A generated six-citation, 573 KB source, shared by the benchmark and the
// request-local work-counter regression. No timing assertion is made.
type sixCitationBackend struct {
	*client.Client
	rules        []semantic.Obligation
	fingerprints int
}

func (b *sixCitationBackend) ObligationsLayer(context.Context, string, string, int, semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	return semantic.Obligations{Service: "sample", File: "sample.go", Rules: append([]semantic.Obligation(nil), b.rules...)}, nil
}
func (b *sixCitationBackend) SourceFingerprint(ctx context.Context, s, f string) (string, bool, error) {
	b.fingerprints++
	return b.Client.SourceFingerprint(ctx, s, f)
}
func sixCitationFixture(t testing.TB) *sixCitationBackend {
	t.Helper()
	root := t.TempDir()
	data := []byte(strings.Repeat("Citation source line.\n", 27300))
	data = data[:573285]
	if err := os.WriteFile(filepath.Join(root, "guide.md"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(citationSource), 0600); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "sample", Path: root}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFingerprint(context.Background(), "sample", "guide.md", fusion.FingerprintHash(data)); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { _ = c.Close() })
	b := &sixCitationBackend{Client: c}
	for i := 1; i <= 6; i++ {
		b.rules = append(b.rules, semantic.Obligation{Rule: fmt.Sprintf("Retain contract %d.", i), Src: fmt.Sprintf("guide.md#L%d-L%d", i, i), Tier: "doc-claimed", Layer: "obligations"})
	}
	return b
}
func TestCitationSixSnippetsReadFileOnce(t *testing.T) {
	b := sixCitationFixture(t)
	p, _ := citationCall(t, b, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	rules := p["rules"].([]any)
	if len(rules) != 6 {
		t.Fatalf("rules=%d", len(rules))
	}
	for _, r := range rules {
		if r.(map[string]any)["snippet"] != "Citation source line." {
			t.Fatalf("snippet=%v", r)
		}
	}
	if b.fingerprints != 2 {
		t.Fatalf("fingerprint reads=%d, want 2 for one distinct file", b.fingerprints)
	}
}
func BenchmarkCitationSix573KB(b *testing.B) {
	for _, snippets := range []bool{false, true} {
		b.Run(fmt.Sprintf("snippets=%t", snippets), func(b *testing.B) {
			backend := sixCitationFixture(b)
			s := NewServer(backend)
			p, _ := json.Marshal(map[string]any{"name": "obligations", "arguments": map[string]any{"service": "sample", "file": "sample.go", "include_snippets": snippets}})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.handleToolsCall(context.Background(), p); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestCitationEpochReceipt(t *testing.T) {
	c, _ := citationFixture(t)
	hash, _, _ := c.SourceFingerprint(context.Background(), "sample", "sample.go")
	rows, _, _ := c.FileStats(context.Background(), "sample")
	epoch, err := c.SourceFingerprintEpoch(context.Background(), "sample", "sample.go", hash)
	t.Logf("hash=%s rows=%+v epoch=%s err=%v", hash, rows, epoch, err)
	if epoch == "" {
		t.Fatal("missing epoch for an ingested file")
	}
}

func TestCitationWriteBetweenFingerprintReads(t *testing.T) {
	c, root := citationFixture(t)
	ctx := context.WithValue(context.Background(), citationReadHookKey{}, func() {
		if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0600); err != nil {
			t.Fatal(err)
		}
	})
	s := NewServer(c)
	args := json.RawMessage(`{"name":"symbol_context","arguments":{"service":"sample","symbol":"Alpha"}}`)
	result, err := s.handleToolsCall(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	raw := result.(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var p map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&p); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "Bravo") && p["def"].(map[string]any)["citation_status"] == nil {
		t.Fatalf("new bytes labeled clean: %s", raw)
	}
	if p["snippet"] == nil {
		t.Fatalf("original checked snippet missing: %s", raw)
	}
}

func TestCitationLiveSpecRefsAreUnmarked(t *testing.T) {
	b := sixCitationFixture(t)
	root, _, _, _ := b.ServiceMeta(context.Background(), "sample")
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample // [US-123]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "live.md"), []byte("# Current [US-123]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, _ := citationCall(t, b, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	loc := p["spec_refs"].(map[string]any)["US-123"].([]any)[0].(map[string]any)
	if loc["file"] != "live.md" || loc["citation_status"] != nil {
		t.Fatalf("live-tree location marked: %v", loc)
	}
	if b.fingerprints != 2 {
		t.Fatalf("live locations caused hashing: %d fingerprint reads", b.fingerprints)
	}
}

func TestCitationExtractorEpochDoesNotImplyFileChange(t *testing.T) {
	b := sixCitationFixture(t)
	ctx := context.Background()
	root, _, _, _ := b.ServiceMeta(ctx, "sample")
	data, err := os.ReadFile(filepath.Join(root, "guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	const oldEpoch = "fixture-previous-epoch"
	hash, err := fusion.FingerprintHashReaderAtEpoch(strings.NewReader(string(data)), oldEpoch)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Store().SetFingerprint(ctx, "sample", "guide.md", hash); err != nil {
		t.Fatal(err)
	}
	if err := b.Store().WriteServiceSummary(ctx, store.ServiceMeta{ServiceID: "sample", Path: root}, store.ServiceSummary{}, []store.FileStat{{Path: "guide.md", ContentHash: oldEpoch + ":unvouched:" + hash}}); err != nil {
		t.Fatal(err)
	}
	p, _ := citationCall(t, b, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	for _, raw := range p["rules"].([]any) {
		if raw.(map[string]any)["citation_status"] != nil {
			t.Fatalf("epoch bump reported as file change: %v", raw)
		}
	}
	data[0] = 'x'
	if err := os.WriteFile(filepath.Join(root, "guide.md"), data, 0600); err != nil {
		t.Fatal(err)
	}
	p, _ = citationCall(t, b, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	requireCitationState(t, p["rules"].([]any)[0].(map[string]any), "stale")
}

func TestCitationUnsupportedFingerprintBackendErrors(t *testing.T) {
	b := &autoRefreshBackend{}
	if _, _, err := b.SourceFingerprint(context.Background(), "sample", "sample.go"); err == nil {
		t.Fatal("unsupported backend returned not-found instead of error")
	}
}

func TestCitationBudgetCountsMarksAndPreservesSummary(t *testing.T) {
	rows := make([]symbolNodeDTO, 10)
	for i := range rows {
		rows[i] = symbolNodeDTO{Service: "sample", Name: "Alpha", FilePath: "sample.go", Line: 3}
	}
	plain := &summaryEnvelope{summary: "cited definitions", body: map[string]any{"rules": rows}}
	before, err := json.Marshal(budgetedResult(plain, 1600))
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		rows[i].CitationStatus = &core.CitationStatus{State: "stale", Reason: "file changed since the index; read_source serves the current text"}
	}
	after, err := json.Marshal(budgetedResult(plain, 1600))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), `{"summary":`) {
		t.Fatalf("budget reordered summary: %s", after)
	}
	if strings.Contains(string(before), `"elided"`) || !strings.Contains(string(after), `"elided"`) {
		t.Fatalf("budget did not count marks: before=%s after=%s", before, after)
	}
}

func TestCitationWindowsAreBoundedAndReadOnce(t *testing.T) {
	b := sixCitationFixture(t)
	gate := tenancy.NewGate(tenancy.GateConfig{})
	ticket, refusal := gate.Admit(context.Background(), tenancy.Request{Tool: "obligations", Service: "sample", Class: tenancy.ClassPoint})
	if refusal != nil {
		t.Fatal(refusal)
	}
	defer ticket.Release(tenancy.OutcomeOK)
	ctx := ticket.Context()
	reads := newCitationReads(ctx, b, "sample", nil)
	ctx = context.WithValue(ctx, citationReadsKey{}, reads)
	prepareObligationWindows(ctx, "sample", b.rules)
	for _, rule := range b.rules {
		if _, ok := obligationSnippet(ctx, b, "sample", rule.Src, false); !ok {
			t.Fatal("missing snippet")
		}
	}
	if ticket.BytesRead() != 573285 {
		t.Fatalf("cited file read %d bytes, want exactly one 573285-byte read", ticket.BytesRead())
	}
	for _, file := range reads.files {
		for _, window := range file.windows {
			if len(window.data) > obligationSnippetMaxBytes*2+1 {
				t.Fatalf("retained %d source bytes", len(window.data))
			}
		}
	}
	reads.finish(ctx, b)
	if len(reads.files) != 0 || len(reads.bindings) != 0 {
		t.Fatal("request retained source windows or citation bindings after finalization")
	}
}

type citationEncodingProbe struct{ calls *int }

func (p citationEncodingProbe) MarshalJSON() ([]byte, error) {
	*p.calls++
	return []byte(`{"source":"sample.go"}`), nil
}
func TestCitationCleanMarkingDoesNotEncode(t *testing.T) {
	calls := 0
	h := citationHandler(nil, func(context.Context, json.RawMessage) (any, *rpcError) { return citationEncodingProbe{&calls}, nil })
	if _, err := h(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("marking encoded the clean result %d times", calls)
	}
}

func TestCitationPathAliasesReadOnce(t *testing.T) {
	b := sixCitationFixture(t)
	for i := range b.rules {
		if i%2 == 0 {
			b.rules[i].Src = "./" + b.rules[i].Src
		}
	}
	p, _ := citationCall(t, b, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	if len(p["rules"].([]any)) != 6 || b.fingerprints != 2 {
		t.Fatalf("path aliases caused repeated reads: %d, %v", b.fingerprints, p)
	}
}

func TestReadSourceReusesIndexMatch(t *testing.T) {
	c, root := citationFixture(t)
	for _, dirty := range []bool{false, true} {
		if dirty {
			if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0600); err != nil {
				t.Fatal(err)
			}
		}
		b := &citationProbe{Client: c}
		p, _ := citationCall(t, b, "read_source", map[string]any{"service": "sample", "file": "sample.go"})
		identity := p["source_identity"].(map[string]any)
		if identity["matches_index"] != !dirty || b.fingerprintReads != 2 {
			t.Fatalf("identity=%v, fingerprint reads=%d", identity, b.fingerprintReads)
		}
	}
}

// The typed snippet projection must keep every additive rule field and the
// alphabetical key order previously supplied by its map encoding.
func TestCitationObligationWireKeepsTypedFields(t *testing.T) {
	r := semantic.Obligation{}
	value := reflect.ValueOf(&r).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		switch field.Kind() {
		case reflect.String:
			field.SetString(value.Type().Field(i).Name)
		case reflect.Bool:
			field.SetBool(true)
		}
	}
	current, relevance := true, 0.75
	r.AnchorCurrent = &current
	r.Relevance = &relevance
	r.CitationStatus = &core.CitationStatus{State: "stale", Reason: "source changed"}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]json.RawMessage
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(obligationSnippetRow(r))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, canonical) {
		t.Fatalf("typed rule projection lost fields or key order\ngot %s\nwant %s", got, canonical)
	}
}

func TestCitationSummaryFirstOnPreviewFallback(t *testing.T) {
	result := withSummary("qualified citation [citation stale]", map[string]any{"detail": strings.Repeat("source", 1000)})
	raw, err := json.Marshal(budgetedResult(result, 300))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), `{"summary":`) {
		t.Fatalf("preview lost answer-first summary: %s", raw)
	}
}

func TestCitationRawFallbackIsLiveAndUnhashed(t *testing.T) {
	b := sixCitationFixture(t)
	for i := range b.rules {
		b.rules[i].Tier = "raw"
	}
	p, _ := citationCall(t, b, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	if b.fingerprints != 0 {
		t.Fatalf("live raw citations made %d fingerprint reads", b.fingerprints)
	}
	rows := p["rules"].([]any)
	if len(rows) != 6 {
		t.Fatalf("raw rules=%d", len(rows))
	}
	for _, row := range rows {
		r := row.(map[string]any)
		if r["citation_status"] != nil || r["snippet"] != "Citation source line." {
			t.Fatalf("live raw rule=%v", r)
		}
	}
}
