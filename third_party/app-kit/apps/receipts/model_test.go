package receipts

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

type expectedData struct {
	Freshness struct {
		FreshAgeSeconds    int64 `json:"fresh_age_seconds"`
		BoundaryAgeSeconds int64 `json:"boundary_age_seconds"`
		StaleAgeSeconds    int64 `json:"stale_age_seconds"`
	} `json:"freshness"`
	Facts     map[string][]fact `json:"facts"`
	September struct {
		All    expectedAggregate `json:"all"`
		App    expectedAggregate `json:"app"`
		Mobile expectedAggregate `json:"mobile"`
		Rows   []struct {
			ID                       string `json:"id"`
			Current, Previous, Delta int64
		} `json:"rows"`
	} `json:"september"`
	October  expectedIncomplete `json:"october"`
	November expectedIncomplete `json:"november"`
}

type expectedAggregate struct {
	Current        int64  `json:"current"`
	Previous       int64  `json:"previous"`
	Delta          int64  `json:"delta"`
	LargestDecline string `json:"largest_decline"`
}

type expectedIncomplete struct {
	KnownSubtotal int64  `json:"known_subtotal"`
	Known         int    `json:"known"`
	Selected      int    `json:"selected"`
	ExactTotal    *int64 `json:"exact_total"`
	Delta         *int64 `json:"delta"`
}

func loadCUE(t *testing.T) (fixtureData, expectedData) {
	t.Helper()
	body, err := os.ReadFile("fixture.cue")
	if err != nil {
		t.Fatal(err)
	}
	root := cuecontext.New().CompileBytes(body, cue.Filename("fixture.cue"))
	if err := root.Validate(); err != nil {
		t.Fatalf("fixture CUE is invalid: %v", err)
	}
	var input fixtureData
	if err := root.LookupPath(cue.ParsePath("fixture")).Decode(&input); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	var expected expectedData
	if err := root.LookupPath(cue.ParsePath("expected")).Decode(&expected); err != nil {
		t.Fatalf("decode independent expectations: %v", err)
	}
	return input, expected
}

func TestAll72SelectionsValidateWithoutChangingTruthAxes(t *testing.T) {
	input, expected := loadCUE(t)
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, period := range []string{"2024-09", "2024-10", "2024-11"} {
		for _, channel := range []string{"all", "app", "mobile"} {
			for _, compare := range []string{"none", "previous"} {
				for _, stale := range []bool{false, true} {
					for _, withDetail := range []bool{false, true} {
						detailID := ""
						if withDetail {
							detailID = map[string]string{"all": "atlas", "app": "atlas", "mobile": "beacon"}[channel]
						}
						selected := selection{Period: period, Channel: channel, Compare: compare, Stale: stale, DetailID: detailID}
						got, err := project(input, selected)
						if err != nil {
							t.Fatalf("project %+v: %v", selected, err)
						}
						count++
						if err := got.Current.Validate(got.Now); err != nil {
							t.Fatalf("current %+v: %v", selected, err)
						}
						wantCount := map[string]int{"all": 3, "app": 2, "mobile": 1}[channel]
						if withDetail {
							wantCount = 1
						}
						if got.Summary.SelectedCount != wantCount || len(got.Facts) != wantCount || len(got.Current.Rows) != wantCount {
							t.Fatalf("selection %+v did not stay filtered: summary=%+v facts=%d rows=%d", selected, got.Summary, len(got.Facts), len(got.Current.Rows))
						}
						if got.Current.Contract.Source != vizframe.Derived(projectionSourceText) {
							t.Fatalf("selection %+v changed source: %+v", selected, got.Current.Contract.Source)
						}
						wantAge := expected.Freshness.FreshAgeSeconds
						if stale {
							wantAge = expected.Freshness.StaleAgeSeconds
						}
						if age := int64(got.Now.Sub(got.Current.Contract.Freshness.ObservedAt).Seconds()); age != wantAge {
							t.Fatalf("selection %+v source age=%d, want %d", selected, age, wantAge)
						}
						if compare == "none" {
							if got.Previous != nil || len(got.PreviousFacts) != 0 || got.Summary.PreviousExactTotalCents != nil || got.Summary.DeltaCents != nil || len(got.Summary.LargestDeclineIDs) != 0 {
								t.Fatalf("comparison-none leaked comparison data or claims: %+v", got)
							}
						} else if got.Previous == nil || len(got.PreviousFacts) != wantCount || got.Previous.Validate(got.Now) != nil {
							t.Fatalf("comparison previous missing or invalid for %+v", selected)
						}
						if withDetail != (got.Detail != nil) {
							t.Fatalf("detail presence mismatch for %+v", selected)
						}
					}
				}
			}
		}
	}
	if count != 72 {
		t.Fatalf("exercised %d combinations, want 72", count)
	}
	after, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("projection mutated the first-party source fixture")
	}
	canary, _ := loadCUE(t)
	canaryBefore, _ := json.Marshal(canary)
	*canary.Snapshots[0].Facts[0].ReceiptsCents++
	canaryAfter, _ := json.Marshal(canary)
	if bytes.Equal(canaryAfter, canaryBefore) {
		t.Fatal("source-mutation guard cannot see a changed amount pointee")
	}
}

