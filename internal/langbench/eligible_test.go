package langbench

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestQueueEligibility (§13.0 step 2, owner decision D6): a queue item is
// eligible when every depends_on row is [x] or [-] — or, for a dependency
// on another queue item (cpp on c, scss/less on css), when that item has a
// committed reading at L1 or better, even while its own row is open.
func TestQueueEligibility(t *testing.T) {
	q, err := ParseQueue([]byte(`[
	 {"id":"c","plan_row":"LB-Q-c","kind":"language","target_level":"L4","engine":"tags","depends_on":["LB-F1"]},
	 {"id":"cpp","plan_row":"LB-Q-cpp","kind":"language","target_level":"L4","engine":"tags","depends_on":["LB-F1","LB-Q-c"]},
	 {"id":"css","plan_row":"LB-Q-css","kind":"language","target_level":"L3","engine":"tags","depends_on":["LB-F1"]},
	 {"id":"scss","plan_row":"LB-Q-scss","kind":"language","target_level":"L3","engine":"tags","depends_on":["LB-Q-css"]},
	 {"id":"less","plan_row":"LB-Q-less","kind":"language","target_level":"L3","engine":"tags","depends_on":["LB-Q-css","LB-F2"]},
	 {"id":"done","plan_row":"LB-Q-done","kind":"language","target_level":"L1","engine":"tags","depends_on":["LB-F1"]}]`))
	if err != nil {
		t.Fatal(err)
	}
	plan := verify.ParseMarkdownPlan("s", []byte(`- [x] F1 {id: LB-F1, symbol: X, package: p}
- [ ] F2 {id: LB-F2, symbol: Y, package: p}
- [ ] c {id: LB-Q-c, oracle: lexical, path: x, pattern: y}
- [ ] cpp {id: LB-Q-cpp, oracle: lexical, path: x, pattern: y}
- [ ] css {id: LB-Q-css, oracle: lexical, path: x, pattern: y}
- [ ] scss {id: LB-Q-scss, oracle: lexical, path: x, pattern: y}
- [ ] less {id: LB-Q-less, oracle: lexical, path: x, pattern: y}
- [x] done {id: LB-Q-done, oracle: lexical, path: x, pattern: y}
`))
	levels := map[string]string{"c": "L1", "css": LevelNone}
	got := blockedOf(QueueEligibility(q, plan, func(id string) (string, bool) { l, ok := levels[id]; return l, ok }))
	want := map[string][]string{
		"c":    nil,
		"done": nil,
		"cpp":  nil,                   // c's row is open, but its reading is at L1 (D6)
		"css":  nil,                   // no dependency on a queue item
		"scss": {"LB-Q-css"},          // css has a reading, but below L1
		"less": {"LB-F2", "LB-Q-css"}, // a foundation row is never satisfied by a reading
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("QueueEligibility =\n%v\nwant\n%v", got, want)
	}
	delete(levels, "c") // no committed reading at all: blocked on the open row
	if got := blockedOf(QueueEligibility(q, plan, func(id string) (string, bool) { l, ok := levels[id]; return l, ok })); !reflect.DeepEqual(got["cpp"], []string{"LB-Q-c"}) {
		t.Fatalf("cpp without c's reading: %v", got["cpp"])
	}
}

func blockedOf(m map[string]QueueBlock) map[string][]string {
	out := map[string][]string{}
	for id, b := range m {
		out[id] = b.Blocked
	}
	return out
}

// TestQueueEligibilityDeferredIsNotDone is G2-6 in the language queue (§13.0
// step 2, amended 2026-09-29): a [-] depends_on row no longer satisfies it. A
// deferred one (or one with no disposition) keeps the item blocked until it
// revives; a refused one will not be built, so the item is reported as needing
// re-planning instead of waiting. D6 still holds: a queue item whose own row is
// open or deferred satisfies its dependents with a committed reading at L1 or
// better, but a refused one never does.
func TestQueueEligibilityDeferredIsNotDone(t *testing.T) {
	q, err := ParseQueue([]byte(`[
	 {"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L2","engine":"tags","depends_on":["LB-F1","LB-F3"]},
	 {"id":"b","plan_row":"LB-Q-b","kind":"language","target_level":"L2","engine":"tags","depends_on":["LB-F1","LB-F4"]},
	 {"id":"c","plan_row":"LB-Q-c","kind":"language","target_level":"L2","engine":"tags","depends_on":["LB-F5"]},
	 {"id":"p","plan_row":"LB-Q-p","kind":"language","target_level":"L4","engine":"tags","depends_on":["LB-F1"]},
	 {"id":"r","plan_row":"LB-Q-r","kind":"language","target_level":"L4","engine":"tags","depends_on":["LB-F1"]},
	 {"id":"d","plan_row":"LB-Q-d","kind":"language","target_level":"L2","engine":"tags","depends_on":["LB-Q-p"]},
	 {"id":"e","plan_row":"LB-Q-e","kind":"language","target_level":"L2","engine":"tags","depends_on":["LB-Q-r","LB-F3"]}]`))
	if err != nil {
		t.Fatal(err)
	}
	plan := verify.ParseMarkdownPlan("s", []byte(`- [x] F1 {id: LB-F1, symbol: X, package: p}
- [-] F3 {id: LB-F3, disposition: deferred, reason: "later", trigger: "t"}
- [-] F4 {id: LB-F4, disposition: refused, reason: "never"}
- [-] F5 {id: LB-F5}
- [ ] a {id: LB-Q-a, oracle: lexical, path: x, pattern: y}
- [ ] b {id: LB-Q-b, oracle: lexical, path: x, pattern: y}
- [ ] c {id: LB-Q-c, oracle: lexical, path: x, pattern: y}
- [-] p {id: LB-Q-p, disposition: deferred, reason: "partial", trigger: "t"}
- [-] r {id: LB-Q-r, disposition: refused, reason: "licence"}
- [ ] d {id: LB-Q-d, oracle: lexical, path: x, pattern: y}
- [ ] e {id: LB-Q-e, oracle: lexical, path: x, pattern: y}
`))
	levels := map[string]string{"p": "L1", "r": "L2"}
	got := QueueEligibility(q, plan, func(id string) (string, bool) { l, ok := levels[id]; return l, ok })
	want := map[string]QueueBlock{
		"a": {Blocked: []string{"LB-F3"}},                             // deferred is not done
		"b": {Replan: []string{"LB-F4"}},                              // refused: re-plan, do not wait
		"c": {Blocked: []string{"LB-F5"}},                             // [-] with no disposition: still unmet
		"p": {},                                                       // its own row is not its dependency
		"r": {},                                                       //
		"d": {},                                                       // D6: p's reading is at L1
		"e": {Blocked: []string{"LB-F3"}, Replan: []string{"LB-Q-r"}}, // a refused item's reading satisfies nothing
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("QueueEligibility =\n%+v\nwant\n%+v", got, want)
	}
	for id, b := range want {
		if b.Eligible() != (len(b.Blocked) == 0 && len(b.Replan) == 0) {
			t.Errorf("%s: Eligible() = %v", id, b.Eligible())
		}
	}
}
