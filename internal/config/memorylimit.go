package config

// memorylimit.go — W-N (docs/plans/daemon-footprint.md): parses
// [daemon].memory_limit / FATHOM_MEMORY_LIMIT's Go-size-syntax string
// ("3GiB", "512MiB", "1.5GiB") or the literal "off" into the bytes
// serve-http passes to runtime/debug.SetMemoryLimit. schema.cue's
// memory_limit pattern bounds the SHAPE (so a TOML typo is caught by `cue
// vet`/config load, not left to the daemon's own startup error path); this
// parser is the second, stricter gate every value — however it arrived —
// must also pass before it is ever applied.
//
// IEC units only (B/KiB/MiB/GiB/TiB), never the decimal SI ones (KB/MB/…):
// GOMEMLIMIT (the runtime's own env-var equivalent, since Go 1.19) uses the
// same IEC suffixes, so a value copied between FATHOM_MEMORY_LIMIT and
// GOMEMLIMIT means the same number of bytes either way — no silent
// KB-vs-KiB drift between the two knobs an operator might reach for.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// memoryLimitPattern mirrors schema.cue #Daemon.memory_limit's shape
// (kept here rather than derived from it — the CUE pattern only needs to
// bound TOML input syntactically; this one also has to be a
// FindStringSubmatch target).
var memoryLimitPattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)(B|KiB|MiB|GiB|TiB)$`)

// memoryLimitUnits maps each accepted suffix to its byte multiplier.
var memoryLimitUnits = map[string]int64{
	"B":   1,
	"KiB": 1 << 10,
	"MiB": 1 << 20,
	"GiB": 1 << 30,
	"TiB": 1 << 40,
}

// ParseMemoryLimit parses s (Go size syntax, e.g. "3GiB", or the literal
// "off") into bytes. off is true exactly when s is "off" (case-insensitive)
// — bytes is meaningless in that case (always 0) and the caller must not
// apply a limit at all, rather than applying a limit of 0 bytes (which
// would pathologically GC on every allocation). An empty string is
// refused — "unset, use the default" is a decision callers make
// themselves before calling this, not something this parser guesses at.
func ParseMemoryLimit(s string) (bytes int64, off bool, err error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, false, fmt.Errorf("config: memory_limit: empty (want Go size syntax, e.g. %q, or %q)", "3GiB", "off")
	}
	if strings.EqualFold(trimmed, "off") {
		return 0, true, nil
	}
	m := memoryLimitPattern.FindStringSubmatch(trimmed)
	if m == nil {
		return 0, false, fmt.Errorf("config: memory_limit %q: want Go size syntax (e.g. %q) or %q", s, "3GiB", "off")
	}
	value, convErr := strconv.ParseFloat(m[1], 64)
	if convErr != nil {
		// Unreachable given the pattern's own numeric group, but never
		// trust a regex match alone over a real parse failure.
		return 0, false, fmt.Errorf("config: memory_limit %q: %w", s, convErr)
	}
	mult := memoryLimitUnits[m[2]]
	bytesF := value * float64(mult)
	if bytesF <= 0 {
		return 0, false, fmt.Errorf("config: memory_limit %q: must be greater than zero (use %q to disable the limit entirely)", s, "off")
	}
	return int64(bytesF), false, nil
}
