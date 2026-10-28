package extractor

// obsdialect_facts_test.go — the timers and annotations the prometheus and
// alertmanager doc specs declare as facts (WAE-6): a rule's `for`,
// `keep_firing_for` and annotations, a route's `group_wait`,
// `group_interval` and `repeat_interval`, each at its key's line, beside
// the facts the alert-route passes read. The goldens carry nodes only, so
// DocResult.Facts is asserted here, by hand from the fixtures.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestObsDialectFactsTimersAndAnnotations (WAE-6).
func TestObsDialectFactsTimersAndAnnotations(t *testing.T) {
	for _, tc := range []struct {
		dialect, fixture string
		want             []string
	}{
		{"prometheus", "checkout.rules.yml", []string{
			"20 CheckoutErrorBudgetBurn#for 10m",
			"22 CheckoutErrorBudgetBurn#labels.severity critical",
			"24 CheckoutErrorBudgetBurn#annotations.summary {{ $labels.job }} burns its error budget",
			"27 CartDown#for 5m",
			"34 CheckoutErrorBudgetBurn#for 1h",
			"36 CheckoutErrorBudgetBurn#labels.severity warning",
			"37 CheckoutErrorBudgetBurn#keep_firing_for 10m",
		}},
		{"alertmanager", "alertmanager.yml", []string{
			"13 route#receiver team-default",
			"15 route#group_wait 30s",
			"18 route.routes[0]#receiver null",
			"20 route.routes[1]#receiver pager",
			"21 route.routes[1]#continue true",
			"24 route.routes[2]#receiver team-db",
			"28 route.routes[2].routes[0]#receiver team-db-pg",
		}},
		{"alertmanager", "am-legacy.yml", []string{
			"4 route#receiver default",
			"9 route.routes[0]#receiver api-pager",
			"12 route.routes[1]#receiver foobar",
			"15 route.routes[1].routes[0]#receiver team-x",
			"16 route.routes[1].routes[0]#group_interval 5m",
			"17 route.routes[1].routes[0]#repeat_interval 4h",
		}},
	} {
		x, err := NewDocDialectExtractor(tc.dialect)
		if err != nil {
			t.Fatalf("NewDocDialectExtractor(%s): %v", tc.dialect, err)
		}
		src, err := os.ReadFile(filepath.Join("dialects", tc.dialect, "testdata", tc.fixture))
		if err != nil {
			t.Fatal(err)
		}
		res, err := x.ExtractDoc("svc", "monitoring/"+tc.fixture, src)
		if err != nil {
			t.Fatalf("%s: ExtractDoc: %v", tc.fixture, err)
		}
		var got []string
		for _, f := range res.Facts {
			got = append(got, fmt.Sprintf("%d %s#%s %s", f.Line, f.Entity, f.Key, f.Value))
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s facts\n got %q\nwant %q", tc.fixture, got, tc.want)
		}
	}
}
