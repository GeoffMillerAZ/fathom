package mcp

import (
	"context"
	"encoding/json"
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
	for _, field := range []string{"unknown", "nested", "reason", "refresh", "suppressed", "malformed", "impossible"} {
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
				case "impossible":
					payload["dimensions"].([]any)[0].(map[string]any)["name"] = huge
				}
				return payload, nil
			})
			wire := callToolRaw(t, s, "review_diff")
			if field == "malformed" || field == "impossible" {
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
			fields := out["elided"].(map[string]any)["fields"].(map[string]reviewOmission)
			if len(p["dimensions"].([]any)) == 0 {
				for path, omission := range fields {
					if strings.HasPrefix(path, "/dimensions/") && omission.Returned != 0 {
						t.Errorf("removed dimension detail %s still claims %d returned bytes", path, omission.Returned)
					}
				}
			}
			for path, total := range map[string]int{"/budget/reason": len(huge), "/refresh/svc": len(huge), "/suppressed/svc/ids": 1, "/suppressed/svc/reasons": 1} {
				omission, ok := fields[path]
				if !ok || omission.Total != total || omission.Returned >= omission.Total {
					t.Errorf("omission %s=%v, want total %d and fewer returned", path, omission, total)
				}
			}
		})
	}
}
