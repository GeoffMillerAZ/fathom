package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scope"
)

func reviewEnvelopeFixture() map[string]any {
	dims := make([]any, 8)
	for i, name := range []string{"impact", "obligations", "arch_conformance", "spec_coverage", "coverage", "certifications", "integrity", "conservation"} {
		dims[i] = map[string]any{"name": name, "checked": i < 4, "count": i, "reason": "source unavailable", "findings": []any{}}
	}
	return map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "dimensions": dims, "summary": "four checked", "findings_persisted": false, "test_impact": map[string]any{"checked": true, "affected_tests": []any{}, "test_gap": []any{}}, "resolution": map[string]any{"typed": 0, "heuristic": 0}, "service_ref_hops": []any{}}
}

func TestReviewDiffOversizedFieldCannotBypassBudget(t *testing.T) {
	for _, field := range []string{"unknown", "nested", "reason", "refresh", "suppressed", "malformed", "unknown_name", "cannot_fit"} {
		t.Run(field, func(t *testing.T) {
			payload := reviewEnvelopeFixture()
			huge := strings.Repeat("<&\"\n界", 8000)
			s := NewServer(nil)
			for i, tool := range s.tools {
				if tool.desc.Name == "review_diff" {
					s.tools = append(s.tools[:i], s.tools[i+1:]...)
					break
				}
			}
			s.registerTool(toolDescriptor{Name: "review_diff", InputSchema: map[string]any{"type": "object"}, group: GroupInvestigation}, func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
				switch field {
				case "unknown":
					payload["unexpected"] = huge
				case "nested":
					payload["test_impact"] = map[string]any{"checked": true, "affected_tests": []any{map[string]any{"reaches": []any{huge}}}, "test_gap": []any{}}
				case "reason":
					payload["dimensions"].([]any)[7].(map[string]any)["reason"] = huge
				case "refresh":
					noteRefreshDeferred(ctx, "svc", huge)
				case "suppressed":
					scope.Record(ctx, "svc", &scope.Disclosure{Count: 123, IDs: []string{huge}, Reasons: []string{huge}})
				case "malformed":
					payload["unexpected"] = json.RawMessage(`{"broken":`)
				case "cannot_fit":
					payload[huge] = true // Keys cannot be compacted without changing identity.
				case "unknown_name":
					payload["dimensions"].([]any)[0].(map[string]any)["name"] = huge
				}
				return payload, nil
			})
			wire := callToolRaw(t, s, "review_diff")
			if field == "malformed" || field == "unknown_name" || field == "cannot_fit" {
				e, ok := wire["error"].(map[string]any)
				if !ok || !strings.Contains(e["message"].(string), "review_diff response budget") {
					t.Fatalf("expected named budget refusal, got %v", wire["error"])
				}
				return
			}
			if wire["error"] != nil {
				t.Fatalf("unexpected refusal: %v", wire["error"])
			}
			b, err := json.Marshal(wire["result"])
			if err != nil {
				t.Fatal(err)
			}
			if len(b) > 16000 {
				t.Fatalf("%s escaped whole-result accounting: %d bytes", field, len(b))
			}
			result := wire["result"].(map[string]any)
			p := result["structuredContent"].(map[string]any)
			statuses, ok := p["dimension_status"].(map[string]any)
			if !ok || len(statuses) != 8 || p["details_complete"] != false {
				t.Fatal("lost statuses or incomplete-detail disclosure")
			}
			if field == "suppressed" && result["suppressed"].(map[string]any)["svc"].(map[string]any)["count"] != float64(123) {
				t.Fatal("suppressed count changed")
			}
		})
	}
}

