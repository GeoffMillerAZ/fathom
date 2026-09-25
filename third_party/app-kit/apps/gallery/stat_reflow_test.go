package gallery

import (
	"strings"
	"testing"
)

func TestLayoutIncludesMetricReflowSpecimens(t *testing.T) {
	page, err := buildLayout()
	if err != nil {
		t.Fatal(err)
	}
	_, examples, ok := strings.Cut(string(page), `id="stat-reflow"`)
	if !ok {
		t.Fatal("missing inspectable stat reflow specimen")
	}
	examples, _, _ = strings.Cut(examples, "<h3")
	for text, count := range map[string]int{">0 B<": 2, ">≥ 7<": 2, ">—<": 2, ">123 ms<": 1, ">-12%<": 1} {
		if got := strings.Count(examples, text); got != count {
			t.Errorf("%s count = %d, want %d", text, got, count)
		}
	}
	if !strings.Contains(examples, "synthetic/observation/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") || !strings.Contains(examples, "<svg") {
		t.Fatal("long source and sparkline reflow coverage required")
	}
	_, dense, ok := strings.Cut(string(page), `id="stat-dense-reflow"`)
	if !ok {
		t.Fatal("missing dense-band divider specimen")
	}
	dense, _, _ = strings.Cut(dense, "</dl>")
	if strings.Count(dense, "<dt ") != 5 || !strings.Contains(dense, ">42 ms<") || !strings.Contains(dense, ">300 B<") {
		t.Fatal("five-cell partial-row divider coverage required")
	}
}
