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
	got := QueueEligibility(q, plan, func(id string) (string, bool) { l, ok := levels[id]; return l, ok })
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
	if got := QueueEligibility(q, plan, func(id string) (string, bool) { l, ok := levels[id]; return l, ok }); !reflect.DeepEqual(got["cpp"], []string{"LB-Q-c"}) {
		t.Fatalf("cpp without c's reading: %v", got["cpp"])
	}
}