func TestReviewDiffEnvelopeMetadataAndRefusal(t *testing.T) {
	for _, variant := range []string{"combined", "all_reasons", "impossible", "malformed_metadata"} {
		t.Run(variant, func(t *testing.T) {
			payload := reviewEnvelopeFixture()
			huge := strings.Repeat("\x01<&界", 8000)
			if variant == "all_reasons" {
				for _, d := range payload["dimensions"].([]any) {
					dim := d.(map[string]any)
					dim["checked"] = false
					dim["reason"] = huge
				}
			}
			// A count above the float64 exact-integer range must remain exact.
			payload["dimensions"].([]any)[0].(map[string]any)["count"] = int64(9007199254740993)
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			envelope := map[string]any{
				"content": []any{map[string]any{"type": "text", "text": string(raw)}}, "structuredContent": json.RawMessage(raw),
				"budget":     map[string]any{"partial": true, "elided": 527, "bytes_read": 4096, "reason": huge},
				"refresh":    map[string]string{"svc": huge},
				"suppressed": map[string]any{"svc": map[string]any{"count": 123, "ids": []string{huge}, "reasons": []string{huge}}},
			}
			limit := 16000
			if variant == "impossible" {
				limit = 256
			}
			if variant == "malformed_metadata" {
				envelope["extra"] = json.RawMessage(`{"broken":`)
			}
			before, _ := json.Marshal(envelope)
			out, err := capReviewDiffEnvelope(envelope, limit)
			if variant == "impossible" || variant == "malformed_metadata" {
				if err == nil || out != nil {
					t.Fatal("impossible or malformed result must be refused")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(out)
			if err != nil || len(encoded) > 16000 {
				t.Fatalf("complete result bytes=%d: %v", len(encoded), err)
			}
			after, _ := json.Marshal(envelope)
			if string(before) != string(after) {
				t.Fatal("finalizer mutated original result")
			}
			budget := out["budget"].(map[string]any)
			if budget["partial"] != true || budget["elided"] != json.Number("527") || budget["bytes_read"] != json.Number("4096") {
				t.Fatal("partial counters changed")
			}
			var p map[string]any
			if err := json.Unmarshal(out["structuredContent"].(json.RawMessage), &p); err != nil {
				t.Fatal(err)
			}
			if msg := validateAgainstSchema("$", outputSchemaFor("review_diff"), p); msg != "" {
				t.Fatal(msg)
			}
			text := out["content"].([]any)[0].(map[string]any)["text"].(string)
			if text != string(out["structuredContent"].(json.RawMessage)) {
				t.Fatal("text and structured copies differ")
			}
			if !strings.Contains(text, `"count":9007199254740993`) {
				t.Fatal("integer count lost precision")
			}
			fields := p["elided"].(map[string]any)["fields"].(map[string]any)
			if len(p["dimensions"].([]any)) == 0 {
				for path, value := range fields {
					omission := value.(map[string]any)
					if strings.HasPrefix(path, "/dimensions/") && omission["returned"] != float64(0) {
						t.Errorf("removed dimension detail %s still claims %v returned bytes", path, omission["returned"])
					}
				}
			}
			for path, total := range map[string]int{"/budget/reason": len(huge), "/refresh/svc": len(huge), "/suppressed/svc/ids": 1, "/suppressed/svc/reasons": 1} {
				omission, ok := out["envelope_omissions"].(map[string]reviewOmission)[path]
				if !ok || omission.Total != total || omission.Returned >= omission.Total {
					t.Errorf("omission %s=%v, want total %d and fewer returned", path, omission, total)
				}
			}
		})
	}
}

// Synthetic handler output isolates the finalizer from review analysis ranking.
func reviewDetailFixture(t *testing.T) map[string]any {
	t.Helper()
	p := reviewEnvelopeFixture()
	for i, d := range p["dimensions"].([]any) {
		dim := d.(map[string]any)
		n := 1
		if i == 1 {
			n = 100
		}
		rows := make([]any, n)
		for j := range rows {
			rows[j] = map[string]any{"message": fmt.Sprintf("finding %d/%03d: %s", i, j, strings.Repeat("evidence ", 15))}
		}
		dim["findings"], dim["count"] = rows, n
	}
	p["receipt"] = map[string]any{"band": "medium", "reasons": []any{"governing rule"}}
	p["receipt_sarif"] = map[string]any{"runs": []any{strings.Repeat("duplicate", 1000)}}
	p["receipt_check"] = map[string]any{"summary": strings.Repeat("duplicate", 1000)}
	p["test_impact"] = map[string]any{"checked": true, "affected_tests": []any{"pkg/a_test.go"}, "test_gap": []any{"pkg/b.go"}}
	p["layers"] = []any{map[string]any{"name": "leaf", "files": []any{"pkg/a.go"}}}
	return p
}

func reviewTestEnvelope(t *testing.T, p map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(raw)}}, "structuredContent": json.RawMessage(raw)}
}

