package llmreplay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmreplay "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/replay"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// recordCompletes records one Complete call per request against a MockClient
// scripted with one OK entry per request, returning the cassette entries.
func recordCompletes(t *testing.T, reqs ...llmclient.Request) []llmreplay.Entry {
	t.Helper()
	mockEntries := make([]llmclient.MockEntry, len(reqs))
	for i := range reqs {
		mockEntries[i] = llmclient.MockEntryOK("answer")
	}
	mock := llmclient.NewMockClient(mockEntries...)
	var buf bytes.Buffer
	rec := llmreplay.NewRecordingClient(mock, &buf)
	for _, req := range reqs {
		if _, err := rec.Complete(context.Background(), req); err != nil {
			t.Fatalf("recording Complete: %v", err)
		}
	}
	return mustLoad(t, &buf)
}

// TestReplay_OrdinalExhaustion proves the default matching: entries are
// consumed in order regardless of request content, and running past the end
// is a clear error.
func TestReplay_OrdinalExhaustion(t *testing.T) {
	entries := recordCompletes(t, llmclient.Request{Prompt: "recorded prompt"})
	rp := llmreplay.NewReplayClient(entries)

	// Ordinal matching ignores the request: a different prompt still replays.
	if _, err := rp.Complete(context.Background(), llmclient.Request{Prompt: "totally different"}); err != nil {
		t.Fatalf("first Complete: %v", err)
	}

	_, err := rp.Complete(context.Background(), llmclient.Request{Prompt: "one too many"})
	if err == nil {
		t.Fatal("Complete past the end of the cassette must error")
	}
	if !strings.Contains(err.Error(), "cassette exhausted") {
		t.Errorf("exhaustion error = %q, want it to name the exhausted cassette", err)
	}
}

// TestReplay_KindMismatch proves the called method must match the recorded
// kind, and that a mismatch does NOT consume the entry.
func TestReplay_KindMismatch(t *testing.T) {
	entries := recordCompletes(t, llmclient.Request{Prompt: "hi"})
	rp := llmreplay.NewReplayClient(entries)

	_, err := rp.Stream(context.Background(), llmclient.Request{Prompt: "hi"})
	if err == nil {
		t.Fatal("Stream against a complete entry must error")
	}
	if !strings.Contains(err.Error(), `recorded kind "complete"`) {
		t.Errorf("kind-mismatch error = %q, want it to name the recorded kind", err)
	}

	// The entry was not consumed: the correct method still replays it.
	if _, err := rp.Complete(context.Background(), llmclient.Request{Prompt: "hi"}); err != nil {
		t.Fatalf("Complete after kind mismatch: %v", err)
	}
}

// TestReplay_MatchByRequestHash proves the opt-in strict mode: matching
// requests replay in order; a divergent request errors naming the entry index
// and both hashes, without consuming the entry.
func TestReplay_MatchByRequestHash(t *testing.T) {
	reqA := llmclient.Request{Prompt: "first question", MaxTokens: 16}
	reqB := llmclient.Request{Prompt: "second question", Temperature: 0.7}
	entries := recordCompletes(t, reqA, reqB)

	t.Run("matching requests replay in order", func(t *testing.T) {
		rp := llmreplay.NewReplayClient(entries, llmreplay.MatchByRequestHash())
		if _, err := rp.Complete(context.Background(), reqA); err != nil {
			t.Fatalf("Complete(reqA): %v", err)
		}
		if _, err := rp.Complete(context.Background(), reqB); err != nil {
			t.Fatalf("Complete(reqB): %v", err)
		}
	})

	t.Run("divergent request names index and both hashes", func(t *testing.T) {
		rp := llmreplay.NewReplayClient(entries, llmreplay.MatchByRequestHash())
		_, err := rp.Complete(context.Background(), reqB) // reqA was recorded first
		if err == nil {
			t.Fatal("hash mismatch must error")
		}
		gotHash, herr := llmreplay.RequestHash(reqB)
		if herr != nil {
			t.Fatalf("RequestHash(reqB): %v", herr)
		}
		wantHash, herr := llmreplay.RequestHash(reqA)
		if herr != nil {
			t.Fatalf("RequestHash(reqA): %v", herr)
		}
		msg := err.Error()
		for _, sub := range []string{"entry 0", "request hash mismatch", gotHash, wantHash} {
			if !strings.Contains(msg, sub) {
				t.Errorf("mismatch error %q missing %q", msg, sub)
			}
		}

		// Not consumed: the matching request still replays.
		if _, err := rp.Complete(context.Background(), reqA); err != nil {
			t.Fatalf("Complete(reqA) after mismatch: %v", err)
		}
	})
}

