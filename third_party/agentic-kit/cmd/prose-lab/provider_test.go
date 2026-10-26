package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUsagePresenceAndPricing(t *testing.T) {
	l := testLane(100)
	for _, tc := range []struct {
		raw     string
		wantErr bool
		cost    int64
	}{
		{`{"model":"model","usage":{"input_tokens":15,"output_tokens":0}}`, false, 15},
		{`{"model":"model","usage":{"input_tokens":0,"output_tokens":0}}`, false, 0},
		{`{"model":"model"}`, true, 0},
		{`{"model":"foreign-model","usage":{"input_tokens":1,"output_tokens":0}}`, true, 0},
		{`{"model":"model","usage":{}}`, true, 0},
		{`{"model":"model","usage":{"input_tokens":null,"output_tokens":0}}`, true, 0},
		{`{"model":"model","usage":{"input_tokens":-1,"output_tokens":0}}`, true, 0},
		{`{"model":"model","usage":{"input_tokens":101,"output_tokens":0}}`, true, 0},
		{`{"model":"model","usage":{"input_tokens":1,"input_tokens":2,"output_tokens":0}}`, true, 0},
		{`{"model":"model","usage":{"input_tokens":1,"output_tokens":0,"cache_read_input_tokens":1}}`, true, 0},
	} {
		_, _, cost, err := readUsage([]byte(tc.raw), l)
		if (err != nil) != tc.wantErr || (!tc.wantErr && cost != tc.cost) {
			t.Errorf("usage %s: cost=%d err=%v", tc.raw, cost, err)
		}
	}
	l.InputRate = 0
	if l.validate() == nil {
		t.Fatal("unpriced input accepted")
	}
}

func TestReservationCoversIndependentlyRoundedInputBuckets(t *testing.T) {
	l := testLane(10000)
	l.InputRate, l.ReserveInputRate = 1, 1
	l.CacheWriteRate, l.CacheReadRate = 1, 1
	if err := l.validate(); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"model":"model","usage":{"input_tokens":1,"output_tokens":0,"cache_creation_input_tokens":1,"cache_read_input_tokens":1}}`)
	_, _, cost, err := readUsage(raw, l)
	if err != nil || cost != 3 || l.reserveCost() != 3 {
		t.Fatalf("three rounded buckets must fit reserved cost: cost=%d reserve=%d err=%v", cost, l.reserveCost(), err)
	}
	tooSmall, _ := testJournal(t, 2)
	if _, err := tooSmall.reserve("split-usage", l, []byte("request")); err == nil {
		t.Fatal("insufficient budget must refuse before a dispatch reservation exists")
	}
	exact, _ := testJournal(t, 3)
	r, err := exact.reserve("split-usage", l, []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	if err := exact.finish(r, outcome{Status: "completed", Model: "model", UsagePresent: true, ActualMicroUSD: &cost, Response: string(raw)}); err != nil || exact.charged != 3 {
		t.Fatalf("settlement at exact reserved boundary: charged=%d err=%v", exact.charged, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCaptureTransportRefusesExtraAttemptAndOrigin(t *testing.T) {
	calls := 0
	tr := &captureTransport{origin: "https://fixture.invalid", path: "/v1/messages", inner: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	req, _ := http.NewRequest("POST", "https://fixture.invalid/v1/messages", strings.NewReader("{}"))
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatal("unreserved attempt admitted")
	}
	if err := tr.begin(); err != nil {
		t.Fatal(err)
	}
	tr.bindWire = func([]byte) error { return nil }
	foreign, _ := http.NewRequest("POST", "https://foreign.invalid/v1/messages", nil)
	if _, err := tr.RoundTrip(foreign); err == nil {
		t.Fatal("foreign origin admitted")
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatal("hidden retry admitted")
	}
	tr.end()
	if calls != 1 {
		t.Fatal("wrong wire attempt count")
	}
}

func TestAttemptUnknownUsageStopsAndKeepsReservation(t *testing.T) {
	j, _ := testJournal(t, 20000)
	tr := &captureTransport{origin: "https://fixture.invalid", path: "/v1/messages", inner: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"model"}`))}, nil
	})}
	a := &attemptRunner{journal: j, lane: testLane(8192), capture: tr, now: time.Now, caseID: "missing-usage"}
	err := a.call(context.Background(), map[string]string{"input": "synthetic"}, func() error {
		req, _ := http.NewRequest("POST", "https://fixture.invalid/v1/messages", strings.NewReader(`{"model":"model"}`))
		resp, e := tr.RoundTrip(req)
		if e == nil {
			resp.Body.Close()
		}
		return e
	})
	if err == nil || j.charged != 8192 || !j.closed {
		t.Fatal("missing usage released budget or permitted more calls")
	}
}

