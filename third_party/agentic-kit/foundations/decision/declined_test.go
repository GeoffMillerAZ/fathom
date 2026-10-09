package decision_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// decorDecline answers every question of req except those in refuse, which
// it declines, under provider and model.
func decorDecline(provider, model string, refuse ...string) func(decision.Request) (decision.Response, error) {
	return func(req decision.Request) (decision.Response, error) {
		resp := decision.Response{Provider: provider, Model: model, Answers: map[string]decision.Answer{}, Invalid: map[string]string{}}
		for _, q := range req.Questions {
			declined := false
			for _, id := range refuse {
				declined = declined || id == q.ID
			}
			if declined {
				if resp.Declined == nil {
					resp.Declined = map[string]string{}
				}
				resp.Declined[q.ID] = provider + " refused"
				continue
			}
			resp.Answers[q.ID] = decorPredicate(q.ID, 0.9)
		}
		return resp, nil
	}
}

// testImage is one well-formed image for Requests in these tests.
var testImage = decision.Image{MediaType: "image/png", Data: []byte("not really a png")}

func TestFallbackTreatsDeclinedAsDisclosedMiss(t *testing.T) {
	ctx := context.Background()
	req := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2")}}

	t.Run("a decline moves to the next Decider, and the miss names the declined questions", func(t *testing.T) {
		first := &decorStub{name: "first", fn: decorDecline("first", "f", "q2")}
		second := &decorStub{name: "second", fn: decorEcho("second", "s", 0.8)}
		f, err := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{first, second}, Observe: func(decision.FallbackReport) {}})
		if err != nil {
			t.Fatal(err)
		}
		resp, report, err := f.DecideReport(ctx, req)
		if err != nil {
			t.Fatalf("DecideReport() error = %v", err)
		}
		if resp.Provider != "second" || len(resp.Declined) != 0 || len(resp.Answers) != 2 {
			t.Errorf("Response = %+v, want second's two answers and nothing declined", resp)
		}
		if report.Answered != 1 || len(report.Failures) != 1 {
			t.Fatalf("report = %+v, want answered by 1 after one miss", report)
		}
		miss := report.Failures[0]
		if miss.Index != 0 || miss.Name != "first" || !errors.Is(miss.Err, decision.ErrDeclined) || !strings.Contains(miss.Err.Error(), "q2") {
			t.Errorf("miss = %+v (%v), want first's decline wrapping ErrDeclined and naming q2", miss, miss.Err)
		}
	})

	t.Run("when every Decider declines, the first declined Response is returned unchanged", func(t *testing.T) {
		first := &decorStub{name: "first", fn: decorDecline("first", "f", "q2")}
		second := &decorStub{name: "second", fn: decorDecline("second", "s", "q1", "q2")}
		failing := &decorStub{name: "failing", fn: func(decision.Request) (decision.Response, error) {
			return decision.Response{}, errors.New("provider down")
		}}
		f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{first, second, failing}, Observe: func(decision.FallbackReport) {}})
		resp, report, err := f.DecideReport(ctx, req)
		if err != nil {
			t.Fatalf("DecideReport() error = %v, want the held declined Response", err)
		}
		want, _ := decorDecline("first", "f", "q2")(req)
		if !reflect.DeepEqual(resp, want) {
			t.Errorf("Response = %+v, want first's unchanged %+v", resp, want)
		}
		if _, answered := resp.Answers["q2"]; answered {
			t.Error("a declined question was turned into an answer")
		}
		if report.Answered != 0 || report.Provider != "first" || len(report.Failures) != 3 {
			t.Errorf("report = %+v, want answered by 0 with all three misses disclosed", report)
		}
		if !errors.Is(report.Failures[1].Err, decision.ErrDeclined) || errors.Is(report.Failures[2].Err, decision.ErrDeclined) {
			t.Errorf("failures = %+v, want a decline then an error", report.Failures)
		}
	})

	t.Run("a Response without declines is unchanged behavior", func(t *testing.T) {
		first := &decorStub{name: "first", fn: decorEcho("first", "f", 0.8)}
		second := &decorStub{name: "second", fn: decorEcho("second", "s", 0.8)}
		f, _ := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{first, second}, Observe: func(decision.FallbackReport) {}})
		resp, report, err := f.DecideReport(ctx, req)
		if err != nil || resp.Provider != "first" || report.Answered != 0 || len(report.Failures) != 0 || resp.Declined != nil {
			t.Errorf("DecideReport() = %+v, %+v, %v; want first's Response, no failures, nil Declined", resp, report, err)
		}
		if second.calls.Load() != 0 {
			t.Errorf("second called %d times", second.calls.Load())
		}
	})
}

