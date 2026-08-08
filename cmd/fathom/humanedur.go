package main

// humanedur.go — IQ-9: durations the way humans write them. "14d" and "2w"
// parse (days, weeks); every Go-duration form stays valid; negatives and
// garbage error. Days are 24h — this is an expiry bound, not a calendar.

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseHumaneDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var mult time.Duration
	switch {
	case strings.HasSuffix(s, "d"):
		mult = 24 * time.Hour
	case strings.HasSuffix(s, "w"):
		mult = 7 * 24 * time.Hour
	}
	if mult != 0 {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q (want e.g. 14d, 2w, 336h)", s)
		}
		if n <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %q", s)
		}
		return time.Duration(n) * mult, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (want e.g. 14d, 2w, 336h): %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %q", s)
	}
	return d, nil
}