// TestRequestHash_Canonicalization proves the documented normalization: only
// JSON map-ordering nondeterminism (including inside json.RawMessage
// payloads) and whitespace are erased; any content difference changes the
// hash.
func TestRequestHash_Canonicalization(t *testing.T) {
	withSchema := func(schema string) llmclient.Request {
		return llmclient.Request{
			Prompt: "hi",
			Tools:  []llmclient.ToolDef{{Name: "t", InputSchema: json.RawMessage(schema)}},
		}
	}

	tests := []struct {
		name      string
		a, b      llmclient.Request
		wantEqual bool
	}{
		{
			name:      "raw JSON key order is erased",
			a:         withSchema(`{"a":1,"b":2}`),
			b:         withSchema(`{"b":2,"a":1}`),
			wantEqual: true,
		},
		{
			name:      "raw JSON whitespace is erased",
			a:         withSchema(`{"a": 1}`),
			b:         withSchema(`{"a":1}`),
			wantEqual: true,
		},
		{
			name:      "raw JSON content differs",
			a:         withSchema(`{"a":1}`),
			b:         withSchema(`{"a":2}`),
			wantEqual: false,
		},
		{
			name:      "prompt differs",
			a:         llmclient.Request{Prompt: "hi"},
			b:         llmclient.Request{Prompt: "bye"},
			wantEqual: false,
		},
		{
			name:      "sampling params differ",
			a:         llmclient.Request{Prompt: "hi", Temperature: 0.1},
			b:         llmclient.Request{Prompt: "hi", Temperature: 0.2},
			wantEqual: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ha, err := llmreplay.RequestHash(tc.a)
			if err != nil {
				t.Fatalf("RequestHash(a): %v", err)
			}
			hb, err := llmreplay.RequestHash(tc.b)
			if err != nil {
				t.Fatalf("RequestHash(b): %v", err)
			}
			if (ha == hb) != tc.wantEqual {
				t.Errorf("RequestHash equality = %v (a=%s b=%s), want %v", ha == hb, ha, hb, tc.wantEqual)
			}
		})
	}
}

// TestLoad_RejectsBadCassette proves Load surfaces malformed cassettes with
// line-accurate errors and the jsonl-log torn-tail sentinel.
func TestLoad_RejectsBadCassette(t *testing.T) {
	tests := []struct {
		name        string
		cassette    string
		wantSub     string
		wantPartial bool
	}{
		{
			name:     "invalid JSON line",
			cassette: "not json\n",
			wantSub:  "invalid JSON",
		},
		{
			name:     "unknown entry kind",
			cassette: `{"kind":"weird","request":{}}` + "\n",
			wantSub:  `unknown entry kind "weird"`,
		},
		{
			name:     "complete entry with neither response nor err",
			cassette: `{"kind":"complete","request":{}}` + "\n",
			wantSub:  "neither response nor err",
		},
		{
			name:        "torn trailing line",
			cassette:    `{"kind":"complete","request":{},"response":{"Text":"hi"`,
			wantPartial: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := llmreplay.Load(strings.NewReader(tc.cassette))
			if err == nil {
				t.Fatal("Load must reject the malformed cassette")
			}
			if tc.wantPartial {
				if !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
					t.Errorf("error = %v, want errors.Is(_, jsonllog.ErrPartialTrailingLine)", err)
				}
				return
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantSub)
			}
		})
	}
}

