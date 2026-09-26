package main

// unregistered_test.go — OC-H2 end to end: call-log rows naming a service
// the daemon's registry does not list (the test fixture "svc" rows that
// reached the operator's log) are left out of the catches timeline and of
// every per-service call figure, and named — on the page, in the verb, in
// the fleet report and in calls.total's denominator.

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// consoleFixtureSvcRows are review rows the fixture registry does not list
// ("svc", the polluting fixture service) beside one it does.
const consoleFixtureSvcRows = `{"ts":"2026-09-25T05:15:42Z","tool":"review","service":"svc","surface":"cli","verdict_outcome":{"conservation":1,"impact":0,"integrity":0,"obligations":0,"spec_coverage":0},"latency_ms":0}
{"ts":"2026-09-25T05:15:46Z","tool":"review","service":"svc","surface":"cli","verdict_outcome":{"conservation":1,"impact":0,"integrity":0,"obligations":0,"spec_coverage":0},"latency_ms":0}
{"ts":"2026-09-25T06:00:00Z","tool":"review","service":"payment-service","surface":"cli","verdict_outcome":{"conservation":1,"integrity":0},"latency_ms":0}
`

func appendFixtureCalls(t *testing.T, f *consoleFixture, rows string) {
	t.Helper()
	fh, err := os.OpenFile(f.callLog, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(rows); err != nil {
		t.Fatal(err)
	}
	fh.Close()
	resetObsCache()
	resetImpactReads()
}

func TestCatches_ExcludeServicesTheRegistryDoesNotList(t *testing.T) {
	f := newConsoleFixture(t)
	appendFixtureCalls(t, f, consoleFixtureSvcRows)
	want := console.Unregistered{Rows: 2, Services: []string{"svc"}}

	// The page.
	env := f.pageJSON(t, "/impact")
	var page struct {
		Catches console.CatchTimelineModel `json:"catches"`
	}
	if err := json.Unmarshal(env.Model, &page); err != nil {
		t.Fatal(err)
	}
	conserve := func(m console.CatchTimelineModel) console.Value[int] {
		for _, k := range m.Kinds {
			if k.Kind == console.CatchConserveLoss {
				return k.Count
			}
		}
		t.Fatal("no conserve lane")
		return console.Value[int]{}
	}
	if v := conserve(page.Catches); v.V != 1 || !strings.Contains(v.Denominator, "2 review rows naming a service the registry does not list (svc) excluded") {
		t.Errorf("/impact conserve lane = %+v, want payment-service's 1, naming svc's 2 rows", v)
	}
	if !reflect.DeepEqual(page.Catches.Unregistered, want) {
		t.Errorf("/impact unregistered = %+v, want %+v", page.Catches.Unregistered, want)
	}

	// The verb says the same.
	var out, errb bytes.Buffer
	if code := cmdMetricsCatches([]string{"--json"}, &out, &errb); code != 0 {
		t.Fatalf("metrics catches: %d %s", code, errb.String())
	}
	var verb console.CatchTimelineModel
	if err := json.Unmarshal(out.Bytes(), &verb); err != nil {
		t.Fatal(err)
	}
	if v := conserve(verb); v.V != 1 || !reflect.DeepEqual(verb.Unregistered, want) {
		t.Errorf("metrics catches conserve = %+v, unregistered %+v", v, verb.Unregistered)
	}

	// A filter naming the unregistered service shows nothing of it.
	if code, _, body := f.get(t, "/impact?svc=svc", "Accept", "application/json"); code != 404 || !strings.Contains(body, "registry does not list it") {
		t.Errorf("/impact?svc=svc = %d %s, want the unknown-project 404 saying its rows are excluded", code, body)
	}
	out.Reset()
	errb.Reset()
	if code := cmdMetricsCatches([]string{"--service", "svc"}, &out, &errb); code != 0 || !strings.Contains(errb.String(), "registry does not list \"svc\"") {
		t.Errorf("metrics catches --service svc: %d, stderr %q", code, errb.String())
	}

	// calls.total keeps every row and names the unregistered ones.
	fleet := f.pageJSON(t, "/fleet")
	var calls console.Value[int]
	if err := json.Unmarshal(fleet.Metrics[string(console.MetricCalls)], &calls); err != nil {
		t.Fatal(err)
	}
	if calls.V != f.callRows+3 || !strings.Contains(calls.Denominator, "2 rows naming a service the registry does not list (svc)") {
		t.Errorf("calls.total = %+v, want every row (%d) naming svc's 2", calls, f.callRows+3)
	}

	// The fleet report's per-service calls never count svc, and name it.
	rep, err := buildFleetReport(consoleRPCClient(5e9), f.ts.URL, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Unregistered, want) {
		t.Errorf("fleet report unregistered_calls = %+v, want %+v", rep.Unregistered, want)
	}
	for _, r := range rep.Services {
		if r.Service == "svc" {
			t.Errorf("the fleet report lists svc: %+v", r)
		}
	}
}
