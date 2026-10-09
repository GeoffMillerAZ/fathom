package fusion_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// TestResolveFileEdges_CppMemberCallKeepsReceiver (O523): the cpp dialect
// writes a member call with its receiver (`d.day`, `p->count` as `p.count`),
// so the include-path resolver never binds it to the header's top-level class
// or function of the same last name; the bare `count(1)` still binds the
// header's count. A member call on a receiver named like a header's class
// (`buffer.resize()`, cal::buffer; `widget.resize()`, a top-level class)
// binds none of that class's members, through any matcher: only a dotted
// call the file writes with the scope operator does (LB-F294).
func TestResolveFileEdges_CppMemberCallKeepsReceiver(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cppmember"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "cpp", []string{"calendar.hpp", "main.cpp"})
	for _, s := range symbols {
		if s.Name == "use" {
			want := map[string]bool{"d.day": true, "p.count": true}
			for _, c := range s.Calls {
				if !want[c] {
					t.Errorf("use calls %q, want only d.day and p.count (%v)", c, s.Calls)
				}
				delete(want, c)
			}
			if len(want) > 0 {
				t.Errorf("use calls %v, missing %v", s.Calls, want)
			}
		}
	}
	edges, _ := fusion.ResolveFileEdges(context.Background(), "cpp", dir, files, symbols)
	got := fileResolveEdgeSet(edges)
	want := map[string]string{
		fkey("main", "total"): fkey("calendar", "count"),
		fkey("main", "clear"): fkey("calendar", "cal.level"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
	for from, to := range want {
		if got[from] != to {
			t.Errorf("edge from %s: got %s, want %s", from, got[from], to)
		}
	}
}
