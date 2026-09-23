package config

import "testing"

// TestParseMemoryLimit_Valid covers Task 2's "config parse/validate
// (valid, off, garbage refused)" — the valid half, plus off.
func TestParseMemoryLimit_Valid(t *testing.T) {
	cases := []struct {
		in        string
		wantBytes int64
		wantOff   bool
	}{
		{"3GiB", 3 << 30, false},
		{"512MiB", 512 << 20, false},
		{"1.5GiB", int64(1.5 * float64(int64(1)<<30)), false},
		{"1024B", 1024, false},
		{"2TiB", 2 << 40, false},
		{"off", 0, true},
		{"OFF", 0, true},
		{"Off", 0, true},
		{"  3GiB  ", 3 << 30, false}, // surrounding whitespace tolerated
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			bytes, off, err := ParseMemoryLimit(tc.in)
			if err != nil {
				t.Fatalf("ParseMemoryLimit(%q): unexpected error: %v", tc.in, err)
			}
			if off != tc.wantOff {
				t.Errorf("ParseMemoryLimit(%q) off = %v, want %v", tc.in, off, tc.wantOff)
			}
			if !off && bytes != tc.wantBytes {
				t.Errorf("ParseMemoryLimit(%q) bytes = %d, want %d", tc.in, bytes, tc.wantBytes)
			}
		})
	}
}

// TestParseMemoryLimit_Refused covers the "garbage refused" half: no
// silent fallback, no partial parse — every one of these must return a
// non-nil error.
func TestParseMemoryLimit_Refused(t *testing.T) {
	cases := []string{
		"",
		"3 gigs",
		"3gb",      // decimal SI unit, not the accepted IEC suffix
		"3 GiB",    // internal space
		"GiB",      // no numeric value
		"-3GiB",    // negative
		"0GiB",     // zero — would pathologically GC on every allocation
		"3XiB",     // unknown unit
		"3.5.5GiB", // malformed number
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, _, err := ParseMemoryLimit(in); err == nil {
				t.Errorf("ParseMemoryLimit(%q): want error, got none", in)
			}
		})
	}
}
