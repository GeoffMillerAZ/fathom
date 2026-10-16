package decision_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/llm"
	llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// replayRoundTrip marshals t to JSON and back, as a transcript file would.
func replayRoundTrip(t *testing.T, tr decision.Transcript) decision.Transcript {
	t.Helper()
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("json.Marshal(Transcript) error = %v", err)
	}
	var back decision.Transcript
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("json.Unmarshal(Transcript) error = %v", err)
	}
	return back
}

func TestReplayReproducesRecordedResponses(t *testing.T) {
	ctx := context.Background()
	reqA := decision.Request{State: "diff A", Questions: []decision.Question{
		decorChoiceQuestion("root"),
		{ID: "severity", Kind: decision.KindScore, Instructions: "how bad?", Levels: []string{"low", "mid", "high"}},
		decorQuestion("safe"),
	}}
	reqB := decision.Request{State: "diff B", Questions: []decision.Question{decorQuestion("safe")}}

	probs := map[string]float64{"a": 0.1, "b": 0.7, "c": 0.2}
	first := decision.Response{
		Provider: "mock", Model: "m-1",
		Answers: map[string]decision.Answer{
			"root":     decorChoice("root", "b", probs),
			"severity": {ID: "severity", Kind: decision.KindScore, Score: 2, Confidence: 1},
		},
		Invalid: map[string]string{"safe": "out of range"},
		Usage:   decision.Usage{InputTokens: 512, OutputTokens: 17},
		Latency: 431 * time.Millisecond,
	}
	second := decision.Response{
		Provider: "mock", Model: "m-1",
		Answers: map[string]decision.Answer{
			"root":     decorChoice("root", "a", map[string]float64{"a": 0.8, "b": 0.1, "c": 0.1}),
			"severity": {ID: "severity", Kind: decision.KindScore, Score: 1, Confidence: 1},
			"safe":     decorPredicate("safe", 0.25),
		},
		Invalid: map[string]string{},
		Latency: 402 * time.Millisecond,
	}
	inner := decision.NewMockDecider("mock",
		decision.MockEntry{Response: first},
		decision.MockEntry{Response: decision.Response{Provider: "mock"}, Err: decision.ErrNoProvider},
		decision.MockEntry{Response: second},
	)
	inner.SetCapabilities(decision.Capabilities{
		Declared: true, Kinds: []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		SingleRoundTrip: true, Latency: decision.LatencySubSecond, Cost: decision.CostLow,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
	})

	rec, err := decision.NewRecorder(inner, decision.RecorderOptions{})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}
	if got := rec.Name(); got != "recorder" {
		t.Errorf("Recorder Name() = %q, want recorder", got)
	}
	if got, want := decision.CapabilitiesOf(rec), decision.CapabilitiesOf(inner); !reflect.DeepEqual(got, want) {
		t.Errorf("Recorder capabilities = %+v, want the inner's %+v", got, want)
	}

	// Record: A, B (an abstention), then A again (a different answer).
	gotA1, errA1 := rec.Decide(ctx, reqA)
	gotB, errB := rec.Decide(ctx, reqB)
	gotA2, errA2 := rec.Decide(ctx, reqA)
	if errA1 != nil || errA2 != nil || !errors.Is(errB, decision.ErrNoProvider) {
		t.Fatalf("recording errors = %v, %v, %v", errA1, errB, errA2)
	}
	if !reflect.DeepEqual(gotA1, first) || !reflect.DeepEqual(gotA2, second) {
		t.Fatal("the Recorder changed the inner Decider's Responses")
	}

	tr := rec.Transcript()
	if tr.Version != decision.TranscriptVersion || tr.Decider != "mock" || len(tr.Entries) != 3 {
		t.Fatalf("Transcript = version %d, decider %q, %d entries", tr.Version, tr.Decider, len(tr.Entries))
	}
	// Stable order: sorted by key, then by occurrence.
	if !sort.SliceIsSorted(tr.Entries, func(i, j int) bool {
		a, b := tr.Entries[i], tr.Entries[j]
		return a.Key < b.Key || (a.Key == b.Key && a.Seq < b.Seq)
	}) {
		t.Error("Transcript entries are not sorted by key, then seq")
	}
	for _, e := range tr.Entries {
		if e.Key != decision.RequestKey(e.Request) {
			t.Errorf("entry key %q is not RequestKey of its Request", e.Key)
		}
	}
	data1, _ := json.Marshal(tr)
	data2, _ := json.Marshal(rec.Transcript())
	if !bytes.Equal(data1, data2) {
		t.Error("two snapshots of one transcript marshal differently")
	}

	replay, err := decision.NewReplay(replayRoundTrip(t, tr), decision.ReplayOptions{})
	if err != nil {
		t.Fatalf("NewReplay() error = %v", err)
	}
	if got := replay.Name(); got != "replay" {
		t.Errorf("Replay Name() = %q, want replay", got)
	}

	// Replay: identical Responses, in recorded order per request; the last
	// occurrence repeats.
	for i, want := range []decision.Response{first, second, second} {
		got, err := replay.Decide(ctx, reqA)
		if err != nil {
			t.Fatalf("replay A #%d error = %v", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("replay A #%d =\n %+v\nwant\n %+v", i, got, want)
		}
	}
	gotB2, errB2 := replay.Decide(ctx, reqB)
	if !errors.Is(errB2, decision.ErrNoProvider) || errB2.Error() != errB.Error() {
		t.Errorf("replay B error = %v, want the recorded ErrNoProvider %v", errB2, errB)
	}
	if !reflect.DeepEqual(gotB2, gotB) {
		t.Errorf("replay B Response = %+v, want %+v", gotB2, gotB)
	}

	// An unrecorded Request is an error; the replay never calls a provider.
	if _, err := replay.Decide(ctx, decision.Request{State: "never seen", Questions: []decision.Question{decorQuestion("safe")}}); !errors.Is(err, decision.ErrNotRecorded) {
		t.Errorf("unrecorded err = %v, want ErrNotRecorded", err)
	}
	if n := len(inner.Calls()); n != 3 {
		t.Errorf("inner called %d times, want only the 3 recorded calls", n)
	}

	// The replay declares the recorded kinds, and no latency or cost.
	caps := decision.CapabilitiesOf(replay)
	if !caps.Declared || caps.Latency != decision.LatencyNone || caps.Cost != decision.CostNone ||
		!reflect.DeepEqual(caps.Kinds, decision.CapabilitiesOf(inner).Kinds) || caps.ConfidenceSemantics != decision.ConfidenceNativeProbability {
		t.Errorf("Replay capabilities = %+v", caps)
	}

	t.Run("an LLM over a MockClient replays byte for byte", func(t *testing.T) {
		client := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text:  `{"answers":{"safe":{"noul":0.2}}}`,
			Usage: llmclient.Usage{InputTokens: 90, OutputTokens: 12},
		}})
		clock := time.Unix(0, 0)
		adapter := llm.NewLLM(client, "test-model", llm.WithNow(func() time.Time { clock = clock.Add(time.Millisecond); return clock }))
		rec, _ := decision.NewRecorder(adapter, decision.RecorderOptions{Name: "llm-recorder"})
		want, err := rec.Decide(ctx, reqB)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := decision.NewReplay(replayRoundTrip(t, rec.Transcript()), decision.ReplayOptions{Name: "llm-replay"})
		if err != nil {
			t.Fatal(err)
		}
		got, err := replay.Decide(ctx, reqB)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("replay = %+v, %v; want %+v", got, err, want)
		}
		if client.CallCount() != 1 {
			t.Errorf("MockClient called %d times, want 1", client.CallCount())
		}
	})

	t.Run("a tampered or foreign transcript is refused", func(t *testing.T) {
		bad := replayRoundTrip(t, tr)
		bad.Entries[0].Key = "sha256:0000"
		if _, err := decision.NewReplay(bad, decision.ReplayOptions{}); err == nil {
			t.Error("mismatched key: want an error")
		}
		future := replayRoundTrip(t, tr)
		future.Version = decision.TranscriptVersion + 1
		if _, err := decision.NewReplay(future, decision.ReplayOptions{}); err == nil {
			t.Error("unknown version: want an error")
		}
		dup := replayRoundTrip(t, tr)
		dup.Entries = append(dup.Entries, dup.Entries[0])
		if _, err := decision.NewReplay(dup, decision.ReplayOptions{}); err == nil {
			t.Error("duplicate key and seq: want an error")
		}
	})

	t.Run("the request key ignores nothing but the noul alias", func(t *testing.T) {
		alias := decision.Request{State: reqB.State, Questions: []decision.Question{decorQuestion("safe")}}
		alias.Questions[0].Kind = decision.Kind("noul")
		if decision.RequestKey(alias) != decision.RequestKey(reqB) {
			t.Error("the noul alias changed the request key")
		}
		renamed := decision.Request{State: reqB.State, Questions: []decision.Question{decorQuestion("safe")}}
		renamed.Questions[0].ID = "other"
		if decision.RequestKey(renamed) == decision.RequestKey(reqB) {
			t.Error("a different question ID shares the request key")
		}
		if decision.RequestKey(decision.Request{State: "x"}) == decision.RequestKey(decision.Request{State: "y"}) {
			t.Error("a different State shares the request key")
		}
	})
}

