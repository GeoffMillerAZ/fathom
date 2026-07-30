package main

import (
	"strings"
	"testing"
)

func TestCheckMetricsTier_Unset_Basic(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "")
	c := checkMetricsTier()
	if c.Status != doctorPass {
		t.Fatalf("status = %q, want pass", c.Status)
	}
	if !strings.Contains(c.Reason, "basic") {
		t.Errorf("reason = %q, want it to name the basic tier", c.Reason)
	}
}

func TestCheckMetricsTier_Full(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "full")
	c := checkMetricsTier()
	if c.Status != doctorPass {
		t.Fatalf("status = %q, want pass", c.Status)
	}
	if !strings.Contains(c.Reason, "full") {
		t.Errorf("reason = %q, want it to name the full tier", c.Reason)
	}
}

func TestCheckMetricsTier_Off(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "off")
	c := checkMetricsTier()
	if c.Status != doctorPass {
		t.Fatalf("status = %q, want pass", c.Status)
	}
	if !strings.Contains(c.Reason, "off") {
		t.Errorf("reason = %q, want it to name the off tier", c.Reason)
	}
}

func TestCheckMetricsTier_InvalidValue_NotesFallback(t *testing.T) {
	t.Setenv("FATHOM_METRICS", "extreme")
	c := checkMetricsTier()
	if c.Status != doctorPass {
		t.Fatalf("status = %q, want pass (an invalid value is still a pass, per checkChannelsEnv's own discipline)", c.Status)
	}
	if !strings.Contains(c.Reason, "extreme") {
		t.Errorf("reason = %q, want it to name the invalid value", c.Reason)
	}
}