func TestCascadeEscalatesDeclinedQuestions(t *testing.T) {
	ctx := context.Background()
	req := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2")}}

	t.Run("a declined question escalates and the next stage answers it", func(t *testing.T) {
		cheap := &decorStub{name: "cheap", fn: decorDecline("cheap", "c", "q2")}
		strong := &decorStub{name: "strong", fn: decorEcho("strong", "s", 0.9)}
		c, err := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, strong}})
		if err != nil {
			t.Fatal(err)
		}
		resp, report, err := c.DecideReport(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if got := report.Steps[0].Escalated["q2"]; got != "declined: cheap refused" {
			t.Errorf("step 0 escalated q2 because %q, want the refusal", got)
		}
		if !reflect.DeepEqual(report.Steps[1].Asked, []string{"q2"}) {
			t.Errorf("step 1 asked %v, want [q2]", report.Steps[1].Asked)
		}
		if len(resp.Answers) != 2 || resp.Declined != nil {
			t.Errorf("Response = %+v, want both answered and nothing declined", resp)
		}
	})

	t.Run("a question the last stage declines stays declined, never invalid or answered", func(t *testing.T) {
		cheap := &decorStub{name: "cheap", fn: func(r decision.Request) (decision.Response, error) {
			return decision.Response{Provider: "cheap", Invalid: map[string]string{"q2": "malformed"}, Answers: map[string]decision.Answer{"q1": decorPredicate("q1", 0.9)}}, nil
		}}
		strong := &decorStub{name: "strong", fn: decorDecline("strong", "s", "q2")}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, strong}})
		resp, err := c.Decide(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Declined["q2"]; got != "strong refused" {
			t.Errorf("Declined[q2] = %q, want strong's refusal", got)
		}
		if _, ok := resp.Invalid["q2"]; ok {
			t.Error("q2 is in Invalid as well as Declined")
		}
		if _, ok := resp.Answers["q2"]; ok {
			t.Error("q2 was turned into an answer")
		}
	})

	t.Run("a later invalid reason replaces an earlier decline", func(t *testing.T) {
		cheap := &decorStub{name: "cheap", fn: decorDecline("cheap", "c", "q2")}
		strong := &decorStub{name: "strong", fn: func(r decision.Request) (decision.Response, error) {
			return decision.Response{Provider: "strong", Invalid: map[string]string{"q2": "malformed"}}, nil
		}}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, strong}})
		resp, err := c.Decide(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Invalid["q2"] != "malformed" || resp.Declined != nil {
			t.Errorf("Response = %+v, want q2 invalid with the last reason and nothing declined", resp)
		}
	})
}