func TestIndependentFactsAndSeptemberLiteralOracle(t *testing.T) {
	input, expected := loadCUE(t)
	periods := []string{"2024-09", "2024-10", "2024-11"}
	for _, period := range periods {
		got, err := project(input, selection{Period: period, Channel: "all", Compare: "previous"})
		if err != nil {
			t.Fatal(err)
		}
		if err := factsAgree(got.Facts, expected.Facts[period]); err != nil {
			t.Fatalf("%s current facts: %v", period, err)
		}
		previousPeriod := map[string]string{"2024-09": "2024-08", "2024-10": "2024-09", "2024-11": "2024-10"}[period]
		if err := factsAgree(got.PreviousFacts, expected.Facts[previousPeriod]); err != nil {
			t.Fatalf("%s previous facts: %v", period, err)
		}
	}

	for channel, want := range map[string]expectedAggregate{
		"all": expected.September.All, "app": expected.September.App, "mobile": expected.September.Mobile,
	} {
		got, err := project(input, selection{Period: "2024-09", Channel: channel, Compare: "previous"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Summary.ExactTotalCents == nil || *got.Summary.ExactTotalCents != want.Current ||
			got.Summary.PreviousExactTotalCents == nil || *got.Summary.PreviousExactTotalCents != want.Previous ||
			got.Summary.DeltaCents == nil || *got.Summary.DeltaCents != want.Delta {
			t.Fatalf("%s summary=%+v, want %+v", channel, got.Summary, want)
		}
		wantIDs := []string(nil)
		if want.LargestDecline != "" {
			wantIDs = []string{want.LargestDecline}
		}
		if !reflect.DeepEqual(got.Summary.LargestDeclineIDs, wantIDs) {
			t.Fatalf("%s largest declines=%v, want %v", channel, got.Summary.LargestDeclineIDs, wantIDs)
		}
	}

	got, err := project(input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range expected.September.Rows {
		current := factByID(got.Facts, want.ID)
		previous := factByID(got.PreviousFacts, want.ID)
		if current.NetCents == nil || previous.NetCents == nil || *current.NetCents != want.Current || *previous.NetCents != want.Previous || *current.NetCents-*previous.NetCents != want.Delta {
			t.Fatalf("row %s current=%v previous=%v, want %+v", want.ID, current.NetCents, previous.NetCents, want)
		}
	}
}

func TestMissingZeroPartialAndComparisonClaims(t *testing.T) {
	input, expected := loadCUE(t)
	october, err := project(input, selection{Period: "2024-10", Channel: "all", Compare: "previous"})
	if err != nil {
		t.Fatal(err)
	}
	assertIncomplete(t, october, expected.October)
	if atlas := factByID(october.Facts, "atlas"); atlas.NetCents == nil || *atlas.NetCents != 0 {
		t.Fatalf("October measured zero was lost: %+v", atlas)
	}
	for _, id := range []string{"beacon", "cedar"} {
		if item := factByID(october.Facts, id); item.NetCents != nil || item.MissingReason == "" {
			t.Fatalf("October missing %s became zero or lost reason: %+v", id, item)
		}
	}
	if october.Summary.PreviousExactTotalCents == nil || *october.Summary.PreviousExactTotalCents != expected.September.All.Current {
		t.Fatalf("known prior exact total was lost: %+v", october.Summary)
	}

	november, err := project(input, selection{Period: "2024-11", Channel: "all", Compare: "previous"})
	if err != nil {
		t.Fatal(err)
	}
	assertIncomplete(t, november, expected.November)
	if november.Current.Contract.Completeness != analytics.Partial || november.Current.Contract.Window.End.Format(time.RFC3339) != "2024-11-21T00:00:00Z" {
		t.Fatalf("November did not retain measured-to-date coverage: %+v", november.Current.Contract)
	}
	if atlas := factByID(november.Facts, "atlas"); atlas.NetCents == nil || *atlas.NetCents != -10000 {
		t.Fatalf("signed known subtotal dropped negative Atlas: %+v", atlas)
	}
	if november.Summary.ExactTotalCents != nil || november.Summary.DeltaCents != nil {
		t.Fatalf("partial subtotal became an exact or comparison claim: %+v", november.Summary)
	}

	withoutComparison, err := project(input, selection{Period: "2024-09", Channel: "all", Compare: "none"})
	if err != nil || withoutComparison.Summary.ExactTotalCents == nil || *withoutComparison.Summary.ExactTotalCents != expected.September.All.Current {
		t.Fatalf("comparison-none suppressed a valid current exact total: projection=%+v err=%v", withoutComparison, err)
	}
}

func TestDetailPreservesFilteredFactsAndContext(t *testing.T) {
	input, _ := loadCUE(t)
	for _, period := range []string{"2024-09", "2024-10", "2024-11"} {
		for channel, ids := range map[string][]string{"all": {"atlas", "beacon", "cedar"}, "app": {"atlas", "cedar"}, "mobile": {"beacon"}} {
			overview := mustProject(t, input, selection{Period: period, Channel: channel, Compare: "previous", Stale: true})
			for _, id := range ids {
				selected := selection{Period: period, Channel: channel, Compare: "previous", Stale: true, DetailID: id}
				got, err := project(input, selected)
				if err != nil {
					t.Fatalf("%+v: %v", selected, err)
				}
				if got.Detail == nil || got.Detail.Selection != selected || got.Detail.Product.ID != id || !reflect.DeepEqual(got.Detail.Current, factByID(got.Facts, id)) || !reflect.DeepEqual(*got.Detail.Previous, factByID(got.PreviousFacts, id)) {
					t.Fatalf("detail facts diverged for %+v: %+v", selected, got.Detail)
				}
				if !reflect.DeepEqual(got.Detail.CurrentContract, got.Current.Contract) || !reflect.DeepEqual(*got.Detail.PreviousContract, got.Previous.Contract) {
					t.Fatalf("detail lost window/source/freshness context for %+v", selected)
				}
				if !reflect.DeepEqual(got.Detail.Current, factByID(overview.Facts, id)) || !reflect.DeepEqual(*got.Detail.Previous, factByID(overview.PreviousFacts, id)) {
					t.Fatalf("detail changed the overview's receipt/refund evidence for %+v", selected)
				}
				for _, pair := range []struct{ detail, overview analytics.Dataset }{{got.Current, overview.Current}, {*got.Previous, *overview.Previous}} {
					detailContext, rows := datasetCSV(t, pair.detail, got.Now)
					overviewContext, overviewRows := datasetCSV(t, pair.overview, overview.Now)
					if len(rows) != 1 || rows[id]["id"] != id || !reflect.DeepEqual(rows[id], overviewRows[id]) {
						t.Fatalf("detail export differs from the selected overview row for %+v: %v versus %v", selected, rows, overviewRows[id])
					}
					// Coverage completeness may improve when an unknown product is
					// filtered out; source age, actual window and units cannot change.
					for _, key := range []string{"unit", "window_kind", "window", "source_kind", "source", "evaluated_at", "freshness", "decimal_places"} {
						if detailContext[key] != overviewContext[key] {
							t.Fatalf("detail changed %s for %+v: %q versus %q", key, selected, detailContext[key], overviewContext[key])
						}
					}
				}
			}
		}
	}
	for _, selected := range []selection{
		{Period: "2024-09", Channel: "all", Compare: "none", DetailID: "unknown"},
		{Period: "2024-09", Channel: "mobile", Compare: "none", DetailID: "atlas"},
		{Period: "2024-09", Channel: "app", Compare: "none", DetailID: "beacon"},
	} {
		if _, err := project(input, selected); err == nil {
			t.Fatalf("accepted unknown or filtered detail %+v", selected)
		}
	}
}

func TestCSVRetainsSignedZeroMissingPeriodAndFreshness(t *testing.T) {
	input, expected := loadCUE(t)
	september := mustProject(t, input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	context, rows := datasetCSV(t, september.Current, september.Now)
	if context["unit"] != "USD" || context["decimal_places"] != "2" || !strings.Contains(context["window"], "2024-09-01T00:00:00Z to 2024-10-01T00:00:00Z") || rows["cedar"]["value"] != "-150" {
		t.Fatalf("September CSV lost units/window/negative: context=%v rows=%v", context, rows)
	}
	if !strings.HasPrefix(context["freshness"], "Fresh:") || int64(september.Now.Sub(september.Current.Contract.Freshness.ObservedAt).Seconds()) != expected.Freshness.FreshAgeSeconds {
		t.Fatalf("fresh source observation was mislabeled: %v", context)
	}

	october := mustProject(t, input, selection{Period: "2024-10", Channel: "all", Compare: "none"})
	_, rows = datasetCSV(t, october.Current, october.Now)
	if rows["atlas"]["value"] != "0" || rows["atlas"]["state"] != "present" || rows["beacon"]["value"] != "" || rows["beacon"]["state"] != "missing" || rows["cedar"]["missing_reason"] == "" {
		t.Fatalf("October CSV confused zero and missing: %v", rows)
	}

	november := mustProject(t, input, selection{Period: "2024-11", Channel: "all", Compare: "none", Stale: true})
	context, rows = datasetCSV(t, november.Current, november.Now)
	if !strings.Contains(context["window"], "2024-11-21T00:00:00Z") || context["completeness"] != "partial" || !strings.HasPrefix(context["freshness"], "Stale:") || rows["atlas"]["value"] != "-100" {
		t.Fatalf("November CSV lost partial period, staleness or sign: context=%v rows=%v", context, rows)
	}

	now, _ := time.Parse(time.RFC3339, input.Now)
	for _, test := range []struct {
		name, observed, prefix string
	}{
		{"boundary", now.Add(-time.Duration(expected.Freshness.BoundaryAgeSeconds) * time.Second).Format(time.RFC3339), "Fresh:"},
		{"just after", now.Add(-time.Duration(expected.Freshness.BoundaryAgeSeconds)*time.Second - time.Second).Format(time.RFC3339), "Stale:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := input
			changed.StaleObservedAt = test.observed
			got := mustProject(t, changed, selection{Period: "2024-09", Channel: "all", Compare: "none", Stale: true})
			context, _ := datasetCSV(t, got.Current, got.Now)
			if !strings.HasPrefix(context["freshness"], test.prefix) {
				t.Fatalf("freshness=%q, want prefix %q", context["freshness"], test.prefix)
			}
		})
	}
}

func TestSignedDatasetPreservesRenderBarsRefusal(t *testing.T) {
	input, _ := loadCUE(t)
	got := mustProject(t, input, selection{Period: "2024-09", Channel: "all", Compare: "none"})
	if _, err := analytics.RenderBars(got.Current, viz.BarsOptions{}, got.Now); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("signed source series was silently rendered or changed: %v", err)
	}
}

func TestOneCentSignedValuesReachCSVWithoutPrematureRounding(t *testing.T) {
	input, _ := loadCUE(t)
	september := snapshotByPeriod(&input, "2024-09")
	atlas := sourceFactByID(september, "atlas")
	cedar := sourceFactByID(september, "cedar")
	*atlas.ReceiptsCents, *atlas.RefundsCents = 1, 0
	*cedar.ReceiptsCents, *cedar.RefundsCents = 0, 1
	got := mustProject(t, input, selection{Period: "2024-09", Channel: "all", Compare: "none"})
	if *factByID(got.Facts, "atlas").NetCents != 1 || *factByID(got.Facts, "cedar").NetCents != -1 {
		t.Fatalf("integer-cent projection changed signed cents: %+v", got.Facts)
	}
	_, rows := datasetCSV(t, got.Current, got.Now)
	if rows["atlas"]["value"] != "0.01" || rows["cedar"]["value"] != "-0.01" {
		t.Fatalf("CSV did not retain one-cent signs: %v", rows)
	}
}

func TestLargestDeclineTiesAndNoNegativeContribution(t *testing.T) {
	input, _ := loadCUE(t)
	september := snapshotByPeriod(&input, "2024-09")
	cedar := sourceFactByID(september, "cedar")
	*cedar.ReceiptsCents, *cedar.RefundsCents = 0, 60000
	input.Products[0], input.Products[2] = input.Products[2], input.Products[0]
	got := mustProject(t, input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	if want := []string{"cedar", "atlas"}; !reflect.DeepEqual(got.Summary.LargestDeclineIDs, want) {
		t.Fatalf("tied declines=%v, want stable input order %v", got.Summary.LargestDeclineIDs, want)
	}

	input, _ = loadCUE(t)
	flat := mustProject(t, input, selection{Period: "2024-09", Channel: "mobile", Compare: "previous"})
	if len(flat.Summary.LargestDeclineIDs) != 0 {
		t.Fatalf("flat contribution was called a decline: %v", flat.Summary.LargestDeclineIDs)
	}
	september = snapshotByPeriod(&input, "2024-09")
	atlas := sourceFactByID(september, "atlas")
	cedar = sourceFactByID(september, "cedar")
	*atlas.ReceiptsCents, *atlas.RefundsCents = 160000, 10000
	*cedar.ReceiptsCents, *cedar.RefundsCents = 30000, 20000
	increasing := mustProject(t, input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	if len(increasing.Summary.LargestDeclineIDs) != 0 {
		t.Fatalf("flat/increasing products produced a negative-contribution claim: %v", increasing.Summary.LargestDeclineIDs)
	}
}

func TestValidationAndIndependentOracleCanaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*fixtureData)
	}{
		{"missing fact row", func(f *fixtureData) { f.Snapshots[0].Facts = f.Snapshots[0].Facts[:2] }},
		{"duplicate fact id", func(f *fixtureData) { f.Snapshots[0].Facts[1].Product = "atlas" }},
		{"unknown fact id", func(f *fixtureData) { f.Snapshots[0].Facts[0].Product = "rogue" }},
		{"duplicate product id", func(f *fixtureData) { f.Products[1].ID = "atlas" }},
		{"unknown product id", func(f *fixtureData) { f.Products[0].ID = "rogue" }},
		{"wrong fixed channel", func(f *fixtureData) { f.Products[0].Channel = "mobile" }},
		{"missing snapshot", func(f *fixtureData) { f.Snapshots = f.Snapshots[:3] }},
		{"duplicate snapshot", func(f *fixtureData) { f.Snapshots[1].Period = "2024-08" }},
		{"unknown period", func(f *fixtureData) { f.Snapshots[0].Period = "2024-07" }},
		{"missing source", func(f *fixtureData) { f.Snapshots[0].ObservedAt = "" }},
		{"missing stale source", func(f *fixtureData) { f.StaleObservedAt = "" }},
		{"missing coverage", func(f *fixtureData) { f.Snapshots[0].CoverageEnd = "" }},
		{"coverage outside period", func(f *fixtureData) { f.Snapshots[0].CoverageEnd = "2024-10-01T00:00:00Z" }},
		{"negative input", func(f *fixtureData) { *f.Snapshots[0].Facts[0].ReceiptsCents = -1 }},
		{"oversized input", func(f *fixtureData) { *f.Snapshots[0].Facts[0].RefundsCents = maxAmountCents + 1 }},
		{"missing reason", func(f *fixtureData) { f.Snapshots[2].Facts[1].MissingReason = "" }},
		{"reason on known input", func(f *fixtureData) { f.Snapshots[1].Facts[0].MissingReason = "not really missing" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, _ := loadCUE(t)
			test.change(&input)
			if _, err := project(input, selection{Period: "2024-09", Channel: "all", Compare: "previous"}); err == nil {
				t.Fatal("mutated fixture was accepted")
			}
		})
	}

	for _, test := range []struct {
		name   string
		change func(*fact)
	}{
		{"changed net", func(f *fact) { *f.ReceiptsCents++ }},
		{"unchanged net but changed receipts and refunds", func(f *fact) { *f.ReceiptsCents += 100; *f.RefundsCents += 100 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, expected := loadCUE(t)
			test.change(sourceFactByID(snapshotByPeriod(&input, "2024-09"), "atlas"))
			got := mustProject(t, input, selection{Period: "2024-09", Channel: "all", Compare: "none"})
			if err := factsAgree(got.Facts, expected.Facts["2024-09"]); err == nil {
				t.Fatal("independent receipt/refund oracle did not detect the mutation")
			}
		})
	}

	input, _ := loadCUE(t)
	for _, selected := range []selection{
		{Period: "2024-08", Channel: "all", Compare: "none"},
		{Period: "2024-09", Channel: "desktop", Compare: "none"},
		{Period: "2024-09", Channel: "all", Compare: "annual"},
	} {
		if _, err := project(input, selected); err == nil {
			t.Fatalf("invalid selection was accepted: %+v", selected)
		}
	}
}

func assertIncomplete(t *testing.T, got projection, want expectedIncomplete) {
	t.Helper()
	if got.Summary.KnownSubtotalCents != want.KnownSubtotal || got.Summary.KnownCount != want.Known || got.Summary.SelectedCount != want.Selected ||
		!reflect.DeepEqual(got.Summary.ExactTotalCents, want.ExactTotal) || !reflect.DeepEqual(got.Summary.DeltaCents, want.Delta) {
		t.Fatalf("summary=%+v, want %+v", got.Summary, want)
	}
}

func factsAgree(got []projectedFact, want []fact) error {
	if len(got) != len(want) {
		return fmt.Errorf("fact count %d, want %d", len(got), len(want))
	}
	for index := range got {
		if got[index].Product.ID != want[index].Product || !reflect.DeepEqual(got[index].ReceiptsCents, want[index].ReceiptsCents) ||
			!reflect.DeepEqual(got[index].RefundsCents, want[index].RefundsCents) || got[index].MissingReason != want[index].MissingReason {
			return fmt.Errorf("fact %d=%+v, want %+v", index, got[index], want[index])
		}
	}
	return nil
}

func mustProject(t *testing.T, input fixtureData, selected selection) projection {
	t.Helper()
	got, err := project(input, selected)
	if err != nil {
		t.Fatalf("project %+v: %v", selected, err)
	}
	return got
}

func snapshotByPeriod(input *fixtureData, period string) *snapshot {
	for index := range input.Snapshots {
		if input.Snapshots[index].Period == period {
			return &input.Snapshots[index]
		}
	}
	panic("test snapshot not found")
}

func sourceFactByID(item *snapshot, id string) *fact {
	for index := range item.Facts {
		if item.Facts[index].Product == id {
			return &item.Facts[index]
		}
	}
	panic("test fact not found")
}

func datasetCSV(t *testing.T, dataset analytics.Dataset, now time.Time) (map[string]string, map[string]map[string]string) {
	t.Helper()
	var body bytes.Buffer
	if err := analytics.WriteCSV(&body, dataset, now); err != nil {
		t.Fatalf("write CSV: %v", err)
	}
	reader := csv.NewReader(&body)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	context := map[string]string{}
	wantHeader := []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}
	index := 0
	for ; index < len(records) && !reflect.DeepEqual(records[index], wantHeader); index++ {
		if len(records[index]) != 2 {
			t.Fatalf("CSV context record %d has %d fields, want 2: %v", index, len(records[index]), records[index])
		}
		context[records[index][0]] = records[index][1]
	}
	if index == len(records) {
		t.Fatalf("CSV has no exact data header %v: %v", wantHeader, records)
	}
	header := records[index]
	index++
	rows := map[string]map[string]string{}
	for ; index < len(records); index++ {
		if len(records[index]) != len(header) {
			t.Fatalf("CSV data record %d has %d fields, want %d: %v", index, len(records[index]), len(header), records[index])
		}
		row := map[string]string{}
		for column, name := range header {
			row[name] = records[index][column]
		}
		if _, duplicate := rows[row["id"]]; duplicate {
			t.Fatalf("CSV contains duplicate id %q", row["id"])
		}
		rows[row["id"]] = row
	}
	return context, rows
}
