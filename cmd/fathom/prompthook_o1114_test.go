package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// O1114: a prose word is not a symbol. Plural acronyms never become
// candidates, and a find_symbol hit below the exact tier resolves a token
// only when the prompt wrote it as code.
func TestExtractPromptCandidates_PluralAcronymsAreProse(t *testing.T) {
	for _, w := range []string{"ADRs", "APIs", "URLs", "SDKs"} {
		if got := extractPromptCandidates("make sure we have "+w+" that callout this", nil); len(got) != 0 {
			t.Errorf("%s: got candidates %+v, want none", w, got)
		}
	}
	// Unaffected shapes still resolve.
	cases := map[string][]promptCandidate{
		"look at internal/store/backend.go now": {{candidatePath, "internal/store/backend.go", false}},
		"see `ADRSection` please":               {{candidateSymbol, "ADRSection", true}},
		"see `ADRs` please":                     {{candidateSymbol, "ADRs", true}},
		"call registerTool(ctx) here":           {{candidateSymbol, "registerTool", true}},
		"the ADRSection type":                   {{candidateSymbol, "ADRSection", false}},
	}
	for prompt, want := range cases {
		if got := extractPromptCandidates(prompt, nil); !reflect.DeepEqual(got, want) {
			t.Errorf("%q = %+v, want %+v", prompt, got, want)
		}
	}
}

func TestFetchSymbolFile_OnlyExactOrMarkedCaseFold(t *testing.T) {
	var via string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner, _ := json.Marshal(map[string]any{"symbols": []any{map[string]any{"name": "ADRSection", "file_path": "internal/review/adrrules.go"}}, "matched_via": via})
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": string(inner)}}}})
	}))
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}
	cases := []struct {
		via    string
		marked bool
		want   bool
	}{
		{"exact", false, true},
		{"exact", true, true},
		{"case_insensitive", false, false},
		{"case_insensitive", true, true},
		{"substring", false, false},
		{"substring", true, false},
		{"stem", true, false},
		{"", true, false},
	}
	for _, c := range cases {
		via = c.via
		d, ok := fetchSymbolDef(hc, srv.URL, "svc", "ADRs", c.marked)
		fp := d.File
		if ok != c.want || (ok && !strings.HasSuffix(fp, "adrrules.go")) {
			t.Errorf("via=%q marked=%v: ok=%v fp=%q, want ok=%v", c.via, c.marked, ok, fp, c.want)
		}
	}
}