func TestReviewDiffCompactionRetainsPrefixes(t *testing.T) {
	p := reviewDetailFixture(t)
	env := reviewTestEnvelope(t, p)
	env["suppressed"] = map[string]any{"svc": map[string]any{"count": 1, "reasons": []string{"scope rule"}}}
	out, err := capReviewDiffEnvelope(env, 16000)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	if len(b) < 12000 || len(b) > 16000 {
		t.Errorf("compacted bytes=%d, want 12000..16000", len(b))
	}
	var got map[string]any
	if err := json.Unmarshal(out["structuredContent"].(json.RawMessage), &got); err != nil {
		t.Fatal(err)
	}
	if r, ok := got["receipt"].(map[string]any); !ok || r["band"] != "medium" {
		t.Error("receipt.band lost")
	}
	for i, d := range got["dimensions"].([]any) {
		rows := d.(map[string]any)["findings"].([]any)
		if len(rows) == 0 {
			t.Errorf("dimension %d lost first finding", i)
			continue
		}
		want := p["dimensions"].([]any)[i].(map[string]any)["findings"].([]any)
		a, _ := json.Marshal(rows)
		b, _ := json.Marshal(want[:len(rows)])
		if !bytes.Equal(a, b) {
			t.Errorf("dimension %d is not an original prefix", i)
		}
	}
	for _, k := range []string{"receipt_sarif", "receipt_check"} {
		if got[k] != nil {
			t.Errorf("%s retained duplicate receipt", k)
		}
	}
	if _, ok := out["elided"]; ok {
		t.Error("payload omissions repeated in envelope")
	}
	if len(got["changed_files"].([]any)) == 0 || len(got["layers"].([]any)) == 0 {
		t.Error("lost changed_files or layers")
	}
	reasons := out["suppressed"].(map[string]any)["svc"].(map[string]any)["reasons"].([]any)
	if len(reasons) != 1 || reasons[0] != "scope rule" {
		t.Error("short suppression reason lost")
	}
	el := got["elided"].(map[string]any)
	count, _ := el["count"].(float64)
	total, _ := el["of"].(float64)
	if count <= 0 || total < count {
		t.Errorf("invalid elision counts: %v", el)
	}
	if el["how_to_get_more"] != "narrow changed_files" {
		t.Error("unavailable retrieval advice")
	}
	// Payload omissions appear once per payload copy; envelope losses have their own key.
	if bytes.Count(b, []byte(`"fields"`)) != 1 {
		t.Error("payload fields repeated outside structuredContent")
	}
	again, err := capReviewDiffEnvelope(env, 16000)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := json.Marshal(again)
	if !bytes.Equal(b, next) {
		t.Error("nondeterministic compaction")
	}
}

func TestReviewDiffCompactionLogsElision(t *testing.T) {
	var log bytes.Buffer
	s := NewServerWithLogger(nil, NewCallLogger(&log))
	for i, tool := range s.tools {
		if tool.desc.Name == "review_diff" {
			s.tools = append(s.tools[:i], s.tools[i+1:]...)
			break
		}
	}
	s.registerTool(toolDescriptor{Name: "review_diff", InputSchema: map[string]any{"type": "object"}, group: GroupInvestigation}, func(context.Context, json.RawMessage) (any, *rpcError) { return reviewDetailFixture(t), nil })
	wire := callToolRaw(t, s, "review_diff")
	if wire["error"] != nil {
		t.Fatal(wire["error"])
	}
	var entry callLogEntry
	if err := json.Unmarshal(bytes.TrimSpace(log.Bytes()), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.ElidedCount == nil || *entry.ElidedCount <= 0 {
		t.Fatalf("compacted review logged as complete: %s", log.Bytes())
	}
}

func TestReviewDiffBudgetAvoidsEnvelopeDecode(t *testing.T) {
	p := reviewEnvelopeFixture()
	p["unexpected"] = strings.Repeat("x", 1<<20)
	env := reviewTestEnvelope(t, p)
	raw := env["structuredContent"].(json.RawMessage)
	var err error
	// A payload decode is necessary; a second decode of both payload copies is not.
	got := allocBytes(func() { _, err = capReviewDiffEnvelope(env, 16000) })
	if err != nil {
		t.Fatal(err)
	}
	if got > uint64(len(raw))*8 {
		t.Fatalf("allocated %d bytes for %d-byte payload; duplicated envelope decode", got, len(raw))
	}
}

func TestReviewDiffCompactionSchema(t *testing.T) {
	schema := outputSchemaFor("review_diff")
	props := schema["properties"].(map[string]any)
	for key, typ := range map[string]string{"elided": "object", "details_complete": "boolean"} {
		prop, ok := props[key].(map[string]any)
		if !ok || prop["type"] != typ {
			t.Errorf("%s missing typed schema %s", key, typ)
		}
		for _, req := range schema["required"].([]string) {
			if req == key {
				t.Errorf("%s must be optional", key)
			}
		}
	}
}