func TestNewRecorderRejectsNilInner(t *testing.T) {
	if _, err := decision.NewRecorder(nil, decision.RecorderOptions{}); err == nil {
		t.Error("nil inner: want an error")
	}
}

func TestRecorderAndReplayConcurrentUse(t *testing.T) {
	inner := &decorStub{name: "stub", fn: decorEcho("stub", "s-1", 0.9)}
	rec, err := decision.NewRecorder(inner, decision.RecorderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const n = 16
	reqs := make([]decision.Request, n)
	for i := range n {
		reqs[i] = decision.Request{State: fmt.Sprintf("s%d", i), Questions: []decision.Question{decorQuestion("q1")}}
	}
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = rec.Decide(context.Background(), reqs[i])
			_ = rec.Transcript()
		}()
	}
	wg.Wait()
	replay, err := decision.NewReplay(rec.Transcript(), decision.ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := replay.Decide(context.Background(), reqs[i])
			if err != nil || got.Answers["q1"].Noul != 0.9 {
				errs <- fmt.Errorf("replay %d: %+v, %v", i, got, err)
				return
			}
			// Mutating a replayed Response must not change the transcript.
			delete(got.Answers, "q1")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	for i := range n {
		if got, err := replay.Decide(context.Background(), reqs[i]); err != nil || len(got.Answers) != 1 {
			t.Errorf("replay %d after mutation: %+v, %v", i, got, err)
		}
	}
}
