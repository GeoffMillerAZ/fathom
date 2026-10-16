package hitl_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/hitl"
)

func TestCommitterRejectsTypedNilStore(t *testing.T) {
	t.Parallel()
	f := newCommitFixture(t)
	var store *hitl.MemoryCommitStore[commitState]
	if _, err := hitl.NewCommitter(f.engine, store, f.cfg); err == nil {
		t.Fatal("typed nil store accepted")
	}
}