func TestCachedNeverStoresDeclinedAnswers(t *testing.T) {
	ctx := context.Background()
	store := decision.NewMemoryCacheStore()
	inner := &decorStub{name: "inner", fn: decorDecline("inner", "m", "q2")}
	c, err := decision.NewCached(inner, decision.CachedOptions{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	req := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2")}}
	for i := 0; i < 2; i++ {
		resp, report, err := c.DecideReport(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Declined["q2"] != "inner refused" || len(resp.Invalid) != 0 {
			t.Errorf("call %d: Response = %+v, want q2 declined and nothing invalid", i, resp)
		}
		if fresh := report.Fresh(resp); !reflect.DeepEqual(fresh.Declined, resp.Declined) {
			t.Errorf("call %d: Fresh dropped Declined: %+v", i, fresh.Declined)
		}
		if i == 1 && !reflect.DeepEqual(report.Misses, []string{"q2"}) {
			t.Errorf("call 1 missed %v, want only the declined q2 re-asked", report.Misses)
		}
	}
	if n := store.Len(); n != 1 {
		t.Errorf("store holds %d entries, want 1 (the declined question is never stored)", n)
	}
}

func TestImagesReachEveryDecoratorStage(t *testing.T) {
	ctx := context.Background()
	req := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}, Images: []decision.Image{testImage}}

	var mu sync.Mutex
	var seen [][]decision.Image
	record := func(provider string) *decorStub {
		return &decorStub{name: provider, fn: func(r decision.Request) (decision.Response, error) {
			mu.Lock()
			seen = append(seen, r.Images)
			mu.Unlock()
			return decorEcho(provider, "m", 0.9)(r)
		}}
	}

	t.Run("a cascade stage gets the images with its sub-Request", func(t *testing.T) {
		seen = nil
		low := &decorStub{name: "low", fn: func(r decision.Request) (decision.Response, error) {
			mu.Lock()
			seen = append(seen, r.Images)
			mu.Unlock()
			return decorEcho("low", "m", 0.6)(r)
		}}
		c, _ := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{low, record("high")}, EscalateBelow: 0.8})
		if _, err := c.Decide(ctx, req); err != nil {
			t.Fatal(err)
		}
		if len(seen) != 2 || !reflect.DeepEqual(seen[0], req.Images) || !reflect.DeepEqual(seen[1], req.Images) {
			t.Errorf("stages saw images %v, want both to get %v", seen, req.Images)
		}
	})

	t.Run("a cache passes the images to its inner Decider and never serves or stores them", func(t *testing.T) {
		seen = nil
		store := decision.NewMemoryCacheStore()
		c, _ := decision.NewCached(record("inner"), decision.CachedOptions{Store: store})
		for i := 0; i < 2; i++ {
			_, report, err := c.DecideReport(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Hits) != 0 || !reflect.DeepEqual(report.Misses, []string{"q1"}) {
				t.Errorf("call %d: hits %v misses %v, want every question a miss", i, report.Hits, report.Misses)
			}
		}
		if store.Len() != 0 {
			t.Errorf("store holds %d entries for a Request with images, want 0", store.Len())
		}
		if len(seen) != 2 || !reflect.DeepEqual(seen[0], req.Images) {
			t.Errorf("inner saw images %v, want %v on both calls", seen, req.Images)
		}
		// The same text without the image is still cached as before.
		text := decision.Request{State: req.State, Questions: req.Questions}
		if _, err := c.Decide(ctx, text); err != nil {
			t.Fatal(err)
		}
		if store.Len() != 1 {
			t.Errorf("store holds %d entries after a text-only Request, want 1", store.Len())
		}
	})
}

func TestUnsupportedImages(t *testing.T) {
	text := decision.Request{State: "s"}
	withImages := decision.Request{State: "s", Images: []decision.Image{testImage, testImage}}
	textOnly := decision.Capabilities{Declared: true, Kinds: []decision.Kind{decision.KindPredicate}}
	images := textOnly
	images.Images = true

	for _, c := range []struct {
		name string
		caps decision.Capabilities
		req  decision.Request
		want bool
	}{
		{"text-only Request", textOnly, text, false},
		{"images to a text-only Decider", textOnly, withImages, true},
		{"images to a Decider that declares them", images, withImages, false},
		{"images to an undeclared Decider", decision.Capabilities{}, withImages, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			reason, got := decision.UnsupportedImages("p", c.caps, c.req)
			if got != c.want {
				t.Fatalf("UnsupportedImages() = %v, want %v", got, c.want)
			}
			if got && (!strings.Contains(reason, "p takes no image input") || !strings.Contains(reason, "2 image(s)")) {
				t.Errorf("reason %q does not name the provider and the image count", reason)
			}
		})
	}
}

func TestValidateImages(t *testing.T) {
	q := []decision.Question{decorQuestion("q1")}
	for _, c := range []struct {
		name    string
		images  []decision.Image
		wantErr string
	}{
		{"no images", nil, ""},
		{"one png", []decision.Image{testImage}, ""},
		{"jpeg", []decision.Image{{MediaType: "image/jpeg", Data: []byte{0xff}}}, ""},
		{"text media type", []decision.Image{{MediaType: "text/plain", Data: []byte("x")}}, "image 0: MediaType"},
		{"bare prefix", []decision.Image{{MediaType: "image/", Data: []byte("x")}}, "image 0: MediaType"},
		{"empty data", []decision.Image{testImage, {MediaType: "image/png"}}, "image 1: Data must not be empty"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := decision.Validate(decision.Request{State: "s", Questions: q, Images: c.images})
			switch {
			case c.wantErr == "" && err != nil:
				t.Errorf("Validate() error = %v, want nil", err)
			case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
				t.Errorf("Validate() error = %v, want one containing %q", err, c.wantErr)
			}
		})
	}
}

