package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCheckAdmissionMode_WarnsWhenOff (design §1): the escape hatch
// `admission = "off"` prints a loud startup line AND doctor WARNs on every
// run while the daemon serves with it; observe and enforce pass; a daemon
// predating the gauge (or unreachable) skips with a reason.
func TestCheckAdmissionMode_WarnsWhenOff(t *testing.T) {
	serve := func(body string) string {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		t.Cleanup(ts.Close)
		return ts.URL
	}
	c := checkAdmissionMode(&http.Client{}, serve("fathom_admission_mode{mode=\"off\"} 1\n"), true)
	if c.Status != doctorWarn || !strings.Contains(c.Reason, "off") || c.Remedy == "" {
		t.Fatalf("off: %+v, want a warn naming off with a remedy", c)
	}
	for _, mode := range []string{"observe", "enforce"} {
		c := checkAdmissionMode(&http.Client{}, serve(fmt.Sprintf("fathom_admission_mode{mode=%q} 1\n", mode)), true)
		if c.Status != doctorPass || !strings.Contains(c.Reason, mode) {
			t.Fatalf("%s: %+v, want pass", mode, c)
		}
	}
	if c := checkAdmissionMode(&http.Client{}, serve("fathom_uptime_seconds 1\n"), true); c.Status != doctorSkipped {
		t.Fatalf("no gauge: %+v, want skipped", c)
	}
	if c := checkAdmissionMode(&http.Client{}, "http://127.0.0.1:1", false); c.Status != doctorSkipped {
		t.Fatalf("daemon down: %+v, want skipped", c)
	}
}