func TestAttemptRecordsSuccessfulUsageAndNeverErrorBody(t *testing.T) {
	for _, good := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "error"}[good], func(t *testing.T) {
			j, path := testJournal(t, 20000)
			body := `{"model":"model","usage":{"input_tokens":15,"output_tokens":0}}`
			status := 200
			if !good {
				body = "sensitive error body"
				status = 401
			}
			tr := &captureTransport{origin: "https://fixture.invalid", path: "/v1/messages", inner: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			a := &attemptRunner{journal: j, lane: testLane(8192), capture: tr, now: time.Now, caseID: "capture"}
			err := a.call(context.Background(), "synthetic", func() error {
				req, _ := http.NewRequest("POST", "https://fixture.invalid/v1/messages", strings.NewReader(`{"model":"model"}`))
				resp, e := tr.RoundTrip(req)
				if e != nil {
					return e
				}
				defer resp.Body.Close()
				if !good {
					return errors.New("provider error")
				}
				return nil
			})
			if (err == nil) != good {
				t.Fatal("wrong capture outcome")
			}
			want := int64(8192)
			if good {
				want = 15
			}
			if j.charged != want {
				t.Fatal("wrong accounted amount")
			}
			data, e := readLabBytes(path+"/outcome-000001.json", 4<<20)
			if e != nil {
				t.Fatal(e)
			}
			if !good && strings.Contains(string(data), body) {
				t.Fatal("error body persisted")
			}
		})
	}
}

func TestWireBoundaryBindsActualRequestAndRefusesUnderReservation(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		ceiling    int64
		want       bool
	}{
		{"selected", `{"model":"model","state":"actual wire"}`, 8192, true},
		{"wrong-model", `{"model":"other"}`, 8192, false},
		{"tiny-reservation", `{"model":"model","state":"larger than one token"}`, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j, path := testJournal(t, 20000)
			calls := 0
			tr := &captureTransport{origin: "https://fixture.invalid", path: "/v1/messages", inner: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"model","usage":{"input_tokens":15,"output_tokens":0}}`))}, nil
			})}
			a := &attemptRunner{journal: j, lane: testLane(tc.ceiling), capture: tr, now: time.Now, caseID: tc.name}
			err := a.call(context.Background(), map[string]string{"logical": "different encoding"}, func() error {
				req, _ := http.NewRequest("POST", "https://fixture.invalid/v1/messages", strings.NewReader(tc.wire))
				resp, e := tr.RoundTrip(req)
				if e == nil {
					resp.Body.Close()
				}
				return e
			})
			if (err == nil) != tc.want || (calls == 1) != tc.want {
				t.Fatalf("err=%v wire calls=%d", err, calls)
			}
			if tc.want {
				b, e := readLabBytes(path+"/reserve-000001.json", 1<<20)
				if e != nil {
					t.Fatal(e)
				}
				var r reservation
				if decodeLabJSON(b, &r) != nil || r.RequestHash != hashBytes([]byte(tc.wire)) || r.RequestJSON != tc.wire {
					t.Fatal("reservation did not bind actual wire bytes")
				}
			} else if j.charged != 0 {
				t.Fatal("pre-dispatch refusal consumed reservation")
			}
		})
	}
}

func TestCaptureWithholdsCredentialEcho(t *testing.T) {
	j, _ := testJournal(t, 20000)
	tr := &captureTransport{origin: "https://fixture.invalid", path: "/v1/messages", inner: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"echo":"private-fixture-token"}`))}, nil
	})}
	a := &attemptRunner{journal: j, lane: testLane(8192), capture: tr, now: time.Now, caseID: "echo"}
	err := a.call(context.Background(), "synthetic", func() error {
		req, _ := http.NewRequest("POST", "https://fixture.invalid/v1/messages", strings.NewReader(`{"model":"model"}`))
		req.Header.Set("Authorization", "Bearer private-fixture-token")
		_, e := tr.RoundTrip(req)
		return e
	})
	if err == nil || len(tr.body) != 0 || j.charged != 8192 {
		t.Fatal("echo not withheld with retained uncertain spend")
	}
}

func TestExactResolvedModelAndTierPricing(t *testing.T) {
	l := testLane(100)
	for _, alias := range []string{"^.*$", "model-*", "model|other"} {
		l.ResolvedModel = alias
		if l.validate() == nil {
			t.Fatal("broad alias admitted")
		}
	}
	l.ResolvedModel = "model-build"
	if l.validate() != nil || !l.acceptsModel("model-build") || l.acceptsModel("other") {
		t.Fatal("exact alias binding failed")
	}
	if _, _, _, err := readUsage([]byte(`{"model":"model-build","usage":{"input_tokens":10,"output_tokens":0}}`), l); err != nil {
		t.Fatal(err)
	}
	l.ResolvedModel = ""
	l.TierThreshold = 10
	l.InputRate = 100000
	l.HighInputRate = 200000
	l.ReserveInputRate = 200000
	for _, tc := range []struct{ tokens, cost int64 }{{10, 1}, {11, 3}} {
		raw := []byte(fmt.Sprintf(`{"model":"model","usage":{"input_tokens":%d,"output_tokens":0}}`, tc.tokens))
		_, _, cost, err := readUsage(raw, l)
		if err != nil || cost != tc.cost {
			t.Fatalf("tier cost=%d err=%v", cost, err)
		}
	}
}
