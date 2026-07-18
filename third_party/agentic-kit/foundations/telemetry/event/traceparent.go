// traceparent.go — W3C trace-context codec for cross-process trace carry.
//
// A TraceContext crosses a process boundary as a W3C `traceparent` value:
//
//	00-<trace-id>-<parent-id>-<flags>
//
// mapped onto the kit's fields as RunID → trace-id (32 lowercase hex),
// SpanID → parent-id (16 lowercase hex), flags always emitted as 01
// (sampled). ParentSpanID DOES NOT cross the wire: the receiver derives its
// own span from the parsed context via Child, which sets ParentSpanID to
// the sender's SpanID — so the parent/child link survives without a third
// wire field.
//
// Two carriers are supported, both string-based on purpose so this package
// never imports net/http:
//
//   - HTTP header: callers do header.Set(event.TraceparentHeader, s) /
//     ParseTraceparent(header.Get(event.TraceparentHeader)).
//   - Subprocess environment: AppendTraceparent composes a TRACEPARENT
//     entry onto an env slice (for example boundedexec.Config.Env);
//     TraceFromEnviron recovers it from os.Environ()-shaped input.
//
// Fail-closed: FormatTraceparent never emits all-zero or malformed IDs
// (returns ok=false instead), and ParseTraceparent returns a zero
// TraceContext on ANY error — a malformed inbound header degrades to "no
// trace", never to a bogus one.
//
// Concurrency: all functions here are pure and safe for concurrent use.

package event

import (
	"errors"
	"fmt"
	"strings"
)

// TraceparentHeader is the W3C trace-context HTTP header name (lowercase
// per the W3C registration; HTTP header names are case-insensitive).
const TraceparentHeader = "traceparent"

// TraceparentEnv is the environment variable carrying a traceparent value
// across a subprocess boundary. The name is matched exactly (environment
// variable names are case-sensitive on POSIX systems).
const TraceparentEnv = "TRACEPARENT"

// Width and sentinel constants for the version-00 traceparent layout:
// "vv-tttttttttttttttttttttttttttttttt-pppppppppppppppp-ff" (55 chars).
const (
	traceparentVersion = "00"
	traceparentFlags   = "01" // sampled
	traceIDHexLen      = 2 * runIDBytes
	parentIDHexLen     = 2 * spanIDBytes
	traceparentLen     = 2 + 1 + traceIDHexLen + 1 + parentIDHexLen + 1 + 2
)

// FormatTraceparent renders tc as a version-00 W3C traceparent value
// ("00-<RunID>-<SpanID>-01"). ok is false — and the string empty — when tc
// cannot be represented on the wire: a zero tc, a RunID that is not exactly
// 32 lowercase hex characters, a SpanID that is not exactly 16 lowercase
// hex characters, or an all-zero RunID/SpanID (the W3C format reserves
// all-zero IDs as invalid, so they are never emitted). ParentSpanID is
// intentionally not encoded; see the file comment.
func FormatTraceparent(tc TraceContext) (string, bool) {
	if !validWireID(tc.RunID, traceIDHexLen) || !validWireID(tc.SpanID, parentIDHexLen) {
		return "", false
	}
	return traceparentVersion + "-" + tc.RunID + "-" + tc.SpanID + "-" + traceparentFlags, true
}

