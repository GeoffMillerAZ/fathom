package iacfacts

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
)

// numberValue is the one rule for the Go value of a number every reader returns in Attr.Value, so the same text reads
// the same from HCL, CloudFormation YAML, JSON and Pulumi, and json.Marshal of a value never fails:
//
//   - an integer that fits int64 is an int64, and an integer is a number written without a fraction or an exponent:
//     7 is int64 7, and 1.0, 1e3 and -0.0 are float64 values by the next rule;
//   - a number float64 holds exactly is a float64 (1.5);
//   - any other number that is finite as float64 and valid as JSON keeps its exact text as a json.Number
//     (12345678901234567890, a long fraction, 1e-999);
//   - anything else is the source text as a string: a number past float64's range (1e999, 1e400), which would be Inf,
//     and text that is not a JSON number.
//
// text is the number as the source wrote it.
func numberValue(text string) any {
	if i, err := strconv.ParseInt(text, 0, 64); err == nil {
		return i
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return text
	}
	if sameDecimal(text, f) {
		return f
	}
	if jsonNumberRE.MatchString(text) {
		return json.Number(text)
	}
	return text
}

// jsonNumberRE is the JSON grammar for a number.
var jsonNumberRE = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// jsonNumberType returns "int" for JSON integer text, "float" for other JSON number text, and "" for text that is not
// a JSON number.
func jsonNumberType(text string) string {
	m := jsonNumberRE.FindStringSubmatch(text)
	switch {
	case m == nil:
		return ""
	case m[2] == "" && m[3] == "":
		return "int"
	}
	return "float"
}

// sameDecimal reports whether f is exactly the decimal text names, by comparing the digits and exponent of the text
// with those of f's shortest decimal form. A text with more digits than f's shortest form reads as different, which
// only costs it the exact json.Number form.
func sameDecimal(text string, f float64) bool {
	d1, e1, ok1 := decimalDigits(text)
	d2, e2, ok2 := decimalDigits(strconv.FormatFloat(f, 'e', -1, 64))
	return ok1 && ok2 && d1 == d2 && (d1 == "" || e1 == e2)
}

// decimalDigits returns the significant digits of a decimal number without leading or trailing zeros, and the
// exponent of ten that scales them to the number's value. Zero has no digits.
func decimalDigits(s string) (digits string, exp int, ok bool) {
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	var d []byte
	frac, dot := 0, false
loop:
	for ; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			d = append(d, c)
			if dot {
				frac++
			}
		case c == '.' && !dot:
			dot = true
		default:
			break loop
		}
	}
	e := 0
	if i < len(s) {
		if s[i] != 'e' && s[i] != 'E' {
			return "", 0, false
		}
		n, err := strconv.Atoi(s[i+1:])
		if err != nil {
			return "", 0, false
		}
		e = n
	}
	start := 0
	for start < len(d) && d[start] == '0' {
		start++
	}
	end := len(d)
	for end > start && d[end-1] == '0' {
		end--
	}
	return string(d[start:end]), e - frac + (len(d) - end), true
}
