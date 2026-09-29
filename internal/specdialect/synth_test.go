package specdialect

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// roundTrip parses Synthesize(p) back and zeroes what synthesis does not
// carry (Line/EndLine: the synthesized file's lines are not the source's).
func roundTrip(p verify.Plan) verify.Plan {
	back := verify.ParseMarkdownPlan(p.Service, Synthesize(p))
	for i := range back.Tasks {
		back.Tasks[i].Line, back.Tasks[i].EndLine = 0, 0
	}
	return back
}

func stripLines(p verify.Plan) verify.Plan {
	out := p
	out.Tasks = append([]verify.Task(nil), p.Tasks...)
	for i := range out.Tasks {
		out.Tasks[i].Line, out.Tasks[i].EndLine = 0, 0
	}
	out.Baseline = ""
	return out
}

func TestSynthesizeRoundTrips(t *testing.T) {
	for _, fx := range []struct {
		d         SpecDialect
		root, rel string
	}{
		{SpecKit(), "testdata/speckit", "specs/001-payments/tasks.md"},
		{OpenSpec(), "testdata/openspec", "openspec/changes/add-retry/tasks.md"},
	} {
		p, _, _, err := ParseFile(fx.d, fx.root, fx.rel)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := roundTrip(p), stripLines(p); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: round trip differs\n got %+v\nwant %+v", fx.rel, got.Tasks, want.Tasks)
		}
	}
	plans, _ := filepath.Glob(filepath.Join("..", "..", "docs", "plans", "*.md"))
	for _, pf := range plans {
		b, _ := os.ReadFile(pf)
		p := verify.ParseMarkdownPlan("", b)
		got, want := roundTrip(p), stripLines(p)
		for i := range want.Tasks {
			// A void (mangled) annotation is not a contract to carry.
			want.Tasks[i].AnnotationSuspect = ""
		}
		if !reflect.DeepEqual(got, want) {
			for i := range want.Tasks {
				if i < len(got.Tasks) && !reflect.DeepEqual(got.Tasks[i], want.Tasks[i]) {
					t.Errorf("%s: task %s round trip differs: %s", pf, want.Tasks[i].ID, fieldDiff(got.Tasks[i], want.Tasks[i]))
					break
				}
			}
		}
	}
}

// fieldDiff names the Task fields that differ.
func fieldDiff(a, b verify.Task) string {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	out := ""
	for i := 0; i < va.NumField(); i++ {
		if !reflect.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			out += fmt.Sprintf("\n  %s: got %#v want %#v", va.Type().Field(i).Name, va.Field(i).Interface(), vb.Field(i).Interface())
		}
	}
	return out
}
