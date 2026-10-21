package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewestTestCaseRunsNativePayloadSelection(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "newest-native.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	const service = "svc"
	const sameDigest = "shared-digest"
	const editJSON = `{"suite":"EditMode"}`
	const playJSON = `{"suite":"PlayMode"}`
	key := func(name string) string { return service + "::pkg." + name }
	if err := st.PutTestRuns(ctx, service, []TestRun{
		{RunDigest: "legacy-old", Suite: "unit", Component: "legacy", StartedAt: "2026-10-01T00:00:00Z", Cases: []TestCase{
			{CaseID: "a", Status: "fail", Attempts: 1, TestSymbolKey: key("A")},
			{CaseID: "b", Status: "pass", Attempts: 1, TestSymbolKey: key("B")},
		}},
		{RunDigest: sameDigest, Suite: "EditMode", Component: "edit", StartedAt: "2026-10-02T00:00:00Z", NativeRunID: "edit-native", NativeJSON: editJSON, Commit: strings.Repeat("a", 40), Dirty: true, Cases: []TestCase{
			{CaseID: "a", Status: "pass", Attempts: 2, DurationMS: 31, TestSymbolKey: key("A")},
			{CaseID: "c", Status: "fail", Attempts: 3, DurationMS: 32, TestSymbolKey: key("C")},
			{CaseID: "loose", Status: "pass"},
		}},
		{RunDigest: sameDigest, Suite: "PlayMode", Component: "play", StartedAt: "2026-10-03T00:00:00Z", NativeRunID: "play-native", NativeJSON: playJSON, Race: true, Cases: []TestCase{
			{CaseID: "d", Status: "pass", Attempts: 1, TestSymbolKey: key("D")},
			{CaseID: "e", Status: "fail", Attempts: 2, TestSymbolKey: key("E")},
		}},
		{RunDigest: "tie-old", Suite: "unit", StartedAt: "2026-10-04T00:00:00Z", Cases: []TestCase{{CaseID: "f", Status: "fail", TestSymbolKey: key("F")}}},
	}); err != nil {
		t.Fatal(err)
	}
	// The same started_at must select the later ingestion for F.
	if err := st.PutTestRuns(ctx, service, []TestRun{{RunDigest: "tie-new", Suite: "unit", StartedAt: "2026-10-04T00:00:00Z", Cases: []TestCase{{CaseID: "f", Status: "pass", TestSymbolKey: key("F")}}}}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutTestRuns(ctx, "other", []TestRun{{RunDigest: "other", Suite: "unit", StartedAt: "2026-10-05T00:00:00Z", Cases: []TestCase{{CaseID: "a", Status: "fail", TestSymbolKey: key("A")}}}}); err != nil {
		t.Fatal(err)
	}
	got, err := st.NewestTestCaseRuns(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ name, digest, suite, status, nativeJSON string }{
		{"A", sameDigest, "EditMode", "pass", editJSON},
		{"B", "legacy-old", "unit", "pass", ""},
		{"C", sameDigest, "EditMode", "fail", editJSON},
		{"D", sameDigest, "PlayMode", "pass", playJSON},
		{"E", sameDigest, "PlayMode", "fail", playJSON},
		{"F", "tie-new", "unit", "pass", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		r := got[i]
		if r.TestSymbolKey != key(w.name) || r.RunDigest != w.digest || r.Suite != w.suite || r.Status != w.status || r.NativeJSON != w.nativeJSON {
			t.Errorf("row %d = %+v, want key %q, digest %q, suite %q, status %q, native JSON %q", i, r, key(w.name), w.digest, w.suite, w.status, w.nativeJSON)
		}
	}
	if got[0].Component != "edit" || got[0].Attempts != 2 || got[0].DurationMS != 31 || got[0].Commit != strings.Repeat("a", 40) || !got[0].Dirty || got[0].Race {
		t.Errorf("EditMode metadata changed: %+v", got[0])
	}
	if got[3].Component != "play" || !got[3].Race || got[3].Dirty {
		t.Errorf("PlayMode metadata changed: %+v", got[3])
	}
	if got[5].IngestedAt <= got[1].IngestedAt {
		t.Errorf("ingestion ordering lost: F=%d B=%d", got[5].IngestedAt, got[1].IngestedAt)
	}
}

var newestTestCaseRunsBenchmarkSink []TestCaseRun

func BenchmarkNewestTestCaseRunsNativePayload(b *testing.B) {
	st, err := Open(filepath.Join(b.TempDir(), "native-benchmark.sqlite"))
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	cases := make([]TestCase, 384)
	for i := range cases {
		cases[i] = TestCase{CaseID: fmt.Sprintf("%03d", i), Status: "pass", TestSymbolKey: fmt.Sprintf("svc::pkg.Test%03d", i)}
	}
	payload := `{"receipt":"` + strings.Repeat("x", 128<<10) + `"}`
	if err := st.PutTestRuns(ctx, "svc", []TestRun{{RunDigest: "native", Suite: "EditMode", NativeRunID: "native", NativeJSON: payload, Cases: cases}}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newestTestCaseRunsBenchmarkSink, err = st.NewestTestCaseRuns(ctx, "svc")
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if len(newestTestCaseRunsBenchmarkSink) != len(cases) || newestTestCaseRunsBenchmarkSink[0].NativeJSON != payload {
		b.Fatalf("newest native payload was not retained for all %d rows", len(cases))
	}
}
