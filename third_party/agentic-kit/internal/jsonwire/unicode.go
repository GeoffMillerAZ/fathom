// Package jsonwire supplies private wire checks shared by strict kit decoders.
// It does not replace JSON syntax, shape, duplicate-key or resource-bound checks.
// Functions retain no state and are safe for concurrent use.
package jsonwire

import "unicode/utf8"

// ValidUnicode reports whether raw contains valid UTF-8 and its JSON strings use
// only Unicode scalar values: each escaped high surrogate must immediately pair
// with an escaped low surrogate, and low surrogates cannot stand alone. Check this
// before encoding/json, which otherwise replaces malformed strings with U+FFFD.
// Literal U+FFFD and escaped backslashes remain valid. Callers must separately
// validate JSON syntax/shape and bound input bytes. Work is linear, with no allocation.
func ValidUnicode(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '"':
			inString = !inString
		case '\\':
			if !inString {
				continue
			}
			i++
			if i >= len(raw) {
				return false
			}
			if raw[i] != 'u' {
				continue // Skip escaped quotes/backslashes; syntax is checked separately.
			}
			unit, ok := hexUnit(raw[i+1:])
			if !ok {
				return false
			}
			i += 4
			if unit >= 0xdc00 && unit <= 0xdfff {
				return false
			}
			if unit < 0xd800 || unit > 0xdbff {
				continue
			}
			if len(raw)-i <= 6 || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, ok := hexUnit(raw[i+3:])
			if !ok || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return !inString
}

func hexUnit(raw []byte) (uint16, bool) {
	if len(raw) < 4 {
		return 0, false
	}
	var unit uint16
	for _, b := range raw[:4] {
		unit <<= 4
		switch {
		case b >= '0' && b <= '9':
			unit |= uint16(b - '0')
		case b >= 'a' && b <= 'f':
			unit |= uint16(b-'a') + 10
		case b >= 'A' && b <= 'F':
			unit |= uint16(b-'A') + 10
		default:
			return 0, false
		}
	}
	return unit, true
}