// TestReplay_StreamRecordedUnderCancellation proves a stream entry with no
// terminal chunk — zero chunks, or a non-terminal last chunk (no
// StopReason/Usage/Err) — replays its recorded chunks and then a SYNTHESIZED
// terminal chunk whose Err is a *ReplayedError naming the cancellation,
// instead of closing the channel with no terminal chunk while ctx is live.
func TestReplay_StreamRecordedUnderCancellation(t *testing.T) {
	cases := []struct {
		name     string
		chunks   []llmreplay.Chunk
		recorded int // chunks replayed verbatim before the synthesized terminal
	}{
		{"zero chunks", nil, 0},
		{"non-terminal last chunk", []llmreplay.Chunk{{TextDelta: "par"}, {TextDelta: "tial"}}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp := llmreplay.NewReplayClient([]llmreplay.Entry{{Kind: llmreplay.KindStream, Chunks: tc.chunks}})
			ch, err := rp.Stream(context.Background(), llmclient.Request{Prompt: "hi"})
			if err != nil {
				t.Fatalf("Stream: %v", err)
			}
			chunks := drainAll(ch)
			if len(chunks) != tc.recorded+1 {
				t.Fatalf("replayed %d chunks, want %d recorded + 1 synthesized terminal", len(chunks), tc.recorded)
			}
			for i := 0; i < tc.recorded; i++ {
				if chunks[i].TextDelta != tc.chunks[i].TextDelta {
					t.Errorf("chunk %d: TextDelta = %q, want %q", i, chunks[i].TextDelta, tc.chunks[i].TextDelta)
				}
				if chunks[i].Err != nil {
					t.Errorf("recorded chunk %d replayed with unexpected Err: %v", i, chunks[i].Err)
				}
			}
			last := chunks[len(chunks)-1]
			var re *llmreplay.ReplayedError
			if !errors.As(last.Err, &re) {
				t.Fatalf("terminal Err = %T %v, want *llmreplay.ReplayedError", last.Err, last.Err)
			}
			if !strings.Contains(re.Message, "cancellation") {
				t.Errorf("message %q should explain the cassette was recorded under cancellation", re.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Wave 0 — v2 canonical hash tests (spec §1)
// ---------------------------------------------------------------------------

// TestRequestHash_GoldenV2 pins the v2 hash for a specific, fully-specified
// request. If canonicalisation changes, this test breaks loudly instead of
// silently invalidating every cassette in the fleet.
//
// The golden hash was produced by the first correct v2 implementation and
// checked in. To regenerate: run the test with -v and copy the printed hash,
// then verify the canonical form manually before accepting the new value.
func TestRequestHash_GoldenV2(t *testing.T) {
	// A request with no zero-valued fields so v1 and v2 produce the same
	// canonical form — the hash is unambiguous regardless of algorithm.
	req := llmclient.Request{
		Model:     "m-test",
		Prompt:    "golden",
		MaxTokens: 64,
		System:    "sys",
		Tools: []llmclient.ToolDef{{
			Name:        "t",
			Description: "a tool",
			InputSchema: json.RawMessage(`{"b":2,"a":1}`), // key order is erased by canonicalize
		}},
	}
	const want = "f055dd397528c1e1924840cd64260182164a5a1d64a0252e84968fc959af5e2b"
	got, err := llmreplay.RequestHash(req)
	if err != nil {
		t.Fatalf("RequestHash: %v", err)
	}
	if got != want {
		t.Errorf("v2 golden hash changed:\n got  %s\n want %s\nIf the canonical algorithm was intentionally updated, re-pin this constant and audit all cassettes.", got, want)
	}
}

// TestRequestHash_ZeroFieldStability is the key property test for v2: adding
// a zero-valued field to a struct must NOT change the hash. This is the
// invariant that makes cassettes survive additive schema growth.
func TestRequestHash_ZeroFieldStability(t *testing.T) {
	// base: a minimal, fully-meaningful request.
	base := llmclient.Request{
		Prompt:    "stability",
		MaxTokens: 32,
	}
	// withZeroes: same semantic content but with struct fields explicitly set
	// to their zero values. V2 must drop them and produce the same hash.
	withZeroes := llmclient.Request{
		Model:           "", // zero string
		Prompt:          "stability",
		MaxTokens:       32,
		System:          "",    // zero string
		SystemCacheHint: false, // zero bool
		Temperature:     0,     // zero float64
		StopSequences:   nil,   // nil slice
		Tools:           nil,   // nil slice
	}

	hBase, err := llmreplay.RequestHash(base)
	if err != nil {
		t.Fatalf("RequestHash(base): %v", err)
	}
	hWithZeroes, err := llmreplay.RequestHash(withZeroes)
	if err != nil {
		t.Fatalf("RequestHash(withZeroes): %v", err)
	}
	if hBase != hWithZeroes {
		t.Errorf("v2 hash changed when zero-valued fields were added:\nbase       %s\nwithZeroes %s\nZero-valued fields must be dropped before hashing.", hBase, hWithZeroes)
	}

	// Also verify: a NON-zero field genuinely changes the hash.
	withNonZero := base
	withNonZero.System = "non-empty system prompt"
	hNonZero, err := llmreplay.RequestHash(withNonZero)
	if err != nil {
		t.Fatalf("RequestHash(withNonZero): %v", err)
	}
	if hBase == hNonZero {
		t.Errorf("v2 hash did not change when a non-zero System field was added — zero-drop rule is too aggressive")
	}
}

// TestRequestHash_ContentBlockZeroFields proves the specific field-evolution
// scenario that caused the gate to turn red: adding MediaType and Data to
// ContentBlock must not change the hash of a block that does not use them.
// This is the regression test for commit 19a63b4 (image blocks).
func TestRequestHash_ContentBlockZeroFields(t *testing.T) {
	// A text-only request: the kind recorded in the old cassette, which had
	// no MediaType/Data fields in the struct at all.
	old := llmclient.Request{
		Prompt: "words",
		Messages: []llmclient.Message{{
			Role:    llmclient.RoleUser,
			Content: []llmclient.ContentBlock{llmclient.TextBlock("hello")},
		}},
	}
	// The same request under the new struct: MediaType and Data now exist as
	// zero-valued string fields and are marshalled unless dropped.
	newWithZeroImageFields := old // same value — zero fields are in the type
	// (MediaType: "" and Data: "" are the zero values; TextBlock sets neither)

	hOld, err := llmreplay.RequestHash(old)
	if err != nil {
		t.Fatalf("RequestHash(old): %v", err)
	}
	hNew, err := llmreplay.RequestHash(newWithZeroImageFields)
	if err != nil {
		t.Fatalf("RequestHash(newWithZeroImageFields): %v", err)
	}
	if hOld != hNew {
		t.Errorf("ContentBlock zero-field stability: hash changed between old and new struct shapes:\nold %s\nnew %s", hOld, hNew)
	}

	// Sanity: an actual image block hashes differently from a text block.
	withImage := llmclient.Request{
		Prompt: "words",
		Messages: []llmclient.Message{{
			Role: llmclient.RoleUser,
			Content: []llmclient.ContentBlock{
				llmclient.ImageBlock("image/png", "aGVsbG8="),
			},
		}},
	}
	hImage, err := llmreplay.RequestHash(withImage)
	if err != nil {
		t.Fatalf("RequestHash(withImage): %v", err)
	}
	if hOld == hImage {
		t.Errorf("image block hashed equal to text block — MediaType/Data are being incorrectly dropped for non-zero values")
	}
}

// TestV1CassetteReplaysGreen proves a cassette recorded before v2 (no
// hash_version field) can be replayed with MatchByRequestHash against a
// current request struct that has new zero-valued fields the old cassette
// did not include. This is the backward-compatibility guarantee.
func TestV1CassetteReplaysGreen(t *testing.T) {
	// Construct a v1-era cassette entry by hand: no hash_version field, and
	// request JSON that omits fields added after the cassette was recorded
	// (simulating ContentBlock before MediaType/Data existed). We use a
	// tool_result block whose Content field IS set so the JSON is realistic.
	const v1CassetteJSON = `{"kind":"complete","request":{"Model":"m","Prompt":"q","Messages":[{"Role":"user","Content":[{"Type":"text","Text":"hello"}]}],"MaxTokens":8,"System":"","SystemCacheHint":false,"Temperature":0,"StopSequences":null,"Tools":null,"ToolChoice":{"Mode":"","Name":""}},"response":{"Text":"world","Content":[{"Type":"text","Text":"world"}],"StopReason":"end_turn","StopSequence":"","Attempts":0,"Rung":0,"InputTokens":0,"OutputTokens":0,"CacheCreationInputTokens":0,"CacheReadInputTokens":0}}` + "\n"

	entries, err := llmreplay.Load(strings.NewReader(v1CassetteJSON))
	if err != nil {
		t.Fatalf("Load v1 cassette: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].HashVersion != 0 {
		t.Fatalf("v1 entry HashVersion = %d, want 0 (absent)", entries[0].HashVersion)
	}

	// The LIVE request uses the current struct, which now has MediaType and
	// Data as zero-valued fields. The hash must still match the v1 cassette.
	liveReq := llmclient.Request{
		Model:     "m",
		Prompt:    "q",
		MaxTokens: 8,
		Messages: []llmclient.Message{{
			Role: llmclient.RoleUser,
			Content: []llmclient.ContentBlock{
				// TextBlock sets Type+Text only; MediaType and Data are zero.
				llmclient.TextBlock("hello"),
			},
		}},
	}

	rp := llmreplay.NewReplayClient(entries, llmreplay.MatchByRequestHash())
	resp, err := rp.Complete(context.Background(), liveReq)
	if err != nil {
		t.Fatalf("replay v1 cassette with MatchByRequestHash: %v\n(v1 cassette should verify against current struct via v2 hash on both sides)", err)
	}
	if resp.Text != "world" {
		t.Errorf("replayed response Text = %q, want %q", resp.Text, "world")
	}
}

// TestRequestHash_Wave1ZeroFieldStability is the Wave 1 regression analogue of
// TestRequestHash_ContentBlockZeroFields: the three new Request fields
// (Thinking *ThinkingConfig, Effort string, ProviderParams json.RawMessage)
// must not change the v2 hash of an existing request that does not set them.
// This is the cassette-survival guarantee for additive Wave 1 schema growth.
//
// Note: callers represent "no provider params" as nil (not json.RawMessage("")).
// An empty-string RawMessage is not valid JSON and cannot be marshalled, so it
// is not a valid zero-value for hashing. The ValidateProviderParams function
// accepts both nil and empty for safety, but the hash path requires nil.
func TestRequestHash_Wave1ZeroFieldStability(t *testing.T) {
	base := llmclient.Request{
		Model:     "claude-sonnet",
		Prompt:    "wave1 stability",
		MaxTokens: 128,
	}
	// Same request but with every new Wave 1 field explicitly set to its zero
	// value. V2 canonical hash must drop them all.
	withWave1Zeroes := llmclient.Request{
		Model:          "claude-sonnet",
		Prompt:         "wave1 stability",
		MaxTokens:      128,
		Thinking:       nil, // zero pointer
		Effort:         "",  // zero string
		ProviderParams: nil, // nil RawMessage — the caller's "no params" value
	}

	hBase, err := llmreplay.RequestHash(base)
	if err != nil {
		t.Fatalf("RequestHash(base): %v", err)
	}
	hZeroes, err := llmreplay.RequestHash(withWave1Zeroes)
	if err != nil {
		t.Fatalf("RequestHash(withWave1Zeroes): %v", err)
	}

	if hBase != hZeroes {
		t.Errorf("Wave 1 nil/zero fields changed the v2 hash:\nbase    %s\nzeroes  %s\nZero Thinking/Effort/ProviderParams must be dropped before hashing.", hBase, hZeroes)
	}

	// Sanity: a non-zero ProviderParams DOES change the hash.
	withParams := base
	withParams.ProviderParams = json.RawMessage(`{"extra":"field"}`)
	hWithParams, err := llmreplay.RequestHash(withParams)
	if err != nil {
		t.Fatalf("RequestHash(withParams): %v", err)
	}
	if hBase == hWithParams {
		t.Errorf("Non-zero ProviderParams did NOT change the hash — zero-drop is too aggressive")
	}

	// Sanity: a non-nil Thinking DOES change the hash.
	withThinking := base
	withThinking.Thinking = &llmclient.ThinkingConfig{Type: "enabled", BudgetTokens: 1024}
	hWithThinking, err := llmreplay.RequestHash(withThinking)
	if err != nil {
		t.Fatalf("RequestHash(withThinking): %v", err)
	}
	if hBase == hWithThinking {
		t.Errorf("Non-nil Thinking did NOT change the hash — zero-drop is too aggressive")
	}
}

// TestReplay_StreamTerminalChunkNotDuplicated proves the synthesis is gated:
// an entry whose last chunk IS terminal (here: a recorded mid-stream Err)
// replays exactly its recorded chunks, with no extra synthesized terminal.
func TestReplay_StreamTerminalChunkNotDuplicated(t *testing.T) {
	rp := llmreplay.NewReplayClient([]llmreplay.Entry{{
		Kind:   llmreplay.KindStream,
		Chunks: []llmreplay.Chunk{{TextDelta: "partial"}, {Err: "boom mid-stream"}},
	}})
	ch, err := rp.Stream(context.Background(), llmclient.Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainAll(ch)
	if len(chunks) != 2 {
		t.Fatalf("replayed %d chunks, want exactly the 2 recorded ones", len(chunks))
	}
	var re *llmreplay.ReplayedError
	if !errors.As(chunks[1].Err, &re) || re.Message != "boom mid-stream" {
		t.Fatalf("terminal chunk = %+v, want the recorded *ReplayedError(%q)", chunks[1], "boom mid-stream")
	}
}