func TestRequestKeyWithoutImagesIsUnchanged(t *testing.T) {
	req := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}}
	// Computed at 064100f, before Request.Images existed: a recorded
	// transcript still replays.
	const pinned = "sha256:10bfdb5ecdeea49a49aec976a9588ffe72d9508535d7d1f163312e33db8b4831"
	got := decision.RequestKey(req)
	if got != pinned {
		t.Errorf("RequestKey() = %s, want the pre-Images key %s", got, pinned)
	}
	withImage := req
	withImage.Images = []decision.Image{testImage}
	if decision.RequestKey(withImage) == got {
		t.Error("a Request with an image keys the same as its text alone")
	}
	other := req
	other.Images = []decision.Image{{MediaType: "image/png", Data: []byte("another png")}}
	if decision.RequestKey(other) == decision.RequestKey(withImage) {
		t.Error("two different images key the same")
	}
}

func TestTranscriptCarriesImagesAndDeclined(t *testing.T) {
	ctx := context.Background()
	inner := &decorStub{name: "inner", caps: decision.Capabilities{Declared: true, Kinds: []decision.Kind{decision.KindPredicate}, Images: true}, fn: decorDecline("inner", "m", "q2")}
	rec, err := decision.NewRecorder(inner, decision.RecorderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	text := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1")}}
	withImage := decision.Request{State: "s", Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2")}, Images: []decision.Image{testImage}}
	if _, err := rec.Decide(ctx, text); err != nil {
		t.Fatal(err)
	}
	want, err := rec.Decide(ctx, withImage)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(rec.Transcript())
	if err != nil {
		t.Fatal(err)
	}
	var tr decision.Transcript
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatal(err)
	}
	if first, _ := json.Marshal(tr.Entries[0]); strings.Contains(string(first), "Images") || strings.Contains(string(first), "Declined") {
		t.Errorf("a text-only entry without declines serializes new keys: %s", first)
	}
	if !reflect.DeepEqual(tr.Entries[1].Request.Images, withImage.Images) {
		t.Errorf("transcript images = %v, want %v", tr.Entries[1].Request.Images, withImage.Images)
	}
	if !tr.Capabilities.Images {
		t.Error("transcript capabilities lost Images")
	}

	replay, err := decision.NewReplay(tr, decision.ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := replay.Decide(ctx, withImage)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Declined, want.Declined) {
		t.Errorf("replayed Declined = %v, want %v", got.Declined, want.Declined)
	}
	if _, err := replay.Decide(ctx, decision.Request{State: "s", Questions: withImage.Questions}); err == nil {
		t.Error("the same questions without the image replayed the image entry")
	}
}

func TestMergedCapabilitiesImagesAnyPinningAll(t *testing.T) {
	stub := func(images, pinning bool) *decorStub {
		return &decorStub{name: "s", caps: decision.Capabilities{Declared: true, Kinds: []decision.Kind{decision.KindPredicate}, Images: images, ModelPinning: pinning}}
	}
	for _, c := range []struct {
		name                   string
		stages                 []decision.Decider
		wantImages, wantPinned bool
	}{
		{"one stage takes images", []decision.Decider{stub(false, true), stub(true, true)}, true, true},
		{"one stage moves its model", []decision.Decider{stub(true, true), stub(true, false)}, true, false},
		{"neither", []decision.Decider{stub(false, false), stub(false, false)}, false, false},
		{"an abstaining None does not unpin", []decision.Decider{decision.None{}, stub(false, true)}, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			cascade, _ := decision.NewCascade(decision.CascadeOptions{Stages: c.stages})
			got := decision.CapabilitiesOf(cascade)
			if got.Images != c.wantImages || got.ModelPinning != c.wantPinned {
				t.Errorf("Images, ModelPinning = %v, %v, want %v, %v", got.Images, got.ModelPinning, c.wantImages, c.wantPinned)
			}
		})
	}
	if raw, _ := json.Marshal(decision.Capabilities{Declared: true}); strings.Contains(string(raw), "Images") || strings.Contains(string(raw), "ModelPinning") {
		t.Errorf("a Capabilities without images or pinning serializes the new keys: %s", raw)
	}
}