// ParseTraceparent parses a W3C traceparent value into a TraceContext with
// RunID = trace-id and SpanID = parent-id (ParentSpanID is always empty;
// receivers derive child spans via Child). Version handling follows W3C
// trace-context §versioning: version 00 must be exactly the 55-character
// layout; an unknown future version (any two lowercase hex chars other
// than 00 and ff) is parsed by the version-00 layout and may carry extra
// data only after a '-' separator; version ff is invalid. All-zero
// trace-id or parent-id, wrong field widths, uppercase hex, and malformed
// separators are all rejected. On any error the returned TraceContext is
// the zero value (fail closed).
func ParseTraceparent(s string) (TraceContext, error) {
	if len(s) < traceparentLen {
		return TraceContext{}, fmt.Errorf("traceparent: too short (%d chars, need at least %d)", len(s), traceparentLen)
	}
	version := s[0:2]
	if !isLowerHexStr(version) {
		return TraceContext{}, fmt.Errorf("traceparent: malformed version %q", version)
	}
	switch {
	case version == "ff":
		return TraceContext{}, errors.New("traceparent: version ff is invalid")
	case version == traceparentVersion && len(s) != traceparentLen:
		return TraceContext{}, fmt.Errorf("traceparent: version 00 must be exactly %d chars, got %d", traceparentLen, len(s))
	case version != traceparentVersion && len(s) > traceparentLen && s[traceparentLen] != '-':
		return TraceContext{}, errors.New("traceparent: future-version extra data must follow a '-' separator")
	}
	if s[2] != '-' || s[3+traceIDHexLen] != '-' || s[4+traceIDHexLen+parentIDHexLen] != '-' {
		return TraceContext{}, errors.New("traceparent: malformed field separators")
	}
	traceID := s[3 : 3+traceIDHexLen]
	parentID := s[4+traceIDHexLen : 4+traceIDHexLen+parentIDHexLen]
	flags := s[5+traceIDHexLen+parentIDHexLen : traceparentLen]
	if !validWireID(traceID, traceIDHexLen) {
		return TraceContext{}, fmt.Errorf("traceparent: invalid trace-id %q", traceID)
	}
	if !validWireID(parentID, parentIDHexLen) {
		return TraceContext{}, fmt.Errorf("traceparent: invalid parent-id %q", parentID)
	}
	if !isLowerHexStr(flags) {
		return TraceContext{}, fmt.Errorf("traceparent: malformed flags %q", flags)
	}
	return TraceContext{RunID: traceID, SpanID: parentID}, nil
}

// AppendTraceparent returns env with a TRACEPARENT entry for tc appended.
// When tc cannot be formatted (zero or malformed per FormatTraceparent),
// env is returned UNCHANGED — the never-emit-all-zero-ids rule applied to
// the env carrier. Appending (rather than replacing) is sufficient to
// override an existing entry because both os/exec ("only the last value in
// the slice for each duplicate key is used") and TraceFromEnviron resolve
// duplicates last-wins. env itself is never mutated in place beyond the
// usual append aliasing; callers following the boundedexec.Config.Env
// scrubbed-allowlist discipline compose this as
// AppendTraceparent(cfg.Env, tc).
func AppendTraceparent(env []string, tc TraceContext) []string {
	s, ok := FormatTraceparent(tc)
	if !ok {
		return env
	}
	return append(env, TraceparentEnv+"="+s)
}

// TraceFromEnviron recovers a TraceContext from an os.Environ()-shaped
// slice of "KEY=value" entries. When TRACEPARENT appears more than once
// the LAST entry wins (matching os/exec duplicate-key semantics), and only
// that entry is parsed — an earlier valid value never rescues a later
// malformed one (fail closed). ok is false when no TRACEPARENT entry
// exists or the winning value fails ParseTraceparent; the returned
// TraceContext is then the zero value.
func TraceFromEnviron(environ []string) (TraceContext, bool) {
	value, found := "", false
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, TraceparentEnv+"="); ok {
			value, found = v, true // last wins
		}
	}
	if !found {
		return TraceContext{}, false
	}
	tc, err := ParseTraceparent(value)
	if err != nil {
		return TraceContext{}, false
	}
	return tc, true
}

// validWireID reports whether id is exactly wantLen lowercase hex
// characters and not all zeros (all-zero IDs are invalid on the wire).
func validWireID(id string, wantLen int) bool {
	if len(id) != wantLen || !isLowerHexStr(id) {
		return false
	}
	return strings.ContainsFunc(id, func(r rune) bool { return r != '0' })
}

// isLowerHexStr reports whether s consists solely of [0-9a-f].
func isLowerHexStr(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
