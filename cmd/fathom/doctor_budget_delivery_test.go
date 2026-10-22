package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDoctorIndependentChecksOverlap(t *testing.T) {
	ready := make(chan struct{})
	rows := runDoctorSteps([]doctorStep{
		{probe: func(ctx context.Context) doctorCheck {
			select {
			case <-ready:
				return doctorCheck{Name: "slow", Status: doctorPass}
			case <-ctx.Done():
				return doctorCheck{Name: "slow", Status: doctorTimeout, Reason: "blocked independent check", probeErr: ctx.Err()}
			}
		}},
		{probe: func(context.Context) doctorCheck { close(ready); return doctorCheck{Name: "fast", Status: doctorPass} }},
	})
	if len(rows) != 2 || rows[0].Name != "slow" || rows[0].Status != doctorPass || rows[1].Name != "fast" {
		t.Fatalf("independent checks serialized: %+v", rows)
	}
}

func TestDoctorGuardScopesBounded(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		doctorWriteFixture(t, root, fmt.Sprintf("assets/file%02d.txt", i), "fixture\n")
	}
	var cue strings.Builder
	cue.WriteString("guards: [\n")
	for i := 0; i < 140; i++ {
		fmt.Fprintf(&cue, "{scopes: [\"assets/**\"], note: \"guard %d\"},\n", i)
	}
	cue.WriteString("]\n")
	writeGuardCue(t, root, cue.String())
	hc := &http.Client{Transport: doctorDeliveryTransport{}}
	got := checkGuardDeadScopes(hc, "http://daemon.invalid", root, "fixture", true)
	if got.Status != doctorUnknown || !strings.Contains(got.Reason, "checked 128 of 140") {
		t.Fatalf("unbounded scopes = %+v", got)
	}
}

type doctorDeliveryTransport struct{ body string }

func (tr doctorDeliveryTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if tr.body != "" {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tr.body))}, nil
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"result":{"content":[{"type":"text","text":"{\"sources\":[\"src/main.go\"]}"}]}}`))}, nil
}

func TestDoctorDiskDiscoveryCapsWork(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		doctorWriteFixture(t, root, fmt.Sprintf("src/file%02d.go", i), "package fixture\n")
	}
	paths, complete, err := doctorDiskPaths(context.Background(), root, 8)
	if err != nil || complete || len(paths) > 6 {
		t.Fatalf("unbounded discovery: %d paths complete=%v err=%v", len(paths), complete, err)
	}
	for i := 1; i < len(paths); i++ {
		if paths[i-1] > paths[i] {
			t.Fatal("disk paths not sorted")
		}
	}
}

func TestDoctorGuardCannotCallElidedScopeDead(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["missing/**"]}]`)
	hc := &http.Client{Transport: doctorDeliveryTransport{body: `{"result":{"content":[{"type":"text","text":"{\"sources\":[\"other.go\"],\"elided\":{\"count\":1,\"of\":2}}"}]}}`}}
	got := checkGuardDeadScopes(hc, "http://daemon.invalid", root, "fixture", true)
	if got.Status != doctorUnknown || !strings.Contains(got.Reason, "checked 0 of 1") {
		t.Fatalf("elided evidence reported dead: %+v", got)
	}
}

type doctorTypedRequestTransport struct {
	t     *testing.T
	calls int
}

func (tr *doctorTypedRequestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.calls++
	var body struct {
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		tr.t.Fatal(err)
	}
	if body.Params.Name != "staleness_report" || body.Params.Arguments["section"] != "typed_edges" {
		tr.t.Errorf("doctor requested an unbounded projection: %+v", body.Params)
	}
	return (doctorDeliveryTransport{body: `{"result":{"content":[{"type":"text","text":"{\"typed_edges\":{\"known\":true,\"packages\":140}}"}]}}`}).RoundTrip(req)
}
func TestDoctorTypedEdgesRequestsBoundedProjection(t *testing.T) {
	transport := &doctorTypedRequestTransport{t: t}
	got := checkTypedEdges(&http.Client{Transport: transport}, "http://daemon.invalid", typedEdgesRepo(t), "fixture", true)
	if got.Status != doctorPass || transport.calls != 1 {
		t.Fatalf("typed projection: %+v requests=%d", got, transport.calls)
	}
}
