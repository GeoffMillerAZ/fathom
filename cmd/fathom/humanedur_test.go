package main

import (
	"testing"
	"time"
)

// TestParseHumaneDuration pins IQ-9: days and weeks parse the way humans
// write them; Go-duration forms stay valid; garbage errors.
func TestParseHumaneDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"14d", 14 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"2w", 2 * 7 * 24 * time.Hour, false},
		{"336h", 336 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"", 0, true},
		{"d", 0, true},
		{"14x", 0, true},
		{"-2d", 0, true},
	}
	for _, c := range cases {
		got, err := parseHumaneDuration(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseHumaneDuration(%q): want error, got %v", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("parseHumaneDuration(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}
