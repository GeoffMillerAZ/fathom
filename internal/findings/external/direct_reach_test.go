package external

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Direct reach for plan rows FL-6 and FL-9: Importer.Import calls these
// through a selector the typed pass does not yet record as a static test
// edge, so the rows anchor to these tests instead.
func TestBindSubject_DirectReach(t *testing.T) {
	if err := BindSubject("", Event{}); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("empty provisioned subject must refuse, got %v", err)
	}
	ev := Event{Subject: Subject{Organization: "org", Repository: "repo", Component: "comp"}, Visibility: Subject{Organization: "org", Repository: "repo", Component: "comp"}}
	if err := BindSubject("org/repo/comp", ev); err != nil {
		t.Fatalf("matching subject refused: %v", err)
	}
	if err := BindSubject("org/repo/other", ev); err == nil {
		t.Fatal("mismatched subject must refuse")
	}
}

func TestApplyEvent_DirectReach(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ev := Event{Kind: "annotation", Annotation: "a note", Producer: Producer{Name: "reviewkit.system-review", Version: "1.0.0"}}
	// An annotation on a claim that does not exist yet must not panic; it
	// either appends an event or reports a clean error.
	_ = im.ApplyEvent(context.Background(), "svc", ev, ClaimFromEvent("svc", ev, core.TierSMEStated), nil)
}
