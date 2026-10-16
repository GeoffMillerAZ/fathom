package governedprosekit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func TestSourcedModelCannotNormalizeMalformedFactID(t *testing.T) {
	t.Parallel()
	for _, escaped := range []string{`\ud800`, `\udfff`} {
		t.Run(escaped, func(t *testing.T) {
			s, req, draft := sourcedFixture(t, "12")
			req.Facts[0].Span.ID = "\uFFFD"
			draft.Claims[0].FactIDs = []string{"\uFFFD"}
			raw, err := json.Marshal(draft)
			if err != nil {
				t.Fatal(err)
			}
			response := strings.Replace(string(raw), "\uFFFD", escaped, 1)
			reviews := 0
			p, _ := sourcedPipeline(t, s, reviewFunc(func(ctx context.Context, in SupportReview) (SupportDecision, error) {
				reviews++
				return (ExactSupportReviewer{}).Review(ctx, in)
			}), draft)
			model := llmclient.NewMockClient(llmclient.MockEntryOK(response))
			p.client = model
			artifact, err := p.Generate(context.Background(), req)
			if !errors.Is(err, ErrArtifact) {
				t.Errorf("malformed model fact ID became valid support: err=%v draft=%+v", err, artifact.Draft)
			}
			if reviews != 0 || model.CallCount() != 1 {
				t.Fatalf("malformed output reached reviewer or retried: reviews=%d model=%d", reviews, model.CallCount())
			}
		})
	}
}

func TestSourcedJSONUnicodeScalars(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"high surrogate", `{"id":"\ud800"}`, false},
		{"low surrogate", `{"id":"\udfff"}`, false},
		{"wrong pair", `{"id":"\ud800\u0041"}`, false},
		{"reversed pair", `{"id":"\udc00\ud800"}`, false},
		{"split pair", `{"a":"\ud800","b":"\udc00"}`, false},
		{"key", `{"\ud800":"id"}`, false},
		{"valid pair", `{"id":"\ud834\udd1e"}`, true},
		{"uppercase hex", `{"id":"\uDBFF\uDFFF"}`, true},
		{"replacement rune", `{"id":"\ufffd"}`, true},
		{"literal replacement", `{"id":"�"}`, true},
		{"BMP edge", `{"id":"\ud7ff\ue000"}`, true},
		{"escaped backslash", `{"id":"\\ud800"}`, true},
		{"encoded backslash", `{"id":"\u005cud800"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out map[string]string
			err := decodeSourcedJSON(tc.raw, 4096, &out)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v decoded=%q", tc.valid, err, out)
			}
		})
	}
}
